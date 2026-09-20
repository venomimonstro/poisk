package admin

import (
	"bytes"
	"encoding/base32"
	"testing"
	"time"
)

func TestPasswordHashAndVerify(t *testing.T){
	params:=ArgonParams{MemoryKiB:16*1024,Time:1,Threads:1,SaltBytes:16,KeyBytes:32}
	hash,err:=HashPassword("correct horse battery staple",params);if err!=nil{t.Fatal(err)}
	if !VerifyPassword("correct horse battery staple",hash){t.Fatal("expected password to verify")}
	if VerifyPassword("wrong password value",hash){t.Fatal("wrong password verified")}
	if _,err:=HashPassword("short",params);err==nil{t.Fatal("short password must fail")}
}

func TestSecretBoxRejectsTampering(t *testing.T){
	key:=bytes.Repeat([]byte{7},32);box,err:=NewSecretBox(key);if err!=nil{t.Fatal(err)}
	ciphertext,err:=box.Encrypt([]byte("JBSWY3DPEHPK3PXP"));if err!=nil{t.Fatal(err)}
	plain,err:=box.Decrypt(ciphertext);if err!=nil{t.Fatal(err)};if string(plain)!="JBSWY3DPEHPK3PXP"{t.Fatalf("plain=%q",plain)}
	ciphertext[len(ciphertext)-1]^=0x01
	if _,err:=box.Decrypt(ciphertext);err==nil{t.Fatal("tampered ciphertext must fail")}
}

func TestRecoveryCodeNormalization(t *testing.T){
	codes,hashes,err:=GenerateRecoveryCodes(2);if err!=nil{t.Fatal(err)}
	if len(codes)!=2||len(hashes)!=2{t.Fatalf("codes=%d hashes=%d",len(codes),len(hashes))}
	variant:=codes[0]
	if got,want:=RecoveryCodeHash(variant),hashes[0];!bytes.Equal(got,want){t.Fatal("recovery hash mismatch")}
}

func TestTOTPWindow(t *testing.T){
	secretBytes:=[]byte("01234567890123456789")
	secret:=base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)
	now:=time.Unix(1_700_000_000,0)
	code:=totpCode(secretBytes,now.Unix()/30)
	if !VerifyTOTP(secret,code,now){t.Fatal("current TOTP must verify")}
	if VerifyTOTP(secret,code,now.Add(2*time.Minute)){t.Fatal("expired TOTP must fail")}
}
