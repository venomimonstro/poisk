package mail

import (
	"errors"
	"strconv"
	"testing"
	"time"
)

func TestGatewayHMACVerification(t *testing.T){
	secret:=[]byte("0123456789abcdef0123456789abcdef")
	now:=time.Unix(1770000000,0).UTC();eventID:="evt-0123456789abcdef";body:=[]byte("bounded gateway payload")
	sig,err:=SignGatewayRequest(secret,eventID,now,body);if err!=nil{t.Fatal(err)}
	hash,ts,err:=VerifyGatewayRequest(secret,eventID,strconv.FormatInt(now.Unix(),10),body,sig,now.Add(2*time.Minute));if err!=nil{t.Fatal(err)}
	if hash!=GatewayBodySHA256(body)||!ts.Equal(now){t.Fatalf("hash=%s ts=%v",hash,ts)}
	if _,_,err=VerifyGatewayRequest(secret,eventID,strconv.FormatInt(now.Unix(),10),[]byte("altered"),sig,now);!errors.Is(err,ErrGatewayAuth){t.Fatalf("altered body err=%v",err)}
	if _,_,err=VerifyGatewayRequest(secret,eventID,strconv.FormatInt(now.Unix(),10),body,sig,now.Add(6*time.Minute));!errors.Is(err,ErrGatewayAuth){t.Fatalf("stale timestamp err=%v",err)}
	wrong:=[]byte("abcdef0123456789abcdef0123456789");if _,_,err=VerifyGatewayRequest(wrong,eventID,strconv.FormatInt(now.Unix(),10),body,sig,now);!errors.Is(err,ErrGatewayAuth){t.Fatalf("wrong secret err=%v",err)}
}

func TestGatewaySigningRejectsWeakSecretAndBadEvent(t *testing.T){
	now:=time.Now().UTC();if _,err:=SignGatewayRequest([]byte("short"),"evt-0123456789abcdef",now,[]byte("x"));!errors.Is(err,ErrGatewayAuth){t.Fatalf("weak secret err=%v",err)}
	if _,err:=SignGatewayRequest([]byte("0123456789abcdef0123456789abcdef"),"short",now,[]byte("x"));!errors.Is(err,ErrGatewayAuth){t.Fatalf("short event err=%v",err)}
}
