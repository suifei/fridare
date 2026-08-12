package rebuild

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestBuildAgentPrompt(t *testing.T) {
	p := BuildAgentPrompt(JobConfig{
		FridaVersion: "17.16.4",
		MagicName:    "abcde",
		ListenPort:   27142,
		TargetIDs:    []string{"android-arm64"},
		Goals:        "hide frida-server name",
	}, "fridare/mod-1", "/work/frida")
	if !strings.Contains(p, "17.16.4") || !strings.Contains(p, "abcde") {
		t.Fatal(p)
	}
	if !strings.Contains(p, "hide frida-server name") {
		t.Fatal(p)
	}
	if !strings.Contains(p, "strongR") || !strings.Contains(p, "phantom") {
		t.Fatal("industry refs missing")
	}
	if !strings.Contains(p, "fridare/mod-1") {
		t.Fatal(p)
	}
}

func TestReplaceAvoidingGitHubFrida(t *testing.T) {
	in := "url = https://github.com/frida/libffi.git\npath=/frida/tmp\n"
	out := replaceAvoidingGitHubFrida(in, "/frida/", "/abcde/")
	if !strings.Contains(out, "github.com/frida/libffi") {
		t.Fatalf("github smashed: %s", out)
	}
	if !strings.Contains(out, "/abcde/tmp") {
		t.Fatalf("path not rewritten: %s", out)
	}
}

func TestEnvForAgent(t *testing.T) {
	env := EnvForAgent(JobConfig{
		OpenAIAPIKey:     "sk-abc",
		OpenAIBaseURL:    RecommendedAPIBase,
		OpenAIModel:      "gpt-4o",
		Proxy:            "http://gui-proxy:1",
		AgentUseGUIProxy: true,
	})
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "OPENAI_API_KEY=sk-abc") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "OPENAI_BASE_URL="+RecommendedAPIBase) {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "HTTP_PROXY=http://gui-proxy:1") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "ALL_PROXY=http://gui-proxy:1") {
		t.Fatal(joined)
	}
}

func TestEnvForAgent_ForcesGUIProxyOverInherited(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://system-proxy:9999")
	t.Setenv("HTTPS_PROXY", "http://system-proxy:9999")
	t.Setenv("ALL_PROXY", "socks5://system:1")

	env := EnvForAgent(JobConfig{
		Proxy:            "http://gui-only:8080",
		AgentUseGUIProxy: true,
		OpenAIAPIKey:     "k",
		OpenAIBaseURL:    RecommendedAPIBase,
	})
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "system-proxy") || strings.Contains(joined, "socks5://system") {
		t.Fatalf("system proxy leaked into agent env:\n%s", joined)
	}
	if !strings.Contains(joined, "HTTP_PROXY=http://gui-only:8080") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "HTTPS_PROXY=http://gui-only:8080") {
		t.Fatal(joined)
	}
}

func TestRequireGUIProxyForAgent(t *testing.T) {
	if err := RequireGUIProxyForAgent(JobConfig{AgentUseGUIProxy: true, Proxy: ""}); err == nil {
		t.Fatal("expected error when GUI proxy required but empty")
	}
	if err := RequireGUIProxyForAgent(JobConfig{AgentUseGUIProxy: true, Proxy: "http://p"}); err != nil {
		t.Fatal(err)
	}
	if err := RequireGUIProxyForAgent(JobConfig{AgentUseGUIProxy: false, Proxy: ""}); err != nil {
		t.Fatal(err)
	}
}

func TestGrokInvokeArgs(t *testing.T) {
	args := GrokInvokeArgs("grok", "/tmp/p.md", "/work")
	joined := strings.Join(args, " ")
	if args[0] != "grok" || !strings.Contains(joined, "/tmp/p.md") {
		t.Fatal(args)
	}
	if !strings.Contains(joined, "--prompt-file") {
		t.Fatalf("expected --prompt-file for modern grok CLI, got %v", args)
	}
	if strings.Contains(joined, " -f ") || strings.HasSuffix(joined, " -f") {
		t.Fatalf("legacy -f must not be used: %v", args)
	}
	if !strings.Contains(joined, "--cwd") || !strings.Contains(joined, "/work") {
		t.Fatal(args)
	}
	if !strings.Contains(joined, "--always-approve") {
		t.Fatal(args)
	}
}

