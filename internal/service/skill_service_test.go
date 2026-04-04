package service

import (
	"encoding/json"
	"testing"
)

func TestExtractFrontmatter_WithFrontmatter(t *testing.T) {
	content := []byte("---\ntitle: My Skill\nauthor: test\n---\n# Hello World")
	result := extractFrontmatter(content)

	var parsed map[string]string
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	raw, ok := parsed["raw"]
	if !ok {
		t.Fatal("expected 'raw' key in parsed result")
	}
	if raw == "" {
		t.Error("expected non-empty raw frontmatter")
	}
	if len(raw) == 0 {
		t.Error("frontmatter should contain content")
	}
}

func TestExtractFrontmatter_NoFrontmatter(t *testing.T) {
	content := []byte("# Just markdown\nNo frontmatter here")
	result := extractFrontmatter(content)

	if string(result) != "{}" {
		t.Errorf("got %s, want {}", string(result))
	}
}

func TestExtractFrontmatter_OnlyOpeningDelimiter(t *testing.T) {
	content := []byte("---\nsome content without closing")
	result := extractFrontmatter(content)

	if string(result) != "{}" {
		t.Errorf("got %s, want {}", string(result))
	}
}

func TestExtractFrontmatter_Empty(t *testing.T) {
	result := extractFrontmatter([]byte(""))
	if string(result) != "{}" {
		t.Errorf("got %s, want {}", string(result))
	}
}

func TestDerefStr(t *testing.T) {
	s := "hello"
	if got := derefStr(&s); got != "hello" {
		t.Errorf("derefStr(&%q) = %q", s, got)
	}
	if got := derefStr(nil); got != "" {
		t.Errorf("derefStr(nil) = %q, want empty", got)
	}
}
