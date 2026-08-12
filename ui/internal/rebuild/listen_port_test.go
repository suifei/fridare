package rebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListenPortSourceOps_SkipStockDefault(t *testing.T) {
	if ops := ListenPortSourceOps(0); len(ops) != 0 {
		t.Fatalf("port 0 must be no-op: %+v", ops)
	}
	if ops := ListenPortSourceOps(OfficialListenPort); len(ops) != 0 {
		t.Fatalf("port %d must be no-op: %+v", OfficialListenPort, ops)
	}
	for _, op := range DefaultModOps("abcde", OfficialListenPort) {
		if op.Find == OfficialListenPortASCII {
			t.Fatalf("must not emit 27042→27042: %+v", op)
		}
	}
}

func TestListenPortSourceOps_SurgicalNotTreeWide(t *testing.T) {
	ops := ListenPortSourceOps(27142)
	if len(ops) == 0 {
		t.Fatal("expected port ops")
	}
	var hasConst, hasASCII bool
	for _, op := range ops {
		if op.Path == "**/*" && op.Find == OfficialListenPortASCII {
			t.Fatal("must not tree-wide replace 27042")
		}
		if strings.Contains(op.Find, "DEFAULT_CONTROL_PORT") && op.Replace == "DEFAULT_CONTROL_PORT = 27142" {
			hasConst = true
		}
		if op.Find == OfficialListenPortASCII && op.Replace == "27142" {
			hasASCII = true
			if !strings.Contains(op.Path, "frida-core") {
				t.Fatalf("ASCII 27042 must stay in frida-core: %+v", op)
			}
		}
	}
	if !hasConst || !hasASCII {
		t.Fatalf("missing const/ascii ops: %+v", ops)
	}
}

func TestListenPortSourceOps_DifferentDigitLength_OnlyConst(t *testing.T) {
	ops := ListenPortSourceOps(9999)
	if len(ops) != 1 {
		t.Fatalf("want only DEFAULT_CONTROL_PORT op, got %+v", ops)
	}
	if !strings.Contains(ops[0].Find, "DEFAULT_CONTROL_PORT") {
		t.Fatalf("%+v", ops[0])
	}
}

func TestApplyListenPort_SkipsVendorTables(t *testing.T) {
	root := t.TempDir()
	socket := filepath.Join(root, "subprojects", "frida-core", "lib", "base", "socket.vala")
	vendor := filepath.Join(root, "subprojects", "brotli", "table.c")
	_ = os.MkdirAll(filepath.Dir(socket), 0755)
	_ = os.MkdirAll(filepath.Dir(vendor), 0755)
	_ = os.WriteFile(socket, []byte("namespace Frida {\n\tpublic const uint16 DEFAULT_CONTROL_PORT = 27042;\n}\n"), 0644)
	_ = os.WriteFile(vendor, []byte("static const int k = 27042;\n"), 0644)

	plan := &ModPlan{Operations: ListenPortSourceOps(27142)}
	if err := applyPlanNaive(root, plan); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(socket)
	if !strings.Contains(string(got), "DEFAULT_CONTROL_PORT = 27142") {
		t.Fatalf("socket not patched: %s", got)
	}
	if strings.Contains(string(got), "DEFAULT_CONTROL_PORT = 27042") {
		t.Fatalf("old const remains: %s", got)
	}
	v, _ := os.ReadFile(vendor)
	if !strings.Contains(string(v), "27042") {
		t.Fatalf("vendor table must stay: %s", v)
	}
}

func TestListenPortAgentGuidance_NeverUsesUserPortAsFind(t *testing.T) {
	g := ListenPortAgentGuidance(27142)
	for _, want := range []string{"DEFAULT_CONTROL_PORT", "27042", "27142", "禁止", "brotli"} {
		if !strings.Contains(g, want) {
			t.Fatalf("guidance missing %q\n%s", want, g)
		}
	}
	stock := ListenPortAgentGuidance(OfficialListenPort)
	if !strings.Contains(stock, "不要改 DEFAULT_CONTROL_PORT") {
		t.Fatalf("stock guidance should skip rewrite:\n%s", stock)
	}
}

func TestSkipNumericTableVendorPath(t *testing.T) {
	if !skipNumericTableVendorPath("subprojects/frida-gum/subprojects/capstone/table.c") {
		t.Fatal("capstone")
	}
	if skipNumericTableVendorPath("subprojects/frida-core/lib/base/socket.vala") {
		t.Fatal("frida-core must not skip")
	}
}
