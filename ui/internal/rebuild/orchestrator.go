package rebuild

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// JobStage is a named pipeline stage.
type JobStage string

const (
	StageIdle       JobStage = "idle"
	StageProbe      JobStage = "probe"
	StageBootstrap  JobStage = "bootstrap"
	StageClone      JobStage = "clone"
	StageBranch     JobStage = "branch"
	StageModPlan    JobStage = "mod_plan"
	StageModApply   JobStage = "mod_apply"
	StageBuild      JobStage = "build"
	StageExport     JobStage = "export"
	StageToolsPatch JobStage = "tools_patch"
	StageDone       JobStage = "done"
	StageFailed     JobStage = "failed"
	StageCancelled  JobStage = "cancelled"
)

// OrderedStages is the happy-path stage sequence (excluding terminal states).
var OrderedStages = []JobStage{
	StageProbe,
	StageBootstrap,
	StageClone,
	StageBranch,
	StageModPlan,
	StageModApply,
	StageBuild,
	StageExport,
	StageToolsPatch,
	StageDone,
}

// ProgressEvent is emitted during job execution.
type ProgressEvent struct {
	Time    time.Time `json:"time"`
	Stage   JobStage  `json:"stage"`
	Percent float64   `json:"percent"` // 0..1
	Message string    `json:"message"`
	Err     string    `json:"err,omitempty"`
}

// ModOp is one file-level modification the agent plans/applies.
type ModOp struct {
	Path        string `json:"path"`        // relative to source root
	Operation   string `json:"operation"`   // replace | insert | delete | rewrite
	Description string `json:"description"` // human/agent note
	// Optional structured payload
	Find    string `json:"find,omitempty"`
	Replace string `json:"replace,omitempty"`
}

// ModPlan is the agent-produced plan of source modifications.
type ModPlan struct {
	Goals      string  `json:"goals"` // user dialogue goals
	Branch     string  `json:"branch"`
	Version    string  `json:"version"`
	Operations []ModOp `json:"operations"`
}

// JobMode selects which half of the dual-step rebuild pipeline to run.
type JobMode string

const (
	// JobModeFull runs bootstrap + develop (legacy one-shot).
	JobModeFull JobMode = "full"
	// JobModeBootstrap only prepares the local base builder image (toolchain).
	// Heavy deps (NDK/Node/Go) are installed once and kept for reuse.
	JobModeBootstrap JobMode = "bootstrap"
	// JobModeDevelop runs clone → AI mod → configure/make using the archived base image.
	JobModeDevelop JobMode = "develop"
)

// JobConfig is user input for one rebuild job.
type JobConfig struct {
	FridaVersion  string
	TargetIDs     []string
	MagicName     string
	ListenPort    int
	Goals         string // free-form dialogue goals for the AI agent
	WorkDir       string // host work root
	ArtifactDir   string
	Proxy         string
	OpenAIBaseURL string
	OpenAIAPIKey  string
	OpenAIModel   string
	DockerImage   string
	// DockerMirror is an optional Hub prefix, e.g. "docker.1ms.run"
	// (ubuntu:22.04 → docker.1ms.run/library/ubuntu:22.04).
	DockerMirror string
	// DockerPullDirect: when true, docker pull/build do NOT use HTTP proxy
	// (required for docker.1ms.run). Container runtime still gets Proxy for git.
	DockerPullDirect bool
	// UseExistingGrok: if true and grok is on PATH, invoke it; endpoint still from GUI.
	UseExistingGrok bool
	GrokBinary      string // optional override path
	// AgentUseGUIProxy: when true (default from GUI), host AI agent MUST egress via cfg.Proxy.
	// EnvForAgent forces HTTP(S)_PROXY from GUI and strips conflicting inherited proxy vars.
	AgentUseGUIProxy bool
	// DryRun: exercise stage order without real docker/compile (tests / preview).
	DryRun bool
	// MinDiskGB for probe gate
	MinDiskGB float64
	// Mode selects pipeline half; empty defaults to JobModeFull.
	Mode JobMode
	// ArchiveImage: after bootstrap, also docker save a tar under WorkDir/images for offline reuse.
	ArchiveImage bool
	// DirectionProfile: "safe" (default) or "explore" — drives strip direction list for Agent.
	DirectionProfile string
	// DirectionFile: optional path to write/read DirectionManifest JSON (batch tools + Agent).
	DirectionFile string
}

// EffectiveMode returns Mode or JobModeFull when unset.
func (c JobConfig) EffectiveMode() JobMode {
	switch c.Mode {
	case JobModeBootstrap, JobModeDevelop, JobModeFull:
		return c.Mode
	default:
		return JobModeFull
	}
}

// JobState is the live job snapshot.
type JobState struct {
	ID         string          `json:"id"`
	Stage      JobStage        `json:"stage"`
	Percent    float64         `json:"percent"`
	Message    string          `json:"message"`
	Branch     string          `json:"branch"`
	Plan       *ModPlan        `json:"plan,omitempty"`
	Artifact   string          `json:"artifact_dir,omitempty"`
	LogPath    string          `json:"log_path,omitempty"`    // full text log for this run
	LatestLog  string          `json:"latest_log,omitempty"` // always-updated latest.log
	Events     []ProgressEvent `json:"events"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt time.Time       `json:"finished_at,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// Runner abstracts command execution for tests.
type Runner interface {
	// Run executes a command; cancel via ctx.
	// env is the full process environment (e.g. from EnvForAgent); nil means inherit os.Environ().
	Run(ctx context.Context, env []string, name string, args ...string) (string, error)
}

// ExecRunner is the real process runner.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = env
	}
	b, err := cmd.CombinedOutput()
	return string(b), err
}

// AgentDriver plans and applies source mods (typically via grok-build).
type AgentDriver interface {
	// PlanMods asks the agent for a ModPlan given goals and tree context.
	PlanMods(ctx context.Context, cfg JobConfig, branch string) (*ModPlan, error)
	// ApplyMods applies the plan (or re-invokes agent to edit files in workdir).
	ApplyMods(ctx context.Context, cfg JobConfig, plan *ModPlan, sourceDir string) error
}

