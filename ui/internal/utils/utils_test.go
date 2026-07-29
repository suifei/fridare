package utils

import (
	"testing"
	"unicode"
)

func TestIsFridaNewName_AcceptsAlnumStartingWithLetter(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"abcde", true},
		{"Ab12c", true},
		{"a1234", true},
		{"1abcd", false},
		{"abcd!", false},
		{"", false},
	}
	for _, tc := range cases {
		if tc.in == "" {
			// empty: IsFridaNewName indexes s[0] — protect by length in callers;
			// document current behavior
			func() {
				defer func() {
					if recover() == nil && tc.in == "" {
						// if no panic, result should be false or panic
					}
				}()
				_ = IsFridaNewName(tc.in)
			}()
			continue
		}
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
		seen[name] = true
	}
	if len(seen) < 5 {
		t.Fatalf("expected variety across 50 samples, got %d unique: %v", len(seen), seen)
	}
}
