package rebuild

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// CloneSpec describes a shallow Frida source clone.
type CloneSpec struct {
	// Version is a tag, branch, or commit-ish (e.g. "16.5.9", "17.16.4").
	Version string
	// DestDir is the destination path inside the workspace (host or container).
	DestDir string
	// RepoURL defaults to FridaGitURL.
	RepoURL string
}

// Validate ensures the clone version is non-empty and safe-ish.
func (c CloneSpec) Validate() error {
	v := strings.TrimSpace(c.Version)
	if v == "" {
		return fmt.Errorf("必须指定 Frida 官方版本（tag/commit）")
	}
	// Reject path traversal / spaces that break shell scripts
	if strings.ContainsAny(v, " \t\n\r;&|<>`$\\\"'") {
		return fmt.Errorf("版本字符串包含非法字符: %q", v)
	}
	return nil
}

// ShallowCloneArgs builds: git clone --depth=1 --branch <version> <url> <dest>
// When version looks like a full 40-char commit, uses a two-step approach documented
// in CloneScript instead (clone depth=1 default branch + fetch).
func ShallowCloneArgs(spec CloneSpec) ([]string, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	url := spec.RepoURL
	if url == "" {
		url = FridaGitURL
	}
	dest := spec.DestDir
	if dest == "" {
		dest = "frida"
	}
	// Standard shallow clone of a tag/branch:
	// git clone --depth=1 --branch <version> <url> <dest>
	return []string{
		"git", "clone",
		"--depth=1",
		"--branch", strings.TrimSpace(spec.Version),
		"--single-branch",
		url,
		dest,
	}, nil
}

// CloneCommandLine returns a shell-safe single line for logging / docker exec.
func CloneCommandLine(spec CloneSpec) (string, error) {
	args, err := ShallowCloneArgs(spec)
	if err != nil {
		return "", err
	}
	return shellJoin(args), nil
}

// ManagementBranchName creates a unique branch name for pre-mod source management.
func ManagementBranchName(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return fmt.Sprintf("fridare/mod-%s", now.UTC().Format("20060102-150405"))
}

// CreateBranchArgs returns git commands to create and checkout a management branch.
// Must be run inside the cloned source tree.
func CreateBranchArgs(branch string) []string {
	if branch == "" {
		branch = ManagementBranchName(time.Now())
	}
	return []string{"git", "checkout", "-b", branch}
}

// DockerWorkspace describes the optional compile environment.
type DockerWorkspace struct {
	Image         string // e.g. DefaultBuildImage
	ContainerName string
	// HostWorkDir is bind-mounted into the container.
	HostWorkDir string
	// ContainerWorkDir is the mount point inside the container.
	ContainerWorkDir string
	// Proxy env for container.
	HTTPProxy  string
	HTTPSProxy string
	// Extra env
	Env map[string]string
}

// BootstrapImageArgs returns docker pull/build style command generation.
// For the default image we generate a pull; custom Dockerfiles can extend later.
func BootstrapImageArgs(image string) []string {
	if image == "" {
		image = DefaultBuildImage
	}
	return []string{"docker", "pull", image}
}

// RunContainerArgs builds a long-running (or one-shot) docker run command skeleton.
// Proxy URLs are rewritten for in-container reachability (localhost → host.docker.internal).
func RunContainerArgs(ws DockerWorkspace, cmd []string) []string {
	image := ws.Image
	if image == "" {
		image = DefaultBuildImage
	}
	cWork := ws.ContainerWorkDir
	if cWork == "" {
		cWork = "/work"
	}
	args := []string{
		"docker", "run", "--rm",
	}
	// Linux Engine: resolve host.docker.internal; Desktop already has it.
	args = append(args, DockerExtraHostArgs()...)
	if ws.ContainerName != "" {
		// replace --rm with named container for attachable jobs; keep flexible
		args = []string{"docker", "run", "--name", ws.ContainerName}
		args = append(args, DockerExtraHostArgs()...)
	}
	if ws.HostWorkDir != "" {
		// Cross-OS: normalize host path for Docker Desktop on Windows/macOS/Linux
		hostVol := DockerVolumePath(ws.HostWorkDir)
		args = append(args, "-v", fmt.Sprintf("%s:%s", hostVol, cWork))
		args = append(args, "-w", cWork)
	}
	httpP := RewriteProxyForContainer(ws.HTTPProxy)
	httpsP := RewriteProxyForContainer(ws.HTTPSProxy)
	if httpsP == "" {
		httpsP = httpP
	}
	if httpP != "" {
		args = append(args, "-e", "HTTP_PROXY="+httpP, "-e", "http_proxy="+httpP)
	}
	if httpsP != "" {
		args = append(args, "-e", "HTTPS_PROXY="+httpsP, "-e", "https_proxy="+httpsP)
		// git / curl often honor ALL_PROXY too
		args = append(args, "-e", "ALL_PROXY="+httpsP, "-e", "all_proxy="+httpsP)
	}
	for k, v := range ws.Env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, image)
	args = append(args, cmd...)
	return args
}

