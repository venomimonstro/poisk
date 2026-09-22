package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil { t.Fatal(err) }
	ok, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil || !ok { t.Fatalf("verify ok=%v err=%v", ok, err) }
	ok, err = VerifyPassword(hash, "wrong password")
	if err != nil { t.Fatal(err) }
	if ok { t.Fatal("wrong password verified") }
}

func TestArgon2PasswordCompatibility(t *testing.T) {
	password := "correct horse battery staple"
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil { t.Fatal(err) }
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	hash := fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
	ok, err := VerifyPassword(hash, password)
	if err != nil || !ok { t.Fatalf("verify ok=%v err=%v", ok, err) }
	ok, err = VerifyPassword(hash, "wrong password")
	if err != nil { t.Fatal(err) }
	if ok { t.Fatal("wrong Argon2 password verified") }
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
