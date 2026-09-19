package extractor

import (
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
	"math/bits"
	"strings"
	"unicode"
)

func NormalizeContentForHash(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func ExactHash(text string) [32]byte {
	return sha256.Sum256([]byte(NormalizeContentForHash(text)))
}

func ExactHashHex(text string) string {
	h := ExactHash(text)
	return hex.EncodeToString(h[:])
}

func SimHash64(text string) uint64 {
	tokens := tokenize(text)
	if len(tokens) == 0 { return 0 }
	var vector [64]int
	for _, token := range tokens {
		h := fnv.New64a()
		_, _ = h.Write([]byte(token))
		value := h.Sum64()
		for bit := 0; bit < 64; bit++ {
			if value&(uint64(1)<<bit) != 0 { vector[bit]++ } else { vector[bit]-- }
		}
	}
	var result uint64
	for bit, score := range vector {
		if score >= 0 { result |= uint64(1) << bit }
	}
	return result
}

func HammingDistance64(a, b uint64) int { return bits.OnesCount64(a ^ b) }

func IsNearDuplicate(a, b uint64, maxDistance int) bool {
	if maxDistance < 0 { return false }
	return HammingDistance64(a, b) <= maxDistance
}

func tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}