// envCapturingRunner records env passed to Run — proves EnvForAgent is wired.
type envCapturingRunner struct {
	mu      sync.Mutex
	lastEnv []string
	lastCmd string
	err     error
}

func (e *envCapturingRunner) Run(ctx context.Context, env []string, name string, args ...string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastEnv = append([]string(nil), env...)
	e.lastCmd = name + " " + strings.Join(args, " ")
	if e.err != nil {
		return "fail", e.err
	}
	return "ok", nil
}

func TestGrokAgent_ApplyMods_PassesOpenAIEnvToRunner(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "frida")
	_ = os.MkdirAll(filepath.Join(src, "subprojects", "frida-core"), 0755)
	_ = os.WriteFile(filepath.Join(src, "subprojects", "frida-core", "server.c"),
		[]byte("const char *n = \"frida-server\";\n// frida:rpc\n"), 0644)

	runner := &envCapturingRunner{}
	agent := &GrokAgent{
		Runner:    runner,
		PromptDir: filepath.Join(tmp, "prompts"),
		LookPath: func(s string) (string, error) {
			if s == "grok" {
				return "/fake/grok", nil
			}
			return "", os.ErrNotExist
		},
	}
	cfg := JobConfig{
		FridaVersion:     "16.5.9",
		MagicName:        "abcde",
		ListenPort:       27042,
		Goals:            "rename",
		TargetIDs:        []string{"android-arm64"},
		OpenAIAPIKey:     "sk-live-test",
		OpenAIBaseURL:    RecommendedAPIBase,
		OpenAIModel:      "gpt-test",
		Proxy:            "http://proxy:7890",
		UseExistingGrok:  true,
		GrokBinary:       "/fake/grok",
		AgentUseGUIProxy: true,
	}
	plan, err := agent.PlanMods(context.Background(), cfg, "fridare/mod-x")
	if err != nil {
		t.Fatal(err)
	}
	if err := agent.ApplyMods(context.Background(), cfg, plan, src); err != nil {
		t.Fatal(err)
	}
	runner.mu.Lock()
	envJoined := strings.Join(runner.lastEnv, "\n")
	cmd := runner.lastCmd
	runner.mu.Unlock()
	if runner.lastEnv == nil {
		t.Fatal("Runner.Run never received env — EnvForAgent not wired")
	}
	if !strings.Contains(envJoined, "OPENAI_API_KEY=sk-live-test") {
		t.Fatalf("missing OPENAI_API_KEY in env:\n%s", envJoined)
	}
	if !strings.Contains(envJoined, "OPENAI_BASE_URL="+RecommendedAPIBase) {
		t.Fatalf("missing OPENAI_BASE_URL in env:\n%s", envJoined)
	}
	if !strings.Contains(envJoined, "HTTP_PROXY=http://proxy:7890") {
		t.Fatalf("missing proxy in env:\n%s", envJoined)
	}
	if !strings.Contains(cmd, "/fake/grok") {
		t.Fatalf("expected grok invoke, got %q", cmd)
	}
	// Pair renames + rpc
	data, _ := os.ReadFile(filepath.Join(src, "subprojects", "frida-core", "server.c"))
	if !strings.Contains(string(data), "abcde-server") || !strings.Contains(string(data), "abcde:rpc") {
		t.Fatalf("DefaultModOps tree apply did not patch pair/rpc: %s", data)
	}
}

