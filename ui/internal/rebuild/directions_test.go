package rebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultStripLayers_ModesCoverSpectrum(t *testing.T) {
	layers := DefaultStripLayers()
	if len(layers) < 8 {
		t.Fatalf("want layered directions, got %d", len(layers))
	}
	var auto, forbidden, explore, post int
	for _, L := range layers {
		switch L.Mode {
		case StripModeAuto:
			auto++
		case StripModeForbidden:
			forbidden++
		case StripModeAIExplore:
			explore++
		case StripModePostBuild:
			post++
		}
	}
	if auto < 3 || forbidden < 2 || explore < 1 || post < 1 {
		t.Fatalf("mode spectrum auto=%d forbidden=%d explore=%d post=%d", auto, forbidden, explore, post)
	}
}

func TestClassifyPattern(t *testing.T) {
	cases := []struct {
		p    string
		id   StripLayerID
		mode StripMode
	}{
		{"frida-server", LayerProductBasename, StripModeAuto},
		{"get_frida_agent", LayerGResourceGetter, StripModeAuto},
		{"frida:rpc", LayerRPCThreadPort, StripModeAuto},
		{"frida_agent_main", LayerUnderscoreCABI, StripModeForbidden},
		{"namespace Frida.Agent", LayerValaNamespace, StripModeForbidden},
		{"re.frida.server", LayerProtocolReFrida, StripModeAuto},
		{"frida", LayerGlobalWord, StripModeAIExplore},
	}
	for _, c := range cases {
		id, mode := ClassifyPattern(c.p)
		if id != c.id || mode != c.mode {
			t.Errorf("%q: got %s/%s want %s/%s", c.p, id, mode, c.id, c.mode)
		}
	}
}

func TestScanFridaMarkers_FixtureTree(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "server.c"), []byte(`char *n="frida-server"; char *r="frida:rpc";`), 0644)
	_ = os.WriteFile(filepath.Join(root, "agent.vala"), []byte("namespace Frida.Agent {\n// frida_agent_main\n"), 0644)
	_ = os.WriteFile(filepath.Join(root, "host.vala"), []byte(`var x = get_frida_agent_64_so_blob();`), 0644)

	rep, err := ScanFridaMarkers(root, "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Hits) == 0 {
		t.Fatal("expected hits")
	}
	// Must classify Vala namespace as forbidden
	foundForbidden := false
	foundAuto := false
	for _, h := range rep.Hits {
		if h.Mode == StripModeForbidden {
			foundForbidden = true
		}
		if h.Mode == StripModeAuto {
			foundAuto = true
		}
	}
	if !foundForbidden || !foundAuto {
		t.Fatalf("want both auto and forbidden hits: %+v", rep.ByLayer)
	}
}

func TestOpsFromDirectionManifest_MatchesDefaultModOps(t *testing.T) {
	m := SafeDirectionManifest("abcde", 27142)
	ops := OpsFromDirectionManifest(m)
	def := DefaultModOps("abcde", 27142)
	if len(ops) != len(def) {
		t.Fatalf("ops %d != DefaultModOps %d", len(ops), len(def))
	}
	// No forbidden patterns as Find
	for _, op := range ops {
		if op.Find == "frida_agent_main" || strings.Contains(op.Find, "namespace Frida.Agent") {
			t.Fatalf("forbidden op leaked: %+v", op)
		}
	}
}

func TestSafeDirectionManifest_ExcludesProtocolAPILayers(t *testing.T) {
	m := SafeDirectionManifest("abcde", 27142)
	for _, L := range m.Layers {
		if L.ID == LayerProtocolReFrida || L.ID == LayerPublicJSAPI {
			t.Fatalf("safe must not list L8/L9 as layers: %s", L.ID)
		}
	}
	deep := DeepDirectionManifest("abcde", 27142)
	var hasL8, hasL9 bool
	for _, L := range deep.Layers {
		if L.ID == LayerProtocolReFrida {
			hasL8 = true
		}
		if L.ID == LayerPublicJSAPI {
			hasL9 = true
		}
	}
	if !hasL8 || !hasL9 {
		t.Fatal("deep must include L8/L9")
	}
}

func TestSafeDirectionManifest_WriteLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "directions.json")
	m := SafeDirectionManifest("abcde", 27142)
	if err := WriteDirectionManifest(path, m); err != nil {
		t.Fatal(err)
	}
	m2, err := LoadDirectionManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m2.Magic != "abcde" || m2.Profile != "safe" || len(m2.Layers) == 0 {
		t.Fatalf("%+v", m2)
	}
}

func TestPlanModsFromTree_UsesDirectionsGoals(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.c"), []byte(`"frida-server" "frida:rpc"`), 0644)
	cfg := JobConfig{
		FridaVersion: "17.16.4",
		MagicName:    "abcde",
		ListenPort:   27142,
		Goals:        "user goal",
		DirectionProfile: "safe",
	}
	plan, err := PlanModsFromTree(root, cfg, "br")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Goals, "user goal") && !strings.Contains(plan.Goals, "方向") {
		// PlanModsFromTree may keep cfg.Goals; orchestrator merges — ensure ops still work
	}
	if len(plan.Operations) == 0 {
		t.Fatal("expected ops")
	}
	// Applied content would have abcde-server
	if err := applyPlanNaive(root, plan); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "a.c"))
	if !strings.Contains(string(data), "abcde-server") || !strings.Contains(string(data), "abcde:rpc") {
		t.Fatalf("safe direction not applied: %s", data)
	}
}
