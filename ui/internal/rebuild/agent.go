package rebuild

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// textFileExts are extensions considered for tree-wide text replace (source mods).
var textFileExts = map[string]bool{
	".c": true, ".cc": true, ".cpp": true, ".cxx": true, ".h": true, ".hh": true, ".hpp": true,
	".m": true, ".mm": true, ".vala": true, ".vapi": true, ".py": true, ".js": true, ".ts": true,
	".json": true, ".xml": true, ".plist": true, ".in": true, ".ac": true, ".am": true,
	".meson": true, ".build": true, ".txt": true, ".md": true, ".cmake": true, ".sh": true,
	".rc": true, ".def": true, ".s": true, ".S": true, ".rs": true, ".go": true,
	".java": true, ".kt": true, ".swift": true, ".nib": false,
}

// GrokAgent invokes the grok-build / grok CLI with GUI-provided OpenAI settings.
type GrokAgent struct {
	// Runner executes CLI commands (must honor env for OpenAI/proxy).
	Runner Runner
	// LookPath resolves binaries; defaults inside ResolveGrokBinary.
	LookPath func(string) (string, error)
	// PromptDir stores generated prompts for transparency / resume.
	PromptDir string
}

// EnvForAgent returns environment variables so grok-build uses GUI OpenAI config.
// Common patterns: OPENAI_API_KEY, OPENAI_BASE_URL / OPENAI_API_BASE.
//
// When cfg.AgentUseGUIProxy is true, host AI agent outbound traffic is forced
// through the GUI-configured proxy (cfg.Proxy): inherited system HTTP(S)_PROXY /
// ALL_PROXY are replaced so the agent cannot silently bypass GUI settings.
// When false (product default), GUI proxy is NOT injected — system proxy env is left alone.
func EnvForAgent(cfg JobConfig) []string {
	env := os.Environ()
	if cfg.OpenAIAPIKey != "" {
		env = setEnv(env, "OPENAI_API_KEY", cfg.OpenAIAPIKey)
		env = setEnv(env, "OPENAI_KEY", cfg.OpenAIAPIKey)
	}
	if cfg.OpenAIBaseURL != "" {
		env = setEnv(env, "OPENAI_BASE_URL", cfg.OpenAIBaseURL)
		env = setEnv(env, "OPENAI_API_BASE", cfg.OpenAIBaseURL)
		env = setEnv(env, "OPENAI_API_BASE_URL", cfg.OpenAIBaseURL)
	}
	if cfg.OpenAIModel != "" {
		env = setEnv(env, "OPENAI_MODEL", cfg.OpenAIModel)
		env = setEnv(env, "GROK_MODEL", cfg.OpenAIModel)
	}
	if cfg.AgentUseGUIProxy {
		// Force GUI proxy as the only egress for the host agent process.
		proxy := strings.TrimSpace(cfg.Proxy)
		// Clear inherited proxies first so system env cannot override GUI.
		env = unsetEnv(env, "HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy",
			"ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy")
		if proxy != "" {
			env = setEnv(env, "HTTP_PROXY", proxy)
			env = setEnv(env, "HTTPS_PROXY", proxy)
			env = setEnv(env, "http_proxy", proxy)
			env = setEnv(env, "https_proxy", proxy)
			env = setEnv(env, "ALL_PROXY", proxy)
			env = setEnv(env, "all_proxy", proxy)
		}
	}
	// When AgentUseGUIProxy is false: do not inject cfg.Proxy into the agent env.
	return env
}

// RequireGUIProxyForAgent returns an error when host agent is configured to use
// GUI proxy egress but proxy is empty.
func RequireGUIProxyForAgent(cfg JobConfig) error {
	if !cfg.AgentUseGUIProxy {
		return nil
	}
	if strings.TrimSpace(cfg.Proxy) == "" {
		return fmt.Errorf("Host AI agent 已勾选「使用 GUI 代理出口」，请先在设置/工具栏配置上游代理")
	}
	return nil
}

func unsetEnv(env []string, keys ...string) []string {
	drop := make(map[string]bool, len(keys))
	for _, k := range keys {
		drop[k] = true
	}
	out := make([]string, 0, len(env))
	for _, e := range env {
		eq := strings.IndexByte(e, '=')
		if eq <= 0 {
			out = append(out, e)
			continue
		}
		if drop[e[:eq]] {
			continue
		}
		out = append(out, e)
	}
	return out
}

