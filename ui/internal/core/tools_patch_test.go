package core

import (
	"strings"
	"testing"
)

func TestPatchCorePyRPC_ReplacesChannelOnly(t *testing.T) {
	// Realistic snippets from frida core.py-style sources
	src := `
def _rpc():
    return "frida:rpc"
# module still imports native extension by fixed name
# see also agent path markers: frida-agent
`
	out, n := PatchCorePyRPC(src, "abcde")
	if n != 1 {
		t.Fatalf("expected 1 replacement, got %d", n)
	}
	if !strings.Contains(out, "abcde:rpc") {
		t.Fatalf("expected abcde:rpc in output: %q", out)
	}
	if strings.Contains(out, "frida:rpc") {
		t.Fatalf("frida:rpc should be gone: %q", out)
	}
	// Must NOT touch other frida-prefixed strings
	if !strings.Contains(out, "frida-agent") {
		t.Fatalf("must not rewrite frida-agent: %q", out)
	}
}

func TestPatchCorePyRPC_IdempotentWhenMissing(t *testing.T) {
	src := `return "abcde:rpc"`
	out, n := PatchCorePyRPC(src, "xyzab")
	if n != 0 || out != src {
		t.Fatalf("expected no change when frida:rpc absent, n=%d out=%q", n, out)
	}
}

func TestPatchCorePyRPC_RejectsBadMagicLen(t *testing.T) {
	src := `"frida:rpc"`
	out, n := PatchCorePyRPC(src, "ab")
	if n != 0 || out != src {
		t.Fatalf("short magic must not patch")
	}
}

func TestWouldBreakFridaImport_NaiveGlobalReplace(t *testing.T) {
	initPy := "from . import core\nimport _frida\n"
	if !WouldBreakFridaImport(initPy, "led20") {
		t.Fatal("naive global frida->led20 must be detected as breaking import _frida")
	}
	// Correct path: only RPC channel — import line unchanged
	corePy := `channel = "frida:rpc"`
	patched, _ := PatchCorePyRPC(corePy, "led20")
	if strings.Contains(patched, "import _led20") {
		t.Fatal("PatchCorePyRPC must never invent import _led20")
	}
	if WouldBreakFridaImport(initPy, "led20") && strings.Contains(initPy, "import _frida") {
		// documenting the bug we fixed in GUI: do not apply ReplaceAll("frida", magic) to init
	}
}
