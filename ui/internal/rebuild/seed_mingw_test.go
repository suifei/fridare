package rebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageSeedMinGWWraps(t *testing.T) {
	dir := t.TempDir()
	if err := StageSeedMinGWWraps(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, SeedMinGWWrapsFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "\r") {
		t.Fatal("staged seed script must be LF-only")
	}
	if !strings.Contains(s, "seed-mingw-wraps done") {
		t.Fatalf("missing done marker: %s", s[:min(200, len(s))])
	}
	if !strings.Contains(s, "removing partial tree") {
		t.Fatal("expected failed-checkout cleanup language")
	}
	// re-stage must overwrite
	if err := StageSeedMinGWWraps(dir); err != nil {
		t.Fatal(err)
	}
}

func TestSeedMinGWWrapsShellSnippet(t *testing.T) {
	sh := SeedMinGWWrapsShellSnippet("frida")
	if !strings.Contains(sh, "/work/"+SeedMinGWWrapsFileName) {
		t.Fatalf("must call staged /work script: %s", sh)
	}
	if strings.Contains(sh, "[ -x ") {
		t.Fatal("must not require +x (Windows bind mounts)")
	}
	if !strings.Contains(sh, "[ -f /work/") {
		t.Fatal("prefer -f check")
	}
}

func TestBuildOnlyPipelineScript_MinGWStagesSeedInvocation(t *testing.T) {
	script, err := BuildOnlyPipelineScript(PipelineScriptOptions{
		SourceDir:   "frida",
		ArtifactDir: "artifacts",
		TargetIDs:   []string{"windows-x86_64"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "seed-mingw-wraps.sh") {
		t.Fatal("windows target must invoke seed script")
	}
	if !strings.Contains(script, "/work/seed-mingw-wraps.sh") {
		t.Fatal("must reference staged /work path")
	}
	if !strings.Contains(script, "--without-prebuilds=sdk:host") {
		t.Fatal("mingw without-prebuilds missing")
	}
	// Must not pollute native CFLAGS with -include (breaks windows-x86 build-machine cc)
	if strings.Contains(script, `export CFLAGS="${CFLAGS:-} -include`) ||
		strings.Contains(script, "export CFLAGS=\"${CFLAGS:-} -include") {
		t.Fatal("must not export CFLAGS=-include for MinGW (pollutes build machine)")
	}
	if !strings.Contains(script, "frida-*-mingw.txt") && !strings.Contains(script, "MinGW cross-file") {
		// cross-file patch must appear
		if !strings.Contains(script, "fridare-mingw-dns.h") || !strings.Contains(script, "cross-file") {
			// accept python patch of mingw txt
			if !strings.Contains(script, "mingw.txt") {
				t.Fatal("must patch MinGW cross file for DNS include")
			}
		}
	}
	if !strings.Contains(script, MinGWDNSStubHeaderFileName) {
		t.Fatal("DNS header must be referenced for MinGW cross compile")
	}
}

func TestBuildOnlyPipelineScript_WindowsX86UsesMinGW(t *testing.T) {
	script, err := BuildOnlyPipelineScript(PipelineScriptOptions{
		SourceDir: "frida", ArtifactDir: "artifacts", TargetIDs: []string{"windows-x86"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "--host=i686-w64-mingw32") {
		t.Fatal(script)
	}
	if !strings.Contains(script, "i686-w64-mingw32-gcc") {
		t.Fatal("must check i686 mingw gcc present")
	}
}

func TestMinGWCrossFileDNSIncludeShell(t *testing.T) {
	sh := MinGWCrossFileDNSIncludeShell("")
	if !strings.Contains(sh, MinGWDNSStubHeaderFileName) {
		t.Fatal(sh)
	}
	if !strings.Contains(sh, "frida-*-mingw.txt") && !strings.Contains(sh, "mingw.txt") {
		t.Fatal("must target mingw cross files", sh)
	}
}

func TestSkipBroadModCandidate_Shared(t *testing.T) {
	if !skipBroadModCandidate("subprojects/x.o") {
		t.Fatal(".o should skip")
	}
	if skipBroadModCandidate("subprojects/frida-core/server/server.vala") {
		t.Fatal("vala should not skip")
	}
	if !isBroadModGlob("**/*") || !isBroadModGlob("src/**") {
		t.Fatal("broad glob detect")
	}
}