// StubAgent is a deterministic dry-run agent used when DryRun or no real agent.
type StubAgent struct {
	// OnPlan / OnApply optional hooks for tests
	OnPlan  func(ctx context.Context, cfg JobConfig, branch string) (*ModPlan, error)
	OnApply func(ctx context.Context, cfg JobConfig, plan *ModPlan, sourceDir string) error
}

func (s *StubAgent) PlanMods(ctx context.Context, cfg JobConfig, branch string) (*ModPlan, error) {
	if s.OnPlan != nil {
		return s.OnPlan(ctx, cfg, branch)
	}
	// Prefer tree-aware plan when source already exists under work dir
	return PlanModsFromTree("", cfg, branch)
}

func (s *StubAgent) ApplyMods(ctx context.Context, cfg JobConfig, plan *ModPlan, sourceDir string) error {
	if s.OnApply != nil {
		return s.OnApply(ctx, cfg, plan, sourceDir)
	}
	// Same path as production: tree plan → content replace → asset renames
	if sourceDir == "" {
		return fmt.Errorf("sourceDir empty")
	}
	_ = os.MkdirAll(sourceDir, 0755)
	if treePlan, err := PlanModsFromTree(sourceDir, cfg, plan.Branch); err == nil && treePlan != nil {
		treePlan.Goals = plan.Goals
		plan = treePlan
	}
	if err := applyPlanNaive(sourceDir, plan); err != nil {
		return err
	}
	// Explicit rename even if magicFromPlan misses (e2e StubAgent path)
	if _, err := RenameMagicAssetFiles(sourceDir, cfg.MagicName); err != nil {
		return fmt.Errorf("RenameMagicAssetFiles: %w", err)
	}
	if err := ApplyDeepSourceExtras(sourceDir, cfg); err != nil {
		return err
	}
	return nil
}

// DefaultModOps returns source-level renames that keep Frida's Vala tree compilable
// while still producing magic product basenames and runtime string markers.
//
// Strategy (validated against Frida 17.x valac):
//   - Hyphen basenames (frida-server/agent/…) → {magic}-… so meson targets + assets match.
//   - Runtime markers (frida:rpc, gum-js-loop, port) → magic equivalents.
//   - Do NOT rename Vala namespaces (Frida.Agent → X.Agent breaks parent-type lookup;
//     injecting `using Frida` then makes Error ambiguous with GLib.Error).
//   - Do NOT rename underscore C prefixes (frida_agent_*) in source: valac still emits
//     frida_agent_* from namespace Frida.Agent; glue/export must stay consistent.
//     Underscore markers are rewritten same-length in binaries by PatchArtifactBinaryMarkers.
//
// Paths use ** globs; applyPlanNaive / PlanModsFromTree expand against the tree.
// Do NOT rewrite stock-client protocol re.frida.* / public JS Frida API surface.
func DefaultModOps(magicName string, port int) []ModOp {
	if magicName == "" {
		magicName = "frida"
	}
	if port <= 0 {
		port = 27042
	}
	return []ModOp{
		// --- product basenames (hyphen) — meson targets + asset files ---
		{Path: "**/*", Operation: "replace", Description: "frida-server basename", Find: "frida-server", Replace: magicName + "-server"},
		{Path: "**/*", Operation: "replace", Description: "frida-helper basename", Find: "frida-helper", Replace: magicName + "-helper"},
		{Path: "**/*", Operation: "replace", Description: "frida-agent basename", Find: "frida-agent", Replace: magicName + "-agent"},
		{Path: "**/*", Operation: "replace", Description: "frida-gadget basename", Find: "frida-gadget", Replace: magicName + "-gadget"},
		// --- GResource/blob getters generated from magic basenames (get_frida_agent_* → get_{magic}_agent_*) ---
		// Surgical: do NOT replace bare frida_agent (would break valac-emitted frida_agent_main).
		{Path: "**/*", Operation: "replace", Description: "get_frida_agent blob getter", Find: "get_frida_agent", Replace: "get_" + magicName + "_agent"},
		{Path: "**/*", Operation: "replace", Description: "get_frida_helper blob getter", Find: "get_frida_helper", Replace: "get_" + magicName + "_helper"},
		{Path: "**/*", Operation: "replace", Description: "get_frida_gadget blob getter", Find: "get_frida_gadget", Replace: "get_" + magicName + "_gadget"},
		{Path: "**/*", Operation: "replace", Description: "get_frida_server blob getter", Find: "get_frida_server", Replace: "get_" + magicName + "_server"},
		// --- runtime string markers (safe in Vala/C; same-length when magic is 5 chars) ---
		{Path: "**/*", Operation: "replace", Description: "frida:rpc channel", Find: "frida:rpc", Replace: magicName + ":rpc"},
		{Path: "**/*", Operation: "replace", Description: "gum-js-loop thread", Find: "gum-js-loop", Replace: magicName + "-js-loop"},
		{Path: "**/*", Operation: "replace", Description: "frida-main-loop", Find: "frida-main-loop", Replace: magicName + "-main-loop"},
		{Path: "**/*", Operation: "replace", Description: "pool-frida", Find: "pool-frida", Replace: "pool-" + magicName},
		{Path: "**/*", Operation: "replace", Description: fmt.Sprintf("listen port → %d", port), Find: "27042", Replace: fmt.Sprintf("%d", port)},
	}
}

// RenameArtifactBasenames renames collected binary basenames frida-* → {magic}-* after make.
// If the magic-named file already exists (meson built magic targets), the leftover frida-*
// duplicate is removed so catalog/MANIFEST only list product basenames.
func RenameArtifactBasenames(dir, magic string) (int, error) {
	if magic == "" || magic == "frida" {
		return 0, nil
	}
	n := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		// libfrida-gadget-*.so → lib{magic}-gadget-*.so
		if strings.HasPrefix(name, "libfrida-") {
			newName := "lib" + magic + name[len("libfrida"):]
			newPath := filepath.Join(filepath.Dir(path), newName)
			if _, e := os.Stat(newPath); e == nil {
				_ = os.Remove(path)
				n++
				return nil
			}
			if e := os.Rename(path, newPath); e != nil {
				return e
			}
			n++
			return nil
		}
		for _, p := range []string{"frida-server", "frida-agent", "frida-gadget", "frida-helper", "frida-inject"} {
			if name == p || strings.HasPrefix(name, p+"-") || strings.HasPrefix(name, p+".") {
				newName := magic + name[len("frida"):]
				newPath := filepath.Join(filepath.Dir(path), newName)
				if _, e := os.Stat(newPath); e == nil {
					// magic product already present — drop frida-* leftover
					_ = os.Remove(path)
					n++
					return nil
				}
				if e := os.Rename(path, newPath); e != nil {
					return e
				}
				n++
				return nil
			}
		}
		return nil
	})
	return n, err
}

