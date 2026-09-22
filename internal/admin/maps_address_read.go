package admin

import (
	"context"
	"errors"
	"time"
)

type MapVersionSummary struct {
	Version string `json:"version"`
	Status string `json:"status"`
	PMTilesSize int64 `json:"pmtiles_size"`
	PMTilesSHA256 string `json:"pmtiles_sha256"`
	StyleSHA256 string `json:"style_sha256"`
	MinZoom int `json:"min_zoom"`
	MaxZoom int `json:"max_zoom"`
	CreatedAt time.Time `json:"created_at"`
}

type MapStateSummary struct {
	Active *MapVersionSummary `json:"active,omitempty"`
	Previous *MapVersionSummary `json:"previous,omitempty"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AddressDataSummary struct {
	ActiveAddresses int64 `json:"active_addresses"`
	InactiveAddresses int64 `json:"inactive_addresses"`
	OrphanAddresses int64 `json:"orphan_addresses"`
	IgnoredAddresses int64 `json:"ignored_addresses"`
	LatestBatchID *int64 `json:"latest_batch_id,omitempty"`
	LatestRevision string `json:"latest_revision,omitempty"`
	LatestBatchStatus string `json:"latest_batch_status,omitempty"`
	StagedCount int64 `json:"staged_count"`
	RejectedCount int64 `json:"rejected_count"`
	AppliedCount int64 `json:"applied_count"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

func (s Service) MapState(ctx context.Context,session Session)(MapStateSummary,error){
	if s.Store==nil||s.Store.db==nil{return MapStateSummary{},errors.New("admin service is not initialized")};if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return MapStateSummary{},err}
	var activeID,previousID *string;var out MapStateSummary
	if err:=s.Store.db.QueryRow(ctx,`SELECT active_version,previous_version,activated_at,updated_at FROM map_state WHERE singleton=TRUE`).Scan(&activeID,&previousID,&out.ActivatedAt,&out.UpdatedAt);err!=nil{return MapStateSummary{},err}
	load:=func(id *string)(*MapVersionSummary,error){if id==nil{return nil,nil};var v MapVersionSummary;err:=s.Store.db.QueryRow(ctx,`SELECT map_version,status,pmtiles_size,pmtiles_sha256,style_sha256,min_zoom,max_zoom,created_at FROM map_versions WHERE map_version=$1`,*id).Scan(&v.Version,&v.Status,&v.PMTilesSize,&v.PMTilesSHA256,&v.StyleSHA256,&v.MinZoom,&v.MaxZoom,&v.CreatedAt);return &v,err}
	var err error;if out.Active,err=load(activeID);err!=nil{return MapStateSummary{},err};if out.Previous,err=load(previousID);err!=nil{return MapStateSummary{},err};return out,nil
}

func (s Service) AddressData(ctx context.Context,session Session)(AddressDataSummary,error){
	if s.Store==nil||s.Store.db==nil{return AddressDataSummary{},errors.New("admin service is not initialized")};if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return AddressDataSummary{},err}
	var out AddressDataSummary
	if err:=s.Store.db.QueryRow(ctx,`SELECT count(*) FILTER(WHERE status='ACTIVE'),count(*) FILTER(WHERE status='INACTIVE') FROM addresses`).Scan(&out.ActiveAddresses,&out.InactiveAddresses);err!=nil{return AddressDataSummary{},err}
	_ = s.Store.db.QueryRow(ctx,`SELECT count(*) FROM addresses WHERE status='ACTIVE' AND parent_address_id IS NULL AND level IS NOT NULL AND level>1`).Scan(&out.OrphanAddresses)
	_ = s.Store.db.QueryRow(ctx,`SELECT count(*) FROM address_staging_rows WHERE state='IGNORED'`).Scan(&out.IgnoredAddresses)
	var id int64;var updated time.Time;err:=s.Store.db.QueryRow(ctx,`SELECT batch_id,source_revision,status,staged_count,rejected_count,applied_count,updated_at FROM address_import_batches ORDER BY updated_at DESC,batch_id DESC LIMIT 1`).Scan(&id,&out.LatestRevision,&out.LatestBatchStatus,&out.StagedCount,&out.RejectedCount,&out.AppliedCount,&updated);if err==nil{out.LatestBatchID=&id;out.UpdatedAt=&updated}
	return out,nil
}
