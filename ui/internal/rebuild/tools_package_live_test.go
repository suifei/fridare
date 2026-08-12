package rebuild

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPatchFridaRPCInWheel_Unit drives the shipped patcher on a tiny synthetic wheel
// covering pure-Python and a fake native extension blob.
func TestPatchFridaRPCInWheel_Unit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "frida-17.16.4-py3-none-any.whl")
	// minimal wheel zip with frida:rpc in .py + binary module
	if err := writeTinyWheel(src, map[string]string{
		"frida/__init__.py": "x = ['frida:rpc']\n",
		"frida/aio.py":      "y = 'frida:rpc'\n",
		// native blob: only frida:rpc must change; frida_rpc_client stays
		"frida/_frida.pyd": "pad\x00frida:rpc\x00frida_rpc_client_call\x00end",
	}); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out.whl")
	n, err := patchFridaRPCInWheel(src, dst, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if n < 3 {
		t.Fatalf("replacements=%d want >=3 (2 py + 1 pyd)", n)
	}
	if countInZip(t, dst, "frida:rpc") != 0 {
		t.Fatal("still has frida:rpc")
	}
	if countInZip(t, dst, "abcde:rpc") < 3 {
		t.Fatal("missing abcde:rpc")
	}
	if countInZip(t, dst, "frida_rpc_client_call") < 1 {
		t.Fatal("must not break frida_rpc_client_* symbols")
	}
}

func TestLiveBuildPatchedFridaToolsAndHostWheels(t *testing.T) {
	if os.Getenv("FRIDARE_LIVE_WHEEL") == "" {
		t.Skip("set FRIDARE_LIVE_WHEEL=1 for network live test")
	}
	scratch := os.Getenv("SCRATCH")
	if scratch == "" {
		t.Fatal("SCRATCH required")
	}
	cat := filepath.Join(scratch, "catalog-wheel-test")
	_ = os.RemoveAll(cat)
	cfg := JobConfig{
		FridaVersion: "17.16.4",
		MagicName:    "abcde",
		ListenPort:   27142,
		WorkDir:      filepath.Join(scratch, "work"),
		DockerMirror: "docker.1ms.run",
	}
	entry := CatalogEntryDir(cat, cfg.FridaVersion, "android-arm64", cfg.MagicName)
	_ = os.MkdirAll(filepath.Join(entry, "binaries"), 0755)
	_ = os.WriteFile(filepath.Join(entry, "binaries", "abcde-server"), []byte("x"), 0644)
	wheels, err := BuildPatchedFridaToolsWheels(cfg, cat, []string{entry}, nil)
	if err != nil {
		t.Logf("BuildPatchedFridaToolsWheels err: %v", err)
	}
	hostRoot := filepath.Join(cat, "17.16.4", "_host-tools", "abcde", "python", "host")
	var platforms []string
	ents, _ := os.ReadDir(hostRoot)
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		wh, _ := filepath.Glob(filepath.Join(hostRoot, e.Name(), "frida-*.whl"))
		if len(wh) == 0 {
			continue
		}
		platforms = append(platforms, e.Name())
		if countInZip(t, wh[0], "abcde:rpc") < 1 {
			t.Errorf("%s: patched wheel missing abcde:rpc", e.Name())
		}
		if countInZip(t, wh[0], "frida:rpc") > 0 {
			t.Errorf("%s: still contains frida:rpc", e.Name())
		}
	}
	t.Logf("platforms=%v wheels=%d", platforms, len(wheels))
	// Expect the full host matrix: Win/macOS/Linux × amd64/arm64 (6)
	wantMin := 4 // tolerate one or two PyPI gaps; never a single-host-only build
	if len(platforms) < wantMin {
		t.Fatalf("want multi-platform host frida wheels (>=%d of 6), got %v (err=%v)", wantMin, platforms, err)
	}
	// All expected platform ids that exist must have binary frida:rpc cleared
	for _, p := range platforms {
		wh, _ := filepath.Glob(filepath.Join(hostRoot, p, "frida-*.whl"))
		if len(wh) == 0 {
			continue
		}
		if countInZip(t, wh[0], "frida:rpc") > 0 {
			t.Errorf("%s: native/python still contains frida:rpc after patch", p)
		}
	}
}

func writeTinyWheel(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(body)); err != nil {
			return err
		}
	}
	return zw.Close()
}

func countInZip(t *testing.T, whl, needle string) int {
	t.Helper()
	r, err := zip.OpenReader(whl)
	if err != nil {
		t.Log(err)
		return 0
	}
	defer r.Close()
	n := 0
	buf := make([]byte, 1<<20) // stream — frida wheels embed ~100MB+ native modules
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		// carry suffix across chunk boundaries so multi-byte needles are not split-missed
		carry := ""
		for {
			nr, er := rc.Read(buf)
			if nr > 0 {
				chunk := carry + string(buf[:nr])
				n += strings.Count(chunk, needle)
				if len(needle) > 1 && len(chunk) >= len(needle)-1 {
					carry = chunk[len(chunk)-(len(needle)-1):]
				} else {
					carry = chunk
				}
			}
			if er != nil {
				break
			}
		}
		rc.Close()
	}
	return n
}
