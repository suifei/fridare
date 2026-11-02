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
	img := []byte("Frida.version=1; re.kxmwp.Host=2; 'Frida'; rawFridaType; Friday; FridaScriptOptions")
	n := renameAgentGumJSGlobal(img, "kxmwp")
	if n != 2 {
		t.Fatalf("n=%d want 2 exact tokens; got %s", n, img)
	}
	if !bytes.Contains(img, []byte("Kxmwp.version=1")) || !bytes.Contains(img, []byte("'Kxmwp'")) {
		t.Fatalf("got %s", img)
	}
	if !bytes.Contains(img, []byte("re.kxmwp.Host=2")) {
		t.Fatal("must not rewrite re.kxmwp protocol")
	}
	if !bytes.Contains(img, []byte("rawFridaType")) || !bytes.Contains(img, []byte("Friday")) || !bytes.Contains(img, []byte("FridaScriptOptions")) {
		t.Fatalf("must not smash Friday/Vala/embedded ident: %s", img)
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

func TestPatchProductFile_RepairsHelperDexChecksum(t *testing.T) {
	body := []byte("Lre/frida/HelperBackend;\x00frida-server\x00")
	dex := makeTinyDex(t, body)
	blob := append(append([]byte("PAD"), dex...), make([]byte, MinArtifactBytes)...)
	dir := t.TempDir()
	p := filepath.Join(dir, "frida-server")
	if err := os.WriteFile(p, blob, 0755); err != nil {
		t.Fatal(err)
	}
	changed, err := PatchProductFile(p, "kxmwp")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected android helper + marker rewrite")
	}
	out, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("Lre/frida/HelperBackend")) {
		t.Fatal("dex descriptor still re/frida")
	}
	if !bytes.Contains(out, []byte("Lre/kxmwp/HelperBackend")) {
		t.Fatal("missing renamed descriptor")
	}
	at := bytes.Index(out, []byte("dex\n"))
	if at < 0 {
		t.Fatal("dex header lost")
	}
	size := int(binary.LittleEndian.Uint32(out[at+32 : at+36]))
	got := out[at : at+size]
	if adler32.Checksum(got[12:]) != binary.LittleEndian.Uint32(got[8:12]) {
		t.Fatal("dex adler stale after export patch; Android server aborts backend_class != null")
	}
	sum := sha1.Sum(got[32:])
	if !bytes.Equal(sum[:], got[12:32]) {
		t.Fatal("dex sha1 stale")
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

func TestNeedSourceHelperDexPatch(t *testing.T) {
	if needSourceHelperDexPatch(JobConfig{MagicName: "xcand", DirectionProfile: "deep"}) != true {
		t.Fatal("deep should patch")
	}
	if needSourceHelperDexPatch(JobConfig{MagicName: "xcand", DirectionProfile: ""}) != true {
		t.Fatal("empty profile defaults to deep")
	}
	if needSourceHelperDexPatch(JobConfig{MagicName: "xcand", DirectionProfile: "safe"}) {
		t.Fatal("safe must not rewrite source dex")
	}
	if needSourceHelperDexPatch(JobConfig{MagicName: "xcand", DirectionProfile: "abi"}) {
		t.Fatal("abi must not rewrite source dex")
	}
	if needSourceHelperDexPatch(JobConfig{MagicName: "frida", DirectionProfile: "deep"}) {
		t.Fatal("stock magic skips")
	}
}

func TestPatchAndroidHelperDexFile_RealHelperDex(t *testing.T) {
	src := filepath.Join("testdata", "helper-17.24.3.dex")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skip("testdata/helper-17.24.3.dex missing:", err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "android-helper", "helper.dex")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0644); err != nil {
		t.Fatal(err)
	}
	n, err := PatchAndroidHelperDexFile(p, "xcand")
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("hits=%d", n)
	}
	out, _ := os.ReadFile(p)
	if bytes.Contains(out, []byte("re/frida")) {
		t.Fatal("still re/frida")
	}
	if !bytes.Contains(out, []byte("Lre/xcand/HelperBackend")) {
		t.Fatal("missing Lre/xcand/HelperBackend")
	}
	if adler32.Checksum(out[12:]) != binary.LittleEndian.Uint32(out[8:12]) {
		t.Fatal("adler")
	}
	sum := sha1.Sum(out[32:])
	if !bytes.Equal(sum[:], out[12:32]) {
		t.Fatal("sha1")
	}
}

