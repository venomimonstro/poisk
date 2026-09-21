package agency

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("agency not found")
	ErrForbidden = errors.New("agency action forbidden")
	ErrInvalid   = errors.New("invalid agency input")
)

type Repository struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type Agency struct {
	ID int64 `json:"agency_id"`
	Name string `json:"name"`
	PublicCode string `json:"public_code"`
	Role string `json:"role,omitempty"`
}

type DelegatedSite struct {
	SiteID int64 `json:"site_id"`
	Origin string `json:"origin"`
	Host string `json:"host"`
	Permission string `json:"permission"`
}

func newPublicCode()(string,error){raw:=make([]byte,18);if _,err:=rand.Read(raw);err!=nil{return "",err};return "ag_"+base64.RawURLEncoding.EncodeToString(raw),nil}

func (r *Repository) Create(ctx context.Context,userID int64,name string)(Agency,error){
	name=strings.TrimSpace(name);if r==nil||r.db==nil||userID<=0||len(name)<2||len(name)>160{return Agency{},ErrInvalid}
	code,err:=newPublicCode();if err!=nil{return Agency{},err}
	tx,err:=r.db.Begin(ctx);if err!=nil{return Agency{},err};defer func(){_=tx.Rollback(ctx)}()
	var out Agency
	if err=tx.QueryRow(ctx,`INSERT INTO agencies(name,public_code,created_by_user_id) VALUES($1,$2,$3) RETURNING agency_id,name,public_code`,name,code,userID).Scan(&out.ID,&out.Name,&out.PublicCode);err!=nil{return Agency{},err}
	if _,err=tx.Exec(ctx,`INSERT INTO agency_members(agency_id,user_id,role) VALUES($1,$2,'OWNER')`,out.ID,userID);err!=nil{return Agency{},err}
	if _,err=tx.Exec(ctx,`INSERT INTO agency_audit_events(agency_id,actor_user_id,action) VALUES($1,$2,'AGENCY_CREATE')`,out.ID,userID);err!=nil{return Agency{},err}
	if err=tx.Commit(ctx);err!=nil{return Agency{},err};out.Role="OWNER";return out,nil
}

func (r *Repository) ListForUser(ctx context.Context,userID int64)([]Agency,error){
	if r==nil||r.db==nil||userID<=0{return nil,ErrInvalid}
	rows,err:=r.db.Query(ctx,`SELECT a.agency_id,a.name,a.public_code,m.role FROM agencies a JOIN agency_members m ON m.agency_id=a.agency_id WHERE m.user_id=$1 AND m.status='ACTIVE' AND a.status='ACTIVE' ORDER BY a.agency_id`,userID);if err!=nil{return nil,err};defer rows.Close()
	out:=[]Agency{};for rows.Next(){var item Agency;if err:=rows.Scan(&item.ID,&item.Name,&item.PublicCode,&item.Role);err!=nil{return nil,err};out=append(out,item)};return out,rows.Err()
}

func (r *Repository) AddMemberByEmail(ctx context.Context,actorUserID,agencyID int64,email,role string)error{
	email=strings.ToLower(strings.TrimSpace(email));role=strings.ToUpper(strings.TrimSpace(role));if r==nil||r.db==nil||actorUserID<=0||agencyID<=0||email==""||(role!="MANAGER"&&role!="ANALYST"){return ErrInvalid}
	tx,err:=r.db.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var actorRole string;if err=tx.QueryRow(ctx,`SELECT role FROM agency_members WHERE agency_id=$1 AND user_id=$2 AND status='ACTIVE' FOR UPDATE`,agencyID,actorUserID).Scan(&actorRole);errors.Is(err,pgx.ErrNoRows){return ErrForbidden};if err!=nil{return err};if actorRole!="OWNER"{return ErrForbidden}
	var targetID int64;if err=tx.QueryRow(ctx,`SELECT user_id FROM webmaster_users WHERE lower(email)=lower($1) AND status='ACTIVE'`,email).Scan(&targetID);errors.Is(err,pgx.ErrNoRows){return ErrNotFound};if err!=nil{return err}
	_,err=tx.Exec(ctx,`INSERT INTO agency_members(agency_id,user_id,role,status,revoked_at) VALUES($1,$2,$3,'ACTIVE',NULL) ON CONFLICT(agency_id,user_id) DO UPDATE SET role=EXCLUDED.role,status='ACTIVE',revoked_at=NULL`,agencyID,targetID,role);if err!=nil{return err}
	details,_:=json.Marshal(map[string]any{"member_user_id":targetID,"role":role});if _,err=tx.Exec(ctx,`INSERT INTO agency_audit_events(agency_id,actor_user_id,action,details) VALUES($1,$2,'MEMBER_GRANT',$3::jsonb)`,agencyID,actorUserID,string(details));err!=nil{return err};return tx.Commit(ctx)
}

