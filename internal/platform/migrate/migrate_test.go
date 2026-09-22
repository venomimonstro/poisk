package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMigrationsRejectsDuplicateVersions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"000001_first.sql", "000001_second.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o600); err != nil { t.Fatal(err) }
	}
	_, err := loadMigrations(dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate migration version") { t.Fatalf("err=%v", err) }
}

func TestLoadMigrationsSortsAndValidatesVersions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"000010_ten.sql", "000002_two.sql", "README.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o600); err != nil { t.Fatal(err) }
	}
	items, err := loadMigrations(dir)
	if err != nil { t.Fatal(err) }
	if len(items) != 2 || items[0].version != 2 || items[1].version != 10 { t.Fatalf("items=%+v", items) }
}
