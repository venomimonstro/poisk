package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	passwordIterations = 210000
	passwordSaltBytes  = 16
	passwordKeyBytes   = 32
	passwordScheme     = "pbkdf2-sha256"
)

var (
	ErrWeakPassword = errors.New("password does not meet minimum requirements")
	ErrInvalidHash  = errors.New("invalid password hash")
)

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 256 { return "", ErrWeakPassword }
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil { return "", fmt.Errorf("generate password salt: %w", err) }
	key := pbkdf2SHA256([]byte(password), salt, passwordIterations, passwordKeyBytes)
	return fmt.Sprintf("%s$%d$%s$%s", passwordScheme, passwordIterations,
		base64.RawURLEncoding.EncodeToString(salt), base64.RawURLEncoding.EncodeToString(key)), nil
}

func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordScheme { return false, ErrInvalidHash }
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 100000 || iterations > 1000000 { return false, ErrInvalidHash }
	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < 16 || len(salt) > 64 { return false, ErrInvalidHash }
	want, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil || len(want) != passwordKeyBytes { return false, ErrInvalidHash }
	got := pbkdf2SHA256([]byte(password), salt, iterations, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hLen := sha256.Size
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t { t[j] ^= u[j] }
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}
