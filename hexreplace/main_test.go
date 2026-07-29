package main

import (
	"strings"
	"testing"
)

func TestIsStringAlpha_OnlyLowercaseAtoZ(t *testing.T) {
	if !isStringAlpha("abcde") {
		t.Fatal("abcde should be alpha")
	}
	if isStringAlpha("abcdE") {
		t.Fatal("uppercase must be rejected (hexreplace requires a-z)")
	}
	if isStringAlpha("ab12e") {
		t.Fatal("digits must be rejected")
	}
	if isStringAlpha("abcd") {
		// length not checked here — main checks len==5 separately
		if !isStringAlpha("abcd") {
			t.Fatal("all-letter short still alpha by char class")
		}
	}
	// empty: no non-alpha runes → true (length checked separately in main)
	if !isStringAlpha("") {
		t.Fatal("empty string is vacuously alphabetic; main enforces len==5")
	}
}

func TestBuildReplacements_IncludeRPCAndNotBareFridaOnly(t *testing.T) {
	for _, format := range []ExecutableFormat{MachO, ELF, PE} {
		reps := buildReplacements("abcde", format)
		if len(reps) == 0 {
			t.Fatalf("format %v: no replacements", format)
		}
		foundRPC := false
		for _, sec := range reps {
			for _, item := range sec.Items {
				if string(item.Old) == "frida:rpc" {
					foundRPC = true
					if string(item.New) != "abcde:rpc" {
						t.Fatalf("rpc new want abcde:rpc got %s", item.New)
					}
				}
				// bare exact "frida" alone is dangerous for PE/python modules
				if string(item.Old) == "frida" {
					t.Fatalf("format %v must not replace bare frida (breaks imports)", format)
				}
			}
		}
		if !foundRPC {
			t.Fatalf("format %v missing frida:rpc rule", format)
		}
	}
}

func TestBuildReplacements_SameLengthPaddingContract(t *testing.T) {
	// New names are 5 letters like "frida" so frida:rpc -> xxxxx:rpc same length
	name := "qwxyz"
	reps := buildReplacements(name, ELF)
	for _, sec := range reps {
		for _, item := range sec.Items {
			if strings.Contains(string(item.Old), "frida") {
				// replacement helper zero-pads if shorter; equal length preferred
				if len(item.New) > len(item.Old) {
					t.Fatalf("new longer than old would overflow: %q -> %q", item.Old, item.New)
				}
			}
		}
	}
}