// CompileIsolationPolicy documents the product rule: never run Frida configure/make
// on the host OS. Windows toolchains are especially painful; all compile work is
// Docker + Linux. Host only runs GUI, probes, optional git metadata, and AI agent
// file edits on a bind-mounted workspace.
const CompileIsolationPolicy = `Frida 源码编译隔离策略:
- configure / make / NDK 交叉编译：仅在 Docker（Linux）容器内执行
- 禁止在 Windows/macOS Host 上安装或调用完整 Frida 编译工具链
- Host 仅负责：GUI、依赖探测、AI agent 改文件（bind-mount 工作区）、docker 客户端
- 产物通过 volume 写回 Host 工作目录供测试
`

// DockerBuildStageMarker is emitted by container build scripts (for tests/logs).
const DockerBuildStageMarker = "[fridare] docker-only compile (no host toolchain)"

// BuildPipelineScript generates a portable shell script body run inside Docker:
// clone (depth=1) → branch → (mod placeholder) → configure/make for each target → collect artifacts.
// The AI agent fills the mod section; this script is the skeleton orchestrator emits.
// IMPORTANT: This script is intended to run ONLY inside Docker — never on Windows host shells.
func BuildPipelineScript(opts PipelineScriptOptions) (string, error) {
	if err := opts.Clone.Validate(); err != nil {
		return "", err
	}
	if len(opts.TargetIDs) == 0 {
		return "", fmt.Errorf("至少选择一个编译目标")
	}
	branch := opts.Branch
	if branch == "" {
		branch = ManagementBranchName(time.Now())
	}
	srcDir := opts.SourceDir
	if srcDir == "" {
		srcDir = "frida-src"
	}
	artifactDir := opts.ArtifactDir
	if artifactDir == "" {
		artifactDir = "artifacts"
	}

	cloneArgs, err := ShallowCloneArgs(CloneSpec{
		Version: opts.Clone.Version,
		DestDir: srcDir,
		RepoURL: opts.Clone.RepoURL,
	})
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n")
	b.WriteString("echo '[fridare] pipeline start'\n")
	b.WriteString("echo '" + DockerBuildStageMarker + "'\n")
	b.WriteString(fmt.Sprintf("mkdir -p %s %s\n", shellQuote(artifactDir), shellQuote(path.Dir(srcDir))))
	// Avoid re-clone if present
	b.WriteString(fmt.Sprintf("if [ ! -d %s/.git ]; then\n", shellQuote(srcDir)))
	b.WriteString("  " + shellJoin(cloneArgs) + "\n")
	b.WriteString("else\n  echo '[fridare] source already present, skip clone'\nfi\n")
	b.WriteString(fmt.Sprintf("cd %s\n", shellQuote(srcDir)))
	b.WriteString("git config user.email 'fridare@local' || true\n")
	b.WriteString("git config user.name 'fridare' || true\n")
	// Create management branch before mods
	b.WriteString(fmt.Sprintf("git checkout -B %s\n", shellQuote(branch)))
	b.WriteString("echo '[fridare] management branch: " + branch + "'\n")

	if opts.ModScript != "" {
		b.WriteString("echo '[fridare] applying source modifications'\n")
		b.WriteString(opts.ModScript)
		b.WriteString("\n")
		b.WriteString("git add -A\n")
		b.WriteString("git commit -m 'fridare: source modifications' || true\n")
	} else {
		b.WriteString("echo '[fridare] no mod script embedded; AI agent applies mods separately on host bind-mount'\n")
	}

	compileBody, err := compileTargetsShell(opts.TargetIDs, srcDir, artifactDir, opts.StealthSeed)
	if err != nil {
		return "", err
	}
	b.WriteString(compileBody)
	b.WriteString("echo '[fridare] pipeline done'\n")
	return b.String(), nil
}

