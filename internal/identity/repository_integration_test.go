//go:build integration

package identity

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func identityIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" { t.Fatal("TEST_DATABASE_URL is required for integration tests") }
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil { t.Fatal(err) }
	if err := pool.Ping(context.Background()); err != nil { pool.Close(); t.Fatal(err) }
	t.Cleanup(pool.Close)
	_, err = pool.Exec(context.Background(), `
TRUNCATE TABLE consumer_security_events,consumer_auth_tokens,consumer_sessions,
webmaster_metrics_daily,webmaster_url_requests,webmaster_sitemaps,webmaster_verifications,
webmaster_sites,webmaster_sessions,webmaster_users,consumer_users
RESTART IDENTITY CASCADE`)
	if err != nil { t.Fatalf("reset identity fixture: %v", err) }
	return pool
}

func TestConsumerSessionIsolationAndRevocation(t *testing.T) {
	pool := identityIntegrationDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	hash, err := HashPassword("correct horse battery staple")
	if err != nil { t.Fatal(err) }
	first, err := repo.CreateUser(ctx, "first@example.com", hash)
	if err != nil { t.Fatal(err) }
	second, err := repo.CreateUser(ctx, "second@example.com", hash)
	if err != nil { t.Fatal(err) }

	_, firstTokenHash, err := RandomToken(32)
	if err != nil { t.Fatal(err) }
	_, firstCSRFHash, err := RandomToken(32)
	if err != nil { t.Fatal(err) }
	firstSession, err := repo.CreateSession(ctx, first.ID, firstTokenHash, firstCSRFHash, time.Now().UTC().Add(time.Hour))
	if err != nil { t.Fatal(err) }

	if err := repo.RevokeSession(ctx, second.ID, firstSession.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user revoke err=%v", err)
	}
	var revoked bool
	if err := pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM consumer_sessions WHERE session_id=$1`, firstSession.ID).Scan(&revoked); err != nil { t.Fatal(err) }
	if revoked { t.Fatal("cross-user revoke changed session") }
	if err := repo.RevokeSession(ctx, first.ID, firstSession.ID); err != nil { t.Fatal(err) }
	if err := pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM consumer_sessions WHERE session_id=$1`, firstSession.ID).Scan(&revoked); err != nil { t.Fatal(err) }
	if !revoked { t.Fatal("owner revoke did not revoke session") }
}

func TestConsumerActiveSessionsAreCapped(t *testing.T) {
	pool := identityIntegrationDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	hash, err := HashPassword("correct horse battery staple")
	if err != nil { t.Fatal(err) }
	u, err := repo.CreateUser(ctx, "owner@example.com", hash)
	if err != nil { t.Fatal(err) }
	for i := 0; i < 12; i++ {
		_, tokenHash, err := RandomToken(32)
		if err != nil { t.Fatal(err) }
		_, csrfHash, err := RandomToken(32)
		if err != nil { t.Fatal(err) }
		if _, err := repo.CreateSession(ctx, u.ID, tokenHash, csrfHash, time.Now().UTC().Add(24*time.Hour)); err != nil { t.Fatal(err) }
		time.Sleep(time.Millisecond)
	}
	var active int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM consumer_sessions WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>now()`, u.ID).Scan(&active); err != nil { t.Fatal(err) }
	if active != 8 { t.Fatalf("active sessions=%d, want 8", active) }
}

func TestEnsureWebmasterProfileAndPasswordSync(t *testing.T) {
	pool := identityIntegrationDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	oldHash, err := HashPassword("correct horse battery staple")
	if err != nil { t.Fatal(err) }
	u, err := repo.CreateUser(ctx, "owner@example.com", oldHash)
	if err != nil { t.Fatal(err) }
	webmasterID, err := repo.EnsureWebmasterProfile(ctx, u.ID)
	if err != nil { t.Fatal(err) }
	var consumerID int64
	var webmasterHash string
	if err := pool.QueryRow(ctx, `SELECT consumer_user_id,password_hash FROM webmaster_users WHERE user_id=$1`, webmasterID).Scan(&consumerID, &webmasterHash); err != nil { t.Fatal(err) }
	if consumerID != u.ID || webmasterHash != oldHash { t.Fatalf("consumer_id=%d hash_synced=%v", consumerID, webmasterHash == oldHash) }

	newHash, err := HashPassword("another correct horse battery staple")
	if err != nil { t.Fatal(err) }
	if err := repo.UpdatePasswordHash(ctx, u.ID, newHash); err != nil { t.Fatal(err) }
	var consumerHash string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM consumer_users WHERE user_id=$1`, u.ID).Scan(&consumerHash); err != nil { t.Fatal(err) }
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM webmaster_users WHERE consumer_user_id=$1`, u.ID).Scan(&webmasterHash); err != nil { t.Fatal(err) }
	if consumerHash != newHash || webmasterHash != newHash { t.Fatal("consumer/webmaster password hashes diverged") }
}

func TestLegacyWebmasterInsertLinksExistingConsumer(t *testing.T) {
	pool := identityIntegrationDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	hash, err := HashPassword("correct horse battery staple")
	if err != nil { t.Fatal(err) }
	u, err := repo.CreateUser(ctx, "legacy@example.com", hash)
	if err != nil { t.Fatal(err) }
	var webmasterID, consumerID int64
	if err := pool.QueryRow(ctx, `INSERT INTO webmaster_users(email,password_hash,status) VALUES($1,$2,'ACTIVE') RETURNING user_id,consumer_user_id`, u.Email, hash).Scan(&webmasterID, &consumerID); err != nil { t.Fatal(err) }
	if webmasterID <= 0 || consumerID != u.ID { t.Fatalf("webmaster_id=%d consumer_id=%d want=%d", webmasterID, consumerID, u.ID) }
	var consumers int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM consumer_users WHERE lower(email)=lower($1)`, u.Email).Scan(&consumers); err != nil { t.Fatal(err) }
	if consumers != 1 { t.Fatalf("consumer duplicates=%d", consumers) }
}
