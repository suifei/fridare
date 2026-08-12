package rebuild

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceFridaMarkersInStringLiterals_PreservesIdentifiers(t *testing.T) {
	src := `
void frida_agent_main(void);
const char *s = "frida_agent_main";
const char *r = "frida:rpc";
// frida_agent in comment
char *p = "frida-server";
`
	out, n := ReplaceFridaMarkersInStringLiterals(src, "abcde")
	if n < 3 {
		t.Fatalf("replacements=%d out=%s", n, out)
	}
	// identifier untouched
	if !strings.Contains(out, "void frida_agent_main(void)") {
		t.Fatalf("identifier must stay: %s", out)
	}
	// quoted rewritten
	if strings.Contains(out, `"frida_agent_main"`) || strings.Contains(out, `"frida:rpc"`) || strings.Contains(out, `"frida-server"`) {
		t.Fatalf("quoted markers remain: %s", out)
	}
	if !strings.Contains(out, `"abcde_agent_main"`) || !strings.Contains(out, `"abcde:rpc"`) {
		t.Fatalf("magic quoted missing: %s", out)
	}
}

func TestDeepModOps_NoBareFridaHyphenCatchAll(t *testing.T) {
	ops := DeepModOps("abcde", 27142)
	for _, op := range ops {
		if op.Find == "frida-" {
			t.Fatal("bare frida- catch-all would break frida-core subprojects")
		}
		if op.Find == "libfrida-" {
			t.Fatal("global libfrida- rename is too aggressive for meson")
		}
	}
	// still has inject + safe tmp path (not bare /frida/ which smashes github.com/frida wraps)
	var hasInject, hasTmpPath bool
	for _, op := range ops {
		if op.Find == "frida-inject" {
			hasInject = true
		}
		if op.Find == "/tmp/frida" {
			hasTmpPath = true
		}
	}
	if !hasInject || !hasTmpPath {
		t.Fatalf("deep extras missing inject/tmp-path: inject=%v tmp=%v", hasInject, hasTmpPath)
	}
}

func TestApplyDeepStringLiteralStrip_AndDigPlan(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.c"), []byte(`
void frida_agent_main(void) {}
const char *entry = "frida_agent_main";
const char *rpc = "frida:rpc";
`), 0644)
	_ = os.WriteFile(filepath.Join(root, "ns.vala"), []byte("namespace Frida.Agent {\n"), 0644)

	ft, n, err := ApplyDeepStringLiteralStrip(root, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if ft != 1 || n < 2 {
		t.Fatalf("files=%d n=%d", ft, n)
	}
	data, _ := os.ReadFile(filepath.Join(root, "a.c"))
	s := string(data)
	if !strings.Contains(s, "void frida_agent_main") {
		t.Fatal("ABI id broken")
	}
	if !strings.Contains(s, `"abcde_agent_main"`) {
		t.Fatal("string not rewritten")
	}

	// full deep extras via StubAgent
	_ = os.WriteFile(filepath.Join(root, "b.c"), []byte(`"frida-server" get_frida_agent_x()`), 0644)
	cfg := JobConfig{MagicName: "abcde", ListenPort: 27142, DirectionProfile: "deep", FridaVersion: "17.16.4"}
	plan, err := PlanModsFromTree(root, cfg, "br")
	if err != nil {
		t.Fatal(err)
	}
	ag := &StubAgent{}
	if err := ag.ApplyMods(nil, cfg, plan, root); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "b.c"))
	if !strings.Contains(string(b), "abcde-server") || !strings.Contains(string(b), "get_abcde_agent") {
		t.Fatalf("deep ops not applied: %s", b)
	}
	if _, err := os.Stat(filepath.Join(root, "fridare-deep-dig.md")); err != nil {
		t.Fatal("expected dig brief", err)
	}
}

func TestOpsForProfile(t *testing.T) {
	if len(OpsForProfile("deep", "abcde", 1)) <= len(OpsForProfile("safe", "abcde", 1)) {
		t.Fatal("deep should have more ops than safe")
	}
}

func TestRepairFridaGitWraps(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "libffi.wrap")
	_ = os.WriteFile(p, []byte("[wrap-git]\nurl = https://github.com/abcde/libffi.git\n"), 0644)
	n, err := RepairFridaGitWraps(root, "abcde")
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "github.com/frida/libffi") {
		t.Fatalf("%s", b)
	}
	if strings.Contains(string(b), "github.com/abcde/") {
		t.Fatal(string(b))
	}
}

func TestDeepModOps_DoesNotSmashGitHubFridaPath(t *testing.T) {
	for _, op := range DeepModOps("abcde", 1) {
		if op.Find == "/frida/" || op.Find == "\\frida\\" {
			t.Fatalf("destructive path op must not exist: %+v", op)
		}
	}
}

func TestNormalizeSourceTreeLF(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "tool.py")
	_ = os.WriteFile(p, []byte("#!/usr/bin/env python3\r\nprint(1)\r\n"), 0644)
	n, err := NormalizeSourceTreeLF(root)
	if err != nil || n < 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	b, _ := os.ReadFile(p)
	if bytes.Contains(b, []byte("\r")) {
		t.Fatalf("still has CR: %q", b)
	}
	if !bytes.HasPrefix(b, []byte("#!/usr/bin/env python3\n")) {
		t.Fatalf("%q", b)
	}
}

func TestDeepModOps_ServerClientProtocolPairs(t *testing.T) {
	ops := DeepModOps("abcde", 27142)
	var hasProto, hasAPI, hasRPC bool
	for _, op := range ops {
		switch op.Find {
		case "re.frida.":
			hasProto = op.Replace == "re.abcde."
		case "\"Frida.":
			hasAPI = op.Replace == "\"Abcde."
		case "frida:rpc":
			hasRPC = op.Replace == "abcde:rpc"
		}
	}
	if !hasProto || !hasAPI || !hasRPC {
		t.Fatalf("deep must sync protocol+API+rpc: proto=%v api=%v rpc=%v", hasProto, hasAPI, hasRPC)
	}
	// safe must NOT rewrite protocol/API in source ops
	for _, op := range DefaultModOps("abcde", 27142) {
		if op.Find == "re.frida." || op.Find == "\"Frida." {
			t.Fatalf("safe must not include protocol/API op: %+v", op)
		}
	}
}

func TestBuildDeepDigTasks(t *testing.T) {
	rep := ScanReport{Hits: []StripHit{
		{Layer: LayerProductBasename, Mode: StripModeAuto, Path: "a.c", Pattern: "frida-server", Count: 1},
		{Layer: LayerUnderscoreCABI, Mode: StripModeForbidden, Path: "a.c", Pattern: "frida_agent_main", Count: 1},
		{Layer: LayerGlobalWord, Mode: StripModeAIExplore, Path: "a.c", Pattern: "frida", Count: 3},
	}}
	tasks := BuildDeepDigTasks(rep, "abcde")
	if len(tasks) != 3 {
		t.Fatal(len(tasks))
	}
	brief := FormatDeepDigBrief(tasks, "abcde")
	if !strings.Contains(brief, "forbidden") || !strings.Contains(brief, "ai_explore") {
		t.Fatal(brief)
	}
}
