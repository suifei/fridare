package rebuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ProbeResult is one dependency check outcome.
type ProbeResult struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Detail    string `json:"detail"`
	Required  bool   `json:"required"` // true when source-mod path needs it
}

// DepsReport aggregates environment probes for the source-rebuild feature.
type DepsReport struct {
	Docker     ProbeResult `json:"docker"`
	GrokBuild  ProbeResult `json:"grok_build"`
	Git        ProbeResult `json:"git"`
	Proxy      ProbeResult `json:"proxy"`
	Disk       ProbeResult `json:"disk"`
	OpenAI     ProbeResult `json:"openai"`
	CheckedAt  time.Time   `json:"checked_at"`
	Ready      bool        `json:"ready"` // all required probes pass
	Messages   []string    `json:"messages"`
	MinDiskGB  float64     `json:"min_disk_gb"`
	FreeDiskGB float64     `json:"free_disk_gb"`
}

// ProbeOptions configures dependency probing.
type ProbeOptions struct {
	// WorkDir is the path used for free-disk checks (defaults to user home).
	WorkDir string
	// Proxy is the configured HTTP(S) proxy URL; empty means missing.
	Proxy string
	// OpenAIBaseURL / OpenAIAPIKey for endpoint readiness.
	OpenAIBaseURL string
	OpenAIAPIKey  string
	// MinDiskGB threshold (defaults to DefaultMinDiskGB).
	MinDiskGB float64
	// LookPath is injectable for tests; defaults to exec.LookPath.
	LookPath func(file string) (string, error)
	// RunCommand is injectable for tests; defaults to exec.Command(...).CombinedOutput.
	RunCommand func(name string, args ...string) (string, error)
	// FreeDiskGB is injectable for tests; defaults to platform free-space probe.
	FreeDiskGB func(path string) (float64, error)
}

func (o *ProbeOptions) withDefaults() ProbeOptions {
	out := *o
	if out.MinDiskGB <= 0 {
		out.MinDiskGB = DefaultMinDiskGB
	}
	if out.LookPath == nil {
		out.LookPath = exec.LookPath
	}
	if out.RunCommand == nil {
		out.RunCommand = func(name string, args ...string) (string, error) {
			cmd := exec.Command(name, args...)
			b, err := cmd.CombinedOutput()
			return string(b), err
		}
	}
	if out.FreeDiskGB == nil {
		out.FreeDiskGB = freeDiskGB
	}
	if out.WorkDir == "" {
		home, _ := os.UserHomeDir()
		out.WorkDir = home
	}
	return out
}

// ProbeDeps checks Docker, disk, grok-build, git, proxy, and OpenAI settings.
// Source-mod path requires: Docker (daemon), git, proxy, disk, OpenAI endpoint.
// grok-build is strongly recommended but can be installed later; if missing,
// Ready is false with a clear message (user may install or point to local binary).
func ProbeDeps(opts ProbeOptions) DepsReport {
	o := opts.withDefaults()
	report := DepsReport{
		CheckedAt: time.Now(),
		MinDiskGB: o.MinDiskGB,
	}

	// Docker
	report.Docker = probeDocker(o)

	// grok / grok-build CLI (product may ship as "grok" or "grok-build")
	report.GrokBuild = probeGrokBuild(o)

	// git
	report.Git = probeBinary(o, "git", true, []string{"--version"})

	// proxy
	proxy := strings.TrimSpace(o.Proxy)
	if proxy == "" {
		report.Proxy = ProbeResult{
			Name:      "proxy",
			Available: false,
			Required:  true,
			Detail:    "源码魔改需要配置上游代理（设置页 / 工具栏）。克隆 Frida 与拉取依赖通常需要代理。",
		}
	} else {
		report.Proxy = ProbeResult{
			Name:      "proxy",
			Available: true,
			Required:  true,
			Detail:    "已配置: " + maskProxy(proxy),
		}
	}

	// disk
	freeGB, err := o.FreeDiskGB(o.WorkDir)
	report.FreeDiskGB = freeGB
	if err != nil {
		report.Disk = ProbeResult{
			Name:      "disk",
			Available: false,
			Required:  true,
			Detail:    fmt.Sprintf("无法探测磁盘空间 (%s): %v", o.WorkDir, err),
		}
	} else if freeGB < o.MinDiskGB {
		report.Disk = ProbeResult{
			Name:      "disk",
			Available: false,
			Required:  true,
			Detail:    fmt.Sprintf("空闲 %.1f GB < 建议最低 %.0f GB（路径: %s）", freeGB, o.MinDiskGB, o.WorkDir),
		}
	} else {
		report.Disk = ProbeResult{
			Name:      "disk",
			Available: true,
			Required:  true,
			Detail:    fmt.Sprintf("空闲 %.1f GB（建议 ≥ %.0f GB）", freeGB, o.MinDiskGB),
		}
	}

	// OpenAI endpoint
	base := strings.TrimSpace(o.OpenAIBaseURL)
	key := strings.TrimSpace(o.OpenAIAPIKey)
	if base == "" {
		report.OpenAI = ProbeResult{
			Name:      "openai",
			Available: false,
			Required:  true,
			Detail:    "未配置 OpenAI 兼容 API 端点。推荐: " + RecommendedAPIBase,
		}
	} else if key == "" {
		report.OpenAI = ProbeResult{
			Name:      "openai",
			Available: false,
			Required:  true,
			Detail:    "已配置端点但缺少 API Key: " + base,
		}
	} else {
		report.OpenAI = ProbeResult{
			Name:      "openai",
			Available: true,
			Required:  true,
			Detail:    fmt.Sprintf("端点 %s（Key 已设置）", base),
		}
	}

	// Aggregate readiness: all Required && Available
	report.Ready = true
	for _, p := range []ProbeResult{report.Docker, report.GrokBuild, report.Git, report.Proxy, report.Disk, report.OpenAI} {
		if p.Required && !p.Available {
			report.Ready = false
			report.Messages = append(report.Messages, fmt.Sprintf("[%s] %s", p.Name, p.Detail))
		}
	}
	if report.Ready {
		report.Messages = append(report.Messages, "依赖检查通过，可启动源码重编译任务。")
	}
	return report
}

