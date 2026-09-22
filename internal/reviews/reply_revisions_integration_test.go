//go:build integration

package reviews

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestOwnerReplyKeepsImmutableRevisionHistory(t *testing.T) {
	pool := reviewDB(t)
	repo := Repository{DB: pool}
	ctx := context.Background()
	reviewer := createConsumer(t, pool, "reply-history-reviewer")
	owner := createConsumer(t, pool, "reply-history-owner")
	placeID := createPlace(t, pool, "Reply history place")

	review, err := repo.Upsert(ctx, reviewer, placeID, 5, "Отзыв для проверки истории ответов владельца.")
	if err != nil { t.Fatal(err) }

	var domainID int64
	host := fmt.Sprintf("reply-history-%d.example.test", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO domains(host) VALUES($1) RETURNING domain_id`, host).Scan(&domainID); err != nil { t.Fatal(err) }
	var webmasterID int64
	if err := pool.QueryRow(ctx, `INSERT INTO webmaster_users(email,password_hash,status,consumer_user_id) SELECT email,password_hash,'ACTIVE',user_id FROM consumer_users WHERE user_id=$1 RETURNING user_id`, owner).Scan(&webmasterID); err != nil { t.Fatal(err) }
	var siteID int64
	if err := pool.QueryRow(ctx, `INSERT INTO webmaster_sites(user_id,domain_id,origin,host,status,verified_at,verification_method) VALUES($1,$2,$3,$4,'VERIFIED',now(),'DNS_TXT') RETURNING site_id`, webmasterID, domainID, "https://"+host, host).Scan(&siteID); err != nil { t.Fatal(err) }
	if _, err := pool.Exec(ctx, `INSERT INTO organization_claims(place_id,user_id,site_id,proof_type,proof_host,status) VALUES($1,$2,$3,'VERIFIED_WEBSITE_HOST',$4,'ACTIVE')`, placeID, webmasterID, siteID, host); err != nil { t.Fatal(err) }

	first, err := repo.Reply(ctx, owner, review.ID, "Спасибо за первый отзыв!")
	if err != nil { t.Fatal(err) }
	second, err := repo.Reply(ctx, owner, review.ID, "Спасибо, мы уточнили информацию и обновили ответ.")
	if err != nil { t.Fatal(err) }
	if second.ID != first.ID || second.Version != first.Version+1 { t.Fatalf("first=%+v second=%+v", first, second) }

	var revisions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM organization_review_reply_revisions WHERE reply_id=$1`, first.ID).Scan(&revisions); err != nil { t.Fatal(err) }
	if revisions != 2 { t.Fatalf("reply revisions=%d", revisions) }
	if _, err := pool.Exec(ctx, `UPDATE organization_review_reply_revisions SET body='tampered' WHERE reply_id=$1 AND version=1`, first.ID); err == nil { t.Fatal("reply revision update unexpectedly succeeded") }
	if _, err := pool.Exec(ctx, `DELETE FROM organization_review_replies WHERE reply_id=$1`, first.ID); err == nil { t.Fatal("owner reply hard delete unexpectedly succeeded") }
}