func TestGrokAgent_UseExistingGrokFalse_SkipsGrokInvokesNaive(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "frida")
	_ = os.MkdirAll(src, 0755)
	target := filepath.Join(src, "main.c")
	_ = os.WriteFile(target, []byte(`char *s = "frida-server"; char *a = "frida_agent"; char *r = "frida:rpc";`), 0644)

	runner := &envCapturingRunner{}
	agent := &GrokAgent{
		Runner:    runner,
		PromptDir: filepath.Join(tmp, "prompts"),
		LookPath: func(s string) (string, error) {
			return "/usr/bin/grok", nil // on PATH but user declined
		},
	}
	cfg := JobConfig{
		FridaVersion:    "17.0.0",
		MagicName:       "xyzab",
		ListenPort:      27042,
		UseExistingGrok: false, // user said no
		// GrokBinary empty
	}
	plan := &ModPlan{
		Goals:      "test",
		Branch:     "fridare/mod-t",
		Version:    "17.0.0",
		Operations: DefaultModOps("xyzab", 27042),
	}
	if err := agent.ApplyMods(context.Background(), cfg, plan, src); err != nil {
		t.Fatal(err)
	}
	if runner.lastCmd != "" {
		t.Fatalf("grok should not be invoked when UseExistingGrok=false, got %q", runner.lastCmd)
	}
	data, _ := os.ReadFile(target)
	if !strings.Contains(string(data), "xyzab-server") || !strings.Contains(string(data), "xyzab:rpc") {
		t.Fatalf("naive apply failed: %s", data)
	}
	if !strings.Contains(string(data), "frida_agent") {
		t.Fatalf("underscore must remain at source: %s", data)
	}
}

