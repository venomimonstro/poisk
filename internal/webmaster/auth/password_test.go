package auth

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil { t.Fatal(err) }
	ok, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil || !ok { t.Fatalf("verify ok=%v err=%v", ok, err) }
	ok, err = VerifyPassword(hash, "wrong password")
	if err != nil { t.Fatal(err) }
	if ok { t.Fatal("wrong password verified") }
}

func TestWeakPasswordRejected(t *testing.T) {
	if _, err := HashPassword("short"); err == nil { t.Fatal("expected weak password error") }
}

func TestOpaqueTokenHashRoundTrip(t *testing.T) {
	plain, hash, err := NewOpaqueToken()
	if err != nil { t.Fatal(err) }
	got, err := HashToken(plain)
	if err != nil { t.Fatal(err) }
	if got != hash { t.Fatal("token hash mismatch") }
}
