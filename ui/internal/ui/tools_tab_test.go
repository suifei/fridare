package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fridare-gui/internal/core"
)

// newTestToolsTab builds a minimal ToolsTab that exercises the real performPatch path
// without constructing a Fyne window.
func newTestToolsTab(installPath string) *ToolsTab {
	return &ToolsTab{
		fridaInfo: &FridaInfo{
			InstallPath: installPath,
			BackupPath:  filepath.Join(installPath, "_original_backup"),
		},
		addLog:       func(string) {},
		updateStatus: func(string) {},
		hexReplacer:  core.NewHexReplacer(),
	}
}

func TestToolsTabMagicNameValidator_RejectsFridareDefault(t *testing.T) {
	// Mirrors the Validator wired in NewToolsTab UI setup
	validate := func(text string) error {
		return core.ValidateMagicName(strings.TrimSpace(text))
	}
	if err := validate("fridare"); err == nil {
		t.Fatal(`default-like "fridare" (6 chars) must fail validation`)
	}
	if err := validate("abcde"); err != nil {
		t.Fatalf("valid name rejected: %v", err)
	}
}

func TestPerformPatch_RejectsInvalidMagicNames(t *testing.T) {
	dir := t.TempDir()
	// Even with a patchable core.py, invalid magic must fail before any success path
	if err := os.WriteFile(filepath.Join(dir, "core.py"), []byte(`channel = "frida:rpc"`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tt := newTestToolsTab(dir)

	invalid := []string{"fridare", "abcdef", "ABCDE", "abc12", "ab", "", "a-bcd"}
	for _, name := range invalid {
		err := tt.performPatch(name, "27042")
		if err == nil {
			t.Fatalf("performPatch(%q) must return error, got nil", name)
		}
		// core.py must remain unpatched
		b, _ := os.ReadFile(filepath.Join(dir, "core.py"))
		if !strings.Contains(string(b), "frida:rpc") {
			t.Fatalf("core.py must not be modified after failed performPatch(%q): %s", name, b)
		}
	}
}

func TestPerformPatch_ValidMagicPatchesCorePy(t *testing.T) {
	dir := t.TempDir()
	corePath := filepath.Join(dir, "core.py")
	if err := os.WriteFile(corePath, []byte(`channel = "frida:rpc"`+"\nimport _frida\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tt := newTestToolsTab(dir)

	if err := tt.performPatch("abcde", "27042"); err != nil {
		t.Fatalf("performPatch valid name: %v", err)
	}
	b, err := os.ReadFile(corePath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "abcde:rpc") {
		t.Fatalf("expected abcde:rpc, got: %s", s)
	}
	if strings.Contains(s, "frida:rpc") {
		t.Fatalf("frida:rpc should be gone: %s", s)
	}
	// import line must stay intact (no bare frida replace)
	if !strings.Contains(s, "import _frida") {
		t.Fatalf("import _frida must be preserved: %s", s)
	}
}

func TestPatchPythonFiles_InvalidMagicDoesNotReportSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "core.py"), []byte(`"frida:rpc"`), 0644); err != nil {
		t.Fatal(err)
	}
	tt := newTestToolsTab(dir)
	var logs []string
	tt.addLog = func(m string) { logs = append(logs, m) }

	err := tt.patchPythonFiles("fridare", "27042")
	if err == nil {
		t.Fatal("patchPythonFiles(fridare) must error")
	}
	joined := strings.Join(logs, "\n")
	if strings.Contains(joined, "SUCCESS") {
		t.Fatalf("must not log SUCCESS on invalid magic: %s", joined)
	}
	if strings.Contains(joined, "未找到 frida:rpc") {
		t.Fatalf("must not mislead with missing-rpc message on invalid magic: %s", joined)
	}
}
