package rebuild

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"fridare-gui/internal/core"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildPatchedFridaToolsWheels builds host client packages aligned with the Frida source tag:
//  1. Multi-platform native `frida=={version}` wheels for common host OS/arch combos:
//     windows-amd64, windows-arm64, macos-x86_64, macos-arm64, linux-x86_64, linux-arm64
//  2. Patches frida:rpc → {magic}:rpc in each frida wheel (.py + native .pyd/.so/.dylib)
//  3. Downloads frida-tools (own version line, e.g. 14.x), patches RPC if present, rebuilds pure wheel
//
// Catalog layout under _host-tools/{magic}/python/:
//
//	frida_tools-*.whl
//	host/windows-amd64/frida-*.whl  (patched)
//	host/macos-arm64/...
//	host/linux-x86_64/...
//
// Host-side client wheels must cover every platform the operator might run on;
// server binaries are target-platform only, but pip clients are host-platform.
// Does NOT mutate live site-packages.
func BuildPatchedFridaToolsWheels(cfg JobConfig, catalogRoot string, entryDirs []string, runner Runner) (wheels []string, err error) {
	if err := core.ValidateMagicName(cfg.MagicName); err != nil {
		return nil, err
	}
	version := strings.TrimSpace(cfg.FridaVersion)
	if version == "" {
		return nil, fmt.Errorf("Frida 版本为空，无法下载 host frida/frida-tools")
	}
	// Catalog stays on the product label (17.17.1); pip needs the official tag.
	pipVer := EffectivePipVersion(cfg)
	if pipVer == "" {
		pipVer = version
	}
	if catalogRoot == "" {
		catalogRoot = CatalogRoot(cfg.WorkDir)
	}
	work := filepath.Join(catalogRoot, ".work", sanitizeCatalogSeg(version)+"-"+sanitizeCatalogSeg(cfg.MagicName))
	_ = os.RemoveAll(work)
	if err := os.MkdirAll(work, 0755); err != nil {
		return nil, err
	}
	defer func() {
		if err == nil {
			_ = os.RemoveAll(work)
		}
	}()

	dlDir := filepath.Join(work, "dl")
	srcDir := filepath.Join(work, "src")
	outDir := filepath.Join(work, "out")
	_ = os.MkdirAll(dlDir, 0755)
	_ = os.MkdirAll(srcDir, 0755)
	_ = os.MkdirAll(outDir, 0755)

	py, err := findPython()
	if err != nil {
		return nil, err
	}
	pipEnv := pipProxyEnv(cfg.Proxy)
	indexArgs := pipIndexArgs(cfg)

	shared := CatalogEntryDir(catalogRoot, version, "_host-tools", cfg.MagicName)
	sharedPy := filepath.Join(shared, "python")
	_ = os.MkdirAll(sharedPy, 0755)

	// --- multi-platform frida native wheels (official pip tag, not product label) ---
	hostRaw, herr := downloadHostFridaWheels(runner, py, pipEnv, pipVer, indexArgs, filepath.Join(work, "host-raw"))
	if herr != nil && len(hostRaw) == 0 {
		return nil, fmt.Errorf("multi-platform frida==%s wheels: %w", pipVer, herr)
	}
	var hostPatched []string
	hostOut := filepath.Join(sharedPy, "host")
	for _, raw := range hostRaw {
		// raw path: host-raw/{platform-id}/frida-….whl
		platID := filepath.Base(filepath.Dir(raw))
		destDir := filepath.Join(hostOut, platID)
		_ = os.MkdirAll(destDir, 0755)
		patchedPath := filepath.Join(destDir, filepath.Base(raw))
		n, perr := patchFridaRPCInWheel(raw, patchedPath, cfg.MagicName)
		if perr != nil {
			// copy original if patch fails
			_ = copyFile(raw, patchedPath)
			_ = os.WriteFile(filepath.Join(destDir, "PATCH-WARN.txt"), []byte(perr.Error()), 0644)
		} else {
			_ = os.WriteFile(filepath.Join(destDir, "PATCHED.txt"),
				[]byte(fmt.Sprintf("server+client protocol sync replacements=%d magic=%s frida==%s product=%s\n  frida:rpc / re.frida.* / Frida.*\n", n, cfg.MagicName, pipVer, version)), 0644)
		}
		hostPatched = append(hostPatched, patchedPath)
		_ = copyFile(filepath.Join(filepath.Dir(raw), "PLATFORM.txt"), filepath.Join(destDir, "PLATFORM.txt"))
	}

	// --- frida-tools pure package (version line independent of Frida 17.x) ---
	var toolsBuilt []string
	arch, toolsVer, terr := downloadFridaToolsForFrida(runner, py, pipEnv, dlDir, pipVer, indexArgs)
	if terr != nil {
		_ = os.WriteFile(filepath.Join(sharedPy, "TOOLS-WARN.txt"), []byte(terr.Error()+"\n"), 0644)
	} else {
		_ = os.WriteFile(filepath.Join(work, "RESOLVED.txt"),
			[]byte(fmt.Sprintf("frida_source=%s\npip_frida=%s\nfrida_tools_pypi=%s\narchive=%s\n", version, pipVer, toolsVer, filepath.Base(arch))), 0644)
		if err := extractArchive(arch, srcDir); err != nil {
			_ = os.WriteFile(filepath.Join(sharedPy, "TOOLS-WARN.txt"), []byte("extract: "+err.Error()), 0644)
		} else if pkgRoot, err := findFridaToolsPackageRoot(srcDir); err != nil {
			_ = os.WriteFile(filepath.Join(sharedPy, "TOOLS-WARN.txt"), []byte(err.Error()), 0644)
		} else {
			_, _ = patchFridaToolsTree(pkgRoot, cfg.MagicName)
			localVer := pinFridaToolsPackageVersion(pkgRoot, pipVer, toolsVer, cfg.MagicName)
			// Prefer setup.py bdist_wheel (uses local setuptools; no isolation / network).
			// Fall back to pip wheel, then hand-rolled pure wheel zip.
			var buildErrs []string
			if err := runPython(runner, py, &runOpts{dir: pkgRoot, env: pipEnv},
				"setup.py", "bdist_wheel", "-d", outDir); err != nil {
				buildErrs = append(buildErrs, "setup.py bdist_wheel: "+err.Error())
				if err2 := runPython(runner, py, &runOpts{env: pipEnv},
					"-m", "pip", "wheel", pkgRoot, "-w", outDir, "--no-deps", "--no-build-isolation"); err2 != nil {
					buildErrs = append(buildErrs, "pip wheel: "+err2.Error())
					if hand, herr := buildPureFridaToolsWheel(pkgRoot, outDir, localVer); herr != nil {
						buildErrs = append(buildErrs, "hand zip: "+herr.Error())
						_ = os.WriteFile(filepath.Join(sharedPy, "TOOLS-WARN.txt"),
							[]byte(strings.Join(buildErrs, "\n")+"\n"), 0644)
					} else {
						toolsBuilt = []string{hand}
					}
				}
			}
			if len(toolsBuilt) == 0 {
				toolsBuilt, _ = filepath.Glob(filepath.Join(outDir, "*.whl"))
			}
			// GitHub #36: older hand-rolled wheels used '.' instead of '+' in the
			// local version; pip rejects those filenames. Normalize before copy.
			toolsBuilt = sanitizeToolsWheelFilenames(toolsBuilt)
		}
	}

	installTxt := fridaToolsInstallNotes(cfg, toolsBuilt, hostPatched)
	_ = os.WriteFile(filepath.Join(sharedPy, "INSTALL.txt"), []byte(installTxt), 0644)

	var allWheels []string
	for _, w := range toolsBuilt {
		base := filepath.Base(w)
		dst := filepath.Join(sharedPy, base)
		if err := copyFile(w, dst); err != nil {
			return allWheels, err
		}
		allWheels = append(allWheels, dst)
	}
	allWheels = append(allWheels, hostPatched...)

	for _, entry := range entryDirs {
		pyDir := filepath.Join(entry, "python")
		_ = os.MkdirAll(pyDir, 0755)
		for _, w := range toolsBuilt {
			_ = copyFile(w, filepath.Join(pyDir, filepath.Base(w)))
		}
		if len(hostPatched) > 0 {
			_ = copyPath(hostOut, filepath.Join(pyDir, "host"))
		}
		_ = os.WriteFile(filepath.Join(pyDir, "INSTALL.txt"), []byte(installTxt), 0644)
		_ = os.WriteFile(filepath.Join(pyDir, "WHEELS.txt"), []byte(strings.Join(basenameList(allWheels), "\n")+"\n"), 0644)
	}

	if len(allWheels) == 0 {
		return nil, fmt.Errorf("no host wheels produced (frida multi-platform + tools)")
	}
	_ = WriteManifest(shared, ArtifactManifest{
		FridaVersion: version,
		Platform:     "_host-tools",
		MagicName:    cfg.MagicName,
		ListenPort:   cfg.ListenPort,
		Wheels:       basenameList(allWheels),
		Notes:        "patched multi-platform frida wheels + frida-tools; frida==source tag",
	})
	_ = writeCatalogIndex(catalogRoot)
	return allWheels, nil
}

