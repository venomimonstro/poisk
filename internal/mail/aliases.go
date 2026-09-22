package mail

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

type ExternalAlias struct{ID int64 `json:"alias_id"`;MailboxID int64 `json:"mailbox_id"`;Address string `json:"address"`;Status string `json:"status"`;Primary bool `json:"is_primary"`}

func (r Repository) EnsurePrimaryExternalAlias(ctx context.Context,userID int64,domain string)(ExternalAlias,error){
	if r.DB==nil||userID<=0{return ExternalAlias{},ErrInvalid};domain=strings.ToLower(strings.TrimSpace(domain));if domain==""||strings.ContainsAny(domain," /:@?#\\"){return ExternalAlias{},ErrInvalid}
	box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return ExternalAlias{},err}
	var existing ExternalAlias
	err=r.DB.QueryRow(ctx,`SELECT alias_id,mailbox_id,address,status,is_primary FROM mail_external_aliases WHERE mailbox_id=$1 AND is_primary AND status='ACTIVE'`,box.ID).Scan(&existing.ID,&existing.MailboxID,&existing.Address,&existing.Status,&existing.Primary)
	if err==nil{return existing,nil};if !errors.Is(err,pgx.ErrNoRows){return ExternalAlias{},err}
	at:=strings.LastIndexByte(box.Address,'@');if at<=0{return ExternalAlias{},ErrConflict};local:=strings.ToLower(box.Address[:at]);var out ExternalAlias
	err=r.DB.QueryRow(ctx,`INSERT INTO mail_external_aliases(mailbox_id,local_part,domain,status,is_primary) VALUES($1,$2,$3,'ACTIVE',TRUE) ON CONFLICT(local_part,domain) DO UPDATE SET updated_at=now() WHERE mail_external_aliases.mailbox_id=EXCLUDED.mailbox_id RETURNING alias_id,mailbox_id,address,status,is_primary`,box.ID,local,domain).Scan(&out.ID,&out.MailboxID,&out.Address,&out.Status,&out.Primary)
	if errors.Is(err,pgx.ErrNoRows){return ExternalAlias{},ErrConflict};return out,err
}

func (r Repository) PrimaryExternalAliasForMailbox(ctx context.Context,mailboxID int64)(ExternalAlias,error){
	if r.DB==nil||mailboxID<=0{return ExternalAlias{},ErrInvalid};var out ExternalAlias
	err:=r.DB.QueryRow(ctx,`SELECT a.alias_id,a.mailbox_id,a.address,a.status,a.is_primary FROM mail_external_aliases a JOIN mailboxes mb ON mb.mailbox_id=a.mailbox_id WHERE a.mailbox_id=$1 AND a.is_primary AND a.status='ACTIVE' AND mb.status='ACTIVE'`,mailboxID).Scan(&out.ID,&out.MailboxID,&out.Address,&out.Status,&out.Primary);if errors.Is(err,pgx.ErrNoRows){return ExternalAlias{},ErrNotFound};return out,err
}

func (r Repository) ResolveInboundAlias(ctx context.Context,address string)(ExternalAlias,error){
	if r.DB==nil{return ExternalAlias{},ErrInvalid};address=strings.ToLower(strings.TrimSpace(address));if address==""||len(address)>320{return ExternalAlias{},ErrInvalid};var out ExternalAlias
	err:=r.DB.QueryRow(ctx,`SELECT a.alias_id,a.mailbox_id,a.address,a.status,a.is_primary FROM mail_external_aliases a JOIN mailboxes mb ON mb.mailbox_id=a.mailbox_id WHERE lower(a.address)=lower($1) AND a.status='ACTIVE' AND mb.status='ACTIVE'`,address).Scan(&out.ID,&out.MailboxID,&out.Address,&out.Status,&out.Primary);if errors.Is(err,pgx.ErrNoRows){return ExternalAlias{},ErrNotFound};return out,err
}
