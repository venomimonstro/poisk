package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMTAClientSubmitUsesSignedFixedEndpoint(t *testing.T){
	secret:=bytes.Repeat([]byte("s"),32)
	envelope:=MTAEnvelope{DeliveryID:42,IdempotencyKey:"mail-delivery-42",From:"sender@example.test",Recipient:"user@remote.test",RecipientType:"TO",Subject:"hello",BodyText:"body"}
	var seenPath string
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		seenPath=r.URL.Path
		if r.Method!=http.MethodPost{t.Fatalf("method=%s",r.Method)}
		if r.URL.RawQuery!=""{t.Fatalf("unexpected query=%q",r.URL.RawQuery)}
		body,err:=io.ReadAll(r.Body);if err!=nil{t.Fatal(err)}
		event:=r.Header.Get("X-Poisk-Gateway-Event");ts:=r.Header.Get("X-Poisk-Gateway-Timestamp");sig:=r.Header.Get("X-Poisk-Gateway-Signature")
		if event==""||ts==""||sig==""{t.Fatalf("missing gateway auth headers")}
		if _,_,err:=VerifyGatewayRequest(secret,event,ts,body,sig,time.Now().UTC());err!=nil{t.Fatalf("signature verification: %v",err)}
		var got MTAEnvelope;if err=json.Unmarshal(body,&got);err!=nil{t.Fatal(err)}
		if got.DeliveryID!=envelope.DeliveryID||got.IdempotencyKey!=envelope.IdempotencyKey||got.Recipient!=envelope.Recipient{t.Fatalf("unexpected envelope: %+v",got)}
		w.Header().Set("Content-Type","application/json")
		_,_=io.WriteString(w,`{"remote_queue_id":"Q-123"}`)
	}));defer server.Close()
	client:=MTAClient{BaseURL:server.URL+"/ignored/path?relay=evil",Secret:secret,HTTP:server.Client()}
	remote,err:=client.Submit(context.Background(),envelope);if err!=nil{t.Fatal(err)}
	if remote!="Q-123"{t.Fatalf("remote_queue_id=%q",remote)}
	if seenPath!="/ignored/path/v1/outbound"{t.Fatalf("path=%q",seenPath)}
}

func TestMTAClientRejectsWeakSecretAndInvalidBaseURL(t *testing.T){
	envelope:=MTAEnvelope{DeliveryID:1,IdempotencyKey:"delivery-1",From:"a@example.test",Recipient:"b@remote.test",RecipientType:"TO"}
	_,err:= (MTAClient{BaseURL:"https://mta.example.test",Secret:[]byte("short")}).Submit(context.Background(),envelope)
	var mtaErr *MTAError;if !errors.As(err,&mtaErr)||!mtaErr.Permanent||mtaErr.Code!="MTA_CONFIG"{t.Fatalf("weak secret err=%v",err)}
	_,err=(MTAClient{BaseURL:"file:///tmp/relay",Secret:bytes.Repeat([]byte("x"),32)}).Submit(context.Background(),envelope)
	if !errors.As(err,&mtaErr)||!mtaErr.Permanent||mtaErr.Code!="MTA_CONFIG"{t.Fatalf("invalid base err=%v",err)}
}

func TestMTAClientClassifiesRemoteFailures(t *testing.T){
	secret:=bytes.Repeat([]byte("z"),32)
	cases:=[]struct{name string;status int;permanent bool}{
		{name:"bad request",status:http.StatusBadRequest,permanent:true},
		{name:"rate limited",status:http.StatusTooManyRequests,permanent:false},
		{name:"server error",status:http.StatusBadGateway,permanent:false},
	}
	for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
		server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(tc.status);_,_=io.WriteString(w,"bounded failure")}));defer server.Close()
		_,err:=(MTAClient{BaseURL:server.URL,Secret:secret,HTTP:server.Client()}).Submit(context.Background(),MTAEnvelope{DeliveryID:7,IdempotencyKey:"delivery-7",From:"a@example.test",Recipient:"b@remote.test",RecipientType:"TO"})
		var mtaErr *MTAError;if !errors.As(err,&mtaErr){t.Fatalf("err=%v",err)}
		if mtaErr.Permanent!=tc.permanent{t.Fatalf("permanent=%v want %v",mtaErr.Permanent,tc.permanent)}
		if !strings.Contains(mtaErr.Code,"MTA_HTTP_"){t.Fatalf("code=%q",mtaErr.Code)}
	})}
}
