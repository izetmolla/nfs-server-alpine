package mcpoauth

import (
	"crypto/sha256"
	"encoding/base64"
)

// VerifyPKCE checks an RFC 7636 PKCE code_verifier against the stored challenge.
// Only S256 is accepted (required by the MCP auth spec).
func VerifyPKCE(codeVerifier, codeChallenge, method string) bool {
	if method != "S256" {
		return false
	}
	sum := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return computed == codeChallenge
}
