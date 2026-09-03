package rebuild

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackReleaseZips(t *testing.T) {
	root := t.TempDir()
	cfg := JobConfig{FridaVersion: "16.7.19", MagicName: "kxmwp", ListenPort: 27042}
	bin := filepath.Join(CatalogEntryDir(root, cfg.FridaVersion, "android-arm64", cfg.MagicName), "binaries")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "kxmwp-server"), make([]byte, MinArtifactBytes+8), 0755); err != nil {
		t.Fatal(err)
	}
	py := filepath.Join(CatalogEntryDir(root, cfg.FridaVersion, "_host-tools", cfg.MagicName), "python")
	host := filepath.Join(py, "host", "windows-amd64")
	if err := os.MkdirAll(host, 0755); err != nil {
		t.Fatal(err)
	}
	toolsName := "frida_tools-13.7.1+frida.16.7.19.fridare.kxmwp-py3-none-any.whl"
	if err := os.WriteFile(filepath.Join(py, toolsName), []byte("whl"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(host, "frida-16.7.19-cp37-abi3-win_amd64.whl"), []byte("host"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(py, "INSTALL.txt"), []byte("host/windows-amd64/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := PackReleaseZips(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("packed %d want 2: %v", len(got), got)
	}
	rel := CatalogReleaseDir(root, cfg.FridaVersion)
	server := filepath.Join(rel, "frida-kxmwp-16.7.19-android-arm64-server.zip")
	wheels := filepath.Join(rel, "frida-kxmwp-16.7.19-host-wheels.zip")
	if _, err := os.Stat(server); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wheels); err != nil {
		t.Fatal(err)
	}
	assertZipHas(t, server, "kxmwp-server", "ORIGIN.txt", "USAGE.md")
	assertZipHas(t, wheels, toolsName, "host/windows-amd64/frida-16.7.19-cp37-abi3-win_amd64.whl", "INSTALL.txt")
}

func assertZipHas(t *testing.T, zipPath string, names ...string) {
	t.Helper()
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	have := map[string]bool{}
	for _, f := range r.File {
		have[f.Name] = true
	}
	for _, n := range names {
		if !have[n] {
			var all []string
			for k := range have {
				all = append(all, k)
			}
			t.Fatalf("%s missing %s; have %s", zipPath, n, strings.Join(all, ","))
		}
	}
}
