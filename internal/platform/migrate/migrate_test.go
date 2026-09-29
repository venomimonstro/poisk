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

func TestExpectedMigrationsBindsContentChecksum(t *testing.T){
	dir:=t.TempDir();path:=filepath.Join(dir,"000001_first.sql")
	if err:=os.WriteFile(path,[]byte("SELECT 1;\n"),0o600);err!=nil{t.Fatal(err)}
	first,err:=ExpectedMigrations(dir);if err!=nil{t.Fatal(err)};if len(first)!=1||len(first[0].SHA256)!=64{t.Fatalf("first=%+v",first)}
	if err:=os.WriteFile(path,[]byte("SELECT 2;\n"),0o600);err!=nil{t.Fatal(err)}
	second,err:=ExpectedMigrations(dir);if err!=nil{t.Fatal(err)}
	if first[0].SHA256==second[0].SHA256{t.Fatalf("checksum did not change after SQL drift: %s",first[0].SHA256)}
	if first[0].Version!=second[0].Version{t.Fatalf("version unexpectedly changed")}
}

func TestExpectedChecksumsMatchesExpectedMigrations(t *testing.T){
	dir:=t.TempDir();if err:=os.WriteFile(filepath.Join(dir,"000007_seven.sql"),[]byte("SELECT 7;"),0o600);err!=nil{t.Fatal(err)}
	items,err:=ExpectedMigrations(dir);if err!=nil{t.Fatal(err)};checksums,err:=ExpectedChecksums(dir);if err!=nil{t.Fatal(err)}
	if got:=checksums[7];got!=items[0].SHA256{t.Fatalf("checksum=%q expected=%q",got,items[0].SHA256)}
}
