package admin

import (
	"context"
	"errors"
)

func (r *Repository) RotateSessionCSRF(ctx context.Context,sessionID,adminID int64,hash []byte)error{
	if r==nil||r.db==nil||sessionID<=0||adminID<=0||len(hash)!=32{return ErrInvalidCredential}
	tag,err:=r.db.Exec(ctx,`UPDATE admin_sessions SET csrf_hash=$3,last_seen_at=now() WHERE session_id=$1 AND admin_id=$2 AND revoked_at IS NULL AND expires_at>now()`,sessionID,adminID,hash)
	if err!=nil{return err};if tag.RowsAffected()!=1{return ErrSessionNotFound};return nil
}

func (s Service) RotateCSRF(ctx context.Context,session Session)(string,error){
	if s.Store==nil||session.ID<=0||session.AdminID<=0{return "",errors.New("admin service is not initialized")}
	token,hash,err:=RandomToken(24);if err!=nil{return "",err};if err:=s.Store.RotateSessionCSRF(ctx,session.ID,session.AdminID,hash);err!=nil{return "",err};return token,nil
}
