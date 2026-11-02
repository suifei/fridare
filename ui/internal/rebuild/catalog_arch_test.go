package rebuild

import (
	"os"
	"path/filepath"
	"testing"
)

// Multi-platform export must not let sibling artifact dirs overwrite each other.
// Regression: android-arm64 catalog previously received android-x86_64 xcand-server
// because copyNonEmptyArtifacts walked the parent artifacts root.
func TestOrganizeExportToCatalog_KeepsPerPlatformArch(t *testing.T) {
	tmp := t.TempDir()
	arts := filepath.Join(tmp, "artifacts")
	arm := filepath.Join(arts, "android-arm64")
	x64 := filepath.Join(arts, "android-x86_64")
	_ = os.MkdirAll(arm, 0755)
	_ = os.MkdirAll(x64, 0755)

	mk := func(dir, name string, tag byte) {
		payload := append([]byte{tag}, make([]byte, 2048)...)
		if err := os.WriteFile(filepath.Join(dir, name), payload, 0644); err != nil {
			t.Fatal(err)
		}
	}
	mk(arm, "frida-server", 'A')
	mk(arm, "frida-agent.so", 'A')
	mk(x64, "frida-server", 'X')
	mk(x64, "frida-agent.so", 'X')
	mk(x64, "frida-agent-arm64.so", 'X') // must not leak into arm64 catalog

	catalog := filepath.Join(tmp, "catalog")
	cfg := JobConfig{
		FridaVersion: "17.24.3",
		TargetIDs:    []string{"android-arm64", "android-x86_64"},
		MagicName:    "xcand",
		ListenPort:   27042,
		WorkDir:      tmp,
	}
	_, entries, err := OrganizeExportToCatalog(catalog, cfg, arts, filepath.Join(tmp, "flat"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries=%d", len(entries))
	}

	armServer := filepath.Join(CatalogEntryDir(catalog, "17.24.3", "android-arm64", "xcand"), "binaries", "xcand-server")
	x64Server := filepath.Join(CatalogEntryDir(catalog, "17.24.3", "android-x86_64", "xcand"), "binaries", "xcand-server")
	armData, err := os.ReadFile(armServer)
	if err != nil {
		t.Fatal(err)
	}
	x64Data, err := os.ReadFile(x64Server)
	if err != nil {
		t.Fatal(err)
	}
	if armData[0] != 'A' {
		t.Fatalf("arm64 catalog server tag=%q want A (got wrong arch merge)", armData[0])
	}
	if x64Data[0] != 'X' {
		t.Fatalf("x86_64 catalog server tag=%q want X", x64Data[0])
	}
	leak := filepath.Join(CatalogEntryDir(catalog, "17.24.3", "android-arm64", "xcand"), "binaries", "xcand-agent-arm64.so")
	if _, err := os.Stat(leak); !os.IsNotExist(err) {
		t.Fatal("x86_64-only agent-arm64.so must not appear in android-arm64 catalog")
	}
}

func TestCopyNonEmptyArtifacts_DoesNotRecurseSiblingPlatforms(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	_ = os.MkdirAll(filepath.Join(src, "android-x86_64"), 0755)
	_ = os.WriteFile(filepath.Join(src, "android-x86_64", "xcand-server"), append([]byte{'X'}, make([]byte, 2048)...), 0644)
	_ = os.WriteFile(filepath.Join(src, "xcand-server"), append([]byte{'F'}, make([]byte, 2048)...), 0644) // flat legacy
	if err := copyNonEmptyArtifacts(src, dst); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "xcand-server"))
	if err != nil {
		t.Fatal(err)
	}
	if data[0] != 'F' {
		t.Fatalf("expected flat file only, tag=%q", data[0])
	}
}