func (r *Repository) GrantSite(ctx context.Context,siteOwnerID,agencyID,siteID int64,permission string)error{
	permission=strings.ToUpper(strings.TrimSpace(permission));if r==nil||r.db==nil||siteOwnerID<=0||agencyID<=0||siteID<=0||(permission!="READ"&&permission!="MANAGE"){return ErrInvalid}
	tx,err:=r.db.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var exists bool;if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM webmaster_sites s JOIN domains d ON d.domain_id=s.domain_id WHERE s.site_id=$1 AND s.user_id=$2 AND s.status='VERIFIED' AND d.status='ACTIVE' AND d.policy<>'BLOCK')`,siteID,siteOwnerID).Scan(&exists);err!=nil{return err};if !exists{return ErrForbidden}
	if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM agencies WHERE agency_id=$1 AND status='ACTIVE')`,agencyID).Scan(&exists);err!=nil{return err};if !exists{return ErrNotFound}
	_,err=tx.Exec(ctx,`INSERT INTO agency_site_access(agency_id,site_id,permission,status,granted_by_user_id,revoked_at,updated_at) VALUES($1,$2,$3,'ACTIVE',$4,NULL,now()) ON CONFLICT(agency_id,site_id) DO UPDATE SET permission=EXCLUDED.permission,status='ACTIVE',granted_by_user_id=EXCLUDED.granted_by_user_id,granted_at=now(),revoked_at=NULL,updated_at=now()`,agencyID,siteID,permission,siteOwnerID);if err!=nil{return err}
	details,_:=json.Marshal(map[string]any{"permission":permission});if _,err=tx.Exec(ctx,`INSERT INTO agency_audit_events(agency_id,actor_user_id,site_id,action,details) VALUES($1,$2,$3,'SITE_GRANT',$4::jsonb)`,agencyID,siteOwnerID,siteID,string(details));err!=nil{return err};return tx.Commit(ctx)
}

func (r *Repository) RevokeSite(ctx context.Context,siteOwnerID,agencyID,siteID int64)error{
	if r==nil||r.db==nil||siteOwnerID<=0||agencyID<=0||siteID<=0{return ErrInvalid}
	tx,err:=r.db.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	tag,err:=tx.Exec(ctx,`UPDATE agency_site_access a SET status='REVOKED',revoked_at=now(),updated_at=now() FROM webmaster_sites s WHERE a.agency_id=$1 AND a.site_id=$2 AND s.site_id=a.site_id AND s.user_id=$3 AND a.status='ACTIVE'`,agencyID,siteID,siteOwnerID);if err!=nil{return err};if tag.RowsAffected()!=1{return ErrForbidden}
	if _,err=tx.Exec(ctx,`INSERT INTO agency_audit_events(agency_id,actor_user_id,site_id,action) VALUES($1,$2,$3,'SITE_REVOKE')`,agencyID,siteOwnerID,siteID);err!=nil{return err};return tx.Commit(ctx)
}

func (r *Repository) ListSites(ctx context.Context,userID,agencyID int64)([]DelegatedSite,error){
	if r==nil||r.db==nil||userID<=0||agencyID<=0{return nil,ErrInvalid}
	rows,err:=r.db.Query(ctx,`SELECT s.site_id,s.origin,s.host,a.permission FROM agency_members m JOIN agencies g ON g.agency_id=m.agency_id JOIN agency_site_access a ON a.agency_id=g.agency_id JOIN webmaster_sites s ON s.site_id=a.site_id JOIN domains d ON d.domain_id=s.domain_id WHERE m.agency_id=$1 AND m.user_id=$2 AND m.status='ACTIVE' AND g.status='ACTIVE' AND a.status='ACTIVE' AND s.status='VERIFIED' AND d.status='ACTIVE' AND d.policy<>'BLOCK' ORDER BY s.site_id`,agencyID,userID);if err!=nil{return nil,err};defer rows.Close()
	out:=[]DelegatedSite{};for rows.Next(){var item DelegatedSite;if err:=rows.Scan(&item.SiteID,&item.Origin,&item.Host,&item.Permission);err!=nil{return nil,err};out=append(out,item)};return out,rows.Err()
}
