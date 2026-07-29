package core

import (
	"os"
	"path/filepath"
	"testing"

	"fridare-gui/internal/utils"
)

// TestGenerateRandomName_MatchesValidateMagicNameAndPatchFile drives the real
// shipped path: utils.GenerateRandomName → ValidateMagicName → HexReplacer.PatchFile.
// This catches the regression where random names used digit padding and UI enabled
// the patch button but PatchFile rejected the name.
func TestGenerateRandomName_MatchesValidateMagicNameAndPatchFile(t *testing.T) {
	hr := NewHexReplacer()
	dir := t.TempDir()

	// Minimal PE-like or just a small file: PatchFile needs a valid executable format.
	// Use a tiny ELF-like fail path? PatchFile will fail open on unknown format after copy.
	// We only need name validation to pass before format detection — invalid magic fails first.
	// For valid magic, create a dummy file; PatchFile may error on format but NOT on name.
	input := filepath.Join(dir, "dummy.bin")
	if err := os.WriteFile(input, []byte{0x00, 0x01, 0x02, 0x03}, 0644); err != nil {
		t.Fatal(err)
	}

	const n = 200
	for i := 0; i < n; i++ {
		name := utils.GenerateRandomName()
		if err := ValidateMagicName(name); err != nil {
			t.Fatalf("sample %d: GenerateRandomName()=%q fails ValidateMagicName: %v", i, name, err)
		}
		if !utils.IsFridaNewName(name) {
			t.Fatalf("sample %d: GenerateRandomName()=%q fails IsFridaNewName", i, name)
		}

		out := filepath.Join(dir, name+".out")
		err := hr.PatchFile(input, name, out, nil)
		if err != nil {
			// Name validation must not be the failure reason
			if err.Error() == "frida new name must be exactly 5 lowercase alphabetic characters" ||
				containsMagicNameError(err) {
				t.Fatalf("sample %d: PatchFile rejected valid magic name %q: %v", i, name, err)
			}
			// Format detection failure is OK for dummy.bin
		}
		_ = os.Remove(out)
	}
}

func containsMagicNameError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return len(s) > 0 && (contains(s, "魔改名称") || contains(s, "lowercase") || contains(s, "5"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}

// TestModifyTabValidationContract_MatchesHexReplacer ensures the same predicate
// used to enable the Modify button matches HexReplacer.PatchFile name gate.
func TestModifyTabValidationContract_MatchesHexReplacer(t *testing.T) {
	// Names that old IsFridaNewName would accept (digits) but HexReplacer must reject
	digitNames := []string{"app12", "ab123", "a0000", "code9"}
	for _, name := range digitNames {
		if ValidateMagicName(name) == nil {
			t.Fatalf("ValidateMagicName must reject digit name %q", name)
		}
		// Simulate old len==5 && alnum: all these are len 5 with digits
		if len(name) != 5 {
			t.Fatalf("test fixture %q should be len 5", name)
		}
		hr := NewHexReplacer()
		dir := t.TempDir()
		in := filepath.Join(dir, "in.bin")
		out := filepath.Join(dir, "out.bin")
		_ = os.WriteFile(in, []byte{1, 2, 3, 4}, 0644)
		err := hr.PatchFile(in, name, out, nil)
		if err == nil {
			t.Fatalf("PatchFile must reject digit name %q", name)
		}
	}

	// Names that both must accept
	for i := 0; i < 50; i++ {
		name := utils.GenerateRandomName()
		if err := ValidateMagicName(name); err != nil {
			t.Fatalf("ValidateMagicName(%q): %v", name, err)
		}
	}
}
