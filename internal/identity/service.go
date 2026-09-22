package identity

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/mail"
	"strings"
	"time"
)

type Service struct{
	Repo *Repository
	SessionTTL time.Duration
}

type LoginResult struct{
	User User
	Session Session
	SessionToken string
	CSRFToken string
}

func (s *Service) Register(ctx context.Context,email,password string)(LoginResult,error){
	if s==nil||s.Repo==nil{return LoginResult{},errors.New("identity service is not initialized")}
	email,err:=normalizeEmail(email);if err!=nil{return LoginResult{},err}
	hash,err:=HashPassword(password);if err!=nil{return LoginResult{},err}
	u,err:=s.Repo.CreateUser(ctx,email,hash);if err!=nil{return LoginResult{},err}
	return s.newSession(ctx,u)
}

func (s *Service) Login(ctx context.Context,email,password string)(LoginResult,error){
	if s==nil||s.Repo==nil{return LoginResult{},ErrUnauthorized}
	email,err:=normalizeEmail(email);if err!=nil{return LoginResult{},ErrUnauthorized}
	u,err:=s.Repo.UserByEmail(ctx,email)
	if err!=nil{
		_,_=HashPassword(password)
		return LoginResult{},ErrUnauthorized
	}
	now:=time.Now().UTC();if u.Status!="ACTIVE"||(u.LockedUntil!=nil&&u.LockedUntil.After(now)){return LoginResult{},ErrUnauthorized}
	valid,rehash:=VerifyPassword(u.PasswordHash,password)
	if !valid{_ = s.Repo.RecordLoginFailure(ctx,email);return LoginResult{},ErrUnauthorized}
	if rehash{if next,hashErr:=HashPassword(password);hashErr==nil{_ = s.Repo.UpdatePasswordHash(ctx,u.ID,next);u.PasswordHash=next}}
	if err:=s.Repo.RecordLoginSuccess(ctx,u.ID);err!=nil{return LoginResult{},err}
	return s.newSession(ctx,u)
}

func (s *Service) newSession(ctx context.Context,u User)(LoginResult,error){
	token,tokenHash,err:=RandomToken(32);if err!=nil{return LoginResult{},err}
	csrf,csrfHash,err:=RandomToken(32);if err!=nil{return LoginResult{},err}
	ttl:=s.SessionTTL;if ttl<=0{ttl=7*24*time.Hour};expires:=time.Now().UTC().Add(ttl)
	session,err:=s.Repo.CreateSession(ctx,u.ID,tokenHash,csrfHash,expires);if err!=nil{return LoginResult{},err}
	return LoginResult{User:u,Session:session,SessionToken:token,CSRFToken:csrf},nil
}

func (s *Service) Authenticate(ctx context.Context,rawToken string)(User,Session,error){
	if s==nil||s.Repo==nil||strings.TrimSpace(rawToken)==""{return User{},Session{},ErrUnauthorized}
	return s.Repo.AuthenticateSession(ctx,TokenHash(rawToken))
}

func (s *Service) RotateCSRF(ctx context.Context,u User,session Session)(string,error){
	raw,hash,err:=RandomToken(32);if err!=nil{return "",err};if err:=s.Repo.RotateCSRF(ctx,u.ID,session.ID,hash);err!=nil{return "",err};return raw,nil
}

func VerifyCSRF(session Session,raw string)bool{
	if strings.TrimSpace(raw)==""||len(session.CSRFHash)!=32{return false}
	got:=TokenHash(raw);return subtle.ConstantTimeCompare(got,session.CSRFHash)==1
}

func (s *Service) Logout(ctx context.Context,u User,session Session)error{return s.Repo.RevokeSession(ctx,u.ID,session.ID)}
func (s *Service) Sessions(ctx context.Context,u User)([]Session,error){return s.Repo.ListSessions(ctx,u.ID)}
func (s *Service) RevokeSession(ctx context.Context,u User,sessionID int64)error{if sessionID<=0{return ErrNotFound};return s.Repo.RevokeSession(ctx,u.ID,sessionID)}
func (s *Service) EnsureWebmasterProfile(ctx context.Context,u User)(int64,error){return s.Repo.EnsureWebmasterProfile(ctx,u.ID)}

func normalizeEmail(raw string)(string,error){
	email:=strings.ToLower(strings.TrimSpace(raw));if len(email)<3||len(email)>254{return "",ErrInvalidCredential}
	addr,err:=mail.ParseAddress(email);if err!=nil||strings.ToLower(addr.Address)!=email{return "",ErrInvalidCredential};return email,nil
}
