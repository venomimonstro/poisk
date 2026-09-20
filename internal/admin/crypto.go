package admin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

var ErrInvalidCredential = errors.New("invalid credential")

type ArgonParams struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
	SaltBytes uint32
	KeyBytes  uint32
}

func DefaultArgonParams() ArgonParams {
	return ArgonParams{MemoryKiB:64*1024,Time:3,Threads:2,SaltBytes:16,KeyBytes:32}
}

func HashPassword(password string,params ArgonParams)(string,error){
	if len(password)<12||len(password)>1024{return "",ErrInvalidCredential}
	if params.MemoryKiB<16*1024||params.MemoryKiB>1024*1024||params.Time<1||params.Time>10||params.Threads<1||params.Threads>16||params.SaltBytes<16||params.SaltBytes>64||params.KeyBytes<16||params.KeyBytes>64{return "",errors.New("invalid Argon2id parameters")}
	salt:=make([]byte,params.SaltBytes);if _,err:=rand.Read(salt);err!=nil{return "",err}
	key:=argon2.IDKey([]byte(password),salt,params.Time,params.MemoryKiB,params.Threads,params.KeyBytes)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",params.MemoryKiB,params.Time,params.Threads,base64.RawStdEncoding.EncodeToString(salt),base64.RawStdEncoding.EncodeToString(key)),nil
}

func VerifyPassword(password,encoded string)bool{
	parts:=strings.Split(encoded,"$");if len(parts)!=6||parts[1]!="argon2id"||parts[2]!="v=19"{return false}
	var memory,timeCost uint32;var threads uint8
	if _,err:=fmt.Sscanf(parts[3],"m=%d,t=%d,p=%d",&memory,&timeCost,&threads);err!=nil{return false}
	if memory<16*1024||memory>1024*1024||timeCost<1||timeCost>10||threads<1||threads>16{return false}
	salt,err:=base64.RawStdEncoding.DecodeString(parts[4]);if err!=nil||len(salt)<16||len(salt)>64{return false}
	want,err:=base64.RawStdEncoding.DecodeString(parts[5]);if err!=nil||len(want)<16||len(want)>64{return false}
	got:=argon2.IDKey([]byte(password),salt,timeCost,memory,threads,uint32(len(want)))
	return subtle.ConstantTimeCompare(got,want)==1
}

func RandomToken(bytes int)(string,[]byte,error){
	if bytes<16||bytes>128{return "",nil,errors.New("invalid token size")}
	raw:=make([]byte,bytes);if _,err:=rand.Read(raw);err!=nil{return "",nil,err}
	token:=base64.RawURLEncoding.EncodeToString(raw);sum:=sha256.Sum256([]byte(token));return token,sum[:],nil
}

func HashToken(token string)[]byte{sum:=sha256.Sum256([]byte(token));return sum[:]}

func GenerateRecoveryCodes(count int)([]string,[][]byte,error){
	if count<1||count>20{return nil,nil,errors.New("invalid recovery code count")}
	codes:=make([]string,0,count);hashes:=make([][]byte,0,count)
	for i:=0;i<count;i++{raw:=make([]byte,10);if _,err:=rand.Read(raw);err!=nil{return nil,nil,err};code:=strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw));code=code[:8]+"-"+code[8:];codes=append(codes,code);sum:=sha256.Sum256([]byte(normalizeRecoveryCode(code)));hash:=make([]byte,len(sum));copy(hash,sum[:]);hashes=append(hashes,hash)}
	return codes,hashes,nil
}
func normalizeRecoveryCode(code string)string{return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code),"-",""))}
func RecoveryCodeHash(code string)[]byte{sum:=sha256.Sum256([]byte(normalizeRecoveryCode(code)));return sum[:]}

func NewTOTPSecret()(string,error){raw:=make([]byte,20);if _,err:=rand.Read(raw);err!=nil{return "",err};return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw),nil}

func VerifyTOTP(secret,code string,now time.Time)bool{
	code=strings.TrimSpace(code);if len(code)!=6{return false};if _,err:=strconv.Atoi(code);err!=nil{return false}
	secretBytes,err:=base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret)," ","")));if err!=nil||len(secretBytes)<10{return false}
	counter:=now.Unix()/30
	for offset:=int64(-1);offset<=1;offset++{if subtle.ConstantTimeCompare([]byte(totpCode(secretBytes,counter+offset)),[]byte(code))==1{return true}}
	return false
}
func totpCode(secret []byte,counter int64)string{
	var msg [8]byte;value:=uint64(counter);for i:=7;i>=0;i--{msg[i]=byte(value);value>>=8}
	mac:=hmac.New(sha1.New,secret);_,_=mac.Write(msg[:]);sum:=mac.Sum(nil);offset:=sum[len(sum)-1]&0x0f;binary:=uint32(sum[offset]&0x7f)<<24|uint32(sum[offset+1])<<16|uint32(sum[offset+2])<<8|uint32(sum[offset+3]);return fmt.Sprintf("%06d",binary%1000000)
}

type SecretBox struct{aead cipher.AEAD}
func NewSecretBox(key []byte)(*SecretBox,error){if len(key)!=32{return nil,errors.New("admin secret key must be 32 bytes")};block,err:=aes.NewCipher(key);if err!=nil{return nil,err};aead,err:=cipher.NewGCM(block);if err!=nil{return nil,err};return &SecretBox{aead:aead},nil}
func (b *SecretBox) Encrypt(plaintext []byte)([]byte,error){if b==nil||b.aead==nil{return nil,errors.New("secret box is not initialized")};nonce:=make([]byte,b.aead.NonceSize());if _,err:=rand.Read(nonce);err!=nil{return nil,err};sealed:=b.aead.Seal(nil,nonce,plaintext,nil);return append(nonce,sealed...),nil}
func (b *SecretBox) Decrypt(ciphertext []byte)([]byte,error){if b==nil||b.aead==nil{return nil,errors.New("secret box is not initialized")};n:=b.aead.NonceSize();if len(ciphertext)<=n{return nil,ErrInvalidCredential};plain,err:=b.aead.Open(nil,ciphertext[:n],ciphertext[n:],nil);if err!=nil{return nil,ErrInvalidCredential};return plain,nil}