// PatchFridaRPCInWheel is the exported alias for host-wheel protocol sync
// (frida:rpc + re.frida.* + Frida.*). Same-length when magic is 5 letters.
func PatchFridaRPCInWheel(srcWheel, dstWheel, magic string) (int, error) {
	return patchFridaRPCInWheel(srcWheel, dstWheel, magic)
}

// patchFridaRPCInWheel extracts a wheel, replaces frida:rpc → {magic}:rpc in:
//   - pure Python (.py) via text patch
//   - native extensions (.pyd / .so / .dylib) via same-length binary replace
// Then re-zips. Magic must be len("frida") so binary lengths stay valid.
func patchFridaRPCInWheel(srcWheel, dstWheel, magic string) (int, error) {
	if err := core.ValidateMagicName(magic); err != nil {
		return 0, err
	}
	tmp, err := os.MkdirTemp("", "fridare-whl-*")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(tmp)
	if err := extractArchive(srcWheel, tmp); err != nil {
		return 0, err
	}
	// Server+client sync: rpc + re.frida.* + Frida.* (same length when magic is 5 letters)
	binPairs, err := core.ClientProtocolBinaryPairs(magic)
	if err != nil {
		return 0, err
	}
	n := 0
	_ = filepath.Walk(tmp, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := strings.ToLower(info.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		switch {
		case strings.HasSuffix(name, ".py"):
			// full=true: host client matches deep server protocol/API surface
			s, count, perr := core.PatchClientProtocolSurface(string(data), magic, true)
			if perr != nil || count == 0 {
				return nil
			}
			if err := os.WriteFile(path, []byte(s), info.Mode()); err != nil {
				return err
			}
			n += count
		case strings.HasSuffix(name, ".pyd"), strings.HasSuffix(name, ".so"),
			strings.HasSuffix(name, ".dylib"), strings.Contains(name, ".so."):
			out := data
			changed := false
			for _, pr := range binPairs {
				if !bytes.Contains(out, pr[0]) {
					continue
				}
				n += bytes.Count(out, pr[0])
				out = bytes.ReplaceAll(out, pr[0], pr[1])
				changed = true
			}
			if changed {
				if err := os.WriteFile(path, out, info.Mode()); err != nil {
					return err
				}
			}
		}
		return nil
	})
	_ = os.MkdirAll(filepath.Dir(dstWheel), 0755)
	if err := zipDir(tmp, dstWheel); err != nil {
		return n, err
	}
	return n, nil
}

