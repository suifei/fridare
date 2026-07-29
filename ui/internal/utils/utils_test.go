package utils

import (
	"testing"
	"unicode"
)

func TestIsFridaNewName_ExactlyFiveLowercaseAZ(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"abcde", true},
		{"qwxyz", true},
		{"Ab12c", false}, // uppercase + digits
		{"a1234", false}, // digits
		{"ABCDE", false}, // uppercase
		{"1abcd", false},
		{"abcd!", false},
		{"abcd", false},   // too short
		{"abcdef", false}, // too long
		{"fridare", false},
		{"", false},
	}
	for _, tc := range cases {
		got := IsFridaNewName(tc.in)
		if got != tc.want {
			t.Errorf("IsFridaNewName(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestGenerateRandomName_LengthAndCharset(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		name := GenerateRandomName()
		if len(name) != 5 {
			t.Fatalf("GenerateRandomName length=%d name=%q", len(name), name)
		}
		if !unicode.IsLetter(rune(name[0])) {
			t.Fatalf("first char must be letter: %q", name)
		}
		if !IsFridaNewName(name) {
			t.Fatalf("GenerateRandomName not valid IsFridaNewName: %q", name)
		}
		// no digits allowed (hexreplace contract)
		for _, c := range name {
			if c < 'a' || c > 'z' {
				t.Fatalf("GenerateRandomName must be [a-z]{5}, got %q", name)
			}
		}
		seen[name] = true
	}
	if len(seen) < 5 {
		t.Fatalf("expected variety across 50 samples, got %d unique: %v", len(seen), seen)
	}
}