// BuildOnlyPipelineScript assumes source already exists (host agent already patched
// the bind-mounted tree). Runs ONLY configure/make/collect inside Docker — never on host.
func BuildOnlyPipelineScript(opts PipelineScriptOptions) (string, error) {
	if len(opts.TargetIDs) == 0 {
		return "", fmt.Errorf("至少选择一个编译目标")
	}
	srcDir := opts.SourceDir
	if srcDir == "" {
		srcDir = "frida"
	}
	artifactDir := opts.ArtifactDir
	if artifactDir == "" {
		artifactDir = "artifacts"
	}
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n")
	b.WriteString("echo '" + DockerBuildStageMarker + "'\n")
	b.WriteString("echo '[fridare] build-only: source must already exist at " + srcDir + "'\n")
	b.WriteString(fmt.Sprintf("if [ ! -d %s ]; then echo 'missing source tree' >&2; exit 1; fi\n", shellQuote(srcDir)))
	b.WriteString(fmt.Sprintf("mkdir -p %s\n", shellQuote(artifactDir)))
	compileBody, err := compileTargetsShell(opts.TargetIDs, srcDir, artifactDir, opts.StealthSeed)
	if err != nil {
		return "", err
	}
	b.WriteString(compileBody)
	b.WriteString("echo '[fridare] build-only done'\n")
	return b.String(), nil
}

// Android NDK r29 (Frida 17.x configure requires ANDROID_NDK_ROOT = r29).
// Install path: ONLY via Dockerfile (docker build). Runtime only verifies.
const (
	AndroidNDKVersion = "r29"
	AndroidNDKURL     = "https://dl.google.com/android/repository/android-ndk-r29-linux.zip"
	AndroidNDKDirName = "android-ndk-r29" // legacy workdir name; not used for install
)

// NeedsAndroidNDK reports whether any selected target needs ANDROID_NDK_ROOT.
func NeedsAndroidNDK(targetIDs []string) bool {
	for _, id := range targetIDs {
		t, err := TargetByID(id)
		if err != nil {
			continue
		}
		if t.Platform == "android" || strings.HasPrefix(t.Host, "android-") {
			return true
		}
	}
	return false
}

// NeedsLinuxArm64Cross reports whether any target needs aarch64-linux-gnu-gcc
// (Frida linux-arm64 product on an amd64 Linux Docker builder).
func NeedsLinuxArm64Cross(targetIDs []string) bool {
	for _, id := range targetIDs {
		t, err := TargetByID(id)
		if err != nil {
			continue
		}
		if t.ID == "linux-arm64" || t.Host == "linux-arm64" || t.Host == "aarch64-linux-gnu" ||
			strings.HasPrefix(t.Host, "aarch64-linux-") {
			return true
		}
	}
	return false
}

// VerifyBuildEnvShell checks image-provided toolchain only.
// Heavy deps (NDK / Node ≥18 / Go ≥1.24 / apt tools / aarch64 cross) MUST be baked in Dockerfile.
// AI agent + compile stages never download toolchains — only confirm env, then
// Frida configure may fetch version-specific subprojects.
// When needArm64Cross is true, require aarch64-linux-gnu-gcc (linux-arm64 product builds).
func VerifyBuildEnvShell(needNDK bool) string {
	return VerifyBuildEnvShellEx(needNDK, false)
}

