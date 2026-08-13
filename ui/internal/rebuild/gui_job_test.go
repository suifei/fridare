package rebuild

import "testing"

func TestEffectiveCloneRef_ProductLabel(t *testing.T) {
	if got := EffectiveCloneRef(JobConfig{FridaVersion: "17.17.1"}); got != "17.17.0" {
		t.Fatalf("17.17.1 must clone official 17.17.0, got %s", got)
	}
	if got := EffectiveCloneRef(JobConfig{FridaVersion: "16.7.19"}); got != "16.7.19" {
		t.Fatalf("16.7.19 is official: %s", got)
	}
	if got := EffectiveCloneRef(JobConfig{FridaVersion: "17.17.1", CloneRef: "deadbeef"}); got != "deadbeef" {
		t.Fatalf("explicit CloneRef wins: %s", got)
	}
}

func TestEffectivePipVersion_ProductLabel(t *testing.T) {
	if got := EffectivePipVersion(JobConfig{FridaVersion: "17.17.1"}); got != "17.17.0" {
		t.Fatalf("17.17.1 wheels must be frida==17.17.0, got %s", got)
	}
	if got := EffectivePipVersion(JobConfig{FridaVersion: "16.7.19"}); got != "16.7.19" {
		t.Fatalf("16.7.19 wheels stay official: %s", got)
	}
	// CloneRef must not become a pip version (it can be a commit).
	if got := EffectivePipVersion(JobConfig{FridaVersion: "17.17.1", CloneRef: "deadbeef"}); got != "17.17.0" {
		t.Fatalf("CloneRef must not override pip version: %s", got)
	}
}

func TestApplyGUIRebuildStealthDefaults(t *testing.T) {
	cfg := JobConfig{FridaVersion: "16.7.19", MagicName: "kxmwp"}
	ApplyGUIRebuildStealthDefaults(&cfg)
	if cfg.DirectionProfile != "deep" || !cfg.StripSymbols || cfg.DisableJunk || cfg.DisableStealthMarkers {
		t.Fatalf("%+v", cfg)
	}
	if cfg.ListenPort != OfficialListenPort {
		t.Fatalf("port %d", cfg.ListenPort)
	}
	if !ShouldStripProductSymbols(cfg) {
		t.Fatal("GUI deep defaults must strip")
	}
}
