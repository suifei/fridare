package rebuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"fridare-gui/internal/core"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Catalog layout (host + Docker-visible under rebuild work root):
//
//	{work}/catalog/{fridaVersion}/{platform}/{magic}/
//	  MANIFEST.json
//	  README.txt
//	  binaries/          # server / agent / gadget from compile
//	  python/            # patched frida-tools wheel + install notes
//	{work}/catalog/{fridaVersion}/_release/
//	  frida-{magic}-{ver}-{platform}-server.zip
//	  frida-{magic}-{ver}-host-wheels.zip
//
// Shared with Docker when work root is the parent of the bind-mounted src/.

// CatalogRoot returns ~/.fridare/rebuild/catalog (or under workDir).
func CatalogRoot(workDir string) string {
	if workDir == "" {
		home, _ := os.UserHomeDir()
		workDir = filepath.Join(home, ".fridare", "rebuild")
	}
	// If workDir is already …/rebuild or …/rebuild-e2e, catalog is sibling of src
	base := workDir
	if filepath.Base(base) == "src" {
		base = filepath.Dir(base)
	}
	return filepath.Join(base, "catalog")
}

// CatalogEntryDir is version/platform/magic under the catalog root.
func CatalogEntryDir(catalogRoot, version, platform, magic string) string {
	return filepath.Join(catalogRoot,
		sanitizeCatalogSeg(version),
		sanitizeCatalogSeg(platform),
		sanitizeCatalogSeg(magic),
	)
}

func sanitizeCatalogSeg(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "_unknown"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || out == "." || out == ".." {
		return "_unknown"
	}
	return out
}