// VerifyBuildEnvShellEx is VerifyBuildEnvShell with optional linux-arm64 cross-compiler check.
func VerifyBuildEnvShellEx(needNDK, needArm64Cross bool) string {
	var b strings.Builder
	b.WriteString("echo '[fridare] verify build environment (toolchain must be preinstalled in image)'\n")
	b.WriteString("command -v git >/dev/null && command -v make >/dev/null && command -v python3 >/dev/null || { echo '[fridare] missing basic tools in image — rebuild fridare/frida-builder' >&2; exit 1; }\n")
	// Node.js >= 18 (Frida meson hard-requires this)
	b.WriteString("command -v node >/dev/null || { echo '[fridare] node missing in image — rebuild builder (Dockerfile installs Node 20)' >&2; exit 1; }\n")
	b.WriteString("node -e \"const M=process.versions.node.split('.')[0]|0; if(M<18){console.error('[fridare] need Node>=18, got',process.version); process.exit(1)}\" || exit 1\n")
	b.WriteString("command -v npm >/dev/null || { echo '[fridare] npm missing in image' >&2; exit 1; }\n")
	// Go is recommended (Compiler backend); warn if absent
	b.WriteString("if command -v go >/dev/null; then echo \"[fridare] go OK: $(go version)\"; else echo '[fridare] WARN: go not found (Compiler backend may be disabled)' >&2; fi\n")
	if needArm64Cross {
		b.WriteString("command -v aarch64-linux-gnu-gcc >/dev/null || { echo '[fridare] aarch64-linux-gnu-gcc missing — rebuild fridare/frida-builder (Dockerfile installs gcc-aarch64-linux-gnu for linux-arm64)' >&2; exit 1; }\n")
		b.WriteString("echo \"[fridare] aarch64 cross OK: $(aarch64-linux-gnu-gcc --version | head -1)\"\n")
	}
	b.WriteString("echo \"[fridare] features=$(cat /etc/fridare-builder-features 2>/dev/null || echo none)\"\n")
	b.WriteString("echo \"[fridare] node=$(node -v) npm=$(npm -v)\"\n")
	if !needNDK {
		return b.String()
	}
	imgNDK := shellQuote(BuilderImageNDKPath)
	// Prefer image ENV /opt/android-ndk-r29 only — no per-job download
	b.WriteString("echo '[fridare] checking Android NDK (must be preinstalled in image)'\n")
	b.WriteString("if [ -n \"${ANDROID_NDK_ROOT:-}\" ] && [ -d \"${ANDROID_NDK_ROOT}\" ]; then\n")
	b.WriteString("  echo \"[fridare] NDK OK (env): $ANDROID_NDK_ROOT\"\n")
	b.WriteString(fmt.Sprintf("elif [ -d %s ]; then\n", imgNDK))
	b.WriteString(fmt.Sprintf("  export ANDROID_NDK_ROOT=%s\n", imgNDK))
	b.WriteString("  echo \"[fridare] NDK OK (image): $ANDROID_NDK_ROOT\"\n")
	b.WriteString("else\n")
	b.WriteString("  echo '[fridare] ANDROID_NDK_ROOT missing — rebuild fridare/frida-builder (NDK is baked at docker build, not per job)' >&2\n")
	b.WriteString("  exit 1\n")
	b.WriteString("fi\n")
	b.WriteString("export ANDROID_NDK_ROOT\n")
	b.WriteString("test -d \"$ANDROID_NDK_ROOT\" || { echo '[fridare] ANDROID_NDK_ROOT invalid' >&2; exit 1; }\n")
	return b.String()
}

// EnsureAndroidNDKShell is kept as an alias for older call sites — verifies env only.
func EnsureAndroidNDKShell() string {
	return VerifyBuildEnvShell(true)
}

