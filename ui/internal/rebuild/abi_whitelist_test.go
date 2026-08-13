package rebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsInjectionABIPath(t *testing.T) {
	on := []string{
		"subprojects/frida-gum/gum/backend-linux/gumprocess-linux.c",
		"subprojects/frida-gum/gum/backend-windows/gumprocess-windows.c",
		"subprojects/frida-core/src/linux/linjector-glue.c",
		"subprojects/frida-core/lib/agent/agent.vala",
		"subprojects/frida-core/lib/agent/agent-glue.c",
	}
	off := []string{
		"subprojects/frida-core/lib/base/session.vala",
		"subprojects/frida-core/meson.build",
		"README.md",
	}
	for _, p := range on {
		if !IsInjectionABIPath(p) {
			t.Fatalf("want on-list: %s", p)
		}
	}
	for _, p := range off {
		if IsInjectionABIPath(p) {
			t.Fatalf("want off-list: %s", p)
		}
	}
}

func TestApplyInjectionABIRename_OnOffList(t *testing.T) {
	root := t.TempDir()
	onRel := filepath.Join("subprojects", "frida-gum", "gum", "backend-linux", "gumprocess-linux.c")
	offRel := filepath.Join("subprojects", "frida-core", "lib", "base", "session.vala")
	coreDir := filepath.Join(root, "subprojects", "frida-core")
	_ = os.MkdirAll(filepath.Join(root, filepath.Dir(onRel)), 0755)
	_ = os.MkdirAll(filepath.Join(root, filepath.Dir(offRel)), 0755)
	onBody := "void frida_agent_main(void);\nconst char *dbus = \"re.frida.HostSession\";\n"
	offBody := "void frida_agent_main(void);\nconst char *dbus = \"re.frida.HostSession\";\n"
	_ = os.WriteFile(filepath.Join(root, onRel), []byte(onBody), 0644)
	_ = os.WriteFile(filepath.Join(root, offRel), []byte(offBody), 0644)

	ft, n, err := ApplyInjectionABIRename(root, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if ft != 1 || n < 1 {
		t.Fatalf("files=%d n=%d", ft, n)
	}
	on, _ := os.ReadFile(filepath.Join(root, onRel))
	ons := string(on)
	if !strings.Contains(ons, "abcde_agent_main") {
		t.Fatalf("on-list ident: %s", ons)
	}
	if !strings.Contains(ons, "re.frida.HostSession") {
		t.Fatalf("ABI step must not rewrite re.frida.: %s", ons)
	}
	off, _ := os.ReadFile(filepath.Join(root, offRel))
	if !strings.Contains(string(off), "void frida_agent_main") {
		t.Fatalf("off-list changed: %s", off)
	}
	if _, err := os.Stat(coreDir); err != nil {
		t.Fatal("frida-core directory must remain")
	}
}
