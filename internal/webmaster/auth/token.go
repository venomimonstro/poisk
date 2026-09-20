package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

const tokenBytes = 32

func NewOpaqueToken() (plain string, hash [32]byte, err error) {
	raw := make([]byte, tokenBytes)
	if _, err = rand.Read(raw); err != nil { return "", hash, err }
	plain = base64.RawURLEncoding.EncodeToString(raw)
	hash = sha256.Sum256([]byte(plain))
	return plain, hash, nil
}

func HashToken(plain string) ([32]byte, error) {
	if plain == "" { return [32]byte{}, errors.New("token is empty") }
	return sha256.Sum256([]byte(plain)), nil
}
