package rebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanModsFromTree_DisableStealthMarkers(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "subprojects", "frida-core", "src", "linux", "linjector-glue.c")
	_ = os.MkdirAll(filepath.Dir(src), 0755)
	_ = os.WriteFile(src, []byte(`const char *inj = "linjector";`+"\n"), 0644)
	cfg := JobConfig{
		MagicName: "abcde", DirectionProfile: "deep", FridaVersion: "17.17.0",
		DisableStealthMarkers: true,
	}
	plan, err := PlanModsFromTree(root, cfg, "br")
	if err != nil {
		t.Fatal(err)
	}
	if err := applyPlanNaive(root, plan); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(src)
	if !strings.Contains(string(got), `"linjector"`) {
		t.Fatalf("markers disabled must leave quoted linjector: %s", got)
	}
}

func TestStealthSeedHex_Stable(t *testing.T) {
	a := StealthSeedHex("abcde", "build1")
	b := StealthSeedHex("abcde", "build1")
	c := StealthSeedHex("abcde", "build2")
	if a != b || a == c || len(a) != 16 {
		t.Fatalf("a=%s b=%s c=%s", a, b, c)
	}
}

func TestAgentDiskPrefix_RandomOptional(t *testing.T) {
	if AgentDiskPrefix("kxmwp", "1", false) != "kxmwp" {
		t.Fatal("non-random is magic")
	}
	r1 := AgentDiskPrefix("kxmwp", "1", true)
	r2 := AgentDiskPrefix("kxmwp", "1", true)
	r3 := AgentDiskPrefix("kxmwp", "2", true)
	if len(r1) != 5 || r1 != r2 || r1 == r3 {
		t.Fatalf("r1=%s r2=%s r3=%s", r1, r2, r3)
	}
	for _, c := range r1 {
		if c < 'a' || c > 'z' {
			t.Fatalf("not a-z: %s", r1)
		}
	}
}

func TestStealthBehaviorOps_ApplyFixture_SkipsVendorAndRandomOff(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "subprojects", "frida-core", "src", "linux", "linjector-glue.c")
	vendor := filepath.Join(root, "subprojects", "brotli", "table.c")
	_ = os.MkdirAll(filepath.Dir(src), 0755)
	_ = os.MkdirAll(filepath.Dir(vendor), 0755)
	body := strings.Join([]string{
		`const char *inj = "linjector";`,
		`const char *sock = "unix:frida";`,
		`const char *maps = "/frida-";`,
		`const char *sel = "u:object_r:frida_file";`,
		`const char *mf = "memfd:frida";`,
		`const char *mfd = "frida_memfd";`,
		`static void frida_file_get_impl(void);`,
		`const char *dump = "frida-agent-64.so";`,
		`files('linjector.vala');`,
	}, "\n")
	_ = os.WriteFile(src, []byte(body+"\n"), 0644)
	_ = os.WriteFile(vendor, []byte(`static int k = 27042; const char *inj = "linjector";`+"\n"), 0644)

	cfg := JobConfig{MagicName: "abcde", DirectionProfile: "deep", RandomAgentPrefix: false}
	plan := &ModPlan{Operations: StealthBehaviorOps(cfg)}
	if err := applyPlanNaive(root, plan); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(src)
	s := string(got)
	if !strings.Contains(s, `"abcdector"`) || strings.Contains(s, `"linjector"`) {
		t.Fatalf("quoted linjector: %s", s)
	}
	if !strings.Contains(s, "files('linjector.vala')") {
		t.Fatalf("meson linjector.vala must stay: %s", s)
	}
	if !strings.Contains(s, "unix:abcde") || !strings.Contains(s, `"/abcde-"`) {
		t.Fatalf("socket/maps: %s", s)
	}
	if !strings.Contains(s, "u:object_r:abcde") || !strings.Contains(s, `"abcde_memfd"`) {
		t.Fatalf("selinux: %s", s)
	}
	if !strings.Contains(s, "frida_file_get_impl") {
		t.Fatalf("must not smash frida_file identifiers: %s", s)
	}
	if !strings.Contains(s, "memfd:abcde") {
		t.Fatalf("memfd: %s", s)
	}
	if !strings.Contains(s, "frida-agent-64.so") {
		t.Fatalf("random prefix must be off: %s", s)
	}
	v, _ := os.ReadFile(vendor)
	if !strings.Contains(string(v), "linjector") || !strings.Contains(string(v), "27042") {
		t.Fatalf("vendor must stay: %s", v)
	}
}