func setEnv(env []string, key, val string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + val
			return env
		}
	}
	return append(env, prefix+val)
}

// BuildAgentPrompt constructs the system/user prompt for source modification.
func BuildAgentPrompt(cfg JobConfig, branch string, sourceDir string) string {
	var b strings.Builder
	b.WriteString("你是 Frida 源码魔改 agent（Fridare）。在已克隆的官方 Frida 源码树上执行可靠、可复现的修改。\n\n")
	b.WriteString("## 约束\n")
	b.WriteString("1. 当前 git 分支: " + branch + "（已创建，请在此分支上提交）\n")
	b.WriteString("2. Frida 版本: " + cfg.FridaVersion + "\n")
	b.WriteString("3. 源码目录: " + sourceDir + "\n")
	b.WriteString("4. 魔改名称(magic): " + cfg.MagicName + "（通常 5 个小写字母，与 frida-tools 对齐）\n")
	b.WriteString(fmt.Sprintf("5. 监听端口: %d（官方默认 %s，常量 DEFAULT_CONTROL_PORT）\n",
		NormalizeListenPort(cfg.ListenPort), OfficialListenPortASCII))
	b.WriteString("6. 编译目标: " + strings.Join(cfg.TargetIDs, ", ") + "\n")
	b.WriteString("7. 参考业界 strongR-frida / phantom-frida：改可观测标识（进程名、so 名、线程名、路径、rpc 等）。\n")
	prof := strings.ToLower(strings.TrimSpace(cfg.DirectionProfile))
	if prof == "deep" || prof == "abi" || prof == "full" || prof == "explore" {
		b.WriteString("7b. profile=" + prof + "：服务端与客户端必须同步魔改。改 re.frida.* → re." + cfg.MagicName +
			".、/re/frida/ → /re/" + cfg.MagicName + "/（DBus 对象路径，缺了会 UNKNOWN_METHOD）、frida:rpc → " +
			cfg.MagicName + ":rpc、\"Frida. → \"" + pascalMagic(cfg.MagicName) +
			".；流水线会同步打包 host wheel。stock 官方 pip 客户端将无法连接。\n")
	} else {
		b.WriteString("7b. profile=safe：尽量兼容 stock frida-tools；不要改 re.frida.* 协议面与公开 Frida.* API 字面量。\n")
	}
	b.WriteString("8. 不要浪费空间：不要额外 git clone 完整历史；不要下载无关大文件。\n")
	b.WriteString("9. 先扫描再改：按实际文件路径制定计划，兼容不同官方版本目录布局。\n")
	b.WriteString("10. 每改一批文件后 git add && git commit，便于回滚。\n")
	b.WriteString("11. 必须遵守下方「方向清单」：forbidden 层禁止写入可执行 replace；auto 层必须覆盖。\n\n")
	b.WriteString("## 用户目标（对话）\n")
	if strings.TrimSpace(cfg.Goals) == "" {
		b.WriteString("（未额外说明）默认：替换 frida 特征字符串/路径/线程名，magic=" + cfg.MagicName + "\n")
	} else {
		b.WriteString(cfg.Goals)
		b.WriteString("\n")
	}
	// Direction list gives the Agent a concrete dig direction (batch-tool aligned).
	dirMan := directionManifestForConfig(cfg)
	b.WriteString(DirectionGoalsPrompt(dirMan))
	b.WriteString(ListenPortAgentGuidance(cfg.ListenPort))
	b.WriteString("\n## 输出要求\n")
	b.WriteString("- 先输出 JSON 魔改计划：paths + operations（replace/insert/delete/rewrite）\n")
	b.WriteString("- 再在源码树执行修改；可选写入 fridare-agent-ops.tsv（path\\top\\tfind\\treplace）\n")
	b.WriteString("- 最后打印变更摘要\n")
	return b.String()
}

