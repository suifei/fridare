package rebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CatalogReleaseDir is catalog/{version}/_release — GitHub-style zip layout.
func CatalogReleaseDir(catalogRoot, version string) string {
	return filepath.Join(catalogRoot, sanitizeCatalogSeg(version), "_release")
}

// PackReleaseZips writes platform server zips and host-wheels.zip under
// catalog/{ver}/_release/, matching the GitHub asset names:
//
//	frida-{magic}-{ver}-{platform}-server.zip
//	frida-{magic}-{ver}-host-wheels.zip
//
// Server zip: binaries at zip root + ORIGIN.txt.
// Host-wheels zip: contents of _host-tools/{magic}/python/ (host/, tools wheel, INSTALL.txt).
func PackReleaseZips(catalogRoot string, cfg JobConfig) ([]string, error) {
	if catalogRoot == "" || strings.TrimSpace(cfg.FridaVersion) == "" || strings.TrimSpace(cfg.MagicName) == "" {
		return nil, fmt.Errorf("catalog/version/magic required")
	}
	ver := sanitizeCatalogSeg(cfg.FridaVersion)
	magic := sanitizeCatalogSeg(cfg.MagicName)
	verDir := filepath.Join(catalogRoot, ver)
	st, err := os.Stat(verDir)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("catalog version dir: %w", err)
	}
	outDir := CatalogReleaseDir(catalogRoot, cfg.FridaVersion)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, err
	}
	pipVer := EffectivePipVersion(cfg)
	if pipVer == "" {
		pipVer = cfg.FridaVersion
	}

	var packed []string
	ents, err := os.ReadDir(verDir)
	if err != nil {
		return nil, err
	}
	for _, e := range ents {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		plat := e.Name()
		binDir := filepath.Join(verDir, plat, magic, "binaries")
		files, _ := listNonEmptyFiles(binDir)
		if len(files) == 0 {
			continue
		}
		stage, err := os.MkdirTemp("", "fridare-rel-"+plat+"-*")
		if err != nil {
			return packed, err
		}
		for _, name := range files {
			_ = copyFile(filepath.Join(binDir, name), filepath.Join(stage, name))
		}
		origin := fmt.Sprintf("origin=GUI-path source rebuild\nfrida=%s\npip_frida=%s\nmagic=%s\nplatform=%s\n",
			cfg.FridaVersion, pipVer, cfg.MagicName, plat)
		_ = os.WriteFile(filepath.Join(stage, "ORIGIN.txt"), []byte(origin), 0644)
		usage := fmt.Sprintf("# %s %s (%s)\n\n./%s-server -l 0.0.0.0:%d\n\nClient: same-tag host-wheels zip (do not mix 16/17).\n",
			cfg.MagicName, cfg.FridaVersion, plat, cfg.MagicName, NormalizeListenPort(cfg.ListenPort))
		_ = os.WriteFile(filepath.Join(stage, "USAGE.md"), []byte(usage), 0644)
		zipName := fmt.Sprintf("frida-%s-%s-%s-server.zip", magic, ver, plat)
		zipPath := filepath.Join(outDir, zipName)
		if zerr := zipDir(stage, zipPath); zerr != nil {
			_ = os.RemoveAll(stage)
			return packed, zerr
		}
		_ = os.RemoveAll(stage)
		packed = append(packed, zipPath)
	}

	hostPy := filepath.Join(CatalogEntryDir(catalogRoot, cfg.FridaVersion, "_host-tools", cfg.MagicName), "python")
	if dirExists(hostPy) {
		zipName := fmt.Sprintf("frida-%s-%s-host-wheels.zip", magic, ver)
		zipPath := filepath.Join(outDir, zipName)
		if zerr := zipDir(hostPy, zipPath); zerr != nil {
			return packed, zerr
		}
		packed = append(packed, zipPath)
	}
	return packed, nil
}
