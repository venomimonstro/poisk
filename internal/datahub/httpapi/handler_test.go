package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestIntParamBoundsParsing(t *testing.T){
	cases:=[]struct{raw string;def int;want int;ok bool}{
		{"",20,20,true},
		{"0",20,0,true},
		{"50",20,50,true},
		{"-1",20,-1,false},
		{"abc",20,0,false},
	}
	for _,tc:=range cases{
		r:=httptest.NewRequest("GET","/?limit="+tc.raw,nil)
		got,ok:=intParam(r,"limit",tc.def)
		if ok!=tc.ok||got!=tc.want{t.Fatalf("raw=%q got=(%d,%v) want=(%d,%v)",tc.raw,got,ok,tc.want,tc.ok)}
	}
}
