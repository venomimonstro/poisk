package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAdminNotFound = errors.New("admin not found")
	ErrAdminDisabled = errors.New("admin disabled")
	ErrAdminLocked = errors.New("admin locked")
	ErrSessionNotFound = errors.New("admin session not found")
	ErrRecoveryInvalid = errors.New("recovery code invalid")
)

type Repository struct{db *pgxpool.Pool}
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type Admin struct{ID int64;Email string;PasswordHash string;Role string;Status string;TOTPSecretCipher []byte;TOTPEnabled bool;FailedLogins int;LockedUntil *time.Time}
type Session struct{ID int64;AdminID int64;Email string;Role string;CSRFHash []byte;ExpiresAt time.Time}

func (r *Repository) CreateAdmin(ctx context.Context,email,passwordHash,role string)(Admin,error){
	if r==nil||r.db==nil{return Admin{},errors.New("admin repository is not initialized")};email=strings.ToLower(strings.TrimSpace(email));role=strings.ToUpper(strings.TrimSpace(role));if email==""||len(email)>320||passwordHash==""||!ValidRole(role){return Admin{},ErrInvalidCredential}
	var out Admin;err:=r.db.QueryRow(ctx,`INSERT INTO admin_users(email,password_hash,role) VALUES($1,$2,$3)
RETURNING admin_id,email,password_hash,role,status,totp_secret_cipher,totp_enabled,failed_logins,locked_until`,email,passwordHash,role).Scan(&out.ID,&out.Email,&out.PasswordHash,&out.Role,&out.Status,&out.TOTPSecretCipher,&out.TOTPEnabled,&out.FailedLogins,&out.LockedUntil)
	if err!=nil{return Admin{},fmt.Errorf("create admin: %w",err)};return out,nil
}

func (r *Repository) AdminByEmail(ctx context.Context,email string)(Admin,error){
	if r==nil||r.db==nil{return Admin{},errors.New("admin repository is not initialized")};var out Admin;err:=r.db.QueryRow(ctx,`SELECT admin_id,email,password_hash,role,status,totp_secret_cipher,totp_enabled,failed_logins,locked_until FROM admin_users WHERE lower(email)=lower($1)`,strings.TrimSpace(email)).Scan(&out.ID,&out.Email,&out.PasswordHash,&out.Role,&out.Status,&out.TOTPSecretCipher,&out.TOTPEnabled,&out.FailedLogins,&out.LockedUntil);if errors.Is(err,pgx.ErrNoRows){return Admin{},ErrAdminNotFound};if err!=nil{return Admin{},err};if out.Status!="ACTIVE"{return Admin{},ErrAdminDisabled};if out.LockedUntil!=nil&&out.LockedUntil.After(time.Now()){return Admin{},ErrAdminLocked};return out,nil
}

func (r *Repository) RecordLoginFailure(ctx context.Context,adminID int64)error{
	_,err:=r.db.Exec(ctx,`UPDATE admin_users SET failed_logins=failed_logins+1,
locked_until=CASE WHEN failed_logins+1>=5 THEN now()+interval '15 minutes' ELSE locked_until END,updated_at=now() WHERE admin_id=$1`,adminID);return err
}
func (r *Repository) RecordLoginSuccess(ctx context.Context,adminID int64)error{_,err:=r.db.Exec(ctx,`UPDATE admin_users SET failed_logins=0,locked_until=NULL,last_login_at=now(),updated_at=now() WHERE admin_id=$1`,adminID);return err}

