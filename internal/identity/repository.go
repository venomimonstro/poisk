package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrConflict = errors.New("conflict")
	ErrNotFound = errors.New("not found")
	ErrLocked = errors.New("account temporarily locked")
)

type User struct {
	ID int64 `json:"user_id"`
	Email string `json:"email"`
	Status string `json:"status"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	PasswordHash string `json:"-"`
	LockedUntil *time.Time `json:"-"`
}

type Session struct {
	ID int64 `json:"session_id"`
	UserID int64 `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt time.Time `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CSRFHash []byte `json:"-"`
}

type Repository struct{db *pgxpool.Pool}
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

func (r *Repository) CreateUser(ctx context.Context,email,passwordHash string)(User,error){
	if r==nil||r.db==nil{return User{},errors.New("identity database unavailable")}
	email=strings.ToLower(strings.TrimSpace(email))
	var u User
	err:=r.db.QueryRow(ctx,`INSERT INTO consumer_users(email,password_hash,status) VALUES($1,$2,'ACTIVE') RETURNING user_id,email,status,email_verified_at,password_hash,locked_until`,email,passwordHash).Scan(&u.ID,&u.Email,&u.Status,&u.EmailVerifiedAt,&u.PasswordHash,&u.LockedUntil)
	if err!=nil{if isUnique(err){return User{},ErrConflict};return User{},err}
	_,_=r.db.Exec(ctx,`INSERT INTO consumer_security_events(user_id,event_type,details) VALUES($1,'REGISTER','{}'::jsonb)`,u.ID)
	return u,nil
}

func (r *Repository) UserByEmail(ctx context.Context,email string)(User,error){
	if r==nil||r.db==nil{return User{},ErrUnauthorized}
	var u User
	err:=r.db.QueryRow(ctx,`SELECT user_id,email,status,email_verified_at,password_hash,locked_until FROM consumer_users WHERE lower(email)=lower($1)`,strings.TrimSpace(email)).Scan(&u.ID,&u.Email,&u.Status,&u.EmailVerifiedAt,&u.PasswordHash,&u.LockedUntil)
	if errors.Is(err,pgx.ErrNoRows){return User{},ErrNotFound};return u,err
}

func (r *Repository) UpdatePasswordHash(ctx context.Context,userID int64,hash string)error{
	if r==nil||r.db==nil||userID<=0||strings.TrimSpace(hash)==""{return ErrNotFound}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	tag,err:=tx.Exec(ctx,`UPDATE consumer_users SET password_hash=$2,updated_at=now() WHERE user_id=$1`,userID,hash);if err!=nil{return err};if tag.RowsAffected()!=1{return ErrNotFound}
	if _,err=tx.Exec(ctx,`UPDATE webmaster_users SET password_hash=$2,updated_at=now() WHERE consumer_user_id=$1`,userID,hash);err!=nil{return err}
	return tx.Commit(ctx)
}

func (r *Repository) RecordLoginFailure(ctx context.Context,email string)error{
	_,err:=r.db.Exec(ctx,`UPDATE consumer_users SET
 failed_login_count=LEAST(20,failed_login_count+1),
 locked_until=CASE WHEN failed_login_count+1>=5 THEN GREATEST(COALESCE(locked_until,now()),now()+interval '15 minutes') ELSE locked_until END,
 updated_at=now()
WHERE lower(email)=lower($1)`,strings.TrimSpace(email))
	if err==nil{_,_=r.db.Exec(ctx,`INSERT INTO consumer_security_events(user_id,event_type,details) SELECT user_id,'LOGIN_FAILED','{}'::jsonb FROM consumer_users WHERE lower(email)=lower($1)`,strings.TrimSpace(email))}
	return err
}

func (r *Repository) RecordLoginSuccess(ctx context.Context,userID int64)error{
	_,err:=r.db.Exec(ctx,`UPDATE consumer_users SET failed_login_count=0,locked_until=NULL,updated_at=now() WHERE user_id=$1`,userID)
	if err==nil{_,_=r.db.Exec(ctx,`INSERT INTO consumer_security_events(user_id,event_type,details) VALUES($1,'LOGIN_OK','{}'::jsonb)`,userID)}
	return err
}

func (r *Repository) CreateSession(ctx context.Context,userID int64,tokenHash,csrfHash []byte,expires time.Time)(Session,error){
	if len(tokenHash)!=32||len(csrfHash)!=32||userID<=0{return Session{},ErrUnauthorized}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return Session{},err};defer func(){_=tx.Rollback(ctx)}()
	var status string;var locked *time.Time
	if err=tx.QueryRow(ctx,`SELECT status,locked_until FROM consumer_users WHERE user_id=$1 FOR UPDATE`,userID).Scan(&status,&locked);err!=nil{return Session{},ErrUnauthorized}
	if status!="ACTIVE"||(locked!=nil&&locked.After(time.Now().UTC())){return Session{},ErrLocked}
	var s Session
	err=tx.QueryRow(ctx,`INSERT INTO consumer_sessions(user_id,token_hash,csrf_hash,expires_at) VALUES($1,$2,$3,$4) RETURNING session_id,user_id,expires_at,last_seen_at,created_at,revoked_at`,userID,tokenHash,csrfHash,expires).Scan(&s.ID,&s.UserID,&s.ExpiresAt,&s.LastSeenAt,&s.CreatedAt,&s.RevokedAt);if err!=nil{return Session{},err}
	_,err=tx.Exec(ctx,`UPDATE consumer_sessions SET revoked_at=now() WHERE session_id IN (
 SELECT session_id FROM consumer_sessions WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>now() ORDER BY created_at DESC,session_id DESC OFFSET 8
)`,userID);if err!=nil{return Session{},err}
	if err=tx.Commit(ctx);err!=nil{return Session{},err};return s,nil
}

func (r *Repository) AuthenticateSession(ctx context.Context,tokenHash []byte)(User,Session,error){
	if len(tokenHash)!=32{return User{},Session{},ErrUnauthorized}
	var u User;var s Session
	err:=r.db.QueryRow(ctx,`SELECT u.user_id,u.email,u.status,u.email_verified_at,u.password_hash,u.locked_until,
 s.session_id,s.user_id,s.expires_at,s.last_seen_at,s.created_at,s.revoked_at,s.csrf_hash
FROM consumer_sessions s JOIN consumer_users u ON u.user_id=s.user_id
WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND u.status='ACTIVE'`,tokenHash).Scan(&u.ID,&u.Email,&u.Status,&u.EmailVerifiedAt,&u.PasswordHash,&u.LockedUntil,&s.ID,&s.UserID,&s.ExpiresAt,&s.LastSeenAt,&s.CreatedAt,&s.RevokedAt,&s.CSRFHash)
	if err!=nil{return User{},Session{},ErrUnauthorized}
	_,_=r.db.Exec(ctx,`UPDATE consumer_sessions SET last_seen_at=now() WHERE session_id=$1 AND last_seen_at<now()-interval '5 minutes'`,s.ID)
	return u,s,nil
}

func (r *Repository) RotateCSRF(ctx context.Context,userID,sessionID int64,csrfHash []byte)error{
	if len(csrfHash)!=32{return ErrUnauthorized};tag,err:=r.db.Exec(ctx,`UPDATE consumer_sessions SET csrf_hash=$3 WHERE session_id=$1 AND user_id=$2 AND revoked_at IS NULL AND expires_at>now()`,sessionID,userID,csrfHash);if err!=nil{return err};if tag.RowsAffected()!=1{return ErrUnauthorized};return nil
}
func (r *Repository) RevokeSession(ctx context.Context,userID,sessionID int64)error{tag,err:=r.db.Exec(ctx,`UPDATE consumer_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE session_id=$1 AND user_id=$2`,sessionID,userID);if err!=nil{return err};if tag.RowsAffected()!=1{return ErrNotFound};_,_=r.db.Exec(ctx,`INSERT INTO consumer_security_events(user_id,event_type,details) VALUES($1,'SESSION_REVOKED',jsonb_build_object('session_id',$2))`,userID,sessionID);return nil}
func (r *Repository) ListSessions(ctx context.Context,userID int64)([]Session,error){rows,err:=r.db.Query(ctx,`SELECT session_id,user_id,expires_at,last_seen_at,created_at,revoked_at FROM consumer_sessions WHERE user_id=$1 ORDER BY created_at DESC,session_id DESC LIMIT 50`,userID);if err!=nil{return nil,err};defer rows.Close();out:=make([]Session,0,8);for rows.Next(){var s Session;if err:=rows.Scan(&s.ID,&s.UserID,&s.ExpiresAt,&s.LastSeenAt,&s.CreatedAt,&s.RevokedAt);err!=nil{return nil,err};out=append(out,s)};return out,rows.Err()}

func (r *Repository) EnsureWebmasterProfile(ctx context.Context,consumerID int64)(int64,error){
	var id int64
	err:=r.db.QueryRow(ctx,`SELECT user_id FROM webmaster_users WHERE consumer_user_id=$1`,consumerID).Scan(&id);if err==nil{return id,nil};if !errors.Is(err,pgx.ErrNoRows){return 0,err}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return 0,err};defer func(){_=tx.Rollback(ctx)}()
	var email,hash,status string
	if err=tx.QueryRow(ctx,`SELECT email,password_hash,status FROM consumer_users WHERE user_id=$1 FOR UPDATE`,consumerID).Scan(&email,&hash,&status);err!=nil{return 0,ErrNotFound}
	wmStatus:="ACTIVE";if status!="ACTIVE"{wmStatus="DISABLED"}
	err=tx.QueryRow(ctx,`INSERT INTO webmaster_users(email,password_hash,status,consumer_user_id) VALUES($1,$2,$3,$4) ON CONFLICT(consumer_user_id) DO UPDATE SET password_hash=EXCLUDED.password_hash,status=EXCLUDED.status,updated_at=now() RETURNING user_id`,email,hash,wmStatus,consumerID).Scan(&id);if err!=nil{return 0,err}
	if err=tx.Commit(ctx);err!=nil{return 0,err};return id,nil
}

func isUnique(err error)bool{var pgErr *pgconn.PgError;return errors.As(err,&pgErr)&&pgErr.Code=="23505"}