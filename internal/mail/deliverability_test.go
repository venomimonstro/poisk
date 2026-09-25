package mail

import "testing"

func TestClassifyDeliveryFailure(t *testing.T){
	cases:=[]struct{code string;want DeliveryFailureClass}{
		{"550",FailureHard},
		{"5.1.1",FailureHard},
		{"421",FailureTransient},
		{"4.2.0",FailureTransient},
		{"",FailureUnknown},
		{"timeout",FailureUnknown},
		{"399",FailureUnknown},
	}
	for _,tc:=range cases{if got:=ClassifyDeliveryFailure(tc.code);got!=tc.want{t.Fatalf("code %q: got %s want %s",tc.code,got,tc.want)}}
}

func TestDeliveryAddressHashIsNormalizedAndExact(t *testing.T){
	a,err:=deliveryAddressHash("User@Example.COM");if err!=nil{t.Fatal(err)}
	b,err:=deliveryAddressHash("User@example.com");if err!=nil{t.Fatal(err)}
	c,err:=deliveryAddressHash("Other@example.com");if err!=nil{t.Fatal(err)}
	if a!=b{t.Fatalf("domain normalization changed hash")}
	if a==c{t.Fatalf("different exact recipient produced same test hash")}
	if len(a)!=64{t.Fatalf("hash length=%d",len(a))}
}