func TestPatchAndroidHelperDexInTree_AndDeepExtras(t *testing.T) {
	src := filepath.Join("testdata", "helper-17.24.3.dex")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	dexPath := filepath.Join(root, "subprojects", "frida-core", "src", "android-helper", "helper.dex")
	if err := os.MkdirAll(filepath.Dir(dexPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dexPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	// Vala JNI path that deep ProtocolAPI would rewrite
	vala := filepath.Join(root, "subprojects", "frida-core", "src", "linux", "linux-host-session.vala")
	if err := os.MkdirAll(filepath.Dir(vala), 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(vala, []byte(`var backend_class = (*env)->find_class (env, "re/frida/HelperBackend");`), 0644)

	cfg := JobConfig{MagicName: "xcand", DirectionProfile: "deep", ListenPort: 27142, DisableJunk: true}
	if err := ApplyDeepSourceExtras(root, cfg); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(dexPath)
	if bytes.Contains(out, []byte("re/frida")) {
		t.Fatal("source helper.dex not patched by ApplyDeepSourceExtras")
	}
	if !bytes.Contains(out, []byte("Lre/xcand/HelperBackend")) {
		t.Fatal("expected Lre/xcand in source dex")
	}
	vs, _ := os.ReadFile(vala)
	if !bytes.Contains(vs, []byte(`"re/xcand/HelperBackend"`)) {
		t.Fatalf("vala JNI path not rewritten: %s", vs)
	}
	if _, err := os.Stat(filepath.Join(root, "fridare-android-helper-dex.txt")); err != nil {
		t.Fatal("missing fridare-android-helper-dex.txt marker")
	}
}

func TestAndroidHelperDexAligned_OfficialPatched(t *testing.T) {
	src := filepath.Join("testdata", "helper-17.24.3.dex")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skip(err)
	}
	blob := append([]byte{}, raw...)
	if patchEmbeddedDexJavaPackage(blob, "xcand") < 1 {
		t.Fatal("patch failed")
	}
	ok, detail := AndroidHelperDexAligned(blob, "xcand")
	if !ok {
		t.Fatal(detail)
	}
	// unpatched should fail
	ok2, detail2 := AndroidHelperDexAligned(raw, "xcand")
	if ok2 {
		t.Fatal("unpatched dex should not align:", detail2)
	}
}

func TestDexStringIdsOrder_XcandBreaksKxmwpSafe(t *testing.T) {
	src := filepath.Join("testdata", "helper-17.24.3.dex")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skip(err)
	}
	// Simulate product-strip pairs that hit the dex (same-length).
	apply := func(magic string) []byte {
		d := append([]byte(nil), raw...)
		pairs := [][2][]byte{
			{[]byte("frida-helper"), []byte(magic + "-helper")},
			{[]byte("re/frida"), []byte("re/" + magic)},
			{[]byte("frida-server"), []byte(magic + "-server")},
			{[]byte("frida-agent"), []byte(magic + "-agent")},
			{[]byte("frida-gadget"), []byte(magic + "-gadget")},
		}
		for _, p := range pairs {
			d = bytes.ReplaceAll(d, p[0], p[1])
		}
		return d
	}
	xcand := apply("xcand")
	ok, detail := dexStringIdsOrdered(xcand)
	if ok {
		t.Fatal("xcand rename should break string_ids order")
	}
	if !bytes.Contains([]byte(detail), []byte("xcand-helper")) {
		t.Fatalf("detail should mention xcand-helper: %s", detail)
	}
	kxmwp := apply("kxmwp")
	ok, detail = dexStringIdsOrdered(kxmwp)
	if !ok {
		t.Fatalf("kxmwp should keep order: %s", detail)
	}
}

func TestResortDexStringIds_XcandThenAligned(t *testing.T) {
	src := filepath.Join("testdata", "helper-17.24.3.dex")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skip(err)
	}
	d := append([]byte(nil), raw...)
	for _, p := range [][2][]byte{
		{[]byte("frida-helper"), []byte("xcand-helper")},
		{[]byte("re/frida"), []byte("re/xcand")},
	} {
		d = bytes.ReplaceAll(d, p[0], p[1])
	}
	ok, _ := dexStringIdsOrdered(d)
	if ok {
		t.Fatal("precondition: should be unordered")
	}
	changed, err := resortDexStringIds(d)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected resort")
	}
	ok, detail := dexStringIdsOrdered(d)
	if !ok {
		t.Fatal(detail)
	}
	fixDexChecksum(d)
	ok2, detail2 := AndroidHelperDexAligned(d, "xcand")
	if !ok2 {
		t.Fatal(detail2)
	}
	if !bytes.Contains(d, []byte("Lre/xcand/HelperBackend")) {
		t.Fatal("descriptor lost")
	}
	if bytes.Contains(d, []byte("/frida-helper-")) {
		t.Fatal("frida-helper path left behind")
	}
	if !bytes.Contains(d, []byte("/xcand-helper-")) {
		t.Fatal("missing xcand-helper path")
	}
}