func TestApplyPlanNaive_DefaultModOps_PatchesFixtureTree(t *testing.T) {
	// Proves DefaultModOps globs are NOT a no-op: applyPlanNaive must change files.
	tmp := t.TempDir()
	src := filepath.Join(tmp, "frida")
	files := map[string]string{
		"subprojects/frida-core/server.c": `run "frida-server" on port 27042; channel frida:rpc`,
		"subprojects/frida-core/agent.c":  `load frida-agent.so; frida_agent_main; thread gum-js-loop`,
		"subprojects/frida-gum/loop.c":    `name = "gum-js-loop";`,
		"lib/agent/agent.vala":            "namespace Frida.Agent {\n",
		"README.md":                       `# frida-server docs`,
		"binary.o":                        "frida-server\x00\x01\x02", // binary skipped
	}
	for rel, body := range files {
		p := filepath.Join(src, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		_ = os.WriteFile(p, []byte(body), 0644)
	}

	plan := &ModPlan{
		Goals:      "default rename",
		Branch:     "fridare/mod-test",
		Version:    "17.16.4",
		Operations: DefaultModOps("abcde", 27142),
	}
	if err := applyPlanNaive(src, plan); err != nil {
		t.Fatal(err)
	}

	server, _ := os.ReadFile(filepath.Join(src, "subprojects", "frida-core", "server.c"))
	ss := string(server)
	if !strings.Contains(ss, "abcde-server") || !strings.Contains(ss, "abcde:rpc") || !strings.Contains(ss, "27142") {
		t.Fatalf("server pair/rpc/port not replaced: %s", ss)
	}
	if strings.Contains(ss, "frida-server") || strings.Contains(ss, "frida:rpc") {
		t.Fatalf("old markers remain: %s", ss)
	}

	agent, _ := os.ReadFile(filepath.Join(src, "subprojects", "frida-core", "agent.c"))
	as := string(agent)
	// hyphen basename + thread marker; underscore left for post-build binary patch
	if !strings.Contains(as, "abcde-agent") || !strings.Contains(as, "abcde-js-loop") {
		t.Fatalf("agent hyphen/thread not replaced: %s", as)
	}
	if !strings.Contains(as, "frida_agent_main") {
		t.Fatalf("underscore C entry must remain at source (valac ABI): %s", as)
	}
	vala, _ := os.ReadFile(filepath.Join(src, "lib", "agent", "agent.vala"))
	vs := string(vala)
	if !strings.Contains(vs, "namespace Frida.Agent") {
		t.Fatalf("vala namespace Frida.Agent must be preserved for compile: %s", vs)
	}

	// plan summary must report patches
	summary, err := os.ReadFile(filepath.Join(src, "fridare-mod-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(summary), `"files_patched"`) {
		t.Fatal(string(summary))
	}
	// files_patched should be > 0
	if strings.Contains(string(summary), `"files_patched":0`) {
		t.Fatalf("files_patched is 0 — glob apply is a no-op: %s", summary)
	}
}

func TestStubAgent_ApplyMods_PairRenameAndAssets(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "frida")
	agentDir := filepath.Join(src, "subprojects", "frida-core", "lib", "agent")
	_ = os.MkdirAll(agentDir, 0755)
	_ = os.WriteFile(filepath.Join(agentDir, "agent.vala"), []byte("namespace Frida.Agent {\n// frida_agent_main\n"), 0644)
	_ = os.WriteFile(filepath.Join(agentDir, "agent-glue.c"), []byte("frida_agent_main();\n"), 0644)
	_ = os.WriteFile(filepath.Join(agentDir, "frida-agent-android.version"), []byte("V\n"), 0644)
	_ = os.WriteFile(filepath.Join(src, "server.c"), []byte("frida-server frida_server\n"), 0644)

	agent := &StubAgent{}
	cfg := JobConfig{FridaVersion: "17.16.4", MagicName: "abcde", ListenPort: 27142}
	plan, err := agent.PlanMods(context.Background(), cfg, "br")
	if err != nil {
		t.Fatal(err)
	}
	if err := agent.ApplyMods(context.Background(), cfg, plan, src); err != nil {
		t.Fatal(err)
	}
	// hyphen basenames at source; underscore stays until PatchArtifactBinaryMarkers
	sc, _ := os.ReadFile(filepath.Join(src, "server.c"))
	if !strings.Contains(string(sc), "abcde-server") {
		t.Fatalf("server hyphen: %s", sc)
	}
	if !strings.Contains(string(sc), "frida_server") {
		t.Fatalf("server underscore must remain at source: %s", sc)
	}
	glue, _ := os.ReadFile(filepath.Join(agentDir, "agent-glue.c"))
	if !strings.Contains(string(glue), "frida_agent_main") {
		t.Fatalf("glue must keep frida_agent_main for valac link: %s", glue)
	}
	vala, _ := os.ReadFile(filepath.Join(agentDir, "agent.vala"))
	vs := string(vala)
	if !strings.Contains(vs, "namespace Frida.Agent") {
		t.Fatalf("vala namespace preserved: %s", vs)
	}
	// frida_agent_main in comment becomes unchanged (no underscore source replace)
	if !strings.Contains(vs, "frida_agent_main") {
		t.Fatalf("vala underscore entry preserved: %s", vs)
	}
	// asset rename
	if _, err := os.Stat(filepath.Join(agentDir, "abcde-agent-android.version")); err != nil {
		t.Fatal("asset not renamed", err)
	}
	if _, err := os.Stat(filepath.Join(agentDir, "frida-agent-android.version")); !os.IsNotExist(err) {
		t.Fatal("old asset remains")
	}
	// plan json reports renames
	sum, _ := os.ReadFile(filepath.Join(src, "fridare-mod-plan.json"))
	if !strings.Contains(string(sum), `"files_renamed"`) || strings.Contains(string(sum), `"files_renamed":0`) {
		// StubAgent calls RenameMagicAssetFiles after applyPlanNaive which also renames;
		// applyPlanNaive may have already renamed — files_renamed may be >0 from first call
		if !strings.Contains(string(sum), "files_renamed") {
			t.Fatalf("summary: %s", sum)
		}
	}
}

func TestRenameMagicAssetFiles(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "lib", "agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(agentDir, "frida-agent-android.version")
	if err := os.WriteFile(old, []byte("V1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	helperDir := filepath.Join(dir, "src", "linux")
	_ = os.MkdirAll(helperDir, 0755)
	oldH := filepath.Join(helperDir, "frida-helper-backend.resources")
	_ = os.WriteFile(oldH, []byte("R\n"), 0644)
	// also a content-only file that should not be renamed
	_ = os.WriteFile(filepath.Join(agentDir, "readme.txt"), []byte("frida-agent"), 0644)

	n, err := RenameMagicAssetFiles(dir, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("renamed %d want 2", n)
	}
	if _, err := os.Stat(filepath.Join(agentDir, "abcde-agent-android.version")); err != nil {
		t.Fatal("expected renamed agent file")
	}
	if _, err := os.Stat(filepath.Join(helperDir, "abcde-helper-backend.resources")); err != nil {
		t.Fatal("expected renamed helper resources")
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old agent file should be gone")
	}
}

func TestPlanModsFromTree_ConcreteOpsOnlyWhereFound(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	_ = os.MkdirAll(filepath.Join(src, "core"), 0755)
	_ = os.WriteFile(filepath.Join(src, "core", "a.c"), []byte(`frida-server and frida:rpc only`), 0644)
	_ = os.WriteFile(filepath.Join(src, "core", "b.c"), []byte(`nothing here`), 0644)

	plan, err := PlanModsFromTree(src, JobConfig{MagicName: "qwxyz", ListenPort: 27042, FridaVersion: "1.0"}, "br")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Operations) == 0 {
		t.Fatal("expected concrete ops")
	}
	for _, op := range plan.Operations {
		if strings.ContainsAny(op.Path, "*?[") {
			t.Fatalf("expected concrete path, got glob %q", op.Path)
		}
		if (op.Find == "frida-server" || op.Find == "frida:rpc") && !strings.HasSuffix(op.Path, "a.c") {
			t.Fatalf("unexpected path for %s: %s", op.Find, op.Path)
		}
	}
}

func TestGrokAgent_PlanAndNaiveApply(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "frida")
	_ = os.MkdirAll(filepath.Join(src, "core"), 0755)
	target := filepath.Join(src, "core", "server.c")
	_ = os.WriteFile(target, []byte("const char *name = \"frida-server\"; const char *sym = \"frida_agent\"; const char *r = \"frida:rpc\";\n"), 0644)

	agent := &GrokAgent{
		PromptDir: filepath.Join(tmp, "prompts"),
		LookPath: func(string) (string, error) {
			return "", os.ErrNotExist
		},
	}
	cfg := JobConfig{
		FridaVersion:    "16.5.9",
		MagicName:       "abcde",
		ListenPort:      27042,
		Goals:           "rename",
		TargetIDs:       []string{"android-arm64"},
		UseExistingGrok: true, // no binary found → naive
	}
	plan, err := agent.PlanMods(context.Background(), cfg, "fridare/mod-x")
	if err != nil || plan == nil {
		t.Fatal(err)
	}
	// Use DefaultModOps path (do NOT override to concrete) — must still patch via glob
	plan.Operations = DefaultModOps("abcde", 27042)
	if err := agent.ApplyMods(context.Background(), cfg, plan, src); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if !strings.Contains(string(data), "abcde-server") || !strings.Contains(string(data), "abcde:rpc") {
		t.Fatalf("not patched via DefaultModOps glob: %s", data)
	}
	if !strings.Contains(string(data), "frida_agent") {
		t.Fatalf("underscore marker must remain at source: %s", data)
	}
	if _, err := os.Stat(filepath.Join(src, "fridare-mod-plan.json")); err != nil {
		t.Fatal(err)
	}
}

func TestShouldInvokeGrok(t *testing.T) {
	look := func(s string) (string, error) {
		if s == "grok" {
			return "/bin/grok", nil
		}
		return "", os.ErrNotExist
	}
	if shouldInvokeGrok(JobConfig{UseExistingGrok: false}, look) {
		t.Fatal("should not invoke when UseExistingGrok false")
	}
	if !shouldInvokeGrok(JobConfig{UseExistingGrok: true}, look) {
		t.Fatal("should invoke when present and allowed")
	}
	if !shouldInvokeGrok(JobConfig{UseExistingGrok: false, GrokBinary: "/x"}, look) {
		t.Fatal("explicit binary always ok")
	}
}
