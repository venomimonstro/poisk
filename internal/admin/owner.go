package admin

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OwnerSnapshot struct {
	Users struct {
		Total int64 `json:"total"`
		Active int64 `json:"active"`
		Locked int64 `json:"locked"`
		Disabled int64 `json:"disabled"`
	} `json:"users"`
	Webmaster struct {
		Sites int64 `json:"sites"`
		Verified int64 `json:"verified"`
		Pending int64 `json:"pending"`
		Suspended int64 `json:"suspended"`
	} `json:"webmaster"`
	Directory struct {
		Organizations int64 `json:"organizations"`
		Addresses int64 `json:"addresses"`
	} `json:"directory"`
	Capacity *OwnerCapacity `json:"capacity,omitempty"`
}

type OwnerCapacity struct {
	SnapshotID int64 `json:"snapshot_id"`
	MeasuredDocuments int64 `json:"measured_documents"`
	MeasuredAt time.Time `json:"measured_at"`
	Bottlenecks json.RawMessage `json:"bottlenecks"`
}

type OwnerRepository struct{ DB *pgxpool.Pool }

func (r OwnerRepository) Snapshot(ctx context.Context)(OwnerSnapshot,error){
	if r.DB==nil{return OwnerSnapshot{},errors.New("owner repository database unavailable")}
	var out OwnerSnapshot
	if err:=r.DB.QueryRow(ctx,`SELECT count(*),count(*) FILTER(WHERE status='ACTIVE'),count(*) FILTER(WHERE status='LOCKED'),count(*) FILTER(WHERE status='DISABLED') FROM consumer_users`).Scan(&out.Users.Total,&out.Users.Active,&out.Users.Locked,&out.Users.Disabled);err!=nil{return OwnerSnapshot{},err}
	if err:=r.DB.QueryRow(ctx,`SELECT count(*),count(*) FILTER(WHERE status='VERIFIED'),count(*) FILTER(WHERE status='PENDING'),count(*) FILTER(WHERE status='SUSPENDED') FROM webmaster_sites`).Scan(&out.Webmaster.Sites,&out.Webmaster.Verified,&out.Webmaster.Pending,&out.Webmaster.Suspended);err!=nil{return OwnerSnapshot{},err}
	if err:=r.DB.QueryRow(ctx,`SELECT count(*) FILTER(WHERE status='ACTIVE'),(SELECT count(*) FROM addresses WHERE status='ACTIVE') FROM organizations`).Scan(&out.Directory.Organizations,&out.Directory.Addresses);err!=nil{return OwnerSnapshot{},err}
	var cap OwnerCapacity
	err:=r.DB.QueryRow(ctx,`SELECT snapshot_id,measured_documents,measured_at,bottlenecks FROM capacity_snapshots ORDER BY measured_at DESC,snapshot_id DESC LIMIT 1`).Scan(&cap.SnapshotID,&cap.MeasuredDocuments,&cap.MeasuredAt,&cap.Bottlenecks)
	if err==nil{out.Capacity=&cap}else if !errors.Is(err,pgx.ErrNoRows){return OwnerSnapshot{},err}
	return out,nil
}
