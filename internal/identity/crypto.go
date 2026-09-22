package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	wmauth "github.com/venomimonstro/poisk/internal/webmaster/auth"
	"golang.org/x/crypto/argon2"
)

var (
	ErrWeakPassword = errors.New("password does not meet minimum requirements")
	ErrInvalidCredential = errors.New("invalid credential")
)

const (
	argonMemoryKiB uint32 = 64 * 1024
	argonTime uint32 = 3
	argonThreads uint8 = 2
	argonSaltBytes = 16
	argonKeyBytes = 32
)

func HashPassword(password string)(string,error){
	if len(password)<12||len(password)>256{return "",ErrWeakPassword}
	salt:=make([]byte,argonSaltBytes);if _,err:=rand.Read(salt);err!=nil{return "",err}
	key:=argon2.IDKey([]byte(password),salt,argonTime,argonMemoryKiB,argonThreads,argonKeyBytes)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",argonMemoryKiB,argonTime,argonThreads,base64.RawStdEncoding.EncodeToString(salt),base64.RawStdEncoding.EncodeToString(key)),nil
}

func VerifyPassword(encoded,password string)(valid,needsRehash bool){
	encoded=strings.TrimSpace(encoded)
	if strings.HasPrefix(encoded,"pbkdf2-sha256$"){
		ok,err:=wmauth.VerifyPassword(encoded,password);return err==nil&&ok,err==nil&&ok
	}
	parts:=strings.Split(encoded,"$");if len(parts)!=6||parts[1]!="argon2id"||parts[2]!="v=19"{return false,false}
	var memory,timeCost uint32;var threads uint8
	if _,err:=fmt.Sscanf(parts[3],"m=%d,t=%d,p=%d",&memory,&timeCost,&threads);err!=nil{return false,false}
	if memory<16*1024||memory>1024*1024||timeCost<1||timeCost>10||threads<1||threads>16{return false,false}
	salt,err:=base64.RawStdEncoding.DecodeString(parts[4]);if err!=nil||len(salt)<16||len(salt)>64{return false,false}
	want,err:=base64.RawStdEncoding.DecodeString(parts[5]);if err!=nil||len(want)<16||len(want)>64{return false,false}
	got:=argon2.IDKey([]byte(password),salt,timeCost,memory,threads,uint32(len(want)))
	ok:=subtle.ConstantTimeCompare(got,want)==1
	needs:=ok&&(memory!=argonMemoryKiB||timeCost!=argonTime||threads!=argonThreads||len(want)!=argonKeyBytes)
	return ok,needs
}

func RandomToken(bytes int)(raw string,hash []byte,err error){
	if bytes<16||bytes>128{return "",nil,errors.New("invalid token size")}
	buf:=make([]byte,bytes);if _,err=rand.Read(buf);err!=nil{return "",nil,err}
	raw=base64.RawURLEncoding.EncodeToString(buf);sum:=sha256.Sum256([]byte(raw));hash=make([]byte,len(sum));copy(hash,sum[:]);return raw,hash,nil
}
func TokenHash(raw string)[]byte{sum:=sha256.Sum256([]byte(strings.TrimSpace(raw)));out:=make([]byte,len(sum));copy(out,sum[:]);return out}