func directionManifestForConfig(cfg JobConfig) DirectionManifest {
	if p := strings.TrimSpace(cfg.DirectionFile); p != "" {
		if m, err := LoadDirectionManifest(p); err == nil && len(m.Layers) > 0 {
			return m
		}
	}
	switch strings.ToLower(strings.TrimSpace(cfg.DirectionProfile)) {
	case "deep", "abi", "full":
		return DeepDirectionManifest(cfg.MagicName, cfg.ListenPort)
	case "explore":
		return ExploreDirectionManifest(cfg.MagicName, cfg.ListenPort)
	default:
		return SafeDirectionManifest(cfg.MagicName, cfg.ListenPort)
	}
}

// GrokInvokeArgs builds CLI arguments for a non-interactive (headless) agent run.
// Matches current grok CLI: --prompt-file + --cwd + auto-approve for unattended apply.
// Older "-f" flag is not accepted by modern grok builds.
func GrokInvokeArgs(binary, promptFile, workDir string) []string {
	args := []string{binary, "--cwd", workDir, "--prompt-file", promptFile, "--always-approve", "--output-format", "plain"}
	return args
}

// PlanModsFromTree expands direction-profile auto ops (DefaultModOps) against a
// real source tree into concrete file-level operations (only files that actually
// contain the Find string). Writes DirectionManifest + optional strip scan JSON.
// This is the deterministic baseline used when AI is unavailable or for AI refine.
//
// Performance: broad globs (**/*) are resolved in a single tree walk (not one walk
// per op). Large Frida monorepos (100k+ files under subprojects) otherwise spend
// many minutes and are prone to host job-object kills during e2e runs.
func PlanModsFromTree(sourceDir string, cfg JobConfig, branch string) (*ModPlan, error) {
	dirMan := directionManifestForConfig(cfg)
	// Random dump-prefix ops must precede DefaultModOps (which rewrites frida-agent).
	baseline := prependRandomAgentOps(cfg, OpsFromDirectionManifest(dirMan))
	baseline = append(baseline, StealthBehaviorOps(cfg)...)
	var concrete []ModOp
	if sourceDir != "" {
		if st, err := os.Stat(sourceDir); err == nil && st.IsDir() {
			concrete = expandOpsAgainstTree(sourceDir, baseline)
		}
	}
	ops := concrete
	if len(ops) == 0 {
		ops = baseline
	}
	// Write directions / scan AFTER planning so metadata JSON is not itself "matched"
	if p := strings.TrimSpace(cfg.DirectionFile); p != "" {
		_ = WriteDirectionManifest(p, dirMan)
	} else if sourceDir != "" {
		_ = WriteDirectionManifest(filepath.Join(sourceDir, "fridare-directions.json"), dirMan)
	}
	if sourceDir != "" {
		if st, err := os.Stat(sourceDir); err == nil && st.IsDir() {
			if rep, err := ScanFridaMarkers(sourceDir, cfg.MagicName); err == nil {
				if b, e := json.MarshalIndent(rep, "", "  "); e == nil {
					_ = os.WriteFile(filepath.Join(sourceDir, "fridare-strip-scan.json"), append(b, '\n'), 0644)
				}
			}
		}
	}
	return &ModPlan{
		Goals:      MergeGoalsWithDirections(cfg.Goals, dirMan),
		Branch:     branch,
		Version:    cfg.FridaVersion,
		Operations: ops,
	}, nil
}