// compileTargetsShell is the configure/make body that must only run in Linux Docker.
// Script assumes execution from container workdir /work with source at /work/<srcDir>.
// Build dirs are /work/build-<id> (sibling of source), NOT ../build-* (that escapes the volume).
func compileTargetsShell(targetIDs []string, srcDir, artifactDir, stealthSeed string) (string, error) {
	var b strings.Builder
	srcBase := path.Base(srcDir)
	if srcBase == "" || srcBase == "." {
		srcBase = "frida"
	}
	b.WriteString("echo '[fridare] starting out-of-tree configure/make inside container'\n")
	b.WriteString("echo \"[fridare] pwd=$(pwd); ls -la\"\n")
	b.WriteString(fmt.Sprintf("if [ ! -d %s ]; then echo '[fridare] ERROR: missing source dir %s' >&2; ls -la; exit 127; fi\n",
		shellQuote(srcDir), shellQuote(srcDir)))
	b.WriteString(fmt.Sprintf("if [ ! -f %s/configure ] && [ ! -x %s/configure ]; then echo '[fridare] ERROR: no configure in source; listing:' >&2; ls -la %s | head -50; exit 127; fi\n",
		shellQuote(srcDir), shellQuote(srcDir), shellQuote(srcDir)))
	b.WriteString(fmt.Sprintf("chmod +x %s/configure 2>/dev/null || true\n", shellQuote(srcDir)))
	// Verify image toolchain (NDK / aarch64 cross baked at docker build); do not re-download every job
	b.WriteString(VerifyBuildEnvShellEx(NeedsAndroidNDK(targetIDs), NeedsLinuxArm64Cross(targetIDs)))
	b.WriteString("\n")
	// linux-arm64: ensure cross CC is visible to Frida configure/meson
	// (host triplet aarch64-linux-gnu OR explicit CC= both work; set both for robustness)
	if NeedsLinuxArm64Cross(targetIDs) {
		b.WriteString("export CC=\"${CC:-aarch64-linux-gnu-gcc}\"\n")
		b.WriteString("export CXX=\"${CXX:-aarch64-linux-gnu-g++}\"\n")
		b.WriteString("export CC_FOR_linux_arm64=\"${CC_FOR_linux_arm64:-aarch64-linux-gnu-gcc}\"\n")
		b.WriteString("export CXX_FOR_linux_arm64=\"${CXX_FOR_linux_arm64:-aarch64-linux-gnu-g++}\"\n")
	}
	for _, id := range targetIDs {
		t, err := TargetByID(id)
		if err != nil {
			return "", err
		}
		// Sibling under /work — stays on the bind mount
		buildDir := "build-" + t.ID
		b.WriteString(fmt.Sprintf("echo '[fridare] configure/make target %s (--host=%s)'\n", t.ID, t.Host))
		if !t.DockerFriendly {
			b.WriteString(fmt.Sprintf("echo '[fridare] WARNING: target %s may require Apple host toolchain: %s'\n", t.ID, t.Notes))
		}
		// Wipe prior out-of-tree build so reconfigure is not blocked by "Already configured".
		b.WriteString(fmt.Sprintf("rm -rf %s\n", shellQuote(buildDir)))
		b.WriteString(fmt.Sprintf("mkdir -p %s\n", shellQuote(buildDir)))
		// MinGW Windows cross: no official sdk-*-mingw prebuild → --without-prebuilds=sdk:host.
		// Pre-seed meson wraps so git fetch is reliable under proxy; strip CRLF shebangs.
		extraCfg := ""
		isMinGW := strings.Contains(t.Host, "mingw")
		if isMinGW {
			extraCfg = " --without-prebuilds=sdk:host"
			b.WriteString(fmt.Sprintf(
				"command -v %s-gcc >/dev/null || { echo '[fridare] ERROR: missing MinGW %s-gcc (install mingw-w64 in builder image)' >&2; exit 1; }\n",
				t.Host, t.Host))
			// Seed wrap-git deps under frida-core/gum (libffi etc.) before configure.
			// Script is staged by StageSeedMinGWWraps into /work (see orchestrator).
			b.WriteString(SeedMinGWWrapsShellSnippet(srcDir))
			// Do NOT export CFLAGS/CPPFLAGS=-include (pollutes native build-machine cc with
			// Windows __stdcall headers → "compiler for language c not specified for build machine"
			// especially on i686 MinGW). Inject only into the meson *cross* file after configure.
		}
		b.WriteString(fmt.Sprintf(
			"find %s -type f \\( -name '*.py' -o -name 'configure' -o -name '*.sh' \\) -print0 2>/dev/null | xargs -0 -r sed -i 's/\\r$//' || true\n",
			shellQuote(srcDir)))
		// Always request server/gadget/inject: native host builds disable them under meson auto
		// (disable_auto_if(!cross)). Skip frida-python: deep magic renames break GIR bindgen namespaces.
		// disable-frida-python is not enough: frida-tools still subprojects frida-python
		// (native linux then fails on missing pyconfig.h). Tools wheels are patched on the host.
		productOpts := " --enable-server --enable-gadget --enable-inject --disable-frida-python --disable-frida-tools"
		// Per-TU stealth flags (ninja) after configure; never process-wide CFLAGS.
		// Heredocs must be standalone statements (not `cmd && python <<'PY' ... PY && make`).
		perTU := PerTUStealthFlagsShell(stealthSeed)
		b.WriteString("(\n")
		b.WriteString("export ANDROID_NDK_ROOT=\"${ANDROID_NDK_ROOT:-}\"\n")
		if isMinGW {
			b.WriteString("export CFLAGS=\"$(echo \"${CFLAGS:-}\" | sed -E 's/-include[[:space:]]+[^[:space:]]+//g')\"\n")
			b.WriteString("export CPPFLAGS=\"$(echo \"${CPPFLAGS:-}\" | sed -E 's/-include[[:space:]]+[^[:space:]]+//g')\"\n")
		}
		b.WriteString(fmt.Sprintf("cd %s || exit 1\n", shellQuote(buildDir)))
		// Frida 16.x ships v8-mksnapshot in the official SDK; that binary SIGTRAPs
		// (exit 133) even on `print(1);` under this builder. 17.x dropped the option
		// (esbuild compiler). Only pass -D when meson.options still has it.
		b.WriteString(fmt.Sprintf("snapopt=\"\"\n"+
			"if grep -q \"option('compiler_snapshot'\" ../%s/subprojects/frida-core/meson.options 2>/dev/null; then\n"+
			"  snapopt=\"-Dfrida-core:compiler_snapshot=disabled\"\n"+
			"  echo '[fridare] compiler_snapshot=disabled (16.x SDK snapshot tool is unusable here)'\n"+
			"fi\n", shellQuote(srcBase)))
		// `--` is required so configure's argparse treats -D… as extra meson args.
		b.WriteString(fmt.Sprintf("if [ -n \"$snapopt\" ]; then ../%s/configure --host=%s%s%s -- $snapopt || exit 1; else ../%s/configure --host=%s%s%s || exit 1; fi\n",
			shellQuote(srcBase), shellQuote(t.Host), extraCfg, productOpts,
			shellQuote(srcBase), shellQuote(t.Host), extraCfg, productOpts))
		if isMinGW {
			b.WriteString(MinGWCrossFileDNSIncludeShell(MinGWDNSStubHeaderFileName))
			b.WriteString("\n")
		}
		b.WriteString(perTU)
		b.WriteString("\n")
		b.WriteString("make || exit 1\n")
		// Per-target failure must not abort the remaining matrix (16.7.19 8-zip set).
		b.WriteString(fmt.Sprintf(") || { echo '[fridare] configure/make failed for %s (continue)' >&2; }\n", shellQuote(t.ID)))
		b.WriteString(fmt.Sprintf("mkdir -p %s/%s\n", shellQuote(artifactDir), shellQuote(t.ID)))
		// ELF + PE product blobs (skip 0-byte meson stubs)
		b.WriteString(fmt.Sprintf("find %s -type f -size +1k \\( -name 'frida-server*' -o -name 'frida-agent*' -o -name 'frida-gadget*' -o -name '*-server' -o -name '*-server.exe' -o -name '*-server-raw' -o -name '*-server-raw.exe' -o -name '*-agent*.so' -o -name '*-agent*.dll' -o -name '*-gadget*.so' -o -name '*-gadget*.dll' -o -name '*-helper*.exe' -o -name '*-inject*.exe' \\) -exec cp -a {} %s/%s/ \\; 2>/dev/null || true\n",
			shellQuote(buildDir), shellQuote(artifactDir), shellQuote(t.ID)))
	}
	return b.String(), nil
}

