package identity

import (
	"testing"

	wmauth "github.com/venomimonstro/poisk/internal/webmaster/auth"
)

func TestArgonPasswordRoundTrip(t *testing.T){
	hash,err:=HashPassword("correct horse battery staple");if err!=nil{t.Fatal(err)}
	ok,rehash:=VerifyPassword(hash,"correct horse battery staple");if !ok||rehash{t.Fatalf("ok=%v rehash=%v",ok,rehash)}
	ok,_=VerifyPassword(hash,"wrong password");if ok{t.Fatal("wrong password accepted")}
}

func TestLegacyPBKDF2RequiresRehash(t *testing.T){
	hash,err:=wmauth.HashPassword("legacy password 123!");if err!=nil{t.Fatal(err)}
	ok,rehash:=VerifyPassword(hash,"legacy password 123!");if !ok||!rehash{t.Fatalf("ok=%v rehash=%v",ok,rehash)}
}

func TestRandomTokenHashMatches(t *testing.T){
	raw,hash,err:=RandomToken(32);if err!=nil{t.Fatal(err)}
	got:=TokenHash(raw);if string(got)!=string(hash){t.Fatal("token hash mismatch")}
}
