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

		// Prefer docker artifacts under target subdir
		srcCandidates := []string{
			filepath.Join(dockerArts, tid),
			filepath.Join(flatArtHost, tid),
			dockerArts,
			flatArtHost,
		}
		for _, src := range srcCandidates {
			if st, e := os.Stat(src); e == nil && st.IsDir() {
				_ = copyNonEmptyArtifacts(src, binDir)
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

func copyNonEmptyArtifacts(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			// don't recurse into nested build dirs beyond one level of named blobs
			return nil
		}
		if info.Size() < MinArtifactBytes {
			return nil
		}
		base := info.Name()
		low := strings.ToLower(base)
		if !(strings.Contains(low, "server") || strings.Contains(low, "agent") ||
			strings.Contains(low, "gadget") || strings.Contains(low, "helper") || strings.Contains(low, "inject")) {
			return nil
		}
		// Product binaries only — skip meson intermediates / headers / sources
		if strings.HasSuffix(low, ".h") || strings.HasSuffix(low, ".c") || strings.HasSuffix(low, ".cc") ||
			strings.HasSuffix(low, ".cpp") || strings.HasSuffix(low, ".vala") || strings.HasSuffix(low, ".vapi") ||
			strings.HasSuffix(low, ".txt") || strings.HasSuffix(low, ".json") || strings.HasSuffix(low, ".gir") {
			return nil
		}
		return copyFile(path, filepath.Join(dst, base))
	})
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
		// Stealth markers (same length when magic is 5 letters)
		{[]byte("frida-zymbiote"), []byte(magic + "-zymbiote")},
		{[]byte("u:object_r:frida"), []byte("u:object_r:" + magic)},
		{[]byte("_frida_"), []byte("_" + magic + "_")},
	}
	// Server+client protocol surface (same length when magic is 5 letters)
	if proto, err := core.ClientProtocolBinaryPairs(magic); err == nil {
		pairs = append(pairs, proto...)
	} else {
		// fallback product-only rpc (should not happen after ValidateMagicName)
		pairs = append(pairs, [2][]byte{[]byte("frida:rpc"), []byte(magic + ":rpc")})
	}
	n := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if info.Size() < MinArtifactBytes || info.Size() > 200*1024*1024 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		orig := data
		for _, p := range pairs {
			if len(p[0]) != len(p[1]) {
				continue
			}
			data = bytes.ReplaceAll(data, p[0], p[1])
		}
		if PatchAndroidHelperInBinary(data, magic) > 0 || !bytes.Equal(orig, data) {
			if err := os.WriteFile(path, data, info.Mode()); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
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
