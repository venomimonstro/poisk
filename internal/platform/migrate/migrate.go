package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type migration struct {
	version int64
	path    string
	name    string
}

func Up(ctx context.Context, db *pgxpool.Pool, dir string) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version BIGINT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migrations, err := loadMigrations(dir)
	if err != nil { return err }

	for _, m := range migrations {
		var applied bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, m.version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", m.name, err)
		}
		if applied { continue }

		body, err := os.ReadFile(m.path)
		if err != nil { return fmt.Errorf("read migration %s: %w", m.name, err) }

		tx, err := db.Begin(ctx)
		if err != nil { return fmt.Errorf("begin migration %s: %w", m.name, err) }
		if _, err = tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("execute migration %s: %w", m.name, err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, m.version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", m.name, err)
		}
		if err = tx.Commit(ctx); err != nil { return fmt.Errorf("commit migration %s: %w", m.name, err) }
	}
	return nil
}

// ExpectedVersions returns the exact ordered migration-version set in dir.
// It shares Up's parsing and duplicate validation, so launch-readiness checks
// cannot silently accept a missing migration in the middle of the chain.
func ExpectedVersions(dir string) ([]int64, error) {
	migrations, err := loadMigrations(dir)
	if err != nil { return nil, err }
	if len(migrations) == 0 { return nil, fmt.Errorf("no migrations found in %s", dir) }
	versions := make([]int64, len(migrations))
	for i, m := range migrations { versions[i] = m.version }
	return versions, nil
}

func LatestVersion(dir string) (int64, error) {
	versions, err := ExpectedVersions(dir)
	if err != nil { return 0, err }
	return versions[len(versions)-1], nil
}

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