func zipDir(srcDir, dstZip string) error {
	_ = os.MkdirAll(filepath.Dir(dstZip), 0755)
	f, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		// ZIP entries always use forward slashes
		rel = filepath.ToSlash(rel)
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, in)
		_ = in.Close()
		return copyErr
	})
	if cerr := zw.Close(); cerr != nil && err == nil {
		err = cerr
	}
	return err
}

// HostFridaWheelPlatforms are pip --platform tags for host-side frida bindings.
// frida-tools itself is pure Python (py3-none-any); native code lives in package "frida".
var HostFridaWheelPlatforms = []struct {
	ID       string // catalog subdir under python/host/
	Platform string // pip --platform
}{
	{ID: "windows-amd64", Platform: "win_amd64"},
	{ID: "windows-arm64", Platform: "win_arm64"},
	{ID: "macos-x86_64", Platform: "macosx_11_0_x86_64"},
	{ID: "macos-arm64", Platform: "macosx_11_0_arm64"},
	{ID: "linux-x86_64", Platform: "manylinux2014_x86_64"},
	{ID: "linux-arm64", Platform: "manylinux2014_aarch64"},
}

// downloadHostFridaWheels fetches frida==version wheels for common host OS/arch combos.
func downloadHostFridaWheels(runner Runner, py string, env []string, fridaVer string, indexArgs []string, hostRoot string) ([]string, error) {
	_ = os.MkdirAll(hostRoot, 0755)
	var got []string
	var errs []string
	for _, hp := range HostFridaWheelPlatforms {
		dest := filepath.Join(hostRoot, hp.ID)
		_ = os.MkdirAll(dest, 0755)
		// abi3 cp37 wheels are published for frida
		args := []string{
			"-m", "pip", "download", "frida==" + fridaVer,
			"-d", dest,
			"--only-binary=:all:",
			"--platform", hp.Platform,
			"--python-version", "311",
			"--abi", "abi3",
			"--no-deps",
		}
		args = append(args, indexArgs...)
		if err := runPython(runner, py, &runOpts{env: env}, args...); err != nil {
			// retry with python-version 37 (abi3 baseline)
			args2 := []string{
				"-m", "pip", "download", "frida==" + fridaVer,
				"-d", dest,
				"--only-binary=:all:",
				"--platform", hp.Platform,
				"--python-version", "37",
				"--abi", "abi3",
				"--no-deps",
			}
			args2 = append(args2, indexArgs...)
			if err2 := runPython(runner, py, &runOpts{env: env}, args2...); err2 != nil {
				errs = append(errs, hp.ID+": "+err2.Error())
				continue
			}
		}
		files, _ := filepath.Glob(filepath.Join(dest, "frida-*.whl"))
		if len(files) == 0 {
			files, _ = filepath.Glob(filepath.Join(dest, "*.whl"))
		}
		if len(files) == 0 {
			errs = append(errs, hp.ID+": no wheel file")
			continue
		}
		got = append(got, files...)
		_ = os.WriteFile(filepath.Join(dest, "PLATFORM.txt"),
			[]byte(fmt.Sprintf("id=%s\npip_platform=%s\nfrida==%s\n", hp.ID, hp.Platform, fridaVer)), 0644)
	}
	if len(got) == 0 {
		return nil, fmt.Errorf("no host frida wheels downloaded: %s", strings.Join(errs, "; "))
	}
	if len(errs) > 0 {
		_ = os.WriteFile(filepath.Join(hostRoot, "PARTIAL.txt"), []byte(strings.Join(errs, "\n")+"\n"), 0644)
	}
	return got, nil
}