// expandOpsAgainstTree turns baseline ops into concrete per-file ops.
// Concrete paths are checked directly; glob ops share one Walk of sourceDir.
func expandOpsAgainstTree(sourceDir string, baseline []ModOp) []ModOp {
	var concrete []ModOp
	var globOps []ModOp
	for _, op := range baseline {
		if op.Find == "" {
			continue
		}
		pat := strings.TrimSpace(op.Path)
		if pat == "" {
			pat = "**/*"
		}
		pat = filepath.ToSlash(pat)
		if !strings.ContainsAny(pat, "*?[") {
			rel := pat
			base := filepath.Base(rel)
			if strings.HasPrefix(base, "fridare-") {
				continue
			}
			full := filepath.Join(sourceDir, filepath.FromSlash(rel))
			data, err := os.ReadFile(full)
			if err != nil || !strings.Contains(string(data), op.Find) {
				continue
			}
			concrete = append(concrete, ModOp{
				Path: rel, Operation: op.Operation, Description: op.Description,
				Find: op.Find, Replace: op.Replace,
			})
			continue
		}
		op.Path = pat
		globOps = append(globOps, op)
	}
	if len(globOps) == 0 {
		return concrete
	}
	// One walk for all globs: read each candidate file once, test all Finds.
	_ = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if shouldSkipModDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Size() > 8*1024*1024 {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		base := filepath.Base(relSlash)
		if strings.HasPrefix(base, "fridare-") {
			return nil
		}
		// Quick filter: only consider if any glob matches path
		var hits []ModOp
		for _, op := range globOps {
			if matchGlob(op.Path, relSlash) {
				if isBroadModGlob(op.Path) && skipBroadModCandidate(relSlash) {
					continue
				}
				hits = append(hits, op)
			}
		}
		if len(hits) == 0 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || isLikelyBinary(data) {
			return nil
		}
		s := string(data)
		for _, op := range hits {
			if !strings.Contains(s, op.Find) {
				continue
			}
			if isListenPortFind(op.Find) && skipNumericTableVendorPath(relSlash) {
				continue
			}
			concrete = append(concrete, ModOp{
				Path: relSlash, Operation: op.Operation, Description: op.Description,
				Find: op.Find, Replace: op.Replace,
			})
		}
		return nil
	})
	return concrete
}

// shouldSkipModDir returns true for directories that must not be walked during
// source-mod planning/apply (VCS, deps, non-core language bindings, tests).
func shouldSkipModDir(base string) bool {
	switch base {
	case ".git", "node_modules", "build", ".deps", "deps", "releng", ".github",
		"frida-clr", "frida-go", "frida-node", "frida-qml", "frida-swift",
		"frida-python", "frida-tools",
		"test", "tests", "examples", "docs",
		"brotli", "capstone", "xz", "lzma", "openssl", "zlib", "nghttp2":
		return true
	}
	if strings.HasPrefix(base, "build-") {
		return true
	}
	return false
}

// isBroadModGlob is true for tree-wide patterns that should skip binary-like files.
func isBroadModGlob(pat string) bool {
	return pat == "**/*" || pat == "**" || strings.HasSuffix(pat, "/**") || strings.Contains(pat, "/**/*")
}

// skipBroadModCandidate rejects paths that must not be planned under broad **/* globs.
// Shared by expandOpsAgainstTree and matchOpPaths so plan vs apply stay aligned.
func skipBroadModCandidate(relSlash string) bool {
	ext := strings.ToLower(filepath.Ext(relSlash))
	switch ext {
	case ".o", ".a", ".so", ".dll", ".exe", ".obj", ".png", ".jpg", ".jpeg",
		".zip", ".gz", ".tgz", ".7z", ".bin", ".pyc", ".pyo", ".wasm":
		return true
	}
	return false
}

// PlanMods implements AgentDriver.
// 1) Build tree-aware concrete plan from DefaultModOps scan
// 2) If UseExistingGrok (or explicit GrokBinary), invoke agent to refine plan file;
//    parse is best-effort — tree plan remains the reliable base.
func (g *GrokAgent) PlanMods(ctx context.Context, cfg JobConfig, branch string) (*ModPlan, error) {
	// sourceDir is not in signature; orchestrator plans before we know clone dest
	// relative to work dir. Prefer empty scan → baseline, ApplyMods will re-scan.
	// When PromptDir parent has src/frida, use it.
	sourceHint := ""
	if g.PromptDir != "" {
		cand := filepath.Join(filepath.Dir(g.PromptDir), "src", "frida")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			sourceHint = cand
		}
	}
	plan, err := PlanModsFromTree(sourceHint, cfg, branch)
	if err != nil {
		return nil, err
	}

	if g.PromptDir != "" {
		_ = os.MkdirAll(g.PromptDir, 0755)
		path := filepath.Join(g.PromptDir, "mod-plan-baseline.txt")
		var b strings.Builder
		b.WriteString(fmt.Sprintf("branch=%s version=%s\n", branch, cfg.FridaVersion))
		b.WriteString(fmt.Sprintf("goals=%s\n", cfg.Goals))
		b.WriteString(fmt.Sprintf("ops=%d (tree-refined=%v)\n", len(plan.Operations), sourceHint != ""))
		for i, op := range plan.Operations {
			if i >= 50 {
				b.WriteString(fmt.Sprintf("... +%d more\n", len(plan.Operations)-50))
				break
			}
			b.WriteString(fmt.Sprintf("%d. [%s] %s find=%q → %q — %s\n",
				i+1, op.Operation, op.Path, op.Find, op.Replace, op.Description))
		}
		_ = os.WriteFile(path, []byte(b.String()), 0644)
	}

	// Optional: ask grok to refine plan (does not replace tree plan if agent fails)
	if shouldInvokeGrok(cfg, g.LookPath) {
		if refined := g.tryAgentRefinePlan(ctx, cfg, branch, sourceHint, plan); refined != nil && len(refined.Operations) > 0 {
			plan = refined
		}
	}
	return plan, nil
}

