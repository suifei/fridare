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
