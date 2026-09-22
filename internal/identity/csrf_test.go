package identity

import "testing"

func TestVerifyCSRFRejectsMismatch(t *testing.T) {
	raw, hash, err := RandomToken(32)
	if err != nil { t.Fatal(err) }
	session := Session{CSRFHash: hash}
	if !VerifyCSRF(session, raw) { t.Fatal("valid CSRF token rejected") }
	if VerifyCSRF(session, raw+"x") { t.Fatal("mismatched CSRF token accepted") }
	if VerifyCSRF(Session{}, raw) { t.Fatal("session without CSRF hash accepted") }
}