func TestAndroidHelperDexAligned_RejectsOrderBroken(t *testing.T) {
	src := filepath.Join("testdata", "helper-17.24.3.dex")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skip(err)
	}
	d := append([]byte(nil), raw...)
	d = bytes.ReplaceAll(d, []byte("frida-helper"), []byte("xcand-helper"))
	d = bytes.ReplaceAll(d, []byte("re/frida"), []byte("re/xcand"))
	fixDexChecksum(d) // checksums OK but order broken
	ok, detail := AndroidHelperDexAligned(d, "xcand")
	if ok {
		t.Fatal("order-broken dex must not align")
	}
	if !bytes.Contains([]byte(detail), []byte("out-of-order")) {
		t.Fatalf("want out-of-order detail, got %s", detail)
	}
}

func TestPatchEmbeddedDex_XcandResortsRealHelper(t *testing.T) {
	src := filepath.Join("testdata", "helper-17.24.3.dex")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skip(err)
	}
	// Whole-file style: rename helper basename first (as PatchProductFile markers do),
	// leaving re/frida for patchEmbeddedDexJavaPackage + resort.
	blob := append([]byte(nil), raw...)
	blob = bytes.ReplaceAll(blob, []byte("frida-helper"), []byte("xcand-helper"))
	n := patchEmbeddedDexJavaPackage(blob, "xcand")
	if n < 1 {
		t.Fatalf("hits=%d", n)
	}
	ok, detail := AndroidHelperDexAligned(blob, "xcand")
	if !ok {
		t.Fatal(detail)
	}
	okOrder, d2 := dexStringIdsOrdered(blob)
	if !okOrder {
		t.Fatal(d2)
	}
}

func TestPatchProductFile_XcandRealHelperOrder(t *testing.T) {
	src := filepath.Join("testdata", "helper-17.24.3.dex")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skip(err)
	}
	blob := append(append([]byte("PAD"), raw...), make([]byte, MinArtifactBytes)...)
	dir := t.TempDir()
	p := filepath.Join(dir, "frida-server")
	if err := os.WriteFile(p, blob, 0755); err != nil {
		t.Fatal(err)
	}
	changed, err := PatchProductFile(p, "xcand")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changes")
	}
	out, _ := os.ReadFile(p)
	ok, detail := AndroidHelperDexAligned(out, "xcand")
	if !ok {
		t.Fatal(detail)
	}
}
