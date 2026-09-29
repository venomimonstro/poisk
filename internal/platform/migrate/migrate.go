package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationAdvisoryLock int64 = 72465190218341

type migration struct {
	version int64
	path    string
	name    string
}

type ExpectedMigration struct {
	Version int64
	Name string
	SHA256 string
}

func Up(ctx context.Context, db *pgxpool.Pool, dir string) error {
	if db == nil { return fmt.Errorf("migration database is not initialized") }
	conn, err := db.Acquire(ctx)
	if err != nil { return fmt.Errorf("acquire migration connection: %w", err) }
	defer conn.Release()
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationAdvisoryLock); err != nil { return fmt.Errorf("acquire migration lock: %w", err) }
	defer func(){ _, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationAdvisoryLock) }()

	if _, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version BIGINT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	if _, err = conn.Exec(ctx, `ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS name TEXT, ADD COLUMN IF NOT EXISTS sha256 CHAR(64)`); err != nil {
		return fmt.Errorf("upgrade schema_migrations metadata: %w", err)
	}

	migrations, err := loadMigrations(dir)
	if err != nil { return err }

	for _, m := range migrations {
		body, err := os.ReadFile(m.path)
		if err != nil { return fmt.Errorf("read migration %s: %w", m.name, err) }
		digest := sha256Hex(body)

		var storedName, storedSHA *string
		err = conn.QueryRow(ctx, `SELECT name,sha256 FROM schema_migrations WHERE version=$1`, m.version).Scan(&storedName,&storedSHA)
		if err == nil {
			if storedName == nil || strings.TrimSpace(*storedName) == "" || storedSHA == nil || strings.TrimSpace(*storedSHA) == "" {
				// One-time baseline for a pre-checksum database. If the immutable
				// trigger already exists, this update intentionally fails closed.
				if _, err = conn.Exec(ctx, `UPDATE schema_migrations SET name=$2,sha256=$3 WHERE version=$1 AND (name IS NULL OR sha256 IS NULL OR btrim(name)='' OR btrim(sha256)='')`, m.version,m.name,digest); err != nil { return fmt.Errorf("baseline migration %s checksum: %w",m.name,err) }
				continue
			}
			if *storedName != m.name || strings.ToLower(strings.TrimSpace(*storedSHA)) != digest {
				return fmt.Errorf("applied migration drift detected for version %06d: database name=%q sha256=%s repository name=%q sha256=%s",m.version,*storedName,strings.ToLower(strings.TrimSpace(*storedSHA)),m.name,digest)
			}
			continue
		}
		if !errors.Is(err,pgx.ErrNoRows) { return fmt.Errorf("check migration %s: %w", m.name, err) }

		tx, err := conn.Begin(ctx)
		if err != nil { return fmt.Errorf("begin migration %s: %w", m.name, err) }
		if _, err = tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("execute migration %s: %w", m.name, err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version,name,sha256) VALUES ($1,$2,$3)`, m.version,m.name,digest); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", m.name, err)
		}
		if err = tx.Commit(ctx); err != nil { return fmt.Errorf("commit migration %s: %w", m.name, err) }
	}

	// The migrator owns this metadata. After the one-time legacy checksum
	// baseline, rows are append-only; future schema changes are new INSERTs.
	if _, err = conn.Exec(ctx, `
CREATE OR REPLACE FUNCTION schema_migrations_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'schema_migrations rows are immutable';
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_schema_migrations_immutable ON schema_migrations;
CREATE TRIGGER trg_schema_migrations_immutable
BEFORE UPDATE OR DELETE ON schema_migrations
FOR EACH ROW EXECUTE FUNCTION schema_migrations_immutable();`); err != nil {
		return fmt.Errorf("protect schema_migrations metadata: %w", err)
	}
	return nil
}

func ExpectedMigrations(dir string) ([]ExpectedMigration,error) {
	migrations,err:=loadMigrations(dir);if err!=nil{return nil,err};if len(migrations)==0{return nil,fmt.Errorf("no migrations found in %s",dir)}
	out:=make([]ExpectedMigration,0,len(migrations));for _,m:=range migrations{body,err:=os.ReadFile(m.path);if err!=nil{return nil,fmt.Errorf("read migration %s: %w",m.name,err)};out=append(out,ExpectedMigration{Version:m.version,Name:m.name,SHA256:sha256Hex(body)})};return out,nil
}

// ExpectedVersions returns the exact ordered migration-version set in dir.
// It shares Up's parsing and duplicate validation, so launch-readiness checks
// cannot silently accept a missing migration in the middle of the chain.
func ExpectedVersions(dir string) ([]int64, error) {
	migrations, err := ExpectedMigrations(dir)
	if err != nil { return nil, err }
	versions := make([]int64, len(migrations))
	for i, m := range migrations { versions[i] = m.Version }
	return versions, nil
}

func ExpectedChecksums(dir string)(map[int64]string,error){migrations,err:=ExpectedMigrations(dir);if err!=nil{return nil,err};out:=make(map[int64]string,len(migrations));for _,m:=range migrations{out[m.Version]=m.SHA256};return out,nil}

func LatestVersion(dir string) (int64, error) {
	versions, err := ExpectedVersions(dir)
	if err != nil { return 0, err }
	return versions[len(versions)-1], nil
}

func sha256Hex(body []byte)string{sum:=sha256.Sum256(body);return hex.EncodeToString(sum[:])}

func loadMigrations(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil { return nil, fmt.Errorf("read migrations dir: %w", err) }

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") { continue }
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok { return nil, fmt.Errorf("invalid migration filename %q", entry.Name()) }
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil { return nil, fmt.Errorf("invalid migration version %q: %w", entry.Name(), err) }
		if version <= 0 { return nil, fmt.Errorf("invalid migration version %q: must be positive", entry.Name()) }
		migrations = append(migrations, migration{version: version, path: filepath.Join(dir, entry.Name()), name: entry.Name()})
	}

	sort.Slice(migrations, func(i, j int) bool {
		if migrations[i].version == migrations[j].version { return migrations[i].name < migrations[j].name }
		return migrations[i].version < migrations[j].version
	})
	for i := 1; i < len(migrations); i++ {
		if migrations[i-1].version == migrations[i].version {
			return nil, fmt.Errorf("duplicate migration version %06d: %s and %s", migrations[i].version, migrations[i-1].name, migrations[i].name)
		}
	}
	return migrations, nil
}