func shouldInvokeGrok(cfg JobConfig, lookPath func(string) (string, error)) bool {
	if strings.TrimSpace(cfg.GrokBinary) != "" {
		return true // explicit binary always allowed
	}
	if !cfg.UseExistingGrok {
		return false
	}
	_, ok := ResolveGrokBinary(lookPath)
	return ok
}

func resolveGrokBinary(cfg JobConfig, lookPath func(string) (string, error)) (string, bool) {
	if b := strings.TrimSpace(cfg.GrokBinary); b != "" {
		return b, true
	}
	if !cfg.UseExistingGrok {
		return "", false
	}
	return ResolveGrokBinary(lookPath)
}

// tryAgentRefinePlan writes a plan prompt and runs grok with EnvForAgent; on success
// keeps the original plan unless agent writes a parseable ops file (fridare-plan-ops.txt).
func (g *GrokAgent) tryAgentRefinePlan(ctx context.Context, cfg JobConfig, branch, sourceDir string, base *ModPlan) *ModPlan {
	binary, ok := resolveGrokBinary(cfg, g.LookPath)
	if !ok {
		return nil
	}
	promptDir := g.PromptDir
	if promptDir == "" {
		promptDir = filepath.Join(os.TempDir(), "fridare-agent")
	}
	_ = os.MkdirAll(promptDir, 0755)
	prompt := BuildAgentPrompt(cfg, branch, sourceDir)
	prompt += "\n## 当前基线计划（请 refinement 并写入工作区 fridare-agent-ops.tsv 每行: path\\top\\tfind\\treplace）\n"
	for i, op := range base.Operations {
		if i >= 30 {
			break
		}
		prompt += fmt.Sprintf("%s\t%s\t%s\t%s\n", op.Path, op.Operation, op.Find, op.Replace)
	}
	promptFile := filepath.Join(promptDir, fmt.Sprintf("plan-%d.md", time.Now().UnixNano()))
	_ = os.WriteFile(promptFile, []byte(prompt), 0644)

	if g.Runner == nil {
		g.Runner = ExecRunner{}
	}
	env := EnvForAgent(cfg)
	args := GrokInvokeArgs(binary, promptFile, sourceDir)
	if sourceDir == "" {
		args = GrokInvokeArgs(binary, promptFile, promptDir)
	}
	out, err := g.Runner.Run(ctx, env, args[0], args[1:]...)
	_ = os.WriteFile(filepath.Join(promptDir, "plan-agent-output.txt"), []byte(out), 0644)
	if err != nil {
		return nil
	}
	// Try parse agent ops file from source or prompt dir
	for _, dir := range []string{sourceDir, promptDir} {
		if dir == "" {
			continue
		}
		if p := parseAgentOpsFile(filepath.Join(dir, "fridare-agent-ops.tsv"), base); p != nil {
			return p
		}
	}
	return nil
}

func parseAgentOpsFile(path string, base *ModPlan) *ModPlan {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	var ops []ModOp
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		ops = append(ops, ModOp{
			Path:      parts[0],
			Operation: parts[1],
			Find:      parts[2],
			Replace:   parts[3],
		})
	}
	if len(ops) == 0 {
		return nil
	}
	return &ModPlan{
		Goals:      base.Goals,
		Branch:     base.Branch,
		Version:    base.Version,
		Operations: ops,
	}
}