// ProxyRequiredForSourceMod returns an error if proxy is empty when user enables source mod.
func ProxyRequiredForSourceMod(proxy string) error {
	if strings.TrimSpace(proxy) == "" {
		return fmt.Errorf("启用源码魔改前必须配置上游代理（设置页或工具栏）")
	}
	return nil
}

// CanStartSourceJob validates gates for starting a full develop job (step 2).
func CanStartSourceJob(report DepsReport, proxy string) error {
	if err := ProxyRequiredForSourceMod(proxy); err != nil {
		return err
	}
	if !report.Ready {
		if len(report.Messages) > 0 {
			return fmt.Errorf("依赖未就绪: %s", strings.Join(report.Messages, "; "))
		}
		return fmt.Errorf("依赖未就绪，请先「检查依赖」")
	}
	return nil
}

// CanStartBootstrapJob validates gates for step 1 (base image only).
// Needs Docker + disk; does not require OpenAI / grok (AI is step 2).
func CanStartBootstrapJob(report DepsReport) error {
	if !report.Docker.Available {
		return fmt.Errorf("步骤① 需要 Docker: %s", report.Docker.Detail)
	}
	if !report.Disk.Available {
		return fmt.Errorf("步骤① 磁盘不足: %s", report.Disk.Detail)
	}
	return nil
}

func probeDocker(o ProbeOptions) ProbeResult {
	path, err := o.LookPath("docker")
	if err != nil {
		return ProbeResult{
			Name:      "docker",
			Available: false,
			Required:  true,
			Detail:    "未找到 docker 可执行文件。请安装 Docker Desktop / Engine 后重试（可选功能，不强制改系统）。",
		}
	}
	out, err := o.RunCommand("docker", "info", "--format", "{{.ServerVersion}}")
	if err != nil {
		// client present but daemon down
		return ProbeResult{
			Name:      "docker",
			Available: false,
			Required:  true,
			Detail:    fmt.Sprintf("找到 docker (%s) 但守护进程不可用: %v — %s", path, err, truncate(out, 200)),
		}
	}
	ver := strings.TrimSpace(out)
	if ver == "" {
		ver = "ok"
	}
	return ProbeResult{
		Name:      "docker",
		Available: true,
		Required:  true,
		Detail:    fmt.Sprintf("Docker 可用 (%s)，Server: %s", path, ver),
	}
}

func probeGrokBuild(o ProbeOptions) ProbeResult {
	// Prefer grok-build, then grok (Grok Build CLI is often installed as "grok")
	for _, name := range []string{"grok-build", "grok"} {
		path, err := o.LookPath(name)
		if err != nil {
			continue
		}
		// Try a lightweight version/help call; failure still counts as present.
		out, runErr := o.RunCommand(path, "--version")
		detail := fmt.Sprintf("已找到 %s (%s)", name, path)
		if runErr == nil && strings.TrimSpace(out) != "" {
			detail += ": " + truncate(strings.TrimSpace(out), 120)
		}
		return ProbeResult{
			Name:      "grok_build",
			Available: true,
			Required:  true,
			Detail:    detail,
		}
	}
	return ProbeResult{
		Name:      "grok_build",
		Available: false,
		Required:  true,
		Detail:    "未找到 grok-build / grok。请安装 Grok Build CLI，或在本机 PATH 中提供可执行文件。AI 端点仍使用 GUI 配置。",
	}
}

func probeBinary(o ProbeOptions, name string, required bool, versionArgs []string) ProbeResult {
	path, err := o.LookPath(name)
	if err != nil {
		return ProbeResult{
			Name:      name,
			Available: false,
			Required:  required,
			Detail:    fmt.Sprintf("未找到 %s", name),
		}
	}
	detail := path
	if len(versionArgs) > 0 {
		out, runErr := o.RunCommand(path, versionArgs...)
		if runErr == nil {
			detail = truncate(strings.TrimSpace(out), 120)
		}
	}
	return ProbeResult{
		Name:      name,
		Available: true,
		Required:  required,
		Detail:    detail,
	}
}

func maskProxy(proxy string) string {
	// Avoid dumping credentials if user put user:pass@host
	if i := strings.Index(proxy, "@"); i >= 0 {
		return "***@" + proxy[i+1:]
	}
	return proxy
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ResolveGrokBinary returns the preferred grok-build/grok path if present.
func ResolveGrokBinary(lookPath func(string) (string, error)) (string, bool) {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	for _, name := range []string{"grok-build", "grok"} {
		if p, err := lookPath(name); err == nil && p != "" {
			return p, true
		}
	}
	return "", false
}

// DefaultArtifactDir returns the default host directory for build outputs.
func DefaultArtifactDir(workDir string) string {
	if workDir == "" {
		home, _ := os.UserHomeDir()
		workDir = filepath.Join(home, ".fridare")
	}
	return filepath.Join(workDir, "rebuild", "artifacts")
}

// DefaultSourceWorkDir is the host bind-mount root for Docker source builds.
func DefaultSourceWorkDir(workDir string) string {
	if workDir == "" {
		home, _ := os.UserHomeDir()
		workDir = filepath.Join(home, ".fridare")
	}
	return filepath.Join(workDir, "rebuild", "src")
}

// GOOS helper used by disk probe selection messaging.
func HostGOOS() string {
	return runtime.GOOS
}
