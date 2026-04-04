package handler

import (
	"testing"
)

func TestSplitTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"go,python,rust", []string{"go", "python", "rust"}},
		{"go; python; rust", []string{"go", "python", "rust"}},
		{"go python rust", []string{"go", "python", "rust"}},
		{"go, python, rust", []string{"go", "python", "rust"}},
		{"  go ,  python  ", []string{"go", "python"}},
		{"single", []string{"single"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := splitTags(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitTags(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitTags(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello", "hello"},
		{"  hello  ", "hello"},
		{"\thello\t", "hello"},
		{"\nhello\n", "hello"},
		{"  ", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := trimSpace(tt.input)
		if got != tt.want {
			t.Errorf("trimSpace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSplitAny(t *testing.T) {
	got := splitAny("a,b;c d", ",; ")
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("splitAny() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("splitAny()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

