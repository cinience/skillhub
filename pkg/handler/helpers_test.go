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


