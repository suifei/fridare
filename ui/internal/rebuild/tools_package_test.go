package rebuild

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFridaToolsInstallNotesRelativeHostPaths(t *testing.T) {
	cfg := JobConfig{FridaVersion: "16.7.19", MagicName: "kxmwp", ListenPort: 27042}
	notes := fridaToolsInstallNotes(cfg,
		[]string{filepath.Join("out", "frida_tools-13.7.1+frida.16.7.19.fridare.kxmwp-py3-none-any.whl")},
		[]string{filepath.Join("D:", "works", "catalog", "python", "host", "windows-amd64", "frida-16.7.19-cp37-abi3-win_amd64.whl")},
	)
	if strings.Contains(notes, `D:\`) || strings.Contains(notes, `/works/`) {
		t.Fatalf("INSTALL.txt leaked absolute path:\n%s", notes)
	}
	if strings.Contains(notes, `python/host/`) {
		t.Fatal("INSTALL pip examples must be host/<plat>/ (relative to python/ or zip root)")
	}
	if !strings.Contains(notes, "host/windows-amd64/frida-16.7.19-cp37-abi3-win_amd64.whl") {
		t.Fatalf("missing relative host path:\n%s", notes)
	}
	if !strings.Contains(notes, "frida_tools-13.7.1+frida.16.7.19.fridare.kxmwp-py3-none-any.whl") {
		t.Fatalf("missing tools wheel name:\n%s", notes)
	}
	if strings.Contains(notes, "frida_tools-tools.frida.") {
		t.Fatal("must not advertise illegal tools.frida filename")
	}
}

func TestCanonicalToolsVersionRecoversFromPKGINFO(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "PKG-INFO"), []byte("Name: frida-tools\nVersion: 13.7.1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := canonicalToolsVersion("tools", root, "frida-tools-13.7.1.tar.gz")
	if got != "13.7.1" {
		t.Fatalf("got %q want 13.7.1", got)
	}
	got = pinFridaToolsPackageVersion(root, "16.7.19", "tools", "kxmwp")
	want := "13.7.1+frida.16.7.19.fridare.kxmwp"
	if got != want {
		t.Fatalf("pin %q want %q", got, want)
	}
}

func TestRewriteToolsWheelToVersion(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "frida_tools-tools.frida.16.7.19.fridare.kxmwp-py3-none-any.whl")
	if err := writeTinyToolsWheel(old, "tools.frida.16.7.19.fridare.kxmwp"); err != nil {
		t.Fatal(err)
	}
	local := "13.7.1+frida.16.7.19.fridare.kxmwp"
	got, err := rewriteToolsWheelToVersion(old, local)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "frida_tools-13.7.1+frida.16.7.19.fridare.kxmwp-py3-none-any.whl")
	if got != want {
		t.Fatalf("path %q want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old illegal filename should be gone")
	}
	r, err := zip.OpenReader(got)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	foundMeta := false
	for _, f := range r.File {
		if strings.Contains(f.Name, "tools.frida.") {
			t.Fatalf("leftover dist-info name %s", f.Name)
		}
		if strings.HasSuffix(f.Name, "METADATA") {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			if !strings.Contains(string(b), "Version: 13.7.1+frida.16.7.19.fridare.kxmwp") {
				t.Fatalf("METADATA: %s", b)
			}
			foundMeta = true
		}
	}
	if !foundMeta {
		t.Fatal("missing METADATA")
	}
}

func writeTinyToolsWheel(path, ver string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	info := fmt.Sprintf("frida_tools-%s.dist-info/METADATA", ver)
	w, err := zw.Create(info)
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte("Metadata-Version: 2.1\nName: frida-tools\nVersion: " + ver + "\n")); err != nil {
		return err
	}
	w2, err := zw.Create("frida_tools/__init__.py")
	if err != nil {
		return err
	}
	_, _ = w2.Write([]byte("# patched\n"))
	return zw.Close()
}

func TestParseVersionFromArchiveName(t *testing.T) {
	cases := map[string]string{
		"frida_tools-14.10.4.tar.gz":             "14.10.4",
		"frida-tools-13.7.1.tar.gz":              "13.7.1",
		"frida-tools-13.7.1.zip":                 "13.7.1",
		"frida_tools-14.10.4-py3-none-any.whl":   "14.10.4",
		"frida-tools-13.7.1-py3-none-any.whl":    "13.7.1",
		"frida_tools-14.10.4+frida.17.17.0.fridare.kxmwp-py3-none-any.whl": "14.10.4+frida.17.17.0.fridare.kxmwp",
	}
	for in, want := range cases {
		got := parseVersionFromArchiveName(in)
		if got != want {
			t.Errorf("parseVersionFromArchiveName(%q)=%q want %q", in, got, want)
		}
	}
	// The 16.7.19 host-wheels bug: hyphenated sdist must not yield "tools".
	if got := parseVersionFromArchiveName("frida-tools-13.7.1.tar.gz"); got == "tools" {
		t.Fatal("hyphenated frida-tools sdist must not parse as version tools")
	}
	got := pinFridaToolsPackageVersion(t.TempDir(), "16.7.19", parseVersionFromArchiveName("frida-tools-13.7.1.tar.gz"), "kxmwp")
	want := "13.7.1+frida.16.7.19.fridare.kxmwp"
	if got != want {
		t.Fatalf("16.x local version %q want %q", got, want)
	}
}

func TestPep440WheelFilenameVersion(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"14.10.4+frida.17.17.0.fridare.kxmwp", "14.10.4+frida.17.17.0.fridare.kxmwp", true},
		{"14.10.4.frida.17.17.0.fridare.kxmwp", "14.10.4+frida.17.17.0.fridare.kxmwp", true},
		{"14.10.4_frida.17.17.0.fridare.kxmwp", "14.10.4+frida.17.17.0.fridare.kxmwp", true},
		{"14.10.4", "14.10.4", true},
		{" 14.10.4+frida.17.16.4.fridare.abcde ", "14.10.4+frida.17.16.4.fridare.abcde", true},
	}
	for _, c := range cases {
		got := pep440WheelFilenameVersion(c.in)
		if got != c.want {
			t.Errorf("pep440WheelFilenameVersion(%q)=%q want %q", c.in, got, c.want)
		}
		if isPEP440Version(got) != c.ok {
			t.Errorf("isPEP440Version(%q)=%v want %v", got, isPEP440Version(got), c.ok)
		}
	}
	if isPEP440Version("14.10.4.frida.17.17.0.fridare.kxmwp") {
		t.Fatal("dotted local version must not pass PEP 440")
	}
}

func TestPinFridaToolsPackageVersionLocal(t *testing.T) {
	root := t.TempDir()
	got := pinFridaToolsPackageVersion(root, "17.17.0", "14.10.4", "kxmwp")
	want := "14.10.4+frida.17.17.0.fridare.kxmwp"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if !isPEP440Version(got) {
		t.Fatalf("not PEP 440: %s", got)
	}
}

func TestBuildPureFridaToolsWheelKeepsPlus(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "frida_tools")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "__init__.py"), []byte("# patched\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ver := pinFridaToolsPackageVersion(root, "17.17.0", "14.10.4", "kxmwp")
	out := filepath.Join(root, "out")
	whl, err := buildPureFridaToolsWheel(root, out, ver)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(whl)
	want := "frida_tools-14.10.4+frida.17.17.0.fridare.kxmwp-py3-none-any.whl"
	if base != want {
		t.Fatalf("wheel name %q want %q", base, want)
	}
	if strings.Contains(base, "14.10.4.frida.") {
		t.Fatal("must not emit dotted local version (pip rejects it)")
	}
	infoDir := "frida_tools-14.10.4+frida.17.17.0.fridare.kxmwp.dist-info/"
	if !zipHasPrefix(t, whl, infoDir) {
		t.Fatalf("missing dist-info %s", infoDir)
	}
	if zipHasPrefix(t, whl, "frida_tools-14.10.4.frida.") {
		t.Fatal("dist-info still uses dotted local version")
	}
}

func TestEnsurePEP440ToolsWheelFilename(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "frida_tools-14.10.4.frida.17.17.0.fridare.kxmwp-py3-none-any.whl")
	if err := os.WriteFile(old, []byte("whl"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ensurePEP440ToolsWheelFilename(old)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "frida_tools-14.10.4+frida.17.17.0.fridare.kxmwp-py3-none-any.whl")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old invalid filename should be gone")
	}
}

func zipHasPrefix(t *testing.T, whl, prefix string) bool {
	t.Helper()
	r, err := zip.OpenReader(whl)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, prefix) {
			return true
		}
	}
	return false
}
