package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const (
	DefaultPrefix = "clh_"
	tokenBytes    = 32
	prefixLen     = 12
)

// GenerateToken creates a new API token with the clh_ prefix.
// Returns (rawToken, prefix, tokenHash).
func GenerateToken(prefix string) (string, string, string, error) {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generate random bytes: %w", err)
	}

	rawToken := prefix + base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
	tokenPrefix := rawToken[:len(prefix)+prefixLen]

	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	return rawToken, tokenPrefix, tokenHash, nil
}

// HashToken returns the SHA-256 hex hash of a raw token.
func HashToken(rawToken string) string {
	hash := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(hash[:])
}

// ExtractPrefix extracts the prefix portion used for fast DB lookup.
func ExtractPrefix(rawToken string) string {
	if len(rawToken) < len(DefaultPrefix)+prefixLen {
		return rawToken
	}
	return rawToken[:len(DefaultPrefix)+prefixLen]
}
