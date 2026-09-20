package httpapi

import "testing"

func TestOriginAllowedExactOriginOnly(t *testing.T){
	want:="https://shop.example"
	valid:=[]string{"https://shop.example","HTTPS://SHOP.EXAMPLE"}
	for _,value:=range valid{if !originAllowed(value,want){t.Fatalf("valid origin rejected: %s",value)}}
	invalid:=[]string{"https://evil.example","http://shop.example","https://shop.example.evil","https://shop.example/path","null",""}
	for _,value:=range invalid{if originAllowed(value,want){t.Fatalf("invalid origin accepted: %s",value)}}
}
