package rebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteCSource_PreservesUnquotedIdentifier(t *testing.T) {
	in := "void frida_agent_main(void);\nconst char *s = \"frida:rpc\";\nconst char *a = \"frida_agent_main\";\n"
	out, n := RewriteCSource(in, "abcde")
	if n < 2 {
		t.Fatalf("n=%d out=%s", n, out)
	}
	if !strings.Contains(out, "void frida_agent_main(void)") {
		t.Fatalf("identifier mangled: %s", out)
	}
	if strings.Contains(out, "\"frida:rpc\"") || strings.Contains(out, "\"frida_agent_main\"") {
		t.Fatalf("literals not rewritten: %s", out)
	}
	if !strings.Contains(out, "\"abcde:rpc\"") || !strings.Contains(out, "\"abcde_agent_main\"") {
		t.Fatalf("magic missing: %s", out)
	}
}

func TestRewritePythonSource_PreservesIdentifierRewritesString(t *testing.T) {
	in := "frida_agent_main = 1\nx = \"frida-server\"\ny = 'frida:rpc'\n"
	out, n := RewritePythonSource(in, "abcde")
	if n < 2 {
		t.Fatalf("n=%d out=%s", n, out)
	}
	// left-hand name is identifier — must remain
	if !strings.HasPrefix(strings.TrimSpace(out), "frida_agent_main") && !strings.Contains(out, "frida_agent_main = 1") {
		t.Fatalf("python identifier mangled: %s", out)
	}
	if strings.Contains(out, "\"frida-server\"") || strings.Contains(out, "'frida:rpc'") {
		t.Fatalf("strings not rewritten: %s", out)
	}
	if !strings.Contains(out, "abcde-server") || !strings.Contains(out, "abcde:rpc") {
		t.Fatalf("magic missing: %s", out)
	}
}

func TestStructureAwareRewrite_Dispatch(t *testing.T) {
	c := StructureAwareRewrite("x.c", "void frida_agent_main();\nchar *s=\"frida:rpc\";", "abcde")
	if c.Engine != "c-lex" || c.Replacements < 1 {
		t.Fatalf("%+v", c)
	}
	if !strings.Contains(c.Content, "void frida_agent_main") {
		t.Fatal(c.Content)
	}
	p := StructureAwareRewrite("m.py", "frida_agent_main=1\ns=\"frida:rpc\"\n", "abcde")
	if p.Lang != "python" || p.Replacements < 1 {
		t.Fatalf("%+v", p)
	}
	if !strings.Contains(p.Content, "frida_agent_main=1") && !strings.Contains(p.Content, "frida_agent_main =") {
		// allow spaces from ast.unparse
		if !strings.Contains(p.Content, "frida_agent_main") {
			t.Fatalf("id lost: %s engine=%s", p.Content, p.Engine)
		}
	}
}