func TestPlanModsFromTree_RandomAgentPrefix_AfterDefaultModOps(t *testing.T) {
	// Real shipped path: PlanModsFromTree includes DefaultModOps then applyPlanNaive.
	// Random prefix must still win (not stay as {magic}-agent-).
	root := t.TempDir()
	src := filepath.Join(root, "subprojects", "frida-core", "lib", "agent", "agent-glue.c")
	_ = os.MkdirAll(filepath.Dir(src), 0755)
	_ = os.WriteFile(src, []byte("char *dump = \"frida-agent-64.so\";\nvoid load_frida_agent(void);\n"), 0644)
	cfg := JobConfig{
		MagicName: "abcde", BuildID: "job9", RandomAgentPrefix: true,
		DirectionProfile: "deep", FridaVersion: "17.17.0",
	}
	plan, err := PlanModsFromTree(root, cfg, "br")
	if err != nil {
		t.Fatal(err)
	}
	if err := applyPlanNaive(root, plan); err != nil {
		t.Fatal(err)
	}
	pfx := AgentDiskPrefix("abcde", "job9", true)
	got, _ := os.ReadFile(src)
	s := string(got)
	if !strings.Contains(s, pfx+"-agent-64.so") {
		t.Fatalf("dump prefix not applied via PlanModsFromTree: pfx=%s body=%s", pfx, s)
	}
	if strings.Contains(s, "frida-agent-64") {
		t.Fatalf("stock dump name remains: %s", s)
	}
	if strings.Contains(s, "abcde-agent-64") {
		t.Fatalf("stuck at magic-agent dump name (random op ran too late): %s", s)
	}
}

func TestPlanModsFromTree_RandomAgentPrefix_PreservesMesonAssets(t *testing.T) {
	// meson asset stems stay magic; only dump SO names get the random prefix.
	root := t.TempDir()
	meson := filepath.Join(root, "subprojects", "frida-core", "lib", "agent", "meson.build")
	ver := filepath.Join(root, "subprojects", "frida-core", "lib", "agent", "frida-agent-android.version")
	src := filepath.Join(root, "subprojects", "frida-core", "lib", "agent", "agent-glue.c")
	_ = os.MkdirAll(filepath.Dir(src), 0755)
	_ = os.WriteFile(meson, []byte("files('frida-agent-android.version')\n"), 0644)
	_ = os.WriteFile(ver, []byte("17.17.0\n"), 0644)
	_ = os.WriteFile(src, []byte("char *dump = \"frida-agent-64.so\";\n"), 0644)

	cfg := JobConfig{
		MagicName: "abcde", BuildID: "job9", RandomAgentPrefix: true,
		DirectionProfile: "deep", FridaVersion: "17.17.0",
	}
	plan, err := PlanModsFromTree(root, cfg, "br")
	if err != nil {
		t.Fatal(err)
	}
	if err := applyPlanNaive(root, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := RenameMagicAssetFiles(root, cfg.MagicName); err != nil {
		t.Fatal(err)
	}

	pfx := AgentDiskPrefix("abcde", "job9", true)
	mb, _ := os.ReadFile(filepath.Join(root, "subprojects", "frida-core", "lib", "agent", "meson.build"))
	ms := string(mb)
	if !strings.Contains(ms, "abcde-agent-android.version") {
		t.Fatalf("meson must keep magic asset stem: %s", ms)
	}
	if strings.Contains(ms, pfx+"-agent-android") {
		t.Fatalf("random prefix must not rewrite meson .version: %s", ms)
	}
	magicVer := filepath.Join(root, "subprojects", "frida-core", "lib", "agent", "abcde-agent-android.version")
	if _, err := os.Stat(magicVer); err != nil {
		t.Fatalf("on-disk asset must be magic-named (meson File exists): %v", err)
	}
	randVer := filepath.Join(root, "subprojects", "frida-core", "lib", "agent", pfx+"-agent-android.version")
	if _, err := os.Stat(randVer); err == nil {
		t.Fatal("must not rename .version to random prefix")
	}
	got, _ := os.ReadFile(src)
	if !strings.Contains(string(got), pfx+"-agent-64.so") {
		t.Fatalf("dump SO not randomized: %s", got)
	}
}

func TestStealthBehaviorOps_RandomAgentPrefix(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "subprojects", "frida-core", "lib", "agent", "agent-glue.c")
	_ = os.MkdirAll(filepath.Dir(src), 0755)
	_ = os.WriteFile(src, []byte(`char *p = "frida-agent-64.so";`+"\n"), 0644)
	cfg := JobConfig{MagicName: "abcde", BuildID: "job9", RandomAgentPrefix: true}
	pfx := AgentDiskPrefix("abcde", "job9", true)
	if err := applyPlanNaive(root, &ModPlan{Operations: StealthBehaviorOps(cfg)}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(src)
	if !strings.Contains(string(got), pfx+"-agent-64.so") {
		t.Fatalf("want %s-agent in %s", pfx, got)
	}
	if strings.Contains(string(got), "frida-agent-64") {
		t.Fatalf("old dump name remains: %s", got)
	}
}
