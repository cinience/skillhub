package auth

import (
	"strings"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	raw, prefix, hash, err := GenerateToken("")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Should have default prefix
	if !strings.HasPrefix(raw, DefaultPrefix) {
		t.Errorf("raw token %q should start with %q", raw, DefaultPrefix)
	}

	// Prefix should be the first prefixLen chars after DefaultPrefix
	expectedPrefix := raw[:len(DefaultPrefix)+prefixLen]
	if prefix != expectedPrefix {
		t.Errorf("prefix = %q, want %q", prefix, expectedPrefix)
	}

	// Hash should be non-empty 64-char hex
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}

	// Hash should match HashToken
	if got := HashToken(raw); got != hash {
		t.Errorf("HashToken(raw) = %q, want %q", got, hash)
	}
}

func TestGenerateTokenCustomPrefix(t *testing.T) {
	raw, _, _, err := GenerateToken("sk_")
	if err != nil {
		t.Fatalf("GenerateToken('sk_') error = %v", err)
	}
	if !strings.HasPrefix(raw, "sk_") {
		t.Errorf("raw token %q should start with 'sk_'", raw)
	}
}

func TestGenerateTokenUniqueness(t *testing.T) {
	raw1, _, _, _ := GenerateToken("")
	raw2, _, _, _ := GenerateToken("")
	if raw1 == raw2 {
		t.Error("two generated tokens should be different")
	}
}

func TestHashToken(t *testing.T) {
	hash := HashToken("test-token")
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}
	// Same input should produce same hash
	if hash2 := HashToken("test-token"); hash != hash2 {
		t.Error("same input should produce same hash")
	}
	// Different input should produce different hash
	if hash3 := HashToken("different-token"); hash == hash3 {
		t.Error("different input should produce different hash")
	}
}

func TestExtractPrefix(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{"normal token", "clh_abcdefghijklmnopqrstuvwxyz", "clh_abcdefghijkl"},
		{"short token", "clh_abc", "clh_abc"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPrefix(tt.token)
			if got != tt.want {
				t.Errorf("ExtractPrefix(%q) = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}