func TestApplyStructureAwareStrip_MultiFileFixture(t *testing.T) {
	root := t.TempDir()
	cPath := filepath.Join(root, "agent.c")
	pPath := filepath.Join(root, "tool.py")
	_ = os.WriteFile(cPath, []byte("void frida_agent_main(void);\nconst char *s = \"frida:rpc\";\n"), 0644)
	_ = os.WriteFile(pPath, []byte("frida_agent_main = None\nname = \"frida-server\"\n"), 0644)

	ft, n, err := ApplyStructureAwareStrip(root, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if ft < 2 || n < 2 {
		t.Fatalf("files=%d n=%d", ft, n)
	}
	c, _ := os.ReadFile(cPath)
	cs := string(c)
	if !strings.Contains(cs, "void frida_agent_main(void)") {
		t.Fatalf("C id broken: %s", cs)
	}
	if !strings.Contains(cs, "abcde:rpc") {
		t.Fatalf("C literal: %s", cs)
	}
	p, _ := os.ReadFile(pPath)
	ps := string(p)
	if !strings.Contains(ps, "frida_agent_main") {
		t.Fatalf("Py id broken: %s", ps)
	}
	if !strings.Contains(ps, "abcde-server") {
		t.Fatalf("Py literal: %s", ps)
	}

	// deep extras path (same as Agent apply)
	cfg := JobConfig{MagicName: "abcde", DirectionProfile: "deep", ListenPort: 27142}
	// reset py and re-run via ApplyDeepSourceExtras after content ops already applied — just ensure no error
	if err := ApplyDeepSourceExtras(root, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestRewriteCSource_CommentOnly(t *testing.T) {
	in := "int frida_agent = 1; // frida:rpc here\n"
	out, n := RewriteCSource(in, "abcde")
	if n < 1 {
		t.Fatal(n)
	}
	if !strings.Contains(out, "int frida_agent = 1") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "abcde:rpc") {
		t.Fatal(out)
	}
}

// Default StructureAwareRewrite (safe) keeps protocol/API; deep opts rewrite them.
func TestStructureAwareRewrite_ProtocolOptional(t *testing.T) {
	cIn := `const char *proto = "re.frida.HostSession17";
const char *api = "Frida.version";
const char *rpc = "frida:rpc";
const char *srv = "frida-server";
`
	// safe path: stock-client compatible
	c := StructureAwareRewrite("x.c", cIn, "abcde")
	if !strings.Contains(c.Content, `"re.frida.HostSession17"`) || !strings.Contains(c.Content, `"Frida.version"`) {
		t.Fatalf("safe should keep protocol/API: %s", c.Content)
	}
	if !strings.Contains(c.Content, `"abcde:rpc"`) || !strings.Contains(c.Content, `"abcde-server"`) {
		t.Fatalf("product markers: %s", c.Content)
	}
	// deep path: protocol + API strings rewrite (host client must match)
	d := StructureAwareRewriteOpts("x.c", cIn, "abcde", StringRewriteOpts{ProtocolAPI: true})
	if !strings.Contains(d.Content, `"re.abcde.HostSession17"`) {
		t.Fatalf("deep should rewrite protocol: %s", d.Content)
	}
	if !strings.Contains(d.Content, `"Abcde.version"`) {
		t.Fatalf("deep should rewrite Frida. API: %s", d.Content)
	}
	if strings.Contains(d.Content, "re.frida.") || strings.Contains(d.Content, `"Frida.version"`) {
		t.Fatalf("old protocol/API remain: %s", d.Content)
	}

	pyIn := "#!/usr/bin/env python3\n# keep-me comment\nproto = \"re.frida.AgentSession\"\napi = \"Frida.version\"\nrpc = \"frida:rpc\"\n"
	p := StructureAwareRewriteOpts("m.py", pyIn, "abcde", StringRewriteOpts{ProtocolAPI: true})
	if !strings.Contains(p.Content, "re.abcde.AgentSession") || !strings.Contains(p.Content, "Abcde.version") {
		t.Fatalf("py deep protocol/API: %s engine=%s", p.Content, p.Engine)
	}
	if !strings.Contains(p.Content, "#!/usr/bin/env python3") || !strings.Contains(p.Content, "# keep-me") {
		t.Fatalf("shebang/comment: %s", p.Content)
	}
}

func TestStructureAwareRewrite_PythonPreservesShebangAndComments(t *testing.T) {
	in := "#!/usr/bin/env python3\n# keep-me\nx = \"frida-server\"\n"
	res := StructureAwareRewrite("tool.py", in, "abcde")
	if !strings.Contains(res.Content, "#!/usr/bin/env python3") {
		t.Fatalf("shebang lost (engine=%s): %s", res.Engine, res.Content)
	}
	if !strings.Contains(res.Content, "# keep-me") {
		t.Fatalf("comment lost (engine=%s): %s", res.Engine, res.Content)
	}
	if !strings.Contains(res.Content, "abcde-server") {
		t.Fatalf("string not rewritten: %s", res.Content)
	}
	// Preferred path must not be ast.unparse
	if res.Engine == "python-ast" {
		t.Fatal("python-ast/unparse must not be preferred; use tokenize or lex")
	}
	if res.Engine != "python-tokenize" && res.Engine != "python-lex" {
		t.Fatalf("unexpected engine %s", res.Engine)
	}
	if !res.ParseOKAfter {
		t.Fatalf("ParseOKAfter=false engine=%s", res.Engine)
	}
}

func TestApplyStructureAwareStrip_ProtocolFixture(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "p.c"), []byte(`const char *a="re.frida.HostSession"; const char *b="frida:rpc";`), 0644)
	_ = os.WriteFile(filepath.Join(root, "q.py"), []byte("#!/usr/bin/env python3\n# keep\ns=\"Frida.version\"\nt=\"frida-agent\"\n"), 0644)
	// default ApplyStructureAwareStrip = safe (no protocol)
	_, _, err := ApplyStructureAwareStrip(root, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := os.ReadFile(filepath.Join(root, "p.c"))
	if !strings.Contains(string(c), "re.frida.HostSession") || !strings.Contains(string(c), "abcde:rpc") {
		t.Fatalf("safe: %s", c)
	}
	// deep path rewrites protocol
	root2 := t.TempDir()
	_ = os.WriteFile(filepath.Join(root2, "p.c"), []byte(`const char *a="re.frida.HostSession"; const char *b="frida:rpc";`), 0644)
	_ = os.WriteFile(filepath.Join(root2, "q.py"), []byte("#!/usr/bin/env python3\n# keep\ns=\"Frida.version\"\nt=\"frida-agent\"\n"), 0644)
	_, _, err = ApplyIdentifierRenameStrip(root2, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	c2, _ := os.ReadFile(filepath.Join(root2, "p.c"))
	if !strings.Contains(string(c2), "re.abcde.HostSession") {
		t.Fatalf("deep protocol: %s", c2)
	}
	p2, _ := os.ReadFile(filepath.Join(root2, "q.py"))
	ps := string(p2)
	if !strings.Contains(ps, "#!/usr/bin/env python3") || !strings.Contains(ps, "# keep") {
		t.Fatalf("shebang/comment: %s", ps)
	}
	if !strings.Contains(ps, "Abcde.version") || !strings.Contains(ps, "abcde-agent") {
		t.Fatalf("deep api: %s", ps)
	}
}

func TestQuotedFridaReplacements_NoBareBrand(t *testing.T) {
	for _, p := range quotedFridaReplacements("abcde") {
		if p[0] == "frida" || p[0] == "Frida" || p[0] == "FRIDA" {
			t.Fatalf("bare brand pair not allowed in auto L11: %v", p)
		}
	}
}
