package query

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeWhitespaceAndCase(t *testing.T) {
	got, err := Normalize("  ПРИВЕТ   Мир  ")
	if err != nil { t.Fatal(err) }
	if got.Primary != "привет мир" { t.Fatalf("primary=%q", got.Primary) }
}

func TestKeyboardLayoutVariant(t *testing.T) {
	got, err := Normalize("ghbdtn")
	if err != nil { t.Fatal(err) }
	found := false
	for _, v := range got.Variants { if v == "привет" { found = true } }
	if !found { t.Fatalf("variants=%v", got.Variants) }
}

func TestTransliterationVariant(t *testing.T) {
	got, err := Normalize("яндекс")
	if err != nil { t.Fatal(err) }
	found := false
	for _, v := range got.Variants { if v == "yandeks" { found = true } }
	if !found { t.Fatalf("variants=%v", got.Variants) }
}

func TestVariantCountIsBounded(t *testing.T) {
	got, err := Normalize("privet привет")
	if err != nil { t.Fatal(err) }
	if len(got.Variants) > 3 { t.Fatalf("variants=%v", got.Variants) }
}

func TestRejectsEmptyAndOversizedQueries(t *testing.T) {
	if _, err := Normalize("   "); !errors.Is(err, ErrEmptyQuery) { t.Fatalf("err=%v", err) }
	if _, err := Normalize(strings.Repeat("я", MaxQueryRunes+1)); !errors.Is(err, ErrQueryTooLong) { t.Fatalf("err=%v", err) }
}
