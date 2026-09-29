package release

import (
	"context"
	"strings"

	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
)

// ValidateRollbackCompatibility is intentionally stricter than forward
// activation preflight. Without an explicit forward-compatibility declaration,
// an older binary must not be rolled back onto a newer database schema.
func (r Repository) ValidateRollbackCompatibility(ctx context.Context,manifest Manifest)error{
	if r.DB==nil{return ErrPreflight}
	var dbSchema int64
	if err:=r.DB.QueryRow(ctx,`SELECT COALESCE(max(version),0) FROM schema_migrations`).Scan(&dbSchema);err!=nil{return err}
	var activeMap string
	if err:=r.DB.QueryRow(ctx,`SELECT COALESCE(active_version,'') FROM map_state WHERE singleton=TRUE`).Scan(&activeMap);err!=nil{return err}
	pass:=dbSchema==manifest.RequiredSchemaVersion&&
		manifest.WebIndexSchema==indexmanticore.WebSchemaVersion&&
		manifest.OrganizationIndexSchema==indexmanticore.OrganizationsSchemaVersion&&
		manifest.AddressIndexSchema==indexmanticore.AddressesSchemaVersion&&
		(manifest.MapVersion==""||manifest.MapVersion==activeMap)&&
		strings.TrimSpace(manifest.BackendImage)!=""&&strings.TrimSpace(manifest.FrontendImage)!=""
	if !pass{return ErrPreflight}
	return nil
}
