package rebuild

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	mu      sync.Mutex
	log     []string
	fail    map[string]error
	lastEnv []string
}

func (f *fakeRunner) Run(ctx context.Context, env []string, name string, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	line := name + " " + strings.Join(args, " ")
	f.log = append(f.log, line)
	// capture env keys for assertions
	if env != nil {
		f.lastEnv = append([]string(nil), env...)
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	for k, err := range f.fail {
		if strings.Contains(line, k) {
			return "", err
		}
	}
	return "ok", nil
}

func TestOrchestratorDryRun_StageOrderAndArtifacts(t *testing.T) {
	tmp := t.TempDir()
	runner := &fakeRunner{}
	var planned, applied bool
	agent := &StubAgent{
		OnPlan: func(ctx context.Context, cfg JobConfig, branch string) (*ModPlan, error) {
			planned = true
			if !strings.HasPrefix(branch, "fridare/mod-") {
				t.Fatalf("branch %s", branch)
			}
			return &ModPlan{
				Goals:      cfg.Goals,
				Branch:     branch,
				Version:    cfg.FridaVersion,
				Operations: DefaultModOps(cfg.MagicName, cfg.ListenPort),
			}, nil
		},
		OnApply: func(ctx context.Context, cfg JobConfig, plan *ModPlan, sourceDir string) error {
			applied = true
			if plan == nil || len(plan.Operations) == 0 {
				t.Fatal("empty plan")
			}
			return (&StubAgent{}).ApplyMods(ctx, cfg, plan, sourceDir)
		},
	}
	orch := NewOrchestrator(runner, agent)
	cfg := JobConfig{
		FridaVersion:  "17.16.4",
		TargetIDs:     []string{"android-arm64"},
		MagicName:     "abcde",
		ListenPort:    27142,
		Goals:         "rename server identifiers",
		WorkDir:       tmp,
		ArtifactDir:   filepath.Join(tmp, "out"),
		Proxy:         "http://127.0.0.1:7890",
		OpenAIBaseURL: RecommendedAPIBase,
		OpenAIAPIKey:  "sk-test",
		DryRun:        true,
	}
	if err := orch.RunSync(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st := orch.State()
	if st.Stage != StageDone {
		t.Fatalf("stage=%s err=%s events=%v", st.Stage, st.Error, st.Events)
	}
	if !planned || !applied {
		t.Fatalf("planned=%v applied=%v", planned, applied)
	}
	// Dry-run must still write Docker-only policy + build-only script for real path
	if _, err := os.Stat(filepath.Join(tmp, "src", "COMPILE-IN-DOCKER-ONLY.txt")); err != nil {
		// workRoot is cfg.WorkDir directly as src under it
		if _, err2 := os.Stat(filepath.Join(tmp, "COMPILE-IN-DOCKER-ONLY.txt")); err2 != nil {
			// srcHost = workRoot/src when WorkDir is work root
			found := false
			_ = filepath.Walk(tmp, func(path string, info os.FileInfo, err error) error {
				if info != nil && info.Name() == "COMPILE-IN-DOCKER-ONLY.txt" {
					found = true
				}
				if info != nil && info.Name() == "build-only.sh" {
					data, _ := os.ReadFile(path)
					if !strings.Contains(string(data), DockerBuildStageMarker) {
						t.Errorf("build-only.sh missing docker marker")
					}
				}
				return nil
			})
			if !found {
				t.Fatal("COMPILE-IN-DOCKER-ONLY.txt missing")
			}
		}
	}
	// Stage sequence includes clone → branch → mod → build
	var stages []JobStage
	for _, ev := range st.Events {
		if len(stages) == 0 || stages[len(stages)-1] != ev.Stage {
			stages = append(stages, ev.Stage)
		}
	}
	need := []JobStage{StageProbe, StageBootstrap, StageClone, StageBranch, StageModPlan, StageModApply, StageBuild, StageExport, StageToolsPatch, StageDone}
	idx := 0
	for _, n := range need {
		found := false
		for i := idx; i < len(stages); i++ {
			if stages[i] == n {
				found = true
				idx = i + 1
				break
			}
		}
		if !found {
			t.Fatalf("missing stage %s in %v", n, stages)
		}
	}
	// Artifacts + tips
	if st.Artifact == "" {
		t.Fatal("artifact dir empty")
	}
	if _, err := os.Stat(filepath.Join(st.Artifact, "README-DEPLOY.txt")); err != nil {
		t.Fatal(err)
	}
	// Catalog entry should have README / python install placeholder after dry-run tools stage
	if _, err := os.Stat(filepath.Join(st.Artifact, "README.txt")); err != nil {
		if _, err2 := os.Stat(filepath.Join(st.Artifact, "README-DEPLOY.txt")); err2 != nil {
			t.Fatalf("missing catalog readme: %v / %v", err, err2)
		}
	}
	if _, err := os.Stat(filepath.Join(st.Artifact, "python")); err != nil {
		t.Fatalf("missing python dir in catalog: %v", err)
	}
	// pipeline script written
	if _, err := os.Stat(filepath.Join(tmp, "src", "pipeline.sh")); err != nil {
		// work dir structure: WorkDir/src/pipeline.sh
		// Default uses WorkDir as root with src under it — check
		matches, _ := filepath.Glob(filepath.Join(tmp, "**", "pipeline.sh"))
		if len(matches) == 0 {
			// explicit path
			p := filepath.Join(tmp, "src", "pipeline.sh")
			if _, err2 := os.Stat(p); err2 != nil {
				// list tmp for debug
				_ = filepath.Walk(tmp, func(path string, info os.FileInfo, err error) error {
					t.Log("path", path)
					return nil
				})
				t.Fatalf("pipeline.sh missing: %v", err)
			}
		}
	}
}

func TestOrchestratorCancel(t *testing.T) {
	tmp := t.TempDir()
	// Agent blocks until cancelled
	block := make(chan struct{})
	agent := &StubAgent{
		OnPlan: func(ctx context.Context, cfg JobConfig, branch string) (*ModPlan, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-block:
				return &ModPlan{Branch: branch, Version: cfg.FridaVersion}, nil
			case <-time.After(10 * time.Second):
				return nil, context.DeadlineExceeded
			}
		},
	}
	orch := NewOrchestrator(&fakeRunner{}, agent)
	cfg := JobConfig{
		FridaVersion: "16.5.9",
		TargetIDs:    []string{"android-arm64"},
		MagicName:    "zzzzz",
		WorkDir:      tmp,
		Proxy:        "http://p",
		DryRun:       true,
		Goals:        "x",
	}
	if err := orch.Start(cfg, nil); err != nil {
		t.Fatal(err)
	}
	// Wait until we reach mod_plan
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st := orch.State()
		if st.Stage == StageModPlan || st.Stage == StageModApply {
			break
		}
		if st.Stage == StageFailed || st.Stage == StageCancelled || st.Stage == StageDone {
			t.Fatalf("ended early: %+v", st)
		}
		time.Sleep(20 * time.Millisecond)
	}
	orch.Cancel()
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st := orch.State()
		if st.Stage == StageCancelled || st.Stage == StageFailed {
			// cancelled is success
			if st.Stage == StageCancelled {
				orch.Reset()
				if orch.State().Stage != StageIdle {
					t.Fatalf("reset: %s", orch.State().Stage)
				}
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("cancel not observed: %+v", orch.State())
}

func TestDefaultModOps(t *testing.T) {
	ops := DefaultModOps("abcde", 27142)
	if len(ops) < 3 {
		t.Fatal(ops)
	}
	foundRPC := false
	foundPairHyphen := false
	foundPort := false
	// Underscore / Vala namespace renames must NOT be in source ops (break valac ABI).
	for _, op := range ops {
		if op.Find == "frida:rpc" && op.Replace == "abcde:rpc" {
			foundRPC = true
		}
		if op.Find == "frida-server" && op.Replace == "abcde-server" {
			foundPairHyphen = true
		}
		if op.Find == "frida-policyd" && op.Replace == "abcde-policyd" {
			foundPairHyphen = true
		}
		if strings.Contains(op.Find, "DEFAULT_CONTROL_PORT") && strings.Contains(op.Replace, "27142") {
			foundPort = true
		}
		if op.Find == OfficialListenPortASCII && op.Path == "**/*" {
			t.Fatalf("listen port must not be a tree-wide 27042 replace: %+v", op)
		}
		if op.Find == "frida_agent" || op.Find == "namespace Frida.Agent" {
			t.Fatalf("source ops must not include underscore/vala-namespace renames: %+v", op)
		}
	}
	if !foundRPC {
		t.Fatal("rpc op missing")
	}
	if !foundPairHyphen {
		t.Fatal("hyphen basename renames required")
	}
	if !foundPort {
		t.Fatal("DEFAULT_CONTROL_PORT op missing")
	}
}

func TestArtifactDeployTips(t *testing.T) {
	tips := ArtifactDeployTips("/tmp/out", JobConfig{
		FridaVersion: "17.0.0",
		MagicName:    "abcde",
		ListenPort:   27042,
		TargetIDs:    []string{"android-arm64"},
	})
	if !strings.Contains(tips, "abcde") || !strings.Contains(tips, "whl") || !strings.Contains(tips, "27042") {
		t.Fatal(tips)
	}
	if !strings.Contains(tips, "stealth:") || !strings.Contains(tips, "免杀") {
		t.Fatal(tips)
	}
	g := ToolsPatchGuidance("abcde", 27042)
	if !strings.Contains(g, "frida-tools 魔改") {
		t.Fatal(g)
	}
}

func TestStageIndex(t *testing.T) {
	if StageIndex(StageClone) < 0 {
		t.Fatal("clone")
	}
	if StageIndex(StageCancelled) != -1 {
		t.Fatal("cancelled not in ordered")
	}
}
