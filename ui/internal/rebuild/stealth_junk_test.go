package rebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeededJunkC_TwoSeedsDiffer_SameSeedMatches(t *testing.T) {
	a := SeededJunkC("aaaaaaaa")
	b := SeededJunkC("bbbbbbbb")
	c := SeededJunkC("aaaaaaaa")
	if a == b {
		t.Fatal("different seeds must differ")
	}
	if a != c {
		t.Fatal("same seed must match")
	}
	if !strings.Contains(a, fridareJunkMarker) || !strings.Contains(a, "0x") {
		t.Fatalf("junk shape: %s", a)
	}
}

func TestInjectSeededJunk_ReplacesBlock(t *testing.T) {
	src := "void foo(void) {}\n"
	once := InjectSeededJunk(src, "s1")
	twice := InjectSeededJunk(once, "s2")
	if strings.Count(twice, fridareJunkMarker) != 1 {
		t.Fatalf("should replace not stack: %s", twice)
	}
	if !strings.Contains(twice, "seed=s2") {
		t.Fatalf("updated seed missing: %s", twice)
	}
}

func TestInjectSeededJunk_DoesNotSwallowTrailingCode(t *testing.T) {
	src := "void foo(void) {}\n"
	once := InjectSeededJunk(src, "s1")
	withTail := once + "\n#if 0\nvoid leftover(void) {}\n#endif\n"
	twice := InjectSeededJunk(withTail, "s2")
	if !strings.Contains(twice, "void leftover(void)") {
		t.Fatalf("trailing TU must survive re-inject: %s", twice)
	}
	if strings.Count(twice, fridareJunkMarker) != 1 {
		t.Fatalf("should still be one junk block: %s", twice)
	}
	if !strings.Contains(twice, "__attribute__((used, noinline))") {
		t.Fatal("junk must keep used+noinline so strip cannot drop folded immediates")
	}
	if !strings.Contains(twice, "__attribute__((constructor, used, noinline))") {
		t.Fatal("junk must keep a noinline constructor root so strip cannot drop folded immediates")
	}
	if !strings.Contains(twice, "_words[2]") {
		t.Fatal("junk must emit rodata words that survive strip")
	}
}

func TestApplySeededJunkToInjectionTUs(t *testing.T) {
	root := t.TempDir()
	on := filepath.Join(root, "subprojects", "frida-gum", "gum", "backend-linux", "gumprocess-linux.c")
	off := filepath.Join(root, "subprojects", "frida-core", "lib", "base", "session.c")
	_ = os.MkdirAll(filepath.Dir(on), 0755)
	_ = os.MkdirAll(filepath.Dir(off), 0755)
	_ = os.WriteFile(on, []byte("void gum_process(void) {}\n"), 0644)
	_ = os.WriteFile(off, []byte("void other(void) {}\n"), 0644)
	n, err := ApplySeededJunkToInjectionTUs(root, "deadbeef")
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got, _ := os.ReadFile(on)
	if !strings.Contains(string(got), fridareJunkMarker) {
		t.Fatalf("on-list missing junk: %s", got)
	}
	offB, _ := os.ReadFile(off)
	if strings.Contains(string(offB), fridareJunkMarker) {
		t.Fatalf("off-list got junk: %s", offB)
	}
}

func TestPerTUStealthFlagsShell_NoGlobalCFLAGS(t *testing.T) {
	sh := PerTUStealthFlagsShell("cafebabe")
	if !strings.Contains(sh, "gumprocess-linux.c") || !strings.Contains(sh, "agent-glue.c") ||
		!strings.Contains(sh, "linjector-glue.c") || !strings.Contains(sh, "FRIDARE_JUNK_SEED") {
		t.Fatalf("missing per-TU names: %s", sh)
	}
	if strings.Contains(sh, "export CFLAGS=") || strings.Contains(sh, "export CPPFLAGS=") {
		t.Fatal("per-TU helper must not export CFLAGS")
	}
}

func TestBuildOnlyPipelineScript_PerTUAndNoGlobalInclude(t *testing.T) {
	script, err := BuildOnlyPipelineScript(PipelineScriptOptions{
		SourceDir:   "frida",
		ArtifactDir: "artifacts",
		TargetIDs:   []string{"windows-x86_64", "linux-x86_64"},
		StealthSeed: "cafebabe",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "--enable-server") {
		t.Fatal("enable-server missing")
	}
	if !strings.Contains(script, "gumprocess-linux.c") || !strings.Contains(script, "gumprocess-windows.c") {
		t.Fatal("per-TU basenames missing")
	}
	if !strings.Contains(script, "FRIDARE_JUNK_SEED") {
		t.Fatal("per-TU flag missing")
	}
	if strings.Contains(script, `export CFLAGS="${CFLAGS:-} -include`) ||
		strings.Contains(script, "export CFLAGS=\"${CFLAGS:-} -include") {
		t.Fatal("must not add global CFLAGS=-include")
	}
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, "export CFLAGS=") || strings.Contains(line, "export CPPFLAGS=") {
			if strings.Contains(line, "FRIDARE_JUNK") || strings.Contains(line, "-include /work/") {
				t.Fatalf("global CFLAGS/CPPFLAGS must not add stealth flags: %s", line)
			}
		}
	}
	if !strings.Contains(script, "frida-*-mingw.txt") && !strings.Contains(script, "MinGW cross-file") {
		if !strings.Contains(script, "mingw.txt") && !strings.Contains(script, "fridare-mingw-dns.h") {
			t.Fatal("MinGW DNS should stay cross-file")
		}
	}
	if out := strings.TrimSpace(os.Getenv("FRIDARE_COMPILE_SCRIPT_OUT")); out != "" {
		if err := os.WriteFile(out, []byte(script), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