// Orchestrator runs the clone→branch→mod→build→export pipeline.
type Orchestrator struct {
	mu     sync.Mutex
	state  JobState
	cancel context.CancelFunc
	runner Runner
	agent  AgentDriver
	fileLog *JobFileLogger
	// hooks for tests
	ProbeFn func(cfg JobConfig) DepsReport
}

// NewOrchestrator constructs an orchestrator with real runner and optional agent.
func NewOrchestrator(runner Runner, agent AgentDriver) *Orchestrator {
	if runner == nil {
		runner = ExecRunner{}
	}
	if agent == nil {
		agent = &StubAgent{}
	}
	return &Orchestrator{
		runner: runner,
		agent:  agent,
		state: JobState{
			Stage: StageIdle,
		},
	}
}

// State returns a copy of the current job state.
func (o *Orchestrator) State() JobState {
	o.mu.Lock()
	defer o.mu.Unlock()
	st := o.state
	// shallow copy events
	if st.Events != nil {
		ev := make([]ProgressEvent, len(st.Events))
		copy(ev, st.Events)
		st.Events = ev
	}
	return st
}

// Cancel requests termination of the running job.
func (o *Orchestrator) Cancel() {
	o.mu.Lock()
	cancel := o.cancel
	o.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Reset clears job state so the user can restart.
func (o *Orchestrator) Reset() {
	o.Cancel()
	o.mu.Lock()
	defer o.mu.Unlock()
	o.cancel = nil
	o.state = JobState{Stage: StageIdle, Message: "已重置，可重新开始"}
}

func (o *Orchestrator) emit(stage JobStage, pct float64, msg string, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	ev := ProgressEvent{
		Time:    time.Now(),
		Stage:   stage,
		Percent: pct,
		Message: msg,
	}
	if err != nil {
		ev.Err = err.Error()
		o.state.Error = err.Error()
	}
	o.state.Stage = stage
	o.state.Percent = pct
	o.state.Message = msg
	o.state.Events = append(o.state.Events, ev)
	// Persist every event to the text log for offline debugging
	if o.fileLog != nil {
		line := fmt.Sprintf("[%s] %s", stage, msg)
		if err != nil {
			line += " ERR=" + err.Error()
		}
		o.fileLog.Log("%s", line)
	}
}

// Start launches the pipeline asynchronously. Returns immediately.
// progressCb is called on each event (may be nil).
func (o *Orchestrator) Start(cfg JobConfig, progressCb func(ProgressEvent)) error {
	o.mu.Lock()
	if o.state.Stage != StageIdle && o.state.Stage != StageDone && o.state.Stage != StageFailed && o.state.Stage != StageCancelled {
		o.mu.Unlock()
		return fmt.Errorf("任务进行中（%s），请先终止或等待完成", o.state.Stage)
	}
	ctx, cancel := context.WithCancel(context.Background())
	o.cancel = cancel
	id := fmt.Sprintf("job-%d", time.Now().UnixNano())
	o.state = JobState{
		ID:        id,
		Stage:     StageProbe,
		StartedAt: time.Now(),
		Message:   "启动中…",
	}
	o.mu.Unlock()

	go o.run(ctx, cfg, progressCb)
	return nil
}

// RunSync runs the pipeline to completion (used by tests).
func (o *Orchestrator) RunSync(ctx context.Context, cfg JobConfig) error {
	o.mu.Lock()
	inner, cancel := context.WithCancel(ctx)
	o.cancel = cancel
	o.state = JobState{
		ID:        fmt.Sprintf("job-%d", time.Now().UnixNano()),
		Stage:     StageProbe,
		StartedAt: time.Now(),
	}
	o.mu.Unlock()
	return o.run(inner, cfg, nil)
}

func (o *Orchestrator) run(ctx context.Context, cfg JobConfig, progressCb func(ProgressEvent)) (err error) {
	// File log under workdir/logs for user + developer inspection
	workRoot := cfg.WorkDir
	if workRoot == "" {
		workRoot = DefaultSourceWorkDir("")
	}
	jobID := o.State().ID
	origRunner := o.runner
	if origRunner == nil {
		origRunner = ExecRunner{}
	}
	flog, logErr := NewJobFileLogger(workRoot, jobID)
	if logErr == nil && flog != nil {
		o.mu.Lock()
		o.fileLog = flog
		o.state.LogPath = flog.Path()
		o.state.LatestLog = flog.LatestPath()
		// Wrap runner to capture docker/git command output into the same file
		o.runner = LoggingRunner{Inner: origRunner, Logger: flog}
		o.mu.Unlock()
		flog.Log("config version=%s targets=%v magic=%s proxy=%q mirror=%q image=%q dry_run=%v",
			cfg.FridaVersion, cfg.TargetIDs, cfg.MagicName, cfg.Proxy, cfg.DockerMirror, cfg.DockerImage, cfg.DryRun)
		flog.Log("log_file=%s", flog.Path())
		flog.Log("latest_log=%s", flog.LatestPath())
		// surface path early in UI via progress
		notifyEarly := func() {
			if progressCb != nil {
				progressCb(ProgressEvent{
					Time:    time.Now(),
					Stage:   StageProbe,
					Percent: 0.02,
					Message: "文本日志: " + flog.Path(),
				})
			}
		}
		notifyEarly()
	}
	defer func() {
		o.mu.Lock()
		o.runner = origRunner
		o.mu.Unlock()
	}()

	notify := func(stage JobStage, pct float64, msg string, e error) {
		o.emit(stage, pct, msg, e)
		if progressCb != nil {
			st := o.State()
			if len(st.Events) > 0 {
				progressCb(st.Events[len(st.Events)-1])
			}
		}
	}

	defer func() {
		if err != nil {
			if ctx.Err() != nil {
				notify(StageCancelled, o.State().Percent, "用户已终止任务", ctx.Err())
			} else {
				logHint := ""
				if o.fileLog != nil {
					logHint = "；完整日志: " + o.fileLog.Path()
				}
				notify(StageFailed, o.State().Percent, "任务失败: "+err.Error()+logHint, err)
			}
		}
		o.mu.Lock()
		o.state.FinishedAt = time.Now()
		sum := "OK"
		if err != nil {
			sum = "FAIL: " + err.Error()
		}
		if o.fileLog != nil {
			if o.state.LogPath == "" {
				o.state.LogPath = o.fileLog.Path()
			}
			o.fileLog.Close(sum)
		}
		o.mu.Unlock()
	}()

	// --- probe ---
	notify(StageProbe, 0.05, "检查依赖（Docker / 磁盘 / 代理 / grok-build / OpenAI）…", nil)
	if err = ctx.Err(); err != nil {
		return err
	}
	var report DepsReport
	if o.ProbeFn != nil {
		report = o.ProbeFn(cfg)
	} else if cfg.DryRun {
		report = DepsReport{Ready: true, Messages: []string{"dry-run: skip real probes"}}
		report.Docker.Available = true
		report.GrokBuild.Available = true
		report.Git.Available = true
		report.Proxy.Available = true
		report.Disk.Available = true
		report.OpenAI.Available = true
	} else {
		report = ProbeDeps(ProbeOptions{
			WorkDir:       cfg.WorkDir,
			Proxy:         cfg.Proxy,
			OpenAIBaseURL: cfg.OpenAIBaseURL,
			OpenAIAPIKey:  cfg.OpenAIAPIKey,
			MinDiskGB:     cfg.MinDiskGB,
		})
	}
	mode := cfg.EffectiveMode()
	if !cfg.DryRun {
		if mode == JobModeBootstrap {
			if err = CanStartBootstrapJob(report); err != nil {
				return err
			}
		} else if err = CanStartSourceJob(report, cfg.Proxy); err != nil {
			return err
		}
	}
	// Develop / full need version + targets; bootstrap only needs Docker/disk.
	if mode != JobModeBootstrap {
		if strings.TrimSpace(cfg.FridaVersion) == "" {
			return fmt.Errorf("必须指定 Frida 版本")
		}
		if len(cfg.TargetIDs) == 0 {
			return fmt.Errorf("至少选择一个编译目标")
		}
	}

	// --- bootstrap (Step 1: base image with toolchain) ---
	notify(StageBootstrap, 0.12, "步骤① 准备 Docker 基础镜像（工具链）…", nil)
	if err = ctx.Err(); err != nil {
		return err
	}
	// workRoot already set at start for logging
	if workRoot == "" {
		workRoot = cfg.WorkDir
		if workRoot == "" {
			workRoot = DefaultSourceWorkDir("")
		}
	}
	srcHost := filepath.Join(workRoot, "src")
	artHost := cfg.ArtifactDir
	if artHost == "" {
		artHost = DefaultArtifactDir(workRoot)
	}
	if err = os.MkdirAll(srcHost, 0755); err != nil {
		return fmt.Errorf("创建源码工作目录: %w", err)
	}
	if err = os.MkdirAll(artHost, 0755); err != nil {
		return fmt.Errorf("创建产物目录: %w", err)
	}
	// Write Dockerfile with optional China Hub mirror on FROM
	dockerCtx := filepath.Join(workRoot, "docker")
	_ = os.MkdirAll(dockerCtx, 0755)
	mirror := strings.TrimSpace(cfg.DockerMirror)
	df := DockerfileSkeletonForMirror(mirror)
	_ = os.WriteFile(filepath.Join(dockerCtx, "Dockerfile"), []byte(WriteScriptUnixLF(df)), 0644)

	if !SupportsDockerSourceRebuild() {
		return fmt.Errorf("当前操作系统 %s 未作为源码重编译客户端验证；请使用 Windows / macOS / Linux + Docker Desktop 或 Docker Engine", HostPlatformLabel())
	}
	notify(StageBootstrap, 0.13, fmt.Sprintf("Host 客户端 %s — 编译仅在 Docker/Linux 内执行", HostPlatformLabel()), nil)
	if mirror != "" {
		notify(StageBootstrap, 0.135, fmt.Sprintf("Docker 镜像源: %s（pull/build 直连=%v，不走 GUI 代理）", mirror, cfg.DockerPullDirect || mirror != ""), nil)
	}

	image := cfg.DockerImage
	if image == "" {
		image = DefaultBuildImage
	}
	// Local builder tag stays fridare/frida-builder:latest; base layers come via mirror FROM.
	// If user set a full Hub image, resolve through mirror for pull.
	pullRef := ResolveDockerImage(image, mirror)
	// Prefer local image name without forcing mirror onto custom local tags like fridare/...
	localTag := image
	if strings.HasPrefix(image, "fridare/") || !strings.Contains(image, "/") {
		// keep local builder name; we build it ourselves
		localTag = image
		if localTag == "" {
			localTag = DefaultBuildImage
		}
		pullRef = "" // skip pull of non-existent Hub image; go straight to docker build
	}

	pullEnv := DockerPullEnv(cfg.DockerPullDirect || mirror != "")
	if !cfg.DryRun {
		needBuild := true
		if pullRef != "" && pullRef != localTag {
			notify(StageBootstrap, 0.14, "docker pull "+pullRef+" …", nil)
			if _, runErr := o.runner.Run(ctx, pullEnv, "docker", "pull", pullRef); runErr == nil {
				if pullRef != localTag {
					_, _ = o.runner.Run(ctx, pullEnv, "docker", "tag", pullRef, localTag)
				}
				needBuild = false
			} else {
				notify(StageBootstrap, 0.15, "pull 失败: "+runErr.Error()+"，改为本地 docker build（含 NDK 等预置依赖）…", nil)
			}
		} else {
			// Existing image must include baked toolchain (feature marker)
			if out, err := o.runner.Run(ctx, pullEnv, "docker", "images", "-q", localTag); err == nil && strings.TrimSpace(out) != "" {
				probe := RunContainerArgs(DockerWorkspace{Image: localTag}, []string{"bash", "-lc", ImageHasBuilderFeaturesShell()})
				if _, perr := o.runner.Run(ctx, pullEnv, probe[0], probe[1:]...); perr == nil {
					needBuild = false
					notify(StageBootstrap, 0.14, "本地镜像 "+localTag+" 已含预置依赖（"+BuilderImageFeatureTag+"），跳过 docker build", nil)
				} else {
					// try stable feature tag alias
					stable := BuilderStableImageTag(localTag)
					if outS, eS := o.runner.Run(ctx, pullEnv, "docker", "images", "-q", stable); eS == nil && strings.TrimSpace(outS) != "" {
						probeS := RunContainerArgs(DockerWorkspace{Image: stable}, []string{"bash", "-lc", ImageHasBuilderFeaturesShell()})
						if _, pS := o.runner.Run(ctx, pullEnv, probeS[0], probeS[1:]...); pS == nil {
							_, _ = o.runner.Run(ctx, pullEnv, "docker", "tag", stable, localTag)
							needBuild = false
							notify(StageBootstrap, 0.14, "从留档标签复用 "+stable+" → "+localTag, nil)
						}
					}
					if needBuild {
						notify(StageBootstrap, 0.145, "本地镜像缺少预置依赖（NDK/Node/Go），将重建 builder 镜像…", nil)
					}
				}
			}
			// Try docker load from workdir archive if still missing
			if needBuild {
				if path, lerr := LoadArchivedBuilderImage(ctx, o.runner, pullEnv, workRoot, localTag); lerr == nil {
					probe := RunContainerArgs(DockerWorkspace{Image: localTag}, []string{"bash", "-lc", ImageHasBuilderFeaturesShell()})
					if _, perr := o.runner.Run(ctx, pullEnv, probe[0], probe[1:]...); perr == nil {
						needBuild = false
						notify(StageBootstrap, 0.14, "从本地 tar 档案恢复镜像: "+path, nil)
					}
				}
			}
		}
		if needBuild {
			notify(StageBootstrap, 0.16, fmt.Sprintf("docker build -t %s（预装 apt + Node20 + Go1.24 + NDK r29，基座 %s）…", localTag, ResolveDockerImage(DefaultDockerfileBaseImage, mirror)), nil)
			notify(StageBootstrap, 0.165, "首次构建镜像较慢（NDK+工具链），完成后 AI 魔改/编译只需确认环境与版本 subprojects", nil)
			// Node/Go/NDK tarballs need proxy inside RUN steps (build-arg).
			// Client uses pullEnv (no proxy) so Hub/mirror base image pull stays direct.
			buildArgs := DockerBuildImageArgsWithProxy(localTag, dockerCtx, cfg.Proxy)
			if _, berr := o.runner.Run(ctx, pullEnv, buildArgs[0], buildArgs[1:]...); berr != nil {
				return fmt.Errorf("Docker 编译环境构建失败（Dockerfile 预装 NDK/工具链）: %w\n%s", berr, DockerMirrorHelp)
			}
			// verify features after build
			probe := RunContainerArgs(DockerWorkspace{Image: localTag}, []string{"bash", "-lc", ImageHasBuilderFeaturesShell()})
			if out, perr := o.runner.Run(ctx, nil, probe[0], probe[1:]...); perr != nil {
				return fmt.Errorf("builder 镜像自检失败（缺少工具链标记）: %v\n%s", perr, out)
			}
			notify(StageBootstrap, 0.18, "builder 镜像就绪（依赖已预装）", nil)
		}
		// Always tag a stable feature-tag alias for local reuse (留档)
		stable := BuilderStableImageTag(localTag)
		if stable != "" && stable != localTag {
			notify(StageBootstrap, 0.185, "本地留档标签: "+stable, nil)
			_, _ = o.runner.Run(ctx, pullEnv, "docker", "tag", localTag, stable)
		}
		// Optional docker save tar under workdir/images
		if cfg.ArchiveImage || mode == JobModeBootstrap {
			if path, aerr := ArchiveBuilderImage(ctx, o.runner, pullEnv, localTag, workRoot); aerr != nil {
				notify(StageBootstrap, 0.19, "镜像 tar 留档跳过: "+aerr.Error(), nil)
			} else if path != "" {
				notify(StageBootstrap, 0.19, "镜像已本地留档: "+path, nil)
			}
		}
		image = localTag
		notify(StageBootstrap, 0.20, fmt.Sprintf("步骤① 完成：基础镜像 %s 可复用（feature=%s）", localTag, BuilderImageFeatureTag), nil)
	} else {
		notify(StageBootstrap, 0.15, "dry-run: 跳过 docker pull/build", nil)
	}

	// Step 1 only: stop after base image is ready / archived.
	if mode == JobModeBootstrap {
		notify(StageDone, 1.0, fmt.Sprintf("步骤① 基础镜像就绪: %s（下次步骤② 直接复用，无需重装工具链）", image), nil)
		return nil
	}

	// --- clone (Step 2: develop — Docker-first; bind-mount so agent can edit on host) ---
	notify(StageClone, 0.25, fmt.Sprintf("步骤② Docker 内浅克隆 Frida %s (--depth=1)…", cfg.FridaVersion), nil)
	if err = ctx.Err(); err != nil {
		return err
	}
	cloneDest := filepath.Join(srcHost, "frida")
	// Rewrite localhost proxy for container (GUI proxy often http://localhost:8080)
	cProxy := RewriteProxyForContainer(cfg.Proxy)
	notify(StageClone, 0.26, ContainerNetworkHint(cfg.Proxy, cProxy), nil)
	wsBase := DockerWorkspace{
		Image:            image,
		HostWorkDir:      srcHost,
		ContainerWorkDir: "/work",
		HTTPProxy:        cfg.Proxy, // RunContainerArgs rewrites again; keep host form here
		HTTPSProxy:       cfg.Proxy,
	}
	if !cfg.DryRun {
		if _, statErr := os.Stat(filepath.Join(cloneDest, ".git")); statErr != nil {
			_ = os.RemoveAll(cloneDest)
			cloneSh, clErr := DockerCloneShell(cfg.FridaVersion, "frida")
			if clErr != nil {
				return clErr
			}
			// Always clone inside Linux container — do not rely on Windows host git/toolchain.
			runArgs := RunContainerArgs(wsBase, []string{"bash", "-lc", cloneSh})
			out, derr := o.runner.Run(ctx, nil, runArgs[0], runArgs[1:]...)
			if derr != nil {
				// Fallback: host git with original proxy (localhost works on host)
				notify(StageClone, 0.27, "容器克隆失败，尝试 Host git clone（使用本机代理）…", nil)
				if herr := hostGitShallowClone(ctx, o.runner, cfg, cloneDest); herr != nil {
					return FormatDockerRunError("Docker 内克隆 Frida 源码失败", out, derr)
				}
				notify(StageClone, 0.29, "Host git 浅克隆成功", nil)
			}
		} else {
			notify(StageClone, 0.28, "源码树已存在，跳过 clone", nil)
		}
		// Init submodules before mod — otherwise frida-core/gum are empty gitlinks and
		// content replace + asset rename miss agent/server/gadget meson files.
		notify(StageClone, 0.31, "初始化 Frida submodules（魔改前需要完整 frida-core/gum 树）…", nil)
		subSh := DockerInitSubmodulesShell("frida")
		runArgs := RunContainerArgs(wsBase, []string{"bash", "-lc", subSh})
		if out, serr := o.runner.Run(ctx, nil, runArgs[0], runArgs[1:]...); serr != nil {
			notify(StageClone, 0.32, "submodule 初始化警告: "+serr.Error()+" — "+truncate(out, 200), nil)
		} else {
			notify(StageClone, 0.32, "submodules 就绪，开始管理分支与魔改", nil)
		}
	} else {
		// Dry-run: create fake source tree
		_ = os.MkdirAll(filepath.Join(cloneDest, "subprojects", "frida-core", "lib", "agent"), 0755)
		_ = os.WriteFile(filepath.Join(cloneDest, "README.md"), []byte("dry-run frida source\n"), 0644)
		_ = os.WriteFile(filepath.Join(cloneDest, "subprojects", "frida-core", "lib", "agent", "frida-agent-android.version"), []byte("V\n"), 0644)
		_ = os.WriteFile(filepath.Join(cloneDest, "subprojects", "frida-core", "lib", "agent", "meson.build"), []byte("symscript = 'frida-agent-android.version'\n"), 0644)
		notify(StageClone, 0.30, "dry-run: 已创建伪源码树 "+cloneDest, nil)
	}

	// --- branch (prefer Docker git on the bind-mounted tree) ---
	branch := ManagementBranchName(time.Now())
	notify(StageBranch, 0.35, "Docker 内创建管理分支 "+branch+"…", nil)
	if err = ctx.Err(); err != nil {
		return err
	}
	o.mu.Lock()
	o.state.Branch = branch
	o.mu.Unlock()
	if !cfg.DryRun {
		branchSh := DockerBranchShell("frida", branch)
		runArgs := RunContainerArgs(wsBase, []string{"bash", "-lc", branchSh})
		if _, runErr := o.runner.Run(ctx, nil, runArgs[0], runArgs[1:]...); runErr != nil {
			// Fallback: host git -C on bind mount only (metadata, not compile)
			notify(StageBranch, 0.36, "容器内 git 分支失败，尝试 host git 元数据操作（非编译）…", nil)
			bArgs := CreateBranchArgs(branch)
			gitArgs := append([]string{"-C", cloneDest}, bArgs[1:]...)
			if _, herr := o.runner.Run(ctx, nil, "git", gitArgs...); herr != nil {
				notify(StageBranch, 0.36, "git checkout -b 警告: "+herr.Error(), nil)
			}
		}
	} else {
		notify(StageBranch, 0.36, "dry-run: 假定分支 "+branch+" 已创建", nil)
	}

	// Host AI agent must use GUI proxy when option is on
	if err = RequireGUIProxyForAgent(cfg); err != nil {
		return err
	}

	// --- mod plan ---
	notify(StageModPlan, 0.45, "制定源码魔改计划（扫描树 + AI agent）…", nil)
	if err = ctx.Err(); err != nil {
		return err
	}
	// Prefer GrokAgent path with source-aware planning when agent is GrokAgent
	if ga, ok := o.agent.(*GrokAgent); ok && ga.PromptDir == "" {
		ga.PromptDir = filepath.Join(workRoot, "agent-prompts")
	}
	plan, perr := o.agent.PlanMods(ctx, cfg, branch)
	if perr != nil {
		return fmt.Errorf("魔改计划失败: %w", perr)
	}
	if plan == nil {
		return fmt.Errorf("魔改计划为空")
	}
	// If plan still only has globs, refine against cloned tree now
	if treePlan, terr := PlanModsFromTree(cloneDest, cfg, branch); terr == nil && treePlan != nil && len(treePlan.Operations) > 0 {
		treePlan.Goals = plan.Goals
		if plan.Goals == "" {
			treePlan.Goals = cfg.Goals
		}
		plan = treePlan
	}
	o.mu.Lock()
	o.state.Plan = plan
	o.mu.Unlock()
	notify(StageModPlan, 0.50, fmt.Sprintf("魔改计划就绪：%d 项文件操作", len(plan.Operations)), nil)

	// --- mod apply ---
	notify(StageModApply, 0.55, "执行源码魔改（AI agent + 全树文件操作）…", nil)
	if err = ctx.Err(); err != nil {
		return err
	}
	if aerr := o.agent.ApplyMods(ctx, cfg, plan, cloneDest); aerr != nil {
		return fmt.Errorf("应用魔改失败: %w", aerr)
	}

	// --- build: Docker-only configure/make (never host toolchain) ---
	notify(StageBuild, 0.65, fmt.Sprintf("Docker/Linux 内编译目标: %s …（不在 Host 执行 configure/make）", strings.Join(cfg.TargetIDs, ", ")), nil)
	if err = ctx.Err(); err != nil {
		return err
	}
	// Full pipeline script kept for recovery/re-clone; primary path is build-only after host mods.
	fullScript, serr := BuildPipelineScript(PipelineScriptOptions{
		Clone:       CloneSpec{Version: cfg.FridaVersion, DestDir: "frida"},
		Branch:      branch,
		SourceDir:   "frida",
		ArtifactDir: "artifacts",
		TargetIDs:   cfg.TargetIDs,
	})
	if serr != nil {
		return serr
	}
	buildOnly, berr := BuildOnlyPipelineScript(PipelineScriptOptions{
		SourceDir:   "frida",
		ArtifactDir: "artifacts",
		TargetIDs:   cfg.TargetIDs,
	})
	if berr != nil {
		return berr
	}
	// Always LF for Linux container bash (critical when GUI runs on Windows)
	_ = os.WriteFile(filepath.Join(srcHost, "pipeline.sh"), []byte(WriteScriptUnixLF(fullScript)), 0755)
	buildScriptPath := filepath.Join(srcHost, "build-only.sh")
	if werr := os.WriteFile(buildScriptPath, []byte(WriteScriptUnixLF(buildOnly)), 0755); werr != nil {
		return werr
	}
	// Policy marker for host-side readers
	policy := CompileIsolationPolicy + "\nHost client OS: " + HostPlatformLabel() + "\n"
	_ = os.WriteFile(filepath.Join(srcHost, "COMPILE-IN-DOCKER-ONLY.txt"), []byte(WriteScriptUnixLF(policy)), 0644)

	if !cfg.DryRun {
		// Hard rule: compile stage must be docker run of Linux container script.
		runArgs := RunContainerArgs(wsBase, []string{"bash", "-lc", "bash -x /work/build-only.sh 2>&1"})
		if !IsDockerCompileCommand(runArgs) {
			return fmt.Errorf("内部错误: 编译命令未路由到 Docker")
		}
		out, runErr := o.runner.Run(ctx, nil, runArgs[0], runArgs[1:]...)
		if runErr != nil {
			return FormatDockerRunError("Docker 内编译失败（Host 未执行 make）", out, runErr)
		}
	} else {
		// Simulate artifacts
		for _, id := range cfg.TargetIDs {
			dir := filepath.Join(artHost, id)
			_ = os.MkdirAll(dir, 0755)
			_ = os.WriteFile(filepath.Join(dir, cfg.MagicName+"-server-"+cfg.FridaVersion+"-"+id), []byte("dry-run artifact\n"), 0644)
		}
		notify(StageBuild, 0.75, "dry-run: 已写入模拟产物（真实路径为 Docker 内 make）", nil)
	}

	// --- export (catalog: version / platform / magic) ---
	catalogRoot := CatalogRoot(workRoot)
	notify(StageExport, 0.85, "导出产物到分类目录 "+catalogRoot+" …", nil)
	if err = ctx.Err(); err != nil {
		return err
	}
	dockerArts := filepath.Join(srcHost, "artifacts")
	// Post-build basename rename: keep source compile-compatible, magic names only on outputs
	if n, rerr := RenameArtifactBasenames(dockerArts, cfg.MagicName); rerr == nil && n > 0 {
		notify(StageExport, 0.84, fmt.Sprintf("产物 basename 魔改重命名 %d 个 → magic=%s", n, cfg.MagicName), nil)
	}
	// Keep a flat copy under artHost for backward compatibility
	if entries, rdErr := os.ReadDir(dockerArts); rdErr == nil {
		for _, e := range entries {
			src := filepath.Join(dockerArts, e.Name())
			dst := filepath.Join(artHost, e.Name())
			_ = copyPath(src, dst)
		}
	}
	_, _ = RenameArtifactBasenames(artHost, cfg.MagicName)
	primary, catalogEntries, orgErr := OrganizeExportToCatalog(catalogRoot, cfg, dockerArts, artHost)
	if orgErr != nil {
		notify(StageExport, 0.86, "分类导出警告: "+orgErr.Error(), nil)
	}
	if primary == "" {
		primary = artHost
	}
	o.mu.Lock()
	o.state.Artifact = primary
	o.mu.Unlock()
	tips := ArtifactDeployTips(primary, cfg)
	_ = os.WriteFile(filepath.Join(primary, "README-DEPLOY.txt"), []byte(tips), 0644)
	_ = os.WriteFile(filepath.Join(artHost, "README-DEPLOY.txt"), []byte(tips+"\n分类目录: "+catalogRoot+"\n"), 0644)
	notify(StageExport, 0.88, fmt.Sprintf("产物已分类: %s （共 %d 个 platform 条目）", primary, len(catalogEntries)), nil)

	// --- tools package: patch frida-tools + build wheel into catalog ---
	// Always full protocol surface (rpc + re.frida.* + Frida.*) so host client matches
	// PatchArtifactBinaryMarkers on server binaries and deep source rewrites.
	notify(StageToolsPatch, 0.92, "魔改并打包 host frida/frida-tools（服务端+客户端协议同步）…", nil)
	if err = ctx.Err(); err != nil {
		return err
	}
	var wheels []string
	if !cfg.DryRun {
		var werr error
		wheels, werr = BuildPatchedFridaToolsWheels(cfg, catalogRoot, catalogEntries, o.runner)
		if werr != nil {
			// Non-fatal for binary-only success, but surface clearly
			marker := filepath.Join(primary, "NEED_FRIDA_TOOLS_PATCH.txt")
			_ = os.WriteFile(marker, []byte(ToolsPatchGuidance(cfg.MagicName, cfg.ListenPort)+
				"\n\n自动 wheel 打包失败: "+werr.Error()+"\n"), 0644)
			notify(StageToolsPatch, 0.95, "frida-tools wheel 打包失败（二进制仍可用）: "+werr.Error(), nil)
		} else {
			notify(StageToolsPatch, 0.96, fmt.Sprintf("已生成 frida-tools wheel: %v", wheels), nil)
			_ = os.WriteFile(filepath.Join(primary, "PYTHON-TOOLS.txt"),
				[]byte(fridaToolsInstallNotes(cfg, wheels, nil)), 0644)
		}
	} else {
		// dry-run: fake wheel note in catalog
		for _, e := range catalogEntries {
			py := filepath.Join(e, "python")
			_ = os.MkdirAll(py, 0755)
			_ = os.WriteFile(filepath.Join(py, "INSTALL.txt"), []byte("dry-run wheel placeholder\n"), 0644)
		}
		notify(StageToolsPatch, 0.95, "dry-run: 跳过 pip download/wheel", nil)
	}

	// --- protocol cross-check: server binary vs host wheel ---
	if !cfg.DryRun {
		notify(StageToolsPatch, 0.97, "核对服务端与客户端协议面（re.{magic}. / {magic}:rpc）…", nil)
		if report, cerr := RunCatalogProtocolCrossCheck(catalogRoot, cfg, catalogEntries, wheels); cerr != nil {
			notify(StageToolsPatch, 0.98, "协议交叉核对跳过: "+cerr.Error(), nil)
		} else {
			outPath := filepath.Join(primary, "PROTOCOL-SYNC.json")
			_ = WriteProtocolCrossCheckReport(outPath, report)
			for _, e := range catalogEntries {
				_ = WriteProtocolCrossCheckReport(filepath.Join(e, "PROTOCOL-SYNC.json"), report)
			}
			if report.Matched {
				notify(StageToolsPatch, 0.99, fmt.Sprintf("协议交叉核对通过 magic=%s serverOK=%v clientOK=%v",
					cfg.MagicName, report.ServerOK, report.ClientOK), nil)
			} else {
				notify(StageToolsPatch, 0.99, fmt.Sprintf("协议交叉核对有告警: %v（见 PROTOCOL-SYNC.json）", report.Issues), nil)
			}
		}
	}

	notify(StageDone, 1.0, "完成。分类产物: "+primary+" ；总目录: "+catalogRoot+" （服务端+客户端同 magic）", nil)
	return nil
}

// ArtifactDeployTips returns user-facing deploy/use notes.
func ArtifactDeployTips(artifactDir string, cfg JobConfig) string {
	var b strings.Builder
	b.WriteString("Fridare 源码重编译产物\n")
	b.WriteString("====================\n\n")
	b.WriteString(fmt.Sprintf("Frida 版本: %s\n", cfg.FridaVersion))
	b.WriteString(fmt.Sprintf("魔改名称: %s\n", cfg.MagicName))
	b.WriteString(fmt.Sprintf("端口: %d\n", cfg.ListenPort))
	b.WriteString(fmt.Sprintf("魔改强度: %s\n", strings.TrimSpace(cfg.DirectionProfile)))
	b.WriteString(fmt.Sprintf("目标: %s\n", strings.Join(cfg.TargetIDs, ", ")))
	b.WriteString(fmt.Sprintf("产物目录: %s\n\n", artifactDir))
	b.WriteString("目录结构: catalog/{version}/{platform}/{magic}/binaries|python\n\n")
	b.WriteString("【服务端 + 客户端必须同 magic】\n")
	b.WriteString(fmt.Sprintf("  frida:rpc → %s:rpc\n", cfg.MagicName))
	b.WriteString(fmt.Sprintf("  re.frida.* → re.%s.*\n", cfg.MagicName))
	pas := cfg.MagicName
	if len(pas) >= 1 {
		pas = strings.ToUpper(pas[:1]) + pas[1:]
	}
	b.WriteString(fmt.Sprintf("  Frida.* API 字面量 → %s.*\n", pas))
	b.WriteString("  勿 pip install 官方未魔改 frida/frida-tools。\n\n")
	b.WriteString("部署提示:\n")
	b.WriteString("1. 部署 binaries 下 *server* 到设备并 chmod +x 运行\n")
	b.WriteString("2. 本机客户端: 先装 python/host/<你的OS-arch>/frida-*.whl，再装 python/frida_tools-*.whl\n")
	b.WriteString("   （详见 python/INSTALL.txt；PROTOCOL-SYNC.json 为协议交叉核对结果）\n")
	b.WriteString("3. 端口不一致时客户端加 -p <port>\n")
	b.WriteString("4. GUI 源码重编译页可浏览历史 catalog，无需重新编译\n\n")
	b.WriteString("双技术路线:\n- 静态补丁: 无需 Docker，见「frida 魔改」\n- 源码重编译: 本目录产物\n")
	return b.String()
}

// ToolsPatchGuidance is the post-build frida-tools note.
func ToolsPatchGuidance(magicName string, port int) string {
	pas := magicName
	if len(pas) >= 1 {
		pas = strings.ToUpper(pas[:1]) + pas[1:]
	}
	return fmt.Sprintf(`主机 frida / frida-tools 必须与魔改 server 使用相同 magic，否则连不上。

服务端+客户端同步面:
  frida:rpc → %s:rpc
  re.frida.* → re.%s.*
  Frida.* → %s.*

优先使用本任务 catalog 中的 wheel:
  pip install --force-reinstall --no-deps python/host/<os-arch>/frida-*.whl
  pip install --force-reinstall --no-deps python/frida_tools-*.whl

或 GUI「源码重编译」→ 产物目录浏览 → 打开对应 version/platform/magic。

备选: 「🛠️ frida-tools 魔改」标签就地改 site-packages（用 full 协议面；勿全局替换 frida 字符串）。
- magic: %s  端口: %d
`, magicName, magicName, pas, magicName, port)
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, 0755); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyPath(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}

// StageIndex returns the index of stage in OrderedStages, or -1.
func StageIndex(s JobStage) int {
	for i, st := range OrderedStages {
		if st == s {
			return i
		}
	}
	return -1
}

// hostGitShallowClone runs git clone on the host (localhost proxy works here).
// Used as fallback when Docker clone fails due to container network/proxy.
func hostGitShallowClone(ctx context.Context, runner Runner, cfg JobConfig, dest string) error {
	if runner == nil {
		runner = ExecRunner{}
	}
	args, err := ShallowCloneArgs(CloneSpec{Version: cfg.FridaVersion, DestDir: dest})
	if err != nil {
		return err
	}
	env := os.Environ()
	if p := strings.TrimSpace(cfg.Proxy); p != "" {
		env = setEnv(env, "HTTP_PROXY", p)
		env = setEnv(env, "HTTPS_PROXY", p)
		env = setEnv(env, "http_proxy", p)
		env = setEnv(env, "https_proxy", p)
	}
	_ = os.RemoveAll(dest)
	out, runErr := runner.Run(ctx, env, args[0], args[1:]...)
	if runErr != nil {
		return fmt.Errorf("host git clone: %w\n%s", runErr, truncate(out, 400))
	}
	return nil
}
