package webmaster

import (
	"context"
	"errors"
	"net/mail"
	"net/url"
	"strings"
	"time"

	crawlersecurity "github.com/venomimonstro/poisk/internal/crawler/security"
	"github.com/venomimonstro/poisk/internal/crawler/urlnorm"
	wmauth "github.com/venomimonstro/poisk/internal/webmaster/auth"
)

type Store interface {
	CreateUser(context.Context,string,string)(User,error)
	UserByEmail(context.Context,string)(User,error)
	CreateSession(context.Context,int64,[32]byte,time.Time) error
	UserBySession(context.Context,[32]byte)(User,error)
	DeleteSession(context.Context,int64,[32]byte) error
	AddSite(context.Context,int64,SiteOrigin)(Site,error)
	ListSites(context.Context,int64)([]Site,error)
	OwnedSite(context.Context,int64,int64,bool)(Site,error)
	CreateVerification(context.Context,int64,int64,string,[32]byte,string,time.Time)(Verification,error)
	MarkVerified(context.Context,int64,int64,string,[32]byte) error
	SubmitSitemap(context.Context,int64,int64,string)(int64,error)
	QueueURLRequest(context.Context,int64,int64,string,string)(int64,error)
	URLStatus(context.Context,int64,int64,string)(URLStatus,error)
	Metrics(context.Context,int64,int64,time.Time,time.Time)(Metrics,error)
}

type Service struct {
	Store           Store
	Validator       crawlersecurity.Validator
	Verifier        Verifier
	SessionTTL      time.Duration
	VerificationTTL time.Duration
}

