package admin

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrForbidden = errors.New("admin action forbidden")
	ErrCSRF = errors.New("invalid csrf token")
	ErrSecondFactor = errors.New("second factor required or invalid")
)

type Service struct{Store *Repository;Secrets *SecretBox;SessionTTL time.Duration}
type LoginResult struct{SessionToken string `json:"-"`;CSRFToken string `json:"csrf_token"`;Session Session `json:"session"`}

func (s Service) Login(ctx context.Context,email,password,secondFactor,userAgent string)(LoginResult,error){
	if s.Store==nil||s.Secrets==nil{return LoginResult{},errors.New("admin service is not initialized")};admin,err:=s.Store.AdminByEmail(ctx,email);if err!=nil{return LoginResult{},err}
	if !VerifyPassword(password,admin.PasswordHash){_=s.Store.RecordLoginFailure(ctx,admin.ID);_ = s.Store.SecurityEvent(ctx,&admin.ID,"LOGIN_PASSWORD",false,"{}");return LoginResult{},ErrInvalidCredential}
	if !admin.TOTPEnabled||len(admin.TOTPSecretCipher)==0{return LoginResult{},ErrSecondFactor}
	plain,err:=s.Secrets.Decrypt(admin.TOTPSecretCipher);if err!=nil{return LoginResult{},ErrSecondFactor}
	valid:=VerifyTOTP(string(plain),secondFactor,time.Now())
	if !valid&&strings.TrimSpace(secondFactor)!=""{valid,_=s.Store.ConsumeRecoveryCode(ctx,admin.ID,RecoveryCodeHash(secondFactor))}
	if !valid{_=s.Store.RecordLoginFailure(ctx,admin.ID);_ = s.Store.SecurityEvent(ctx,&admin.ID,"LOGIN_2FA",false,"{}");return LoginResult{},ErrSecondFactor}
	if err:=s.Store.RecordLoginSuccess(ctx,admin.ID);err!=nil{return LoginResult{},err}
	token,tokenHash,err:=RandomToken(32);if err!=nil{return LoginResult{},err};csrf,csrfHash,err:=RandomToken(24);if err!=nil{return LoginResult{},err};ua:=sha256.Sum256([]byte(strings.TrimSpace(userAgent)));ttl:=s.SessionTTL;if ttl<=0{ttl=12*time.Hour}
	session,err:=s.Store.CreateSession(ctx,admin.ID,tokenHash,csrfHash,ua[:],ttl);if err!=nil{return LoginResult{},err};session.Email=admin.Email;session.Role=admin.Role;_ = s.Store.SecurityEvent(ctx,&admin.ID,"LOGIN",true,"{}")
	return LoginResult{SessionToken:token,CSRFToken:csrf,Session:session},nil
}

func (s Service) Authenticate(ctx context.Context,token string)(Session,error){if s.Store==nil||strings.TrimSpace(token)==""{return Session{},ErrSessionNotFound};return s.Store.SessionByToken(ctx,HashToken(token))}
func (s Service) RequireRole(session Session,roles ...string)error{for _,role:=range roles{if session.Role==role||session.Role=="SUPERADMIN"{return nil}};return ErrForbidden}
func (s Service) VerifyCSRF(session Session,token string)error{if len(session.CSRFHash)!=32||strings.TrimSpace(token)==""{return ErrCSRF};hash:=HashToken(token);var diff byte;for i:=range hash{diff|=hash[i]^session.CSRFHash[i]};if diff!=0{return ErrCSRF};return nil}
func (s Service) Logout(ctx context.Context,session Session)error{return s.Store.RevokeSession(ctx,session.ID,session.AdminID)}

func (s Service) SetupTOTP(ctx context.Context,adminID int64)(secret string,recovery []string,err error){
	if s.Store==nil||s.Secrets==nil||adminID<=0{return "",nil,ErrInvalidCredential};secret,err=NewTOTPSecret();if err!=nil{return};cipher,err:=s.Secrets.Encrypt([]byte(secret));if err!=nil{return "",nil,err};recovery,hashes,err:=GenerateRecoveryCodes(10);if err!=nil{return "",nil,err};if err=s.Store.SetTOTP(ctx,adminID,cipher,true);err!=nil{return "",nil,err};if err=s.Store.ReplaceRecoveryCodes(ctx,adminID,hashes);err!=nil{return "",nil,err};details,_:=json.Marshal(map[string]any{"recovery_count":len(recovery)});_ = s.Store.SecurityEvent(ctx,&adminID,"TOTP_ENABLED",true,string(details));return secret,recovery,nil
}