// DockerInitSubmodulesShell initializes Frida submodules (frida-core / frida-gum …)
// so source mods can patch real agent/server meson files before configure.
// Must run after clone and BEFORE mod_apply — configure also pulls deps, but too late for rename.
func DockerInitSubmodulesShell(srcDir string) string {
	if srcDir == "" {
		srcDir = "frida"
	}
	return fmt.Sprintf(
		`set -euo pipefail; cd %s; echo '[fridare] init submodules (needed before source mod)'; `+
			`git submodule sync --recursive 2>/dev/null || true; `+
			`git submodule update --init --depth 1 --recursive 2>/dev/null || `+
			`git submodule update --init --recursive 2>/dev/null || true; `+
			`if [ -f tools/ensure-submodules.py ]; then python3 tools/ensure-submodules.py 2>/dev/null || true; fi; `+
			`echo '[fridare] submodules ready'; ls -la subprojects 2>/dev/null | head -20 || true`,
		shellQuote(srcDir),
	)
}

// DockerCloneShell clones Frida with depth=1 inside the container into /work/<dest>.
// Uses a single-line script (more reliable under docker run bash -lc on Windows).
// On failure, prints proxy/env hints for exit 128 (common when localhost proxy is wrong).
func DockerCloneShell(version, dest string) (string, error) {
	spec := CloneSpec{Version: version, DestDir: dest}
	if err := spec.Validate(); err != nil {
		return "", err
	}
	if dest == "" {
		dest = "frida"
	}
	args, err := ShallowCloneArgs(CloneSpec{Version: version, DestDir: dest})
	if err != nil {
		return "", err
	}
	// One line + explicit error context for git exit 128
	// shellJoin is already safely quoted.
	clone := shellJoin(args)
	// Prefer GIT_SSL_NO_VERIFY off; rely on ca-certificates in image.
	script := fmt.Sprintf(
		`set -euo pipefail; echo "[fridare] proxy HTTP_PROXY=${HTTP_PROXY:-} HTTPS_PROXY=${HTTPS_PROXY:-}"; if [ -d %s/.git ]; then echo "[fridare] clone skip, exists"; else echo "[fridare] %s"; %s || { echo "[fridare] git clone failed exit=$?"; echo "[fridare] if proxy is localhost, host must map to host.docker.internal"; exit 128; }; fi`,
		shellQuote(dest), clone, clone,
	)
	return script, nil
}