// ApplyMods implements AgentDriver.
// - If UseExistingGrok / GrokBinary: invoke grok WITH EnvForAgent, then always
//   applyPlanNaive as a reliable tree-wide apply of the plan (agent may have edited too).
// - If user declined local grok and no explicit binary: tree-wide naive apply only.
func (g *GrokAgent) ApplyMods(ctx context.Context, cfg JobConfig, plan *ModPlan, sourceDir string) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}
	if sourceDir == "" {
		return fmt.Errorf("sourceDir empty")
	}

	// Re-plan against the real cloned tree so globs become concrete file ops
	treePlan, err := PlanModsFromTree(sourceDir, cfg, plan.Branch)
	if err == nil && treePlan != nil && len(treePlan.Operations) > 0 {
		// Prefer concrete file ops; keep user goals
		treePlan.Goals = plan.Goals
		if plan.Goals == "" {
			treePlan.Goals = cfg.Goals
		}
		plan = treePlan
	}

	promptDir := g.PromptDir
	if promptDir == "" {
		promptDir = filepath.Join(filepath.Dir(sourceDir), "agent-prompts")
	}
	_ = os.MkdirAll(promptDir, 0755)
	prompt := BuildAgentPrompt(cfg, plan.Branch, sourceDir)
	var pb strings.Builder
	pb.WriteString(prompt)
	pb.WriteString("\n## 已解析的文件级魔改计划（请执行）\n")
	for i, op := range plan.Operations {
		if i >= 100 {
			pb.WriteString(fmt.Sprintf("... +%d more ops\n", len(plan.Operations)-100))
			break
		}
		pb.WriteString(fmt.Sprintf("%d. path=%s op=%s find=%q replace=%q\n   %s\n",
			i+1, op.Path, op.Operation, op.Find, op.Replace, op.Description))
	}
	promptFile := filepath.Join(promptDir, fmt.Sprintf("prompt-%d.md", time.Now().UnixNano()))
	if err := os.WriteFile(promptFile, []byte(pb.String()), 0644); err != nil {
		return err
	}

	binary, useGrok := resolveGrokBinary(cfg, g.LookPath)
	if useGrok {
		if g.Runner == nil {
			g.Runner = ExecRunner{}
		}
		env := EnvForAgent(cfg)
		args := GrokInvokeArgs(binary, promptFile, sourceDir)
		out, err := g.Runner.Run(ctx, env, args[0], args[1:]...)
		_ = os.WriteFile(filepath.Join(promptDir, "agent-output.txt"), []byte(out), 0644)
		if err != nil {
			_ = os.WriteFile(filepath.Join(promptDir, "agent-error.txt"), []byte(out+"\n"+err.Error()), 0644)
			// Fall through to naive apply — must still patch files
		}
	}

	// Always run tree-wide naive apply so mods are guaranteed even if agent no-ops
	if nerr := applyPlanNaive(sourceDir, plan); nerr != nil {
		return fmt.Errorf("应用魔改失败: %w", nerr)
	}
	// After content replace, rename asset basenames meson references by path
	// (e.g. frida-agent-android.version → {magic}-agent-android.version).
	if n, rerr := RenameMagicAssetFiles(sourceDir, cfg.MagicName); rerr != nil {
		return fmt.Errorf("重命名魔改资源文件失败: %w", rerr)
	} else if n > 0 && g.PromptDir != "" {
		_ = os.WriteFile(filepath.Join(g.PromptDir, "renamed-assets.txt"),
			[]byte(fmt.Sprintf("renamed %d asset files for magic=%s\n", n, cfg.MagicName)), 0644)
	}
	if err := ApplyDeepSourceExtras(sourceDir, cfg); err != nil {
		return err
	}
	return nil
}

