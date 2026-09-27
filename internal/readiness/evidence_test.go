package readiness

import "testing"

func TestContainsSensitiveEvidenceKey(t *testing.T){
	cases:=[]struct{name string;value map[string]any;want bool}{
		{name:"safe",value:map[string]any{"suite":"browser","metrics":map[string]any{"passed":12,"failed":0}},want:false},
		{name:"password",value:map[string]any{"password":"do-not-store"},want:true},
		{name:"smtp password",value:map[string]any{"smtp_password":"do-not-store"},want:true},
		{name:"gateway secret",value:map[string]any{"gateway-secret":"do-not-store"},want:true},
		{name:"dkim private key",value:map[string]any{"mail":map[string]any{"dkim_private_key":"do-not-store"}},want:true},
		{name:"nested token",value:map[string]any{"meta":map[string]any{"access-token":"do-not-store"}},want:true},
		{name:"array secret",value:map[string]any{"items":[]any{map[string]any{"private key":"do-not-store"}}},want:true},
		{name:"ordinary session count",value:map[string]any{"session_count":3},want:false},
		{name:"token count metric",value:map[string]any{"token_count":42},want:false},
	}
	for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){if got:=containsSensitiveEvidenceKey(tc.value);got!=tc.want{t.Fatalf("got %v want %v",got,tc.want)}})}
}
