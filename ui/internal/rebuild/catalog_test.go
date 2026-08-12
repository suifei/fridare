package rebuild

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogEntryDirAndList(t *testing.T) {
	root := t.TempDir()
	entry := CatalogEntryDir(root, "17.16.4", "android-arm64", "abcde")
	_ = os.MkdirAll(filepath.Join(entry, "binaries"), 0755)
	_ = os.WriteFile(filepath.Join(entry, "binaries", "abcde-server"), []byte("x"), 0644)
	_ = os.MkdirAll(filepath.Join(entry, "python"), 0755)
	_ = os.WriteFile(filepath.Join(entry, "python", "frida_tools-1-py3-none-any.whl"), []byte("whl"), 0644)
	_ = WriteManifest(entry, ArtifactManifest{
		FridaVersion: "17.16.4",
		Platform:     "android-arm64",
		MagicName:    "abcde",
	})

	ents, err := ListCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Fatalf("got %d", len(ents))
	}
	if !ents[0].HasBin || !ents[0].HasWheel {
		t.Fatalf("%+v", ents[0])
	}
	if ents[0].Version != "17.16.4" || ents[0].Magic != "abcde" {
		t.Fatal(ents[0])
	}
}

func TestOrganizeExportToCatalog(t *testing.T) {
	tmp := t.TempDir()
	dockerArts := filepath.Join(tmp, "docker-arts", "android-arm64")
	_ = os.MkdirAll(dockerArts, 0755)
	// > MinArtifactBytes with detectable markers
	payload := append([]byte("xfrida-server\x00frida_agent\x00"), make([]byte, 2048)...)
	_ = os.WriteFile(filepath.Join(dockerArts, "frida-server"), payload, 0644)
	catalog := filepath.Join(tmp, "catalog")
	cfg := JobConfig{
		FridaVersion: "17.16.4",
		TargetIDs:    []string{"android-arm64"},
		MagicName:    "abcde",
		ListenPort:   27142,
		WorkDir:      tmp,
	}
	primary, entries, err := OrganizeExportToCatalog(catalog, cfg, filepath.Dir(dockerArts), filepath.Join(tmp, "flat"))
	if err != nil {
		t.Fatal(err)
	}
	if primary == "" || len(entries) != 1 {
		t.Fatalf("primary=%s entries=%v", primary, entries)
	}
	// After export: basename magic + MANIFEST lists abcde-server
	if _, err := os.Stat(filepath.Join(primary, "binaries", "abcde-server")); err != nil {
		t.Fatal("expected renamed binary", err)
	}
	data, _ := os.ReadFile(filepath.Join(primary, "MANIFEST.json"))
	if !strings.Contains(string(data), "abcde-server") {
		t.Fatalf("MANIFEST should list actual magic basenames: %s", data)
	}
	// in-binary markers patched
	bin, _ := os.ReadFile(filepath.Join(primary, "binaries", "abcde-server"))
	if bytes.Contains(bin, []byte("frida-server")) || bytes.Contains(bin, []byte("frida_agent")) {
		t.Fatal("binary still has frida markers")
	}
	if !bytes.Contains(bin, []byte("abcde-server")) || !bytes.Contains(bin, []byte("abcde_agent")) {
		t.Fatal("binary missing magic markers")
	}
}

func TestRenameArtifactBasenames_DropsFridaWhenMagicExists(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "abcde-server"), make([]byte, 2048), 0755)
	_ = os.WriteFile(filepath.Join(dir, "frida-server"), make([]byte, 2048), 0755) // leftover duplicate
	_ = os.WriteFile(filepath.Join(dir, "libfrida-gadget.so"), make([]byte, 2048), 0644)
	_ = os.WriteFile(filepath.Join(dir, "libabcde-gadget.so"), make([]byte, 2048), 0644)
	n, err := RenameArtifactBasenames(dir, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("expected drops, n=%d", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "frida-server")); !os.IsNotExist(err) {
		t.Fatal("frida-server leftover should be removed when abcde-server exists")
	}
	if _, err := os.Stat(filepath.Join(dir, "libfrida-gadget.so")); !os.IsNotExist(err) {
		t.Fatal("libfrida-gadget leftover should be removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "abcde-server")); err != nil {
		t.Fatal(err)
	}
}

func TestCopyNonEmptyArtifacts_SkipsSourceIntermediates(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	_ = os.WriteFile(filepath.Join(src, "abcde-server"), make([]byte, 2048), 0755)
	_ = os.WriteFile(filepath.Join(src, "abcde-helper-backend.c"), make([]byte, 2048), 0644)
	_ = os.WriteFile(filepath.Join(src, "abcde-agent.h"), make([]byte, 2048), 0644)
	if err := copyNonEmptyArtifacts(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "abcde-server")); err != nil {
		t.Fatal("server binary should copy")
	}
	if _, err := os.Stat(filepath.Join(dst, "abcde-helper-backend.c")); !os.IsNotExist(err) {
		t.Fatal(".c intermediate must not be catalog product")
	}
	if _, err := os.Stat(filepath.Join(dst, "abcde-agent.h")); !os.IsNotExist(err) {
		t.Fatal(".h must not be catalog product")
	}
}

func TestRenameArtifactBasenames(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "frida-server"), []byte("x"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "frida-agent.so"), []byte("y"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("z"), 0644)
	n, err := RenameArtifactBasenames(dir, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("renamed %d", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "abcde-server")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "abcde-agent.so")); err != nil {
		t.Fatal(err)
	}
}

func TestHostFridaWheelPlatforms_CoversMajorHosts(t *testing.T) {
	need := map[string]bool{
		"windows-amd64": false,
		"windows-arm64": false,
		"macos-x86_64":  false,
		"macos-arm64":   false,
		"linux-x86_64":  false,
		"linux-arm64":   false,
	}
	for _, p := range HostFridaWheelPlatforms {
		if _, ok := need[p.ID]; ok {
			need[p.ID] = true
		}
		if p.Platform == "" {
			t.Fatalf("empty platform for %s", p.ID)
		}
	}
	for id, ok := range need {
		if !ok {
			t.Fatalf("missing host platform %s", id)
		}
	}
}

func TestPatchFridaToolsTree(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "frida_tools")
	_ = os.MkdirAll(pkg, 0755)
	corePy := filepath.Join(pkg, "core.py")
	_ = os.WriteFile(corePy, []byte("CHANNEL = 'frida:rpc'\nimport frida\n"), 0644)
	n, err := patchFridaToolsTree(root, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("patched %d", n)
	}
	data, _ := os.ReadFile(corePy)
	if string(data) != "CHANNEL = 'abcde:rpc'\nimport frida\n" {
		t.Fatalf("%s", data)
	}
}
