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

	"golang.org/x/crypto/argon2"
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
	encoded = strings.TrimSpace(encoded)
	if strings.HasPrefix(encoded, "$argon2id$") {
		return verifyArgon2ID(encoded, password)
	}
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

func verifyArgon2ID(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" { return false, ErrInvalidHash }
	var memory, timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil { return false, ErrInvalidHash }
	if memory < 16*1024 || memory > 1024*1024 || timeCost < 1 || timeCost > 10 || threads < 1 || threads > 16 { return false, ErrInvalidHash }
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 16 || len(salt) > 64 { return false, ErrInvalidHash }
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) < 16 || len(want) > 64 { return false, ErrInvalidHash }
	got := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(want)))
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