type runOpts struct {
	dir string
	env []string // full environ; nil = inherit
}

func pipProxyEnv(proxy string) []string {
	env := os.Environ()
	p := strings.TrimSpace(proxy)
	if p == "" {
		return env
	}
	// Host pip uses localhost proxy as-is (not host.docker.internal)
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy", "ALL_PROXY", "all_proxy"} {
		env = setEnv(env, k, p)
	}
	return env
}

func findPython() (string, error) {
	for _, name := range []string{"python", "python3", "py"} {
		if p, err := exec.LookPath(name); err == nil && p != "" {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到 python（需用于 pip download / wheel 打包 frida-tools）")
}

func runPython(runner Runner, py string, opts *runOpts, args ...string) error {
	full := append([]string{}, args...)
	ctx := context.Background()
	// Always use exec with Dir/Env for reliability on Windows (Runner may not set cwd)
	cmd := exec.CommandContext(ctx, py, full...)
	if opts != nil {
		if opts.dir != "" {
			cmd.Dir = opts.dir
		}
		if opts.env != nil {
			cmd.Env = opts.env
		}
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, truncate(string(out), 500))
	}
	_ = runner // reserved for future containerized pip
	return nil
}

func extractArchive(arch, dest string) error {
	_ = os.MkdirAll(dest, 0755)
	low := strings.ToLower(arch)
	switch {
	case strings.HasSuffix(low, ".tar.gz"), strings.HasSuffix(low, ".tgz"):
		cmd := exec.Command("tar", "-xzf", arch, "-C", dest)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("tar: %w %s", err, truncate(string(out), 200))
		}
		return nil
	case strings.HasSuffix(low, ".zip"), strings.HasSuffix(low, ".whl"):
		// whl is zip
		cmd := exec.Command("python", "-c",
			fmt.Sprintf("import zipfile; zipfile.ZipFile(r'''%s''').extractall(r'''%s''')", arch, dest))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("unzip: %w %s", err, truncate(string(out), 200))
		}
		return nil
	default:
		return fmt.Errorf("未知包格式: %s", filepath.Base(arch))
	}
}