// ArtifactManifest describes one catalog entry (version × platform × magic).
type ArtifactManifest struct {
	FridaVersion string    `json:"frida_version"`
	Platform     string    `json:"platform"`
	MagicName    string    `json:"magic_name"`
	ListenPort   int       `json:"listen_port"`
	CreatedAt    time.Time `json:"created_at"`
	BinariesDir  string    `json:"binaries_dir"`
	PythonDir    string    `json:"python_dir"`
	Wheels       []string  `json:"wheels,omitempty"`
	BinaryFiles  []string  `json:"binary_files,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	EntryPath    string    `json:"entry_path"`
}

// WriteManifest saves MANIFEST.json into entryDir.
func WriteManifest(entryDir string, m ArtifactManifest) error {
	if err := os.MkdirAll(entryDir, 0755); err != nil {
		return err
	}
	m.EntryPath = entryDir
	m.BinariesDir = filepath.Join(entryDir, "binaries")
	m.PythonDir = filepath.Join(entryDir, "python")
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(entryDir, "MANIFEST.json"), append(data, '\n'), 0644)
}

// CatalogEntry is a browsable row for the GUI.
type CatalogEntry struct {
	Version  string
	Platform string
	Magic    string
	Path     string
	HasWheel bool
	HasBin   bool
	Manifest *ArtifactManifest
}

// ListCatalog walks catalogRoot and returns entries sorted by version desc, platform, magic.
func ListCatalog(catalogRoot string) ([]CatalogEntry, error) {
	var out []CatalogEntry
	if catalogRoot == "" {
		return out, nil
	}
	if _, err := os.Stat(catalogRoot); err != nil {
		return out, nil
	}
	versions, err := os.ReadDir(catalogRoot)
	if err != nil {
		return nil, err
	}
	for _, v := range versions {
		if !v.IsDir() {
			continue
		}
		vpath := filepath.Join(catalogRoot, v.Name())
		platforms, err := os.ReadDir(vpath)
		if err != nil {
			continue
		}
		for _, p := range platforms {
			if !p.IsDir() {
				continue
			}
			ppath := filepath.Join(vpath, p.Name())
			magics, err := os.ReadDir(ppath)
			if err != nil {
				continue
			}
			for _, m := range magics {
				if !m.IsDir() {
					continue
				}
				ep := filepath.Join(ppath, m.Name())
				ent := CatalogEntry{
					Version:  v.Name(),
					Platform: p.Name(),
					Magic:    m.Name(),
					Path:     ep,
				}
				if st, err := os.Stat(filepath.Join(ep, "binaries")); err == nil && st.IsDir() {
					if names, _ := listFiles(filepath.Join(ep, "binaries")); len(names) > 0 {
						ent.HasBin = true
					}
				}
				if names, _ := filepath.Glob(filepath.Join(ep, "python", "*.whl")); len(names) > 0 {
					ent.HasWheel = true
				}
				if data, err := os.ReadFile(filepath.Join(ep, "MANIFEST.json")); err == nil {
					var man ArtifactManifest
					if json.Unmarshal(data, &man) == nil {
						ent.Manifest = &man
					}
				}
				out = append(out, ent)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Version != out[j].Version {
			return out[i].Version > out[j].Version
		}
		if out[i].Platform != out[j].Platform {
			return out[i].Platform < out[j].Platform
		}
		return out[i].Magic < out[j].Magic
	})
	return out, nil
}

func listFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// OrganizeExportToCatalog copies compile outputs into catalog/{ver}/{platform}/{magic}/binaries
// and returns the primary entry path (first target) plus all entry paths.
// Order: collect non-empty blobs → magic basename rename → binary string patch → MANIFEST lists actual names.
func OrganizeExportToCatalog(catalogRoot string, cfg JobConfig, dockerArts, flatArtHost string) (primary string, entries []string, err error) {
	if catalogRoot == "" {
		catalogRoot = CatalogRoot(cfg.WorkDir)
	}
	if err := os.MkdirAll(catalogRoot, 0755); err != nil {
		return "", nil, err
	}
	targets := cfg.TargetIDs
	if len(targets) == 0 {
		targets = []string{"unknown"}
	}
	for _, tid := range targets {
		entry := CatalogEntryDir(catalogRoot, cfg.FridaVersion, tid, cfg.MagicName)
		binDir := filepath.Join(entry, "binaries")
		pyDir := filepath.Join(entry, "python")
		_ = os.RemoveAll(binDir) // clean slate for this entry
		_ = os.MkdirAll(binDir, 0755)
		_ = os.MkdirAll(pyDir, 0755)

		// Per-target subdirectory only. Also walking dockerArts / flatArtHost
		// descends into every sibling (android-arm64, android-x86_64, …) and
		// copyNonEmptyArtifacts keeps the basename, so the later directory
		// (x86_64 sorts after arm64) overwrites every catalog entry.
		// Issue #38: selecting both arches left two folders of x86-64 binaries.
		if !copyTargetArtifactDir(dockerArts, flatArtHost, tid, binDir) {
			// Legacy flat layout: files sit directly in the artifact root.
			// Do not descend into other target directories.
			copyFlatArtifactRoot(dockerArts, binDir)
			if flatArtHost != dockerArts {
				copyFlatArtifactRoot(flatArtHost, binDir)
			}
		}
		// Flatten nested target dir if present
		if nested := filepath.Join(binDir, tid); dirExists(nested) {
			_ = copyNonEmptyArtifacts(nested, binDir)
			_ = os.RemoveAll(nested)
		}
		// Basename → magic (disk names match product contract)
		_, _ = RenameArtifactBasenames(binDir, cfg.MagicName)
		// In-binary same-length string markers (detectors scan these)
		_, _ = PatchArtifactBinaryMarkers(binDir, cfg.MagicName)
		if ShouldStripProductSymbols(cfg) {
			_, _ = StripProductBinariesDir(binDir, cfg.MagicName)
		}
		// MANIFEST after rename — list real basenames only
		binFiles, _ := listNonEmptyFiles(binDir)
		man := ArtifactManifest{
			FridaVersion: cfg.FridaVersion,
			Platform:     tid,
			MagicName:    cfg.MagicName,
			ListenPort:   cfg.ListenPort,
			CreatedAt:    time.Now(),
			BinaryFiles:  binFiles,
			Notes:        "server/agent binaries (magic basenames + in-binary markers) + host python wheels",
		}
		_ = WriteManifest(entry, man)
		readme := ArtifactDeployTips(entry, cfg)
		_ = os.WriteFile(filepath.Join(entry, "README.txt"), []byte(readme), 0644)
		entries = append(entries, entry)
		if primary == "" {
			primary = entry
		}
	}
	_ = writeCatalogIndex(catalogRoot)
	return primary, entries, nil
}

// MinArtifactBytes skips empty meson stubs (0-byte agent blobs).
const MinArtifactBytes = 1024

// copyTargetArtifactDir copies product blobs from <root>/<targetID>/ only.
// Returns true when at least one such directory exists (even if it is empty).
func copyTargetArtifactDir(dockerArts, flatArtHost, tid, binDir string) bool {
	found := false
	for _, root := range []string{dockerArts, flatArtHost} {
		if root == "" || tid == "" {
			continue
		}
		src := filepath.Join(root, tid)
		if !dirExists(src) {
			continue
		}
		found = true
		_ = copyNonEmptyArtifacts(src, binDir)
	}
	return found
}

// copyFlatArtifactRoot copies product files that live directly in dir.
// Child directories are other build targets and must not be flattened in.
func copyFlatArtifactRoot(dir, binDir string) {
	if dir == "" || !dirExists(dir) {
		return
	}
	_ = copyProductArtifacts(dir, binDir, false)
}

func copyNonEmptyArtifacts(src, dst string) error {
	return copyProductArtifacts(src, dst, true)
}

func copyProductArtifacts(src, dst string, recurse bool) error {
	src = filepath.Clean(src)
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if !recurse && path != src {
				return filepath.SkipDir
			}
			return nil
		}
		if !isProductArtifact(info) {
			return nil
		}
		return copyFile(path, filepath.Join(dst, info.Name()))
	})
}

func isProductArtifact(info os.FileInfo) bool {
	if info == nil || info.IsDir() || info.Size() < MinArtifactBytes {
		return false
	}
	low := strings.ToLower(info.Name())
	if !(strings.Contains(low, "server") || strings.Contains(low, "agent") ||
		strings.Contains(low, "gadget") || strings.Contains(low, "helper") || strings.Contains(low, "inject")) {
		return false
	}
	// Product binaries only — skip meson intermediates / headers / sources
	switch {
	case strings.HasSuffix(low, ".h"), strings.HasSuffix(low, ".c"), strings.HasSuffix(low, ".cc"),
		strings.HasSuffix(low, ".cpp"), strings.HasSuffix(low, ".vala"), strings.HasSuffix(low, ".vapi"),
		strings.HasSuffix(low, ".txt"), strings.HasSuffix(low, ".json"), strings.HasSuffix(low, ".gir"):
		return false
	default:
		return true
	}
}

func listNonEmptyFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() < MinArtifactBytes {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

// PatchArtifactBinaryMarkers rewrites same-length detection strings inside product binaries.
// magic must be len 5 (ValidateMagicName) so frida-server ↔ {magic}-server lengths match.
// Includes product markers + full client protocol surface (rpc / re.frida.* / Frida.*) so
// server binaries stay aligned with PatchClientProtocolSurface(full=true) host wheels.
func PatchArtifactBinaryMarkers(dir, magic string) (int, error) {
	if err := core.ValidateMagicName(magic); err != nil {
		return 0, err
	}
	n := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if info.Size() < MinArtifactBytes || info.Size() > 200*1024*1024 {
			return nil
		}
		changed, err := PatchProductFile(path, magic)
		if err != nil {
			return err
		}
		if changed {
			n++
		}
		return nil
	})
	return n, err
}

// PatchProductFile rewrites one product binary: same-length markers, then
// Android helper dex checksums and agent ELF hashes.
// The helper step is what stops Android frida-server from aborting with
// `backend_class != null` after `Lre/frida/...` is renamed.
// Returns true when the file was written.
func PatchProductFile(path, magic string) (bool, error) {
	if err := core.ValidateMagicName(magic); err != nil {
		return false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if info.IsDir() || info.Size() < MinArtifactBytes || info.Size() > 200*1024*1024 {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	orig := data
	pairs := productMarkerPairs(magic)
	for _, p := range pairs {
		if len(p[0]) != len(p[1]) {
			continue
		}
		data = bytes.ReplaceAll(data, p[0], p[1])
	}
	// Always run after the string replace. A whole-file replace of `re/frida`
	// leaves the embedded helper dex checksum stale; ART then fails the class
	// lookup and the server aborts with backend_class != null.
	helperHits := PatchAndroidHelperInBinary(data, magic)
	if helperHits == 0 && bytes.Equal(orig, data) {
		return false, nil
	}
	if err := os.WriteFile(path, data, info.Mode()); err != nil {
		return false, err
	}
	return true, nil
}

func productMarkerPairs(magic string) [][2][]byte {
	pairs := [][2][]byte{
		{[]byte("frida-server"), []byte(magic + "-server")},
		{[]byte("frida-helper"), []byte(magic + "-helper")},
		{[]byte("frida-agent"), []byte(magic + "-agent")},
		{[]byte("frida-gadget"), []byte(magic + "-gadget")},
		{[]byte("frida-policyd"), []byte(magic + "-policyd")},
		{[]byte("gum-js-loop"), []byte(magic + "-js-loop")},
		{[]byte("frida-main-loop"), []byte(magic + "-main-loop")},
		{[]byte("pool-frida"), []byte("pool-" + magic)},
		{[]byte("frida_server"), []byte(magic + "_server")},
		{[]byte("frida_helper"), []byte(magic + "_helper")},
		{[]byte("frida_agent"), []byte(magic + "_agent")},
		{[]byte("frida_gadget"), []byte(magic + "_gadget")},
		{[]byte("frida-zymbiote"), []byte(magic + "-zymbiote")},
		{[]byte("u:object_r:frida"), []byte("u:object_r:" + magic)},
		{[]byte("_frida_"), []byte("_" + magic + "_")},
	}
	if proto, err := core.ClientProtocolBinaryPairs(magic); err == nil {
		pairs = append(pairs, proto...)
	} else {
		pairs = append(pairs, [2][]byte{[]byte("frida:rpc"), []byte(magic + ":rpc")})
	}
	return pairs
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0755)
	return os.WriteFile(dst, data, 0644)
}

func uniqStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func writeCatalogIndex(catalogRoot string) error {
	ents, err := ListCatalog(catalogRoot)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Fridare rebuild catalog\n")
	b.WriteString("# layout: {version}/{platform}/{magic}/\n\n")
	for _, e := range ents {
		flags := ""
		if e.HasBin {
			flags += "bin "
		}
		if e.HasWheel {
			flags += "wheel "
		}
		b.WriteString(fmt.Sprintf("%s / %s / %s  [%s]\n  %s\n", e.Version, e.Platform, e.Magic, strings.TrimSpace(flags), e.Path))
	}
	return os.WriteFile(filepath.Join(catalogRoot, "INDEX.txt"), []byte(b.String()), 0644)
}
