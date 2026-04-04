package model

import (
	"testing"
)

func TestUserIsAdmin(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{"admin", true},
		{"user", false},
		{"moderator", false},
		{"", false},
	}
	for _, tt := range tests {
		u := &User{Role: tt.role}
		if got := u.IsAdmin(); got != tt.want {
			t.Errorf("User{Role:%q}.IsAdmin() = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestUserIsModerator(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{"admin", true},
		{"moderator", true},
		{"user", false},
		{"", false},
	}
	for _, tt := range tests {
		u := &User{Role: tt.role}
		if got := u.IsModerator(); got != tt.want {
			t.Errorf("User{Role:%q}.IsModerator() = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestStringArrayScan(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    StringArray
		wantErr bool
	}{
		{"nil", nil, StringArray{}, false},
		{"empty json array", "[]", StringArray{}, false},
		{"single element", `["go"]`, StringArray{"go"}, false},
		{"multiple elements", `["go","python","rust"]`, StringArray{"go", "python", "rust"}, false},
		{"elements with spaces", `["go lang","python 3"]`, StringArray{"go lang", "python 3"}, false},
		{"bytes input", []byte(`["a","b","c"]`), StringArray{"a", "b", "c"}, false},
		{"unsupported type", 42, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a StringArray
			err := a.Scan(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Scan(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(a) != len(tt.want) {
				t.Fatalf("Scan(%v) got %v (len %d), want %v (len %d)", tt.input, a, len(a), tt.want, len(tt.want))
			}
			for i := range a {
				if a[i] != tt.want[i] {
					t.Errorf("Scan(%v)[%d] = %q, want %q", tt.input, i, a[i], tt.want[i])
				}
			}
		})
	}
}

func TestStringArrayValue(t *testing.T) {
	tests := []struct {
		name string
		arr  StringArray
		want string
	}{
		{"nil", nil, "[]"},
		{"empty", StringArray{}, "[]"},
		{"single", StringArray{"go"}, `["go"]`},
		{"multiple", StringArray{"go", "python"}, `["go","python"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.arr.Value()
			if err != nil {
				t.Fatalf("Value() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Value() = %v, want %v", got, tt.want)
			}
		})
	}
}
