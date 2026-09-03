package rebuild

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"hash/adler32"
	"os"
	"path/filepath"
	"testing"
)

func TestGnuHashKnownValues(t *testing.T) {
	// Verified against the shipped kxmwp-agent-64.so DT_GNU_HASH
	if gnuHash([]byte("frida_agent_main")) != 0x6df8f23d {
		t.Fatalf("frida_agent_main hash 0x%x", gnuHash([]byte("frida_agent_main")))
	}
	if gnuHash([]byte("kxmwp_agent_main")) != 0x2e0055ee {
		t.Fatalf("kxmwp_agent_main hash 0x%x", gnuHash([]byte("kxmwp_agent_main")))
	}
}

func makeTinyDex(t *testing.T, body []byte) []byte {
	t.Helper()
	const hdr = 0x70
	buf := make([]byte, hdr+len(body))
	copy(buf, []byte("dex\n035\x00"))
	binary.LittleEndian.PutUint32(buf[32:], uint32(len(buf)))
	copy(buf[hdr:], body)
	fixDexChecksum(buf)
	if adler32.Checksum(buf[12:]) != binary.LittleEndian.Uint32(buf[8:12]) {
		t.Fatal("adler mismatch after make")
	}
	sum := sha1.Sum(buf[32:])
	if !bytes.Equal(sum[:], buf[12:32]) {
		t.Fatal("sha1 mismatch after make")
	}
	return buf
}

func TestPatchEmbeddedDexJavaPackage_RewritesAndChecksums(t *testing.T) {
	body := []byte("Lre/frida/HelperBackend;\x00Lre/frida/Helper;\x00")
	dex := makeTinyDex(t, body)
	// wrap so finder isn't at offset 0 only
	blob := append([]byte("PADPADPAD"), dex...)
	n := patchEmbeddedDexJavaPackage(blob, "kxmwp")
	if n < 2 {
		t.Fatalf("replacements=%d want >=2", n)
	}
	if bytes.Contains(blob, []byte("re/frida")) {
		t.Fatal("still has re/frida")
	}
	if !bytes.Contains(blob, []byte("Lre/kxmwp/HelperBackend;")) {
		t.Fatal("missing renamed descriptor")
	}
	got := blob[9:]
	if int(binary.LittleEndian.Uint32(got[32:36])) != len(dex) {
		t.Fatal("file_size clobbered")
	}
	if adler32.Checksum(got[12:len(dex)]) != binary.LittleEndian.Uint32(got[8:12]) {
		t.Fatal("adler not repaired")
	}
	sum := sha1.Sum(got[32:len(dex)])
	if !bytes.Equal(sum[:], got[12:32]) {
		t.Fatal("sha1 not repaired")
	}
}

func TestRenameAgentGumJSGlobal(t *testing.T) {
	img := []byte("Frida.version=1; re.kxmwp.Host=2; 'Frida'; rawFridaType")
	n := renameAgentGumJSGlobal(img, "kxmwp")
	if n < 3 {
		t.Fatalf("n=%d want >=3", n)
	}
	if bytes.Contains(img, []byte("Frida")) {
		t.Fatalf("Frida leftover: %s", img)
	}
	if !bytes.Contains(img, []byte("Kxmwp.version=1")) || !bytes.Contains(img, []byte("'Kxmwp'")) || !bytes.Contains(img, []byte("rawKxmwpType")) {
		t.Fatalf("got %s", img)
	}
	if !bytes.Contains(img, []byte("re.kxmwp.Host=2")) {
		t.Fatal("must not rewrite re.kxmwp protocol")
	}
}

func TestPatchAndroidHelper_SkipsNonAgentELF(t *testing.T) {
	// A container ELF-looking blob must not rewrite hashes unless dynstr
	// contains *agent_main — otherwise GumJS in 16.x kxmwp-server is smashed.
	js := []byte("internal-agent.js Frida.version Kxmwp-sentinel-keep\x00")
	buf := make([]byte, MinArtifactBytes+len(js)+64)
	copy(buf, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	copy(buf[64:], js)
	n := PatchAndroidHelperInBinary(buf, "kxmwp")
	if n != 0 {
		t.Fatalf("hits=%d want 0 on non-agent ELF", n)
	}
	if !bytes.Contains(buf, []byte("Kxmwp-sentinel-keep")) {
		t.Fatal("corrupted non-agent payload")
	}
}

func TestPatchArtifactBinaryMarkers_RewritesDexJavaDescriptor(t *testing.T) {
	dir := t.TempDir()
	body := []byte("Lre/frida/HelperBackend;\x00kxmwp-server\x00")
	payload := append(body, make([]byte, MinArtifactBytes)...)
	p := filepath.Join(dir, "kxmwp-server")
	if err := os.WriteFile(p, payload, 0644); err != nil {
		t.Fatal(err)
	}
	n, err := PatchArtifactBinaryMarkers(dir, "kxmwp")
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatal("expected at least one patched file")
	}
	out, _ := os.ReadFile(p)
	if bytes.Contains(out, []byte("Lre/frida/HelperBackend")) {
		t.Fatal("dex descriptor still re/frida")
	}
	if !bytes.Contains(out, []byte("Lre/kxmwp/HelperBackend")) {
		t.Fatal("missing Lre/kxmwp/HelperBackend")
	}
}

func TestPatchEmbeddedDex_StaleChecksumOnly(t *testing.T) {
	body := []byte("Lre/kxmwp/HelperBackend;\x00")
	dex := makeTinyDex(t, body)
	// corrupt checksums as if a naive ReplaceAll touched the blob
	binary.LittleEndian.PutUint32(dex[8:12], 0)
	copy(dex[12:32], make([]byte, 20))
	blob := append([]byte{}, dex...)
	n := patchEmbeddedDexJavaPackage(blob, "kxmwp")
	if n < 1 {
		t.Fatalf("expected checksum repair, n=%d", n)
	}
	if adler32.Checksum(blob[12:]) != binary.LittleEndian.Uint32(blob[8:12]) {
		t.Fatal("adler")
	}
	sum := sha1.Sum(blob[32:])
	if !bytes.Equal(sum[:], blob[12:32]) {
		t.Fatal("sha1")
	}
}