// FormatDockerRunError enriches docker/git failures (e.g. exit 128) for the UI log.
func FormatDockerRunError(stage string, out string, err error) error {
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(out)
	if len(msg) > 800 {
		msg = msg[len(msg)-800:]
	}
	hint := ""
	low := strings.ToLower(out + err.Error())
	if strings.Contains(low, "connection refused") && (strings.Contains(low, "localhost") || strings.Contains(low, "127.0.0.1")) {
		hint = "；原因：容器内 localhost 代理不可达，应使用 host.docker.internal（软件已自动改写，请更新到最新构建）"
	}
	if strings.Contains(low, "exit status 128") || strings.Contains(low, "exit=128") {
		hint += "；git exit 128 多为网络/代理/认证失败"
	}
	if strings.Contains(low, "ndk_root must") || strings.Contains(low, "ndk not found") ||
		strings.Contains(low, "ndk missing") || strings.Contains(low, "ndk r29 is required") ||
		strings.Contains(low, "android_ndk_root must") {
		hint += "；需要 Android NDK r29（应在 Dockerfile/docker build 预装；请重建 fridare/frida-builder 镜像）"
	}
	if strings.Contains(low, "need node.js") || strings.Contains(low, "need node.js >=") || strings.Contains(low, "nodejs >=18") {
		hint += "；需要 Node.js >=18（应在 Dockerfile 预装 Node 20；请重建 builder 镜像，勿在编译任务中现装）"
	}
	if strings.Contains(low, "need go >=") || strings.Contains(low, "need go >=1") {
		hint += "；需要 Go >=1.24（应在 Dockerfile 预装；请重建 builder 镜像）"
	}
	if strings.Contains(low, ".version does not exist") || strings.Contains(low, ".symbols does not exist") {
		hint += "；魔改后缺 version/symbols 文件：内容替换后需同步重命名 frida-agent* 等资源文件名"
	}
	if strings.Contains(low, "signals.sigtrap") || strings.Contains(low, "trace/breakpoint trap") {
		hint += "；v8-mksnapshot 在此环境无法运行：Frida 16.x 应关闭 compiler_snapshot（与魔改无关）"
	}
	if msg == "" {
		return fmt.Errorf("%s: %v%s", stage, err, hint)
	}
	return fmt.Errorf("%s: %v%s\n--- docker/git 输出 ---\n%s", stage, err, hint, msg)
}

