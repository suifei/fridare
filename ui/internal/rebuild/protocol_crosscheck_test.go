package rebuild

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrossCheckServerClientProtocol_Aligned(t *testing.T) {
	dir := t.TempDir()
	server := filepath.Join(dir, "abcde-server.exe")
	// synthetic binary with magic protocol surface (interfaces + object paths)
	payload := []byte("xxxre.abcde.Host\x00/re/abcde/HostSession\x00abcde:rpc\x00abcde-server\x00")
	payload = append(payload, make([]byte, 2048)...)
	if err := os.WriteFile(server, payload, 0755); err != nil {
		t.Fatal(err)
	}
	whl := filepath.Join(dir, "frida-17.0-py3-none-any.whl")
	if err := writeZip(whl, map[string]string{
		"frida/__init__.py": "x='re.abcde.Host'\np='/re/abcde/HostSession'\ny='abcde:rpc'\n",
	}); err != nil {
		t.Fatal(err)
	}
	r, err := CrossCheckServerClientProtocol("abcde", server, whl)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Matched || !r.ServerOK || !r.ClientOK {
		t.Fatalf("%+v", r)
	}
}

func TestCrossCheckServerClientProtocol_DetectsStockResidue(t *testing.T) {
	dir := t.TempDir()
	server := filepath.Join(dir, "abcde-server.exe")
	payload := append([]byte("re.frida.Host\x00frida:rpc\x00"), make([]byte, 2048)...)
	_ = os.WriteFile(server, payload, 0755)
	r, err := CrossCheckServerClientProtocol("abcde", server, "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Matched || r.ServerOK {
		t.Fatalf("should fail on stock residue: %+v", r)
	}
	if len(r.Issues) == 0 {
		t.Fatal("expected issues")
	}
}

func TestFindMagicServerBinary(t *testing.T) {
	entry := t.TempDir()
	bin := filepath.Join(entry, "binaries")
	_ = os.MkdirAll(bin, 0755)
	p := filepath.Join(bin, "abcde-server.exe")
	_ = os.WriteFile(p, make([]byte, 2048), 0755)
	got, err := FindMagicServerBinary(entry, "abcde")
	if err != nil || got != p {
		t.Fatalf("got %s err %v", got, err)
	}
}

func TestRunCatalogProtocolCrossCheck(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "17.16.4", "windows-x86_64", "abcde")
	bin := filepath.Join(entry, "binaries")
	_ = os.MkdirAll(bin, 0755)
	server := filepath.Join(bin, "abcde-server.exe")
	payload := append([]byte("re.abcde.Host\x00/re/abcde/HostSession\x00abcde:rpc\x00"), make([]byte, 2048)...)
	_ = os.WriteFile(server, payload, 0755)
	whl := filepath.Join(root, "frida-17.16.4-py3-none-any.whl")
	if err := writeZip(whl, map[string]string{
		"frida/core.py": "rpc='abcde:rpc'\nproto='re.abcde.X'\npath='/re/abcde/HostSession'\n",
	}); err != nil {
		t.Fatal(err)
	}
	cfg := JobConfig{MagicName: "abcde", FridaVersion: "17.16.4"}
	r, err := RunCatalogProtocolCrossCheck(root, cfg, []string{entry}, []string{whl})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Matched {
		t.Fatalf("%+v", r)
	}
	out := filepath.Join(entry, "PROTOCOL-SYNC.json")
	if err := WriteProtocolCrossCheckReport(out, r); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}

func TestPickHostFridaWheel(t *testing.T) {
	got := pickHostFridaWheel([]string{
		"/x/frida_tools-1-py3-none-any.whl",
		"/x/host/windows-amd64/frida-17.0-cp310-win_amd64.whl",
	})
	if !strings.Contains(got, "frida-17.0") {
		t.Fatal(got)
	}
}

func writeZip(path string, files map[string]string) error {
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
