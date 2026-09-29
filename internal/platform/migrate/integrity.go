package migrate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VerifyAppliedChecksums binds the applied database schema to the exact SQL
// files in the release candidate. It is intentionally fail-closed: legacy
// rows without checksum metadata must first pass Up(), which establishes an
// explicit baseline, before commercial-readiness can be evaluated.
func VerifyAppliedChecksums(ctx context.Context,db *pgxpool.Pool,dir string)error{
	if db==nil{return errors.New("migration database is not initialized")}
	expected,err:=ExpectedMigrations(dir);if err!=nil{return err}
	for _,m:=range expected{
		var name,sha *string
		err=db.QueryRow(ctx,`SELECT name,sha256 FROM schema_migrations WHERE version=$1`,m.Version).Scan(&name,&sha)
		if errors.Is(err,pgx.ErrNoRows){return fmt.Errorf("migration %06d is not applied",m.Version)}
		if err!=nil{return fmt.Errorf("read migration %06d checksum metadata: %w",m.Version,err)}
		if name==nil||sha==nil||strings.TrimSpace(*name)==""||strings.TrimSpace(*sha)==""{return fmt.Errorf("migration %06d checksum metadata is missing; run migrator before readiness gate",m.Version)}
		if *name!=m.Name||strings.ToLower(strings.TrimSpace(*sha))!=m.SHA256{return fmt.Errorf("migration %06d drift: database name=%q sha256=%s repository name=%q sha256=%s",m.Version,*name,strings.ToLower(strings.TrimSpace(*sha)),m.Name,m.SHA256)}
	}
	var unexpected int64
	if err:=db.QueryRow(ctx,`SELECT count(*) FROM schema_migrations WHERE NOT (version=ANY($1::bigint[]))`,expectedVersions(expected)).Scan(&unexpected);err!=nil{return fmt.Errorf("check unexpected applied migrations: %w",err)}
	if unexpected!=0{return fmt.Errorf("database contains %d unexpected migration versions",unexpected)}
	return nil
}

func expectedVersions(items []ExpectedMigration)[]int64{out:=make([]int64,len(items));for i,item:=range items{out[i]=item.Version};return out}
