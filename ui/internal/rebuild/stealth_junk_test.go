package rebuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func execPython(dir, script string) (string, error) {
	cmd := exec.Command("python", script)
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	return string(b), err
}

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
	// Must not fall back to "append flag to any matching line" (turns ninja
	// build/DEPFILE edges into implicit deps).
	if strings.Contains(sh, `line = line.rstrip("\n") + flag`) ||
		strings.Contains(sh, "line.rstrip(\"\\n\") + flag") {
		t.Fatal("shell must not append the seed flag to arbitrary ninja lines")
	}
	if !strings.Contains(sh, `stripped.startswith("build ")`) {
		t.Fatal("shell must skip ninja build edges")
	}
	if !strings.Contains(sh, `stripped.startswith("ARGS")`) {
		t.Fatal("shell must write the seed only onto ARGS")
	}
}

func mesonNinjaFixture() string {
	return `# meson ninja
rule c_COMPILER
 command = /usr/bin/cc $ARGS -MD -MQ $out -MF $DEPFILE -o $out -c $in

build subprojects/frida-gum/gum/libfrida-gum-1.0.a.p/gumprocess.c.o: c_COMPILER ../frida/subprojects/frida-gum/gum/gumprocess.c || subprojects/frida-gum/gum/gumenumtypes.h
 DEPFILE = subprojects/frida-gum/gum/libfrida-gum-1.0.a.p/gumprocess.c.o.d
 DEPFILE_UNQUOTED = subprojects/frida-gum/gum/libfrida-gum-1.0.a.p/gumprocess.c.o.d
 ARGS = -Isubprojects/frida-gum/gum -DNDEBUG

build subprojects/frida-gum/gum/libfrida-gum-1.0.a.p/backend-linux_gumprocess-linux.c.o: c_COMPILER ../frida/subprojects/frida-gum/gum/backend-linux/gumprocess-linux.c || subprojects/frida-gum/gum/gumenumtypes.h
 DEPFILE = subprojects/frida-gum/gum/libfrida-gum-1.0.a.p/backend-linux_gumprocess-linux.c.o.d
 DEPFILE_UNQUOTED = subprojects/frida-gum/gum/libfrida-gum-1.0.a.p/backend-linux_gumprocess-linux.c.o.d
 ARGS = -Isubprojects/frida-gum/gum -DNDEBUG

build subprojects/frida-gum/gum/libfrida-gum-1.0.a.p/gumprintf.c.o: c_COMPILER ../frida/subprojects/frida-gum/gum/gumprintf.c || subprojects/frida-gum/gum/gumenumtypes.h
 DEPFILE = subprojects/frida-gum/gum/libfrida-gum-1.0.a.p/gumprintf.c.o.d
 ARGS = -Isubprojects/frida-gum/gum -DNDEBUG

build subprojects/frida-gum/gum/libfrida-gum-1.0.a: STATIC_LINKER subprojects/frida-gum/gum/libfrida-gum-1.0.a.p/gumprocess.c.o subprojects/frida-gum/gum/libfrida-gum-1.0.a.p/backend-linux_gumprocess-linux.c.o
 LINK_ARGS = rcs
`
}

func TestPatchNinjaStealthFlags_DoesNotCreateNinjaDeps(t *testing.T) {
	got, n := PatchNinjaStealthFlags(mesonNinjaFixture(), "0293ace3")
	if n != 2 {
		t.Fatalf("patched ARGS count=%d want 2\n%s", n, got)
	}
	flag := " -DFRIDARE_JUNK_SEED=0x0293ace3"
	for _, line := range strings.Split(got, "\n") {
		trim := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trim, "build ") && strings.Contains(line, "FRIDARE_JUNK_SEED") {
			t.Fatalf("build edge must not gain a seed token (ninja implicit dep):\n%s", line)
		}
		if strings.HasPrefix(trim, "DEPFILE") && strings.Contains(line, "FRIDARE_JUNK_SEED") {
			t.Fatalf("DEPFILE must not gain a seed token:\n%s", line)
		}
		if strings.Contains(line, "STATIC_LINKER") && strings.Contains(line, "FRIDARE_JUNK_SEED") {
			t.Fatalf("linker edge must not gain a seed token:\n%s", line)
		}
		if strings.Contains(line, "gumprintf.c") && strings.Contains(line, "FRIDARE_JUNK_SEED") {
			t.Fatalf("non-injection TU must not gain a seed token:\n%s", line)
		}
	}
	if !strings.Contains(got, "gumprocess.c.o: c_COMPILER") {
		t.Fatal("compile edge lost")
	}
	// The two injection ARGS lines must carry the flag; gumprintf must not.
	var injectionARGS, otherARGS int
	pending := ""
	for _, line := range strings.Split(got, "\n") {
		trim := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trim, "build ") {
			pending = line
			continue
		}
		if strings.HasPrefix(trim, "ARGS") {
			has := strings.Contains(line, flag)
			if strings.Contains(pending, "/gumprocess.c ") || strings.Contains(pending, "/gumprocess.c ||") ||
				strings.Contains(pending, "/gumprocess-linux.c") {
				if !has {
					t.Fatalf("injection ARGS missing flag after:\n%s\n%s", pending, line)
				}
				injectionARGS++
			} else if has {
				t.Fatalf("unexpected flag on ARGS after:\n%s\n%s", pending, line)
			} else {
				otherARGS++
			}
		}
	}
	if injectionARGS != 2 || otherARGS != 1 {
		t.Fatalf("injectionARGS=%d otherARGS=%d", injectionARGS, otherARGS)
	}
}

