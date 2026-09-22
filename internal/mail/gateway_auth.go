package mail

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrGatewayAuth = errors.New("mail gateway authentication failed")
	ErrGatewayReplay = errors.New("mail gateway replay detected")
	ErrGatewayMismatch = errors.New("mail gateway event mismatch")
)

const gatewayClockSkew = 5*time.Minute

func GatewayBodySHA256(body []byte) string { sum:=sha256.Sum256(body); return hex.EncodeToString(sum[:]) }

func gatewaySigningInput(eventID string,timestamp int64,bodyHash string) string {
	return "v1\n"+eventID+"\n"+strconv.FormatInt(timestamp,10)+"\n"+bodyHash
}

func SignGatewayRequest(secret []byte,eventID string,timestamp time.Time,body []byte)(string,error){
	if len(secret)<32||len(secret)>4096{return "",ErrGatewayAuth};eventID=strings.TrimSpace(eventID);if len(eventID)<16||len(eventID)>160||strings.ContainsAny(eventID,"\r\n\x00"){return "",ErrGatewayAuth};if timestamp.IsZero(){return "",ErrGatewayAuth}
	bodyHash:=GatewayBodySHA256(body);mac:=hmac.New(sha256.New,secret);_,_=mac.Write([]byte(gatewaySigningInput(eventID,timestamp.UTC().Unix(),bodyHash)));return "v1="+hex.EncodeToString(mac.Sum(nil)),nil
}

func VerifyGatewayRequest(secret []byte,eventID string,timestampRaw string,body []byte,signature string,now time.Time)(string,time.Time,error){
	if len(secret)<32||len(secret)>4096{return "",time.Time{},ErrGatewayAuth};eventID=strings.TrimSpace(eventID);signature=strings.TrimSpace(signature);timestampRaw=strings.TrimSpace(timestampRaw);if len(eventID)<16||len(eventID)>160||strings.ContainsAny(eventID,"\r\n\x00"){return "",time.Time{},ErrGatewayAuth}
	unix,err:=strconv.ParseInt(timestampRaw,10,64);if err!=nil{return "",time.Time{},ErrGatewayAuth};ts:=time.Unix(unix,0).UTC();if now.IsZero(){now=time.Now().UTC()};delta:=now.UTC().Sub(ts);if delta<0{delta=-delta};if delta>gatewayClockSkew{return "",time.Time{},ErrGatewayAuth}
	if !strings.HasPrefix(signature,"v1="){return "",time.Time{},ErrGatewayAuth};provided,err:=hex.DecodeString(strings.TrimPrefix(signature,"v1="));if err!=nil||len(provided)!=sha256.Size{return "",time.Time{},ErrGatewayAuth}
	bodyHash:=GatewayBodySHA256(body);mac:=hmac.New(sha256.New,secret);_,_=mac.Write([]byte(gatewaySigningInput(eventID,unix,bodyHash)));expected:=mac.Sum(nil);if subtle.ConstantTimeCompare(provided,expected)!=1{return "",time.Time{},ErrGatewayAuth}
	return bodyHash,ts,nil
}

func (r Repository) ClaimGatewayEvent(ctx context.Context,eventID,bodyHash string,now time.Time) error {
	if r.DB==nil{return ErrInvalid};eventID=strings.TrimSpace(eventID);bodyHash=strings.ToLower(strings.TrimSpace(bodyHash));if len(eventID)<16||len(eventID)>160||len(bodyHash)!=64{return ErrInvalid};if _,err:=hex.DecodeString(bodyHash);err!=nil{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()}
	var inserted string
	err:=r.DB.QueryRow(ctx,`INSERT INTO mail_gateway_replay_guard(event_id,body_sha256,received_at,expires_at) VALUES($1,$2,$3,$4) ON CONFLICT(event_id) DO NOTHING RETURNING event_id`,eventID,bodyHash,now.UTC(),now.UTC().Add(24*time.Hour)).Scan(&inserted)
	if err==nil{return nil};if !errors.Is(err,pgx.ErrNoRows){return err}
	var existing string;if err=r.DB.QueryRow(ctx,`SELECT body_sha256 FROM mail_gateway_replay_guard WHERE event_id=$1`,eventID).Scan(&existing);err!=nil{return err}
	if subtle.ConstantTimeCompare([]byte(existing),[]byte(bodyHash))==1{return ErrGatewayReplay};return ErrGatewayMismatch
}

func (r Repository) PruneGatewayReplayGuard(ctx context.Context,now time.Time) error {if r.DB==nil{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()};_,err:=r.DB.Exec(ctx,`DELETE FROM mail_gateway_replay_guard WHERE expires_at<$1`,now.UTC());return err}
