package core

import (
	"strings"
	"testing"
)

func TestValidateMagicName(t *testing.T) {
	valid := []string{"abcde", "qwxyz", "zzzzz"}
	for _, n := range valid {
		if err := ValidateMagicName(n); err != nil {
			t.Errorf("ValidateMagicName(%q) unexpected err: %v", n, err)
		}
	}
	invalid := []string{
		"", "a", "abcd", "abcdef", // length
		"fridare",                  // 6 chars — the old ToolsTab default
		"ABCDE", "AbCdE",           // case
		"abc12", "ab_cd", "ab-cd",  // non a-z
	}
	for _, n := range invalid {
		if err := ValidateMagicName(n); err == nil {
			t.Errorf("ValidateMagicName(%q) want error", n)
		}
	}
}

func TestPatchClientProtocolSurface_FullSync(t *testing.T) {
	src := `ch = "frida:rpc"
iface = "re.frida.HostSession17"
path = "/re/frida/HostSession"
api = "Frida.version"
import _frida
`
	out, n, err := PatchClientProtocolSurface(src, "abcde", true)
	if err != nil {
		t.Fatal(err)
	}
	if n < 4 {
		t.Fatalf("n=%d out=%s", n, out)
	}
	if !strings.Contains(out, "abcde:rpc") || !strings.Contains(out, "re.abcde.HostSession17") ||
		!strings.Contains(out, "/re/abcde/HostSession") || !strings.Contains(out, "Abcde.version") {
		t.Fatalf("%s", out)
	}
	if strings.Contains(out, "/re/frida/") {
		t.Fatalf("object path not rewritten: %s", out)
	}
	if !strings.Contains(out, "import _frida") {
		t.Fatal("must not break _frida import")
	}
	// rpc-only mode leaves protocol
	out2, _, err := PatchClientProtocolSurface(src, "abcde", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out2, "re.frida.HostSession17") || !strings.Contains(out2, "Frida.version") ||
		!strings.Contains(out2, "/re/frida/HostSession") {
		t.Fatalf("rpc-only should keep protocol/API/path: %s", out2)
	}
}

func TestClientProtocolBinaryPairs_SameLength(t *testing.T) {
	pairs, err := ClientProtocolBinaryPairs("abcde")
	if err != nil {
		t.Fatal(err)
	}
	var hasPath bool
	for _, p := range pairs {
		if len(p[0]) != len(p[1]) {
			t.Fatalf("%q vs %q", p[0], p[1])
		}
		if string(p[0]) == "/re/frida/" {
			hasPath = true
			if string(p[1]) != "/re/abcde/" {
				t.Fatalf("path pair: %q", p[1])
			}
		}
	}
	if !hasPath {
		t.Fatal("missing /re/frida/ object-path pair")
	}
}

func TestPatchCorePyRPC_ReplacesChannelOnly(t *testing.T) {
	src := `
def _rpc():
    return "frida:rpc"
# module still imports native extension by fixed name
# see also agent path markers: frida-agent
`
	out, n, err := PatchCorePyRPC(src, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 replacement, got %d", n)
	}
	if !strings.Contains(out, "abcde:rpc") {
		t.Fatalf("expected abcde:rpc in output: %q", out)
	}
	if strings.Contains(out, "frida:rpc") {
		t.Fatalf("frida:rpc should be gone: %q", out)
	}
	if !strings.Contains(out, "frida-agent") {
		t.Fatalf("must not rewrite frida-agent: %q", out)
	}
}

func TestPatchCorePyRPC_IdempotentWhenMissing(t *testing.T) {
	src := `return "abcde:rpc"`
	out, n, err := PatchCorePyRPC(src, "xyzab")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || out != src {
		t.Fatalf("expected no change when frida:rpc absent, n=%d out=%q", n, out)
	}
}

func TestPatchCorePyRPC_RejectsInvalidMagic(t *testing.T) {
	src := `"frida:rpc"`
	for _, bad := range []string{"fridare", "ab", "ABCDE", "abc12"} {
		out, n, err := PatchCorePyRPC(src, bad)
		if err == nil {
			t.Fatalf("PatchCorePyRPC(%q) want error", bad)
		}
		if n != 0 || out != src {
			t.Fatalf("invalid magic must not mutate content: n=%d out=%q", n, out)
		}
		// must not look like a silent "no frida:rpc found" success path
		if !strings.Contains(err.Error(), "魔改名称") && !strings.Contains(err.Error(), "5") {
			t.Fatalf("error should mention magic name rule: %v", err)
		}
	}
}

func TestWouldBreakFridaImport_NaiveGlobalReplace(t *testing.T) {
	initPy := "from . import core\nimport _frida\n"
	if !WouldBreakFridaImport(initPy, "led20") {
		t.Fatal("naive global frida->led20 must be detected as breaking import _frida")
	}
	corePy := `channel = "frida:rpc"`
	patched, _, err := PatchCorePyRPC(corePy, "ledab")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(patched, "import _ledab") {
		t.Fatal("PatchCorePyRPC must never invent import _ledab")
	}
}
