package mail

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var externalLocalPartPattern=regexp.MustCompile(`^[a-z0-9][a-z0-9._+-]{0,63}$`)

func (r Repository) ClaimPrimaryExternalAlias(ctx context.Context,userID int64,localPart,domain string)(ExternalAlias,error){
	if r.DB==nil||userID<=0{return ExternalAlias{},ErrInvalid};localPart=strings.ToLower(strings.TrimSpace(localPart));domain=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain),"."));if !externalLocalPartPattern.MatchString(localPart)||domain==""{return ExternalAlias{},ErrInvalid}
	box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return ExternalAlias{},err};address:=localPart+"@"+domain
	tx,err:=r.DB.Begin(ctx);if err!=nil{return ExternalAlias{},err};defer func(){_=tx.Rollback(ctx)}()
	var active bool;if err=tx.QueryRow(ctx,`SELECT status='ACTIVE' FROM mailboxes WHERE mailbox_id=$1 FOR UPDATE`,box.ID).Scan(&active);err!=nil{return ExternalAlias{},err};if !active{return ExternalAlias{},ErrForbidden}
	if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(hashtext($1))`,address);err!=nil{return ExternalAlias{},err}
	var existing ExternalAlias
	err=tx.QueryRow(ctx,`SELECT alias_id,mailbox_id,local_part,domain,address,status,is_primary FROM mail_external_aliases WHERE mailbox_id=$1 AND is_primary AND status='ACTIVE'`,box.ID).Scan(&existing.ID,&existing.MailboxID,&existing.LocalPart,&existing.Domain,&existing.Address,&existing.Status,&existing.IsPrimary)
	if err==nil{if existing.Address!=address{return ExternalAlias{},ErrConflict};return existing,tx.Commit(ctx)};if !errors.Is(err,pgx.ErrNoRows){return ExternalAlias{},err}
	var taken bool;if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM mail_external_aliases WHERE address=$1)`,address).Scan(&taken);err!=nil{return ExternalAlias{},err};if taken{return ExternalAlias{},ErrConflict}
	var out ExternalAlias;err=tx.QueryRow(ctx,`INSERT INTO mail_external_aliases(mailbox_id,local_part,domain,status,is_primary) VALUES($1,$2,$3,'ACTIVE',TRUE) RETURNING alias_id,mailbox_id,local_part,domain,address,status,is_primary`,box.ID,localPart,domain).Scan(&out.ID,&out.MailboxID,&out.LocalPart,&out.Domain,&out.Address,&out.Status,&out.IsPrimary);if err!=nil{return ExternalAlias{},err};if err=tx.Commit(ctx);err!=nil{return ExternalAlias{},err};return out,nil
}

func (r Repository) UserPrimaryExternalAlias(ctx context.Context,userID int64)(ExternalAlias,error){
	if r.DB==nil||userID<=0{return ExternalAlias{},ErrInvalid};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return ExternalAlias{},err};return r.PrimaryExternalAliasForMailbox(ctx,box.ID)
}