func TestPatchCompileCommandsStealthFlags_OnlyCommandDashC(t *testing.T) {
	in := `[
  {"directory": "/work/b", "command": "cc -I. -c ../frida/gum/gumprocess.c -o gumprocess.c.o", "file": "../frida/gum/gumprocess.c"},
  {"directory": "/work/b", "command": "cc -I. -c ../frida/gum/gumprintf.c -o gumprintf.c.o", "file": "../frida/gum/gumprintf.c"}
]
`
	got, n := PatchCompileCommandsStealthFlags(in, "cafebabe")
	if n != 1 {
		t.Fatalf("n=%d\n%s", n, got)
	}
	if strings.Count(got, "FRIDARE_JUNK_SEED") != 1 {
		t.Fatalf("flag count:\n%s", got)
	}
	if !strings.Contains(got, `"file": "../frida/gum/gumprocess.c"`) {
		t.Fatal("file field must stay intact")
	}
	if !strings.Contains(got, "-DFRIDARE_JUNK_SEED=0xcafebabe -c ../frida/gum/gumprocess.c") {
		t.Fatalf("command flag placement:\n%s", got)
	}
}

func TestPerTUStealthFlagsShell_PythonMatchesGoPatcher(t *testing.T) {
	dir := t.TempDir()
	ninja := filepath.Join(dir, "build.ninja")
	if err := os.WriteFile(ninja, []byte(mesonNinjaFixture()), 0644); err != nil {
		t.Fatal(err)
	}
	cc := filepath.Join(dir, "compile_commands.json")
	ccBody := `{"directory":"/w","command":"cc -c /src/gumprocess.c","file":"/src/gumprocess.c"}` + "\n"
	if err := os.WriteFile(cc, []byte(ccBody), 0644); err != nil {
		t.Fatal(err)
	}
	sh := PerTUStealthFlagsShell("0293ace3")
	// Extract the python between <<'PY' and the closing PY
	start := strings.Index(sh, "python3 - <<'PY'\n")
	if start < 0 {
		t.Fatal("missing python heredoc")
	}
	start += len("python3 - <<'PY'\n")
	end := strings.LastIndex(sh, "\nPY")
	if end < start {
		t.Fatal("missing heredoc end")
	}
	py := sh[start:end]
	script := filepath.Join(dir, "patch.py")
	if err := os.WriteFile(script, []byte(py), 0644); err != nil {
		t.Fatal(err)
	}
	// Run via python - the same interpreter Docker's builder has (3.x).
	out, err := execPython(dir, script)
	if err != nil {
		t.Fatalf("python patcher: %v\n%s", err, out)
	}
	gotNinja, _ := os.ReadFile(ninja)
	wantNinja, n := PatchNinjaStealthFlags(mesonNinjaFixture(), "0293ace3")
	if n != 2 {
		t.Fatalf("go patcher n=%d", n)
	}
	if normNL(string(gotNinja)) != normNL(wantNinja) {
		t.Fatalf("python ninja != Go patcher\npython:\n%s\ngo:\n%s", gotNinja, wantNinja)
	}
	gotCC, _ := os.ReadFile(cc)
	wantCC, _ := PatchCompileCommandsStealthFlags(ccBody, "0293ace3")
	if normNL(string(gotCC)) != normNL(wantCC) {
		t.Fatalf("python compile_commands != Go\npython:\n%s\ngo:\n%s", gotCC, wantCC)
	}
}

func normNL(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "\r\n", "\n")
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
	if !strings.Contains(script, "compiler_snapshot") || !strings.Contains(script, "-Dfrida-core:compiler_snapshot=disabled") {
		t.Fatal("16.x compiler_snapshot disable (SDK snapshot tool workaround) missing")
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