// RenameMagicAssetFiles renames basenames frida-{agent,server,gadget}* → {magic}-…
// Required after content replace: meson.build references 'abcde-agent-android.version'
// but the on-disk file is still frida-agent-android.version until this step.
func RenameMagicAssetFiles(sourceDir, magicName string) (int, error) {
	magicName = strings.TrimSpace(magicName)
	if sourceDir == "" || magicName == "" || magicName == "frida" {
		return 0, nil
	}
	// Keep in sync with DefaultModOps hyphen basenames (agent/server/gadget/helper)
	prefixes := []string{"frida-agent", "frida-server", "frida-gadget", "frida-helper"}
	renamed := 0
	// Collect first to avoid walking while renaming
	var todos [][2]string // oldPath, newPath
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "node_modules" || base == "build" || strings.HasPrefix(base, "build-") {
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		for _, p := range prefixes {
			if name == p || strings.HasPrefix(name, p+".") || strings.HasPrefix(name, p+"-") {
				newName := magicName + name[len("frida"):] // frida-agent… → magic-agent…
				if newName == name {
					return nil
				}
				newPath := filepath.Join(filepath.Dir(path), newName)
				if _, e := os.Stat(newPath); e == nil {
					return nil // already exists
				}
				todos = append(todos, [2]string{path, newPath})
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, t := range todos {
		if e := os.Rename(t[0], t[1]); e != nil {
			return renamed, e
		}
		renamed++
	}
	return renamed, nil
}

// applyPlanNaive walks the tree and applies Find/Replace ops, including ** globs.
// Returns error only on empty sourceDir; missing individual files are skipped.
// Reports how many files were patched via fridare-mod-plan.json stats.
func applyPlanNaive(sourceDir string, plan *ModPlan) error {
	if sourceDir == "" {
		return fmt.Errorf("sourceDir empty")
	}
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	filesPatched := 0
	replacements := 0
	for _, op := range plan.Operations {
		if op.Find == "" || op.Replace == "" || op.Find == op.Replace {
			continue
		}
		paths, err := matchOpPaths(sourceDir, op)
		if err != nil {
			continue
		}
		for _, rel := range paths {
			full := filepath.Join(sourceDir, filepath.FromSlash(rel))
			info, err := os.Stat(full)
			if err != nil || info.IsDir() {
				continue
			}
			// Never patch meson wrap files — clone URLs must stay github.com/frida/*
			baseLow := strings.ToLower(info.Name())
			if strings.HasSuffix(baseLow, ".wrap") {
				continue
			}
			// Skip huge binaries (>8MB) for safety
			if info.Size() > 8*1024*1024 {
				continue
			}
			data, err := os.ReadFile(full)
			if err != nil {
				continue
			}
			// Skip likely-binary content
			if isLikelyBinary(data) {
				continue
			}
			s := string(data)
			if !strings.Contains(s, op.Find) {
				continue
			}
			if isListenPortFind(op.Find) && skipNumericTableVendorPath(rel) {
				continue
			}
			// Guard: never smash github.com/frida/ clone URLs (e.g. Find="/frida/" legacy ops).
			newData := replaceAvoidingGitHubFrida(s, op.Find, op.Replace)
			if newData == s {
				continue
			}
			// Safety: refuse catastrophic shrink (corruption / bad multi-replace)
			if len(s) > 200 && len(newData)*2 < len(s) {
				continue
			}
			count := strings.Count(s, op.Find) - strings.Count(newData, op.Find)
			if count < 0 {
				count = 1
			}
			if err := os.WriteFile(full, []byte(newData), info.Mode()); err != nil {
				continue
			}
			filesPatched++
			replacements += count
		}
	}

	// Infer magic from ops for rename (StubAgent path also needs renames)
	magic := magicFromPlan(plan)
	renamed := 0
	if magic != "" {
		if n, rerr := RenameMagicAssetFiles(sourceDir, magic); rerr == nil {
			renamed = n
		}
	}

	summary := fmt.Sprintf(
		`{"goals":%q,"branch":%q,"version":%q,"ops":%d,"files_patched":%d,"replacements":%d,"files_renamed":%d}`,
		plan.Goals, plan.Branch, plan.Version, len(plan.Operations), filesPatched, replacements, renamed,
	)
	_ = os.WriteFile(filepath.Join(sourceDir, "fridare-mod-plan.json"), []byte(summary+"\n"), 0644)
	return nil
}

// replaceAvoidingGitHubFrida applies Find→Replace but leaves github.com/frida/ intact
// (and restores any accidental rewrite of that prefix within the same pass).
func replaceAvoidingGitHubFrida(s, find, repl string) string {
	if find == "" || find == repl {
		return s
	}
	const keep = "github.com/frida/"
	if !strings.Contains(s, find) {
		return s
	}
	// Fast path: no github frida URL in file
	if !strings.Contains(s, keep) && !strings.Contains(s, "github.com/") {
		return strings.ReplaceAll(s, find, repl)
	}
	// Placeholder protect
	const ph = "\x00FRIDARE_GH_FRIDA\x00"
	tmp := strings.ReplaceAll(s, keep, ph)
	tmp = strings.ReplaceAll(tmp, find, repl)
	return strings.ReplaceAll(tmp, ph, keep)
}

// magicFromPlan extracts magic name from frida-agent → {magic}-agent replace ops.
func magicFromPlan(plan *ModPlan) string {
	if plan == nil {
		return ""
	}
	for _, op := range plan.Operations {
		if op.Find == "frida-agent" && strings.HasSuffix(op.Replace, "-agent") {
			return strings.TrimSuffix(op.Replace, "-agent")
		}
		if op.Find == "frida-server" && strings.HasSuffix(op.Replace, "-server") {
			return strings.TrimSuffix(op.Replace, "-server")
		}
	}
	return ""
}

// matchOpPaths resolves an op's Path (concrete or glob) to relative paths under sourceDir.
func matchOpPaths(sourceDir string, op ModOp) ([]string, error) {
	pat := strings.TrimSpace(op.Path)
	if pat == "" {
		pat = "**/*"
	}
	// Normalize to slash
	pat = filepath.ToSlash(pat)

	// Concrete path (no glob metacharacters)
	if !strings.ContainsAny(pat, "*?[") {
		return []string{pat}, nil
	}

	var out []string
	// Support ** by walking and matching with path.Match on each segment-ish pattern
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if shouldSkipModDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if matchGlob(pat, relSlash) {
			if isBroadModGlob(pat) && skipBroadModCandidate(relSlash) {
				return nil
			}
			out = append(out, relSlash)
		}
		return nil
	})
	return out, err
}

// matchGlob supports ** (any path prefix/suffix) and * within a segment via path rules.
func matchGlob(pattern, name string) bool {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	// Fast paths
	if pattern == "**/*" || pattern == "**" {
		return true
	}
	// Convert ** to a simple matcher:
	// - "**/foo" => suffix /foo or exact foo
	// - "sub/**" => prefix sub/
	// - "a/**/b" => has prefix a/ and suffix /b or ends with b under a
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		// strip leading/trailing slashes from fragments
		left := strings.TrimPrefix(strings.TrimSuffix(parts[0], "/"), "/")
		right := ""
		if len(parts) > 1 {
			right = strings.TrimPrefix(parts[len(parts)-1], "/")
		}
		if left != "" {
			if name != left && !strings.HasPrefix(name, left+"/") {
				return false
			}
		}
		if right != "" {
			// right may still have * 
			if strings.ContainsAny(right, "*?[") {
				// match against basename or full remaining
				if ok, _ := filepath.Match(right, filepath.Base(name)); ok {
					return true
				}
				// try match end of path
				for i := 0; i < len(name); i++ {
					if ok, _ := filepath.Match(right, name[i:]); ok {
						return true
					}
				}
				return false
			}
			if name != right && !strings.HasSuffix(name, "/"+right) && !strings.Contains(name, "/"+right+"/") {
				// also allow right as path suffix without leading requirement when left empty
				if !strings.HasSuffix(name, right) {
					return false
				}
			}
		}
		return true
	}
	ok, err := filepath.Match(pattern, name)
	if err == nil && ok {
		return true
	}
	// Also try matching basenames only
	ok, _ = filepath.Match(pattern, filepath.Base(name))
	return ok
}

func isLikelyBinary(data []byte) bool {
	n := len(data)
	if n > 1024 {
		n = 1024
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// NewDefaultAgent returns GrokAgent (always — ApplyMods honors UseExistingGrok).
func NewDefaultAgent(promptDir string) AgentDriver {
	return &GrokAgent{PromptDir: promptDir, Runner: ExecRunner{}}
}
