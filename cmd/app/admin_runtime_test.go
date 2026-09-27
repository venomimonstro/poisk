package main

import (
	"strings"
	"testing"
)

func TestReadAdminPasswordAcceptsSingleBoundedLine(t *testing.T){
	password,err:=readAdminPassword(strings.NewReader("correct horse battery staple\n"));if err!=nil{t.Fatal(err)}
	if password!="correct horse battery staple"{t.Fatalf("password=%q",password)}
}

func TestReadAdminPasswordRejectsMissingShortAndMultilineInput(t *testing.T){
	cases:=[]string{"","short\n","0123456789ab\nsecond-line\n"}
	for _,input:=range cases{if _,err:=readAdminPassword(strings.NewReader(input));err==nil{t.Fatalf("expected rejection for %q",input)}}
}
