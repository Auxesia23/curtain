// Package token manages opaque session tokens (not JWTs).
//
// Tokens are used by two client types:
//   - web: sent in an HttpOnly cookie
//   - CLI: sent in `Authorization: Bearer <token>`
//
// Only the SHA-256 hash is stored in the database, so a database leak does not
// directly expose usable session tokens.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// Generate creates a random 256-bit token (unpadded base64url).
func Generate() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Hash returns the SHA-256 digest of the raw token as hexadecimal.
func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