func (r *Repository) SetTOTP(ctx context.Context,adminID int64,cipher []byte,enabled bool)error{if adminID<=0||len(cipher)==0{return ErrInvalidCredential};_,err:=r.db.Exec(ctx,`UPDATE admin_users SET totp_secret_cipher=$2,totp_enabled=$3,updated_at=now() WHERE admin_id=$1`,adminID,cipher,enabled);return err}
func (r *Repository) ReplaceRecoveryCodes(ctx context.Context,adminID int64,hashes [][]byte)error{
	if len(hashes)<1||len(hashes)>20{return ErrInvalidCredential};tx,err:=r.db.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();if _,err:=tx.Exec(ctx,`DELETE FROM admin_recovery_codes WHERE admin_id=$1`,adminID);err!=nil{return err};for _,hash:=range hashes{if len(hash)!=32{return ErrInvalidCredential};if _,err:=tx.Exec(ctx,`INSERT INTO admin_recovery_codes(admin_id,code_hash) VALUES($1,$2)`,adminID,hash);err!=nil{return err}};return tx.Commit(ctx)
}
func (r *Repository) ConsumeRecoveryCode(ctx context.Context,adminID int64,hash []byte)(bool,error){tag,err:=r.db.Exec(ctx,`UPDATE admin_recovery_codes SET used_at=now() WHERE admin_id=$1 AND code_hash=$2 AND used_at IS NULL`,adminID,hash);if err!=nil{return false,err};return tag.RowsAffected()==1,nil}

func (r *Repository) CreateSession(ctx context.Context,adminID int64,tokenHash,csrfHash,uaHash []byte,ttl time.Duration)(Session,error){
	if adminID<=0||len(tokenHash)!=32||len(csrfHash)!=32||ttl<5*time.Minute||ttl>7*24*time.Hour{return Session{},ErrInvalidCredential};var out Session;err:=r.db.QueryRow(ctx,`INSERT INTO admin_sessions(admin_id,token_hash,csrf_hash,user_agent_hash,expires_at) VALUES($1,$2,$3,$4,now()+$5::interval)
RETURNING session_id,admin_id,csrf_hash,expires_at`,adminID,tokenHash,csrfHash,nullableHash(uaHash),ttl.String()).Scan(&out.ID,&out.AdminID,&out.CSRFHash,&out.ExpiresAt);if err!=nil{return Session{},err};return out,nil
}
func (r *Repository) SessionByToken(ctx context.Context,tokenHash []byte)(Session,error){
	if len(tokenHash)!=32{return Session{},ErrSessionNotFound};var out Session;err:=r.db.QueryRow(ctx,`SELECT s.session_id,s.admin_id,u.email,u.role,s.csrf_hash,s.expires_at FROM admin_sessions s JOIN admin_users u ON u.admin_id=s.admin_id
WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND u.status='ACTIVE'`,tokenHash).Scan(&out.ID,&out.AdminID,&out.Email,&out.Role,&out.CSRFHash,&out.ExpiresAt);if errors.Is(err,pgx.ErrNoRows){return Session{},ErrSessionNotFound};if err!=nil{return Session{},err};_,_=r.db.Exec(ctx,`UPDATE admin_sessions SET last_seen_at=now() WHERE session_id=$1 AND last_seen_at<now()-interval '5 minutes'`,out.ID);return out,nil
}
func (r *Repository) RevokeSession(ctx context.Context,sessionID,adminID int64)error{tag,err:=r.db.Exec(ctx,`UPDATE admin_sessions SET revoked_at=now() WHERE session_id=$1 AND admin_id=$2 AND revoked_at IS NULL`,sessionID,adminID);if err!=nil{return err};if tag.RowsAffected()==0{return ErrSessionNotFound};return nil}
func (r *Repository) RevokeAllSessions(ctx context.Context,adminID int64)error{_,err:=r.db.Exec(ctx,`UPDATE admin_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE admin_id=$1`,adminID);return err}
func (r *Repository) SecurityEvent(ctx context.Context,adminID *int64,eventType string,success bool,details string)error{eventType=strings.TrimSpace(eventType);if eventType==""||len(eventType)>96{return ErrInvalidCredential};if details==""{details="{}"};_,err:=r.db.Exec(ctx,`INSERT INTO admin_security_events(admin_id,event_type,success,details) VALUES($1,$2,$3,$4::jsonb)`,adminID,eventType,success,details);return err}

func ValidRole(role string)bool{switch role{case "SUPERADMIN","OPERATOR","ANALYST","SUPPORT":return true};return false}
func nullableHash(value []byte)any{if len(value)==0{return nil};return value}
