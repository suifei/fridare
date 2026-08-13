package rebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMapFridaIdentifier(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"frida_agent_main", "abcde_agent_main", true},
		{"frida_server", "abcde_server", true},
		{"_frida_agent_environment_init", "_abcde_agent_environment_init", true},
		{"FridaScriptEngine", "AbcdeScriptEngine", true},
		{"Frida", "Abcde", true},
		{"FRIDA_VERSION", "FRIDA_VERSION", false}, // keep build-system macro/module names
		{"frida_version", "frida_version", false},
		{"frida", "abcde", true},
		{"my_frida_helper", "my_frida_helper", false}, // no prefix match
		{"re", "re", false},
		{"HostSession", "HostSession", false},
	}
	for _, c := range cases {
		got, ok := MapFridaIdentifier(c.in, "abcde")
		if ok != c.ok || got != c.want {
			t.Errorf("%s: got %s/%v want %s/%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestRewriteCIdentifiers_FunctionsAndNamespaces(t *testing.T) {
	in := `
void frida_agent_main(void);
typedef struct _FridaSession FridaSession;
namespace Frida {
  class Agent;
}
Frida.Agent *p;
const char *s = "frida_agent_main"; // string left to string layer; idents pass leaves strings alone
`
	// Identifier pass leaves strings intact
	out, n := RewriteCIdentifiers(in, "abcde")
	if n < 4 {
		t.Fatalf("n=%d out=%s", n, out)
	}
	if strings.Contains(out, "void frida_agent_main") {
		t.Fatalf("function not renamed: %s", out)
	}
	if !strings.Contains(out, "void abcde_agent_main") {
		t.Fatalf("missing renamed fn: %s", out)
	}
	if !strings.Contains(out, "namespace Abcde") {
		t.Fatalf("namespace: %s", out)
	}
	if !strings.Contains(out, "Abcde.Agent") && !strings.Contains(out, "Abcde . Agent") {
		// Abcde.Agent as two tokens Abcde . Agent after Frida→Abcde
		if !strings.Contains(out, "Abcde") {
			t.Fatalf("type prefix: %s", out)
		}
	}
	// string content not touched by identifier pass
	if !strings.Contains(out, `"frida_agent_main"`) {
		t.Fatalf("string should be untouched by ident pass: %s", out)
	}
}

func TestStructureAwareRewriteAll_StringThenIdents(t *testing.T) {
	in := `void frida_agent_main(void);
const char *s = "frida:rpc";
const char *proto = "re.frida.HostSession";
const char *api = "Frida.version";
`
	res := StructureAwareRewriteAll("x.c", in, "abcde", true)
	if !strings.Contains(res.Content, "void abcde_agent_main") {
		t.Fatalf("fn: %s", res.Content)
	}
	if !strings.Contains(res.Content, `"abcde:rpc"`) {
		t.Fatalf("rpc: %s", res.Content)
	}
	// deep/idents path also rewrites protocol + Frida.* API strings (client must match)
	if !strings.Contains(res.Content, `"re.abcde.HostSession"`) {
		t.Fatalf("protocol should rewrite under deep: %s", res.Content)
	}
	if !strings.Contains(res.Content, `"Abcde.version"`) {
		t.Fatalf("API string: %s", res.Content)
	}
}

func TestMapFridaIdentifier_KeepsRelengModuleNames(t *testing.T) {
	for _, id := range []string{"frida_version", "FridaVersion", "frida_core", "frida_gum"} {
		if got, ok := MapFridaIdentifier(id, "abcde"); ok {
			t.Fatalf("%s should not rename, got %s", id, got)
		}
	}
}

func TestSkipIdentifierRenamePath(t *testing.T) {
	if !skipIdentifierRenamePath(`/work/frida/releng/frida_version.py`) {
		t.Fatal("releng should skip")
	}
	if !skipIdentifierRenamePath(`C:\x\subprojects\frida-gum\tools\detect-version.py`) {
		t.Fatal("detect-version should skip")
	}
	if skipIdentifierRenamePath(`/work/frida/subprojects/frida-core/lib/base/session.vala`) {
		t.Fatal("session.vala should rename")
	}
}

func TestApplyIdentifierRenameStrip_MultiFile(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.c"), []byte("void frida_agent_main(void);\nchar *s=\"frida:rpc\";\n"), 0644)
	_ = os.WriteFile(filepath.Join(root, "b.vala"), []byte("namespace Frida.Agent {\npublic void frida_helper_x() {}\n}\n"), 0644)
	_ = os.WriteFile(filepath.Join(root, "c.py"), []byte("def frida_agent_main():\n    return \"frida-server\"\n"), 0644)

	ft, n, err := ApplyIdentifierRenameStrip(root, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if ft < 3 || n < 3 {
		t.Fatalf("files=%d n=%d", ft, n)
	}
	a, _ := os.ReadFile(filepath.Join(root, "a.c"))
	if !strings.Contains(string(a), "abcde_agent_main") || !strings.Contains(string(a), "abcde:rpc") {
		t.Fatalf("%s", a)
	}
	b, _ := os.ReadFile(filepath.Join(root, "b.vala"))
	bs := string(b)
	if !strings.Contains(bs, "namespace Abcde") {
		t.Fatalf("vala ns: %s", bs)
	}
	if strings.Contains(bs, "frida_helper_x") {
		t.Fatalf("vala method not renamed: %s", bs)
	}
	c, _ := os.ReadFile(filepath.Join(root, "c.py"))
	cs := string(c)
	if !strings.Contains(cs, "def abcde_agent_main") {
		t.Fatalf("py fn: %s", cs)
	}
	if !strings.Contains(cs, "abcde-server") {
		t.Fatalf("py str: %s", cs)
	}
}

func TestApplyDeepSourceExtras_DeepKeepsIdents_AbiRenames(t *testing.T) {
	// deep: protocol/string only — C identifiers stay (compile-safe)
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "t.c"), []byte("void frida_server_start(void);\nconst char *p=\"re.frida.Host\";\n"), 0644)
	cfg := JobConfig{MagicName: "abcde", DirectionProfile: "deep", ListenPort: 27142}
	if err := ApplyDeepSourceExtras(root, cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "t.c"))
	s := string(data)
	if !strings.Contains(s, "void frida_server_start") {
		t.Fatalf("deep must keep C idents: %s", s)
	}
	if !strings.Contains(s, "re.abcde.Host") {
		t.Fatalf("deep must rewrite protocol strings: %s", s)
	}

	// abi: token rename only on injection whitelist
	root2 := t.TempDir()
	on := filepath.Join(root2, "subprojects", "frida-gum", "gum", "backend-linux", "gumprocess-linux.c")
	off := filepath.Join(root2, "subprojects", "frida-core", "lib", "base", "session.vala")
	_ = os.MkdirAll(filepath.Dir(on), 0755)
	_ = os.MkdirAll(filepath.Dir(off), 0755)
	_ = os.WriteFile(on, []byte("void frida_server_start(void);\nconst char *p=\"re.frida.Host\";\n"), 0644)
	_ = os.WriteFile(off, []byte("void frida_server_start(void);\n"), 0644)
	cfg2 := JobConfig{MagicName: "abcde", DirectionProfile: "abi", ListenPort: 27142}
	if err := ApplyDeepSourceExtras(root2, cfg2); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(on)
	s2 := string(data2)
	if !strings.Contains(s2, "abcde_server_start") {
		t.Fatalf("abi on-list should rename idents: %s", s2)
	}
	if !strings.Contains(s2, "re.frida.Host") {
		t.Fatalf("abi rename step must not rewrite re.frida.: %s", s2)
	}
	offB, _ := os.ReadFile(off)
	if !strings.Contains(string(offB), "void frida_server_start") {
		t.Fatalf("abi off-list must stay: %s", offB)
	}
}