// DockerBranchShell creates management branch inside container source tree.
func DockerBranchShell(srcDir, branch string) string {
	if srcDir == "" {
		srcDir = "frida"
	}
	if branch == "" {
		branch = ManagementBranchName(time.Now())
	}
	return fmt.Sprintf("set -euo pipefail; cd %s; git config user.email fridare@local || true; git config user.name fridare || true; git checkout -B %s; echo '[fridare] branch %s'\n",
		shellQuote(srcDir), shellQuote(branch), shellQuote(branch))
}

// IsDockerCompileCommand reports whether args represent a docker-run compile (not host make).
func IsDockerCompileCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}
	if args[0] != "docker" {
		return false
	}
	joined := strings.Join(args, " ")
	return strings.Contains(joined, "run") &&
		(strings.Contains(joined, "pipeline") || strings.Contains(joined, "build") ||
			strings.Contains(joined, "configure") || strings.Contains(joined, "make") ||
			strings.Contains(joined, "bash"))
}

// PipelineScriptOptions configures BuildPipelineScript.
type PipelineScriptOptions struct {
	Clone       CloneSpec
	Branch      string
	SourceDir   string
	ArtifactDir string
	TargetIDs   []string
	// ModScript is optional shell snippet applied after branch creation.
	ModScript string
	// StealthSeed is hex material for per-TU -DFRIDARE_JUNK_SEED (empty → "0").
	StealthSeed string
}

// DockerfileSkeleton is a minimal multi-tool image for Frida Android/Linux builds.
// Prefer DockerfileSkeletonForMirror when a China Hub mirror is configured.
var DockerfileSkeleton = DockerfileSkeletonForMirror("")

// EnsureBuilderImageCommands returns commands to create the local builder image if missing.
func EnsureBuilderImageCommands(image string) [][]string {
	if image == "" {
		image = DefaultBuildImage
	}
	return [][]string{
		{"docker", "images", "-q", image},
		// Caller builds if empty: docker build -t <image> -f Dockerfile .
	}
}

// DockerBuildImageArgs returns docker build args for the skeleton Dockerfile in contextDir.
// When proxy is non-empty it is passed as --build-arg so RUN curl (Node/Go/NDK) can use it.
func DockerBuildImageArgs(image, contextDir string) []string {
	return DockerBuildImageArgsWithProxy(image, contextDir, "")
}

// DockerBuildImageArgsWithProxy injects HTTP(S)_PROXY build-args for Node/Go/NDK downloads.
// localhost proxies are rewritten to host.docker.internal (build container cannot use host loopback).
func DockerBuildImageArgsWithProxy(image, contextDir, proxy string) []string {
	if image == "" {
		image = DefaultBuildImage
	}
	if contextDir == "" {
		contextDir = "."
	}
	args := []string{"docker", "build", "-t", image}
	if p := strings.TrimSpace(proxy); p != "" {
		cp := RewriteProxyForContainer(p)
		args = append(args,
			"--add-host=host.docker.internal:host-gateway",
			"--build-arg", "HTTP_PROXY="+cp,
			"--build-arg", "HTTPS_PROXY="+cp,
			"--build-arg", "http_proxy="+cp,
			"--build-arg", "https_proxy="+cp,
		)
	}
	args = append(args, contextDir)
	return args
}

func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = shellQuote(a)
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n'\"\\$`;&|<>(){}[]*?") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