type Session struct {
	UserID    int64     `json:"user_id"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type VerificationChallenge struct {
	Verification Verification `json:"verification"`
	Token        string       `json:"token"`
	Instruction  string       `json:"instruction"`
}

func (s *Service) Register(ctx context.Context,email,password string)(Session,error){
	if s==nil || s.Store==nil { return Session{},errors.New("webmaster service is not initialized") }
	email,err:=normalizeEmail(email); if err!=nil { return Session{},err }
	hash,err:=wmauth.HashPassword(password); if err!=nil { return Session{},err }
	user,err:=s.Store.CreateUser(ctx,email,hash); if err!=nil { return Session{},err }
	return s.newSession(ctx,user)
}

func (s *Service) Login(ctx context.Context,email,password string)(Session,error){
	if s==nil || s.Store==nil { return Session{},errors.New("webmaster service is not initialized") }
	email,err:=normalizeEmail(email); if err!=nil { return Session{},ErrUnauthorized }
	user,err:=s.Store.UserByEmail(ctx,email)
	if err!=nil {
		_,_ = wmauth.HashPassword(password)
		return Session{},ErrUnauthorized
	}
	ok,err:=wmauth.VerifyPassword(user.PasswordHash,password)
	if err!=nil || !ok || user.Status!="ACTIVE" { return Session{},ErrUnauthorized }
	return s.newSession(ctx,user)
}

func (s *Service) Authenticate(ctx context.Context,token string)(User,error){
	if s==nil || s.Store==nil { return User{},ErrUnauthorized }
	hash,err:=wmauth.HashToken(strings.TrimSpace(token)); if err!=nil { return User{},ErrUnauthorized }
	return s.Store.UserBySession(ctx,hash)
}

func (s *Service) Logout(ctx context.Context,userID int64,token string) error {
	hash,err:=wmauth.HashToken(strings.TrimSpace(token)); if err!=nil { return ErrUnauthorized }
	return s.Store.DeleteSession(ctx,userID,hash)
}

func (s *Service) Sites(ctx context.Context,userID int64)([]Site,error){ return s.Store.ListSites(ctx,userID) }

func (s *Service) AddSite(ctx context.Context,userID int64,rawOrigin string)(Site,error){
	origin,err:=ValidateSiteOrigin(ctx,s.Validator,rawOrigin); if err!=nil { return Site{},err }
	return s.Store.AddSite(ctx,userID,origin)
}

func (s *Service) BeginVerification(ctx context.Context,userID,siteID int64,method string)(VerificationChallenge,error){
	if method!=VerificationDNS && method!=VerificationFile && method!=VerificationMeta { return VerificationChallenge{},ErrVerificationFailed }
	if _,err:=s.Store.OwnedSite(ctx,userID,siteID,false); err!=nil { return VerificationChallenge{},err }
	token,hash,err:=wmauth.NewOpaqueToken(); if err!=nil { return VerificationChallenge{},err }
	ttl:=s.VerificationTTL; if ttl<=0 { ttl=30*time.Minute }
	hint:=token; if len(hint)>10 { hint=hint[:10] }
	verification,err:=s.Store.CreateVerification(ctx,userID,siteID,method,hash,hint,time.Now().UTC().Add(ttl)); if err!=nil { return VerificationChallenge{},err }
	return VerificationChallenge{Verification:verification,Token:token,Instruction:verificationInstruction(method,token)},nil
}

func (s *Service) CompleteVerification(ctx context.Context,userID,siteID int64,method,token string) error {
	site,err:=s.Store.OwnedSite(ctx,userID,siteID,false); if err!=nil { return err }
	if err:=s.Verifier.Verify(ctx,site,method,token); err!=nil { return err }
	hash,err:=wmauth.HashToken(token); if err!=nil { return err }
	return s.Store.MarkVerified(ctx,userID,siteID,method,hash)
}

func (s *Service) SubmitSitemap(ctx context.Context,userID,siteID int64,raw string)(int64,error){
	site,err:=s.Store.OwnedSite(ctx,userID,siteID,true); if err!=nil { return 0,err }
	target,err:=s.Validator.Validate(ctx,raw); if err!=nil || !sameSiteHost(target.Host,site.Host) { return 0,ErrInvalidSiteOrigin }
	normalized,err:=urlnorm.Normalize(raw); if err!=nil { return 0,err }
	return s.Store.SubmitSitemap(ctx,userID,siteID,normalized)
}

func (s *Service) SubmitURL(ctx context.Context,userID,siteID int64,raw,operation string)(int64,error){
	operation=strings.ToUpper(strings.TrimSpace(operation))
	if operation!="SUBMIT" && operation!="REINDEX" && operation!="DELETE" { return 0,errors.New("invalid URL operation") }
	site,err:=s.Store.OwnedSite(ctx,userID,siteID,true); if err!=nil { return 0,err }
	target,err:=s.Validator.Validate(ctx,raw); if err!=nil || !sameSiteHost(target.Host,site.Host) { return 0,ErrInvalidSiteOrigin }
	normalized,err:=urlnorm.Normalize(raw); if err!=nil { return 0,err }
	return s.Store.QueueURLRequest(ctx,userID,siteID,normalized,operation)
}

func (s *Service) URLStatus(ctx context.Context,userID,siteID int64,raw string)(URLStatus,error){
	if _,err:=s.Store.OwnedSite(ctx,userID,siteID,false); err!=nil { return URLStatus{},err }
	normalized,err:=urlnorm.Normalize(raw); if err!=nil { return URLStatus{},err }
	return s.Store.URLStatus(ctx,userID,siteID,normalized)
}

func (s *Service) Metrics(ctx context.Context,userID,siteID int64,from,to time.Time)(Metrics,error){
	if _,err:=s.Store.OwnedSite(ctx,userID,siteID,false); err!=nil { return Metrics{},err }
	if from.IsZero() { from=time.Now().UTC().AddDate(0,0,-30) }
	if to.IsZero() { to=time.Now().UTC() }
	if to.Before(from) || to.Sub(from)>366*24*time.Hour { return Metrics{},errors.New("invalid metrics range") }
	return s.Store.Metrics(ctx,userID,siteID,from,to)
}

func (s *Service) newSession(ctx context.Context,user User)(Session,error){
	token,hash,err:=wmauth.NewOpaqueToken(); if err!=nil { return Session{},err }
	ttl:=s.SessionTTL; if ttl<=0 { ttl=7*24*time.Hour }
	expires:=time.Now().UTC().Add(ttl)
	if err:=s.Store.CreateSession(ctx,user.ID,hash,expires); err!=nil { return Session{},err }
	return Session{UserID:user.ID,Email:user.Email,Token:token,ExpiresAt:expires},nil
}

func normalizeEmail(raw string)(string,error){
	email:=strings.ToLower(strings.TrimSpace(raw))
	if len(email)<3 || len(email)>254 { return "",errors.New("invalid email") }
	addr,err:=mail.ParseAddress(email); if err!=nil || strings.ToLower(addr.Address)!=email { return "",errors.New("invalid email") }
	return email,nil
}

func sameSiteHost(a,b string) bool { return strings.EqualFold(strings.TrimSuffix(a,"."),strings.TrimSuffix(b,".")) }

func verificationInstruction(method,token string) string {
	switch method {
	case VerificationDNS: return "Create TXT record _poisk-verification with value poisk-verification="+token
	case VerificationFile: return "Publish the token at /.well-known/poisk-verification/"+url.PathEscape(token)+".txt"
	case VerificationMeta: return "<meta name=\"poisk-verification\" content=\""+token+"\">"
	default: return ""
	}
}