func findFridaToolsPackageRoot(srcDir string) (string, error) {
	var found string
	_ = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || !info.IsDir() {
			return nil
		}
		// frida_tools package dir (core.py removed in 14.x — any .py is enough)
		if filepath.Base(path) == "frida_tools" {
			if ents, e := os.ReadDir(path); e == nil && len(ents) > 0 {
				found = filepath.Dir(path)
				return filepath.SkipAll
			}
		}
		return nil
	})
	if found != "" {
		return found, nil
	}
	if st, e := os.Stat(filepath.Join(srcDir, "frida_tools")); e == nil && st.IsDir() {
		return srcDir, nil
	}
	// one nested top folder
	ents, _ := os.ReadDir(srcDir)
	for _, e := range ents {
		if e.IsDir() {
			if st, err := os.Stat(filepath.Join(srcDir, e.Name(), "frida_tools")); err == nil && st.IsDir() {
				return filepath.Join(srcDir, e.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("解压后未找到 frida_tools 包目录")
}

// patchFridaToolsTree applies safe RPC renames under a frida-tools source/wheel extract.
// Returns number of files patched.
func patchFridaToolsTree(pkgRoot, magic string) (int, error) {
	if err := core.ValidateMagicName(magic); err != nil {
		return 0, err
	}
	n := 0
	err := filepath.Walk(pkgRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if !strings.HasSuffix(name, ".py") {
			return nil
		}
		// Never touch binary extension modules by name
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		s := string(data)
		if core.WouldBreakFridaImport(s, magic) {
			// still only apply RPC patch, not global frida replace
		}
		// full=true: re.frida.* + Frida.* + frida:rpc — matches deep server
		patched, count, err := core.PatchClientProtocolSurface(s, magic, true)
		if err != nil || count == 0 {
			return nil
		}
		if err := os.WriteFile(path, []byte(patched), info.Mode()); err != nil {
			return err
		}
		n++
		return nil
	})
	return n, err
}

// downloadFridaToolsForFrida picks a frida-tools archive compatible with frida==fridaVer.
// Returns archive path and resolved tools version string (from filename).
func downloadFridaToolsForFrida(runner Runner, py string, env []string, dlDir, fridaVer string, indexArgs []string) (archPath, toolsVer string, err error) {
	// 1) Best: let pip resolve frida-tools together with exact frida pin
	args := []string{"-m", "pip", "download", "frida-tools", "frida==" + fridaVer, "-d", dlDir, "--no-deps"}
	// Actually with both packages and --no-deps pip downloads both without resolving deps.
	// Use dependency resolution without --no-deps but only keep frida-tools files:
	args = []string{"-m", "pip", "download", "frida-tools", "-d", dlDir}
	args = append(args, indexArgs...)
	// Constraint file so tools matches frida ABI line
	cfile := filepath.Join(dlDir, "constraints.txt")
	_ = os.WriteFile(cfile, []byte("frida=="+fridaVer+"\n"), 0644)
	args = append(args, "-c", cfile)
	// Prefer no binary for tools only — download all then filter
	if err := runPython(runner, py, &runOpts{env: env}, args...); err != nil {
		// 2) Fallback: latest frida-tools alone (14.x line for Frida 17)
		args2 := []string{"-m", "pip", "download", "frida-tools", "-d", dlDir, "--no-deps"}
		args2 = append(args2, indexArgs...)
		if err2 := runPython(runner, py, &runOpts{env: env}, args2...); err2 != nil {
			return "", "", fmt.Errorf("pip download frida-tools for frida==%s: %v / %v", fridaVer, err, err2)
		}
	}
	// Pick frida_tools / frida-tools archive (not the frida-*.whl native binding)
	cands, _ := filepath.Glob(filepath.Join(dlDir, "*"))
	var toolsFiles []string
	for _, f := range cands {
		base := strings.ToLower(filepath.Base(f))
		if strings.HasPrefix(base, "frida_tools") || strings.HasPrefix(base, "frida-tools") {
			toolsFiles = append(toolsFiles, f)
		}
	}
	if len(toolsFiles) == 0 {
		return "", "", fmt.Errorf("未下载到 frida-tools 包（目录: %s）", dlDir)
	}
	// Prefer sdist .tar.gz for patching
	archPath = toolsFiles[0]
	for _, f := range toolsFiles {
		b := strings.ToLower(f)
		if strings.HasSuffix(b, ".tar.gz") || strings.HasSuffix(b, ".zip") {
			archPath = f
			break
		}
	}
	toolsVer = parseVersionFromArchiveName(filepath.Base(archPath))
	return archPath, toolsVer, nil
}

func pipIndexArgs(cfg JobConfig) []string {
	// China-friendly default index when docker mirror is set (same user environment)
	// Can still use official pypi via proxy.
	if strings.TrimSpace(cfg.DockerMirror) != "" || strings.Contains(strings.ToLower(cfg.Proxy), "localhost") {
		return []string{"-i", "https://pypi.tuna.tsinghua.edu.cn/simple", "--trusted-host", "pypi.tuna.tsinghua.edu.cn"}
	}
	return nil
}

func parseVersionFromArchiveName(name string) string {
	// PyPI sdist is often frida-tools-13.7.1.tar.gz (hyphen). Taking
	// Split("-")[1] yields "tools" and the wheel becomes
	// frida_tools-tools.frida.16.7.19.fridare.kxmwp — pip rejects it.
	base := filepath.Base(name)
	lower := strings.ToLower(base)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		base = base[:len(base)-len(".tar.gz")]
	case strings.HasSuffix(lower, ".zip"):
		base = base[:len(base)-4]
	case strings.HasSuffix(lower, ".whl"):
		base = base[:len(base)-4]
	}
	lower = strings.ToLower(base)
	var rest string
	switch {
	case strings.HasPrefix(lower, "frida-tools-"):
		rest = base[len("frida-tools-"):]
	case strings.HasPrefix(lower, "frida_tools-"):
		rest = base[len("frida_tools-"):]
	default:
		parts := strings.Split(base, "-")
		if len(parts) >= 2 {
			return parts[1]
		}
		return base
	}
	lowRest := strings.ToLower(rest)
	for _, tag := range []string{"-py2", "-py3", "-cp2", "-cp3"} {
		if i := strings.Index(lowRest, tag); i > 0 {
			return rest[:i]
		}
	}
	return rest
}

// pep440WheelFilenameVersion is the version segment written into a wheel filename.
// PEP 440 local versions MUST keep '+' (e.g. 14.10.4+frida.17.17.0.fridare.kxmwp).
// Replacing '+' with '.' or '_' makes packaging.utils.parse_wheel_filename fail:
//
//	Invalid wheel filename (invalid version)
//
// See GitHub suifei/fridare#36.
func pep440WheelFilenameVersion(version string) string {
	v := strings.ReplaceAll(strings.TrimSpace(version), " ", "")
	if isPEP440Version(v) {
		return v
	}
	if fixed := repairDottedFridaLocalVersion(v); isPEP440Version(fixed) {
		return fixed
	}
	return v
}

func isPEP440Version(v string) bool {
	if v == "" {
		return false
	}
	pub, local, hasLocal := strings.Cut(v, "+")
	if !isPEP440Release(pub) {
		return false
	}
	if !hasLocal {
		return true
	}
	if local == "" {
		return false
	}
	for _, seg := range strings.Split(local, ".") {
		if !isAlnum(seg) {
			return false
		}
	}
	return true
}

func isPEP440Release(v string) bool {
	if v == "" {
		return false
	}
	for _, seg := range strings.Split(v, ".") {
		if seg == "" {
			return false
		}
		for _, c := range seg {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

func isAlnum(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

// repairDottedFridaLocalVersion restores '+' after the public version when a
// previous Fridare build wrote 14.10.4.frida.… or 14.10.4_frida.… .
func repairDottedFridaLocalVersion(v string) string {
	for _, sep := range []string{".frida.", "_frida."} {
		if i := strings.Index(v, sep); i > 0 {
			return v[:i] + "+frida." + v[i+len(sep):]
		}
	}
	return v
}

func splitWheelFilename(base string) (name, ver, py, abi, plat string, ok bool) {
	if !strings.HasSuffix(strings.ToLower(base), ".whl") || len(base) < 5 {
		return
	}
	stem := base[:len(base)-4]
	parts := strings.Split(stem, "-")
	if len(parts) < 5 {
		return
	}
	plat = parts[len(parts)-1]
	abi = parts[len(parts)-2]
	py = parts[len(parts)-3]
	name = parts[0]
	ver = strings.Join(parts[1:len(parts)-3], "-")
	if name == "" || ver == "" {
		return
	}
	ok = true
	return
}

// ensurePEP440ToolsWheelFilename renames a wheel on disk if its version segment
// is not PEP 440 (the historical '.'-local-version bug). pip only inspects the
// outer filename; METADATA inside may already be correct.
func ensurePEP440ToolsWheelFilename(path string) (string, error) {
	base := filepath.Base(path)
	name, ver, py, abi, plat, ok := splitWheelFilename(base)
	if !ok {
		return path, nil
	}
	newVer := pep440WheelFilenameVersion(ver)
	if newVer == ver {
		return path, nil
	}
	newPath := filepath.Join(filepath.Dir(path), fmt.Sprintf("%s-%s-%s-%s-%s.whl", name, newVer, py, abi, plat))
	if err := os.Rename(path, newPath); err != nil {
		return path, err
	}
	return newPath, nil
}

func sanitizeToolsWheelFilenames(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if nw, err := ensurePEP440ToolsWheelFilename(p); err == nil && nw != "" {
			out = append(out, nw)
		} else {
			out = append(out, p)
		}
	}
	return out
}

// pinFridaToolsPackageVersion patches PKG-INFO / setup.py so the rebuilt wheel
// reports a local version and pins install_requires to frida==sourceVer.
// Returns the PEP 440 local version string (e.g. 14.10.4+frida.17.16.4.fridare.abcde).
// Does NOT overwrite setup.cfg wholesale (that breaks setuptools metadata).
func pinFridaToolsPackageVersion(pkgRoot, fridaSourceVer, toolsVer, magic string) string {
	if toolsVer == "" {
		toolsVer = "0"
	}
	// PEP 440 local version: 14.10.4+frida.17.16.4.fridare.abcde
	localVer := fmt.Sprintf("%s+frida.%s.fridare.%s", toolsVer, fridaSourceVer, magic)
	localVer = strings.ReplaceAll(localVer, " ", "")

	// PKG-INFO Version: line (used by setup.py detect_version when in sdist)
	pkgInfo := filepath.Join(pkgRoot, "PKG-INFO")
	if data, err := os.ReadFile(pkgInfo); err == nil {
		lines := strings.Split(string(data), "\n")
		for i, ln := range lines {
			if strings.HasPrefix(ln, "Version: ") {
				lines[i] = "Version: " + localVer
				break
			}
		}
		_ = os.WriteFile(pkgInfo, []byte(strings.Join(lines, "\n")), 0644)
	}

	// setup.py: pin frida dependency to exact source tag
	setupPy := filepath.Join(pkgRoot, "setup.py")
	if data, err := os.ReadFile(setupPy); err == nil {
		s := string(data)
		// common form in frida-tools 14.x: "frida >= 17.10.0, < 18.0.0"
		repls := []struct{ old, new string }{
			{`"frida >= 17.10.0, < 18.0.0"`, fmt.Sprintf(`"frida==%s"`, fridaSourceVer)},
			{`"frida>=17.10.0,<18.0.0"`, fmt.Sprintf(`"frida==%s"`, fridaSourceVer)},
			{`'frida >= 17.10.0, < 18.0.0'`, fmt.Sprintf(`'frida==%s'`, fridaSourceVer)},
		}
		for _, r := range repls {
			if strings.Contains(s, r.old) {
				s = strings.Replace(s, r.old, r.new, 1)
				break
			}
		}
		_ = os.WriteFile(setupPy, []byte(s), 0644)
	}

	_ = os.WriteFile(filepath.Join(pkgRoot, "FRIDARE_PATCH.txt"),
		[]byte(fmt.Sprintf("frida_source_version=%s\nfrida_tools_base=%s\nwheel_version=%s\nmagic=%s\nrpc=%s:rpc\n",
			fridaSourceVer, toolsVer, localVer, magic, magic)), 0644)
	return localVer
}

// buildPureFridaToolsWheel hand-rolls a py3-none-any wheel when setuptools/pip
// wheel fails (offline, SSL, missing build isolation deps).
func buildPureFridaToolsWheel(pkgRoot, outDir, version string) (string, error) {
	pkgDir := filepath.Join(pkgRoot, "frida_tools")
	if st, err := os.Stat(pkgDir); err != nil || !st.IsDir() {
		return "", fmt.Errorf("frida_tools package dir missing under %s", pkgRoot)
	}
	// Keep PEP 440 '+' in the filename. Do NOT replace with '.' or '_' —
	// pip/packaging parse the version as-is (GitHub #36).
	fileVer := pep440WheelFilenameVersion(version)
	if fileVer == "" {
		return "", fmt.Errorf("empty frida-tools wheel version")
	}
	whlName := fmt.Sprintf("frida_tools-%s-py3-none-any.whl", fileVer)
	dst := filepath.Join(outDir, whlName)
	_ = os.MkdirAll(outDir, 0755)

	tmp, err := os.MkdirTemp("", "fridare-tools-whl-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	// Copy package tree
	if err := copyPath(pkgDir, filepath.Join(tmp, "frida_tools")); err != nil {
		return "", err
	}
	// dist-info
	distInfo := filepath.Join(tmp, fmt.Sprintf("frida_tools-%s.dist-info", fileVer))
	_ = os.MkdirAll(distInfo, 0755)
	meta := fmt.Sprintf(`Metadata-Version: 2.1
Name: frida-tools
Version: %s
Summary: Frida CLI tools (Fridare patched)
Requires-Python: >=3.7
Requires-Dist: frida
`, version)
	// Prefer pin from FRIDARE_PATCH if present
	if b, err := os.ReadFile(filepath.Join(pkgRoot, "FRIDARE_PATCH.txt")); err == nil {
		for _, ln := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(ln, "frida_source_version=") {
				fv := strings.TrimPrefix(ln, "frida_source_version=")
				meta = fmt.Sprintf(`Metadata-Version: 2.1
Name: frida-tools
Version: %s
Summary: Frida CLI tools (Fridare patched)
Requires-Python: >=3.7
Requires-Dist: frida==%s
Requires-Dist: colorama>=0.2.7,<1.0.0
Requires-Dist: prompt-toolkit>=2.0.0,<4.0.0
Requires-Dist: pygments>=2.0.2,<3.0.0
Requires-Dist: websockets>=13.0.0,<14.0.0
`, version, fv)
				break
			}
		}
	}
	_ = os.WriteFile(filepath.Join(distInfo, "METADATA"), []byte(meta), 0644)
	_ = os.WriteFile(filepath.Join(distInfo, "WHEEL"), []byte("Wheel-Version: 1.0\nGenerator: fridare\nRoot-Is-Purelib: true\nTag: py3-none-any\n"), 0644)
	_ = os.WriteFile(filepath.Join(distInfo, "top_level.txt"), []byte("frida_tools\n"), 0644)
	// entry points (subset of official CLI)
	_ = os.WriteFile(filepath.Join(distInfo, "entry_points.txt"), []byte(`[console_scripts]
frida = frida_tools.repl:main
frida-ps = frida_tools.ps:main
frida-ls-devices = frida_tools.lsd:main
frida-trace = frida_tools.tracer:main
frida-kill = frida_tools.kill:main
`), 0644)

	// RECORD is optional for pip install --no-deps in practice; write minimal
	_ = os.WriteFile(filepath.Join(distInfo, "RECORD"), []byte(""), 0644)

	if err := zipDir(tmp, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func fridaToolsInstallNotes(cfg JobConfig, toolsWheels, hostFridaWheels []string) string {
	var toolsNames []string
	for _, w := range toolsWheels {
		toolsNames = append(toolsNames, filepath.Base(w))
	}
	whl := "(none)"
	if len(toolsNames) > 0 {
		whl = toolsNames[0]
	}
	var hostList strings.Builder
	if len(hostFridaWheels) == 0 {
		hostList.WriteString("  (未下载到多平台 frida 原生 wheel，请本机 pip install frida==版本)\n")
	} else {
		for _, w := range hostFridaWheels {
			hostList.WriteString("  - " + w + "\n")
		}
	}
	return fmt.Sprintf(`Fridare 魔改 frida-tools + 多平台 host frida（与源码 tag 对齐）
============================================================
catalog 键 / Frida 源码 tag: %s
魔改 magic: %s
端口: %d

【重要】服务端与客户端已同步魔改，必须成对安装本 catalog 内 wheel。
  - frida:rpc → %s:rpc
  - re.frida.* → re.%s.*          (DBus 接口名)
  - /re/frida/ → /re/%s/          (DBus 对象路径，缺了会 UNKNOWN_METHOD)
  - Frida.* API 字面量 → %s.*
  不要 pip install 官方未魔改的 frida/frida-tools，否则协议对不上。

A) frida-tools（纯 Python，任意宿主机）:
  %s
  - install_requires: frida==%s

B) frida 原生扩展（按宿主机 OS/架构选 python/host/<id>/，共 6 个平台）:
  windows-amd64 | windows-arm64
  macos-x86_64  | macos-arm64
  linux-x86_64  | linux-arm64
  （.py + _frida.pyd/.so 内协议/RPC 已与 server 对齐）
%s
按宿主机选择示例:
  # Windows x64
  pip install --force-reinstall --no-deps "python/host/windows-amd64/frida-*.whl"
  # Windows ARM64
  pip install --force-reinstall --no-deps "python/host/windows-arm64/frida-*.whl"
  # macOS Apple Silicon
  pip install --force-reinstall --no-deps "python/host/macos-arm64/frida-*.whl"
  # macOS Intel
  pip install --force-reinstall --no-deps "python/host/macos-x86_64/frida-*.whl"
  # Linux x86_64
  pip install --force-reinstall --no-deps "python/host/linux-x86_64/frida-*.whl"
  # Linux aarch64
  pip install --force-reinstall --no-deps "python/host/linux-arm64/frida-*.whl"

  # 再装 frida-tools（纯 Python，任意宿主机）。文件名里的 '+' 是 PEP 440 本地版本，不要改成 '.'。
  pip install --force-reinstall --no-deps "%s"

验证:
  python -c "import frida; print(frida.__version__)"   # 须 == %s

连接魔改 server:
  frida -H <device-ip> -p %d -f <package>
`, cfg.FridaVersion, cfg.MagicName, cfg.ListenPort,
		cfg.MagicName, cfg.MagicName, cfg.MagicName, strings.ToUpper(cfg.MagicName[:1])+cfg.MagicName[1:],
		strings.Join(toolsNames, ", "), cfg.FridaVersion,
		hostList.String(), whl, cfg.FridaVersion, cfg.ListenPort)
}

func basenameList(paths []string) []string {
	var out []string
	for _, p := range paths {
		out = append(out, filepath.Base(p))
	}
	return out
}
