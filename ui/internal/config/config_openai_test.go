package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig_OpenAIRecommended(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.OpenAIBaseURL != "https://claudegpt.org/v1" {
		t.Fatalf("base url: %s", cfg.OpenAIBaseURL)
	}
	if cfg.RebuildMinDiskGB != 40 {
		t.Fatalf("disk: %v", cfg.RebuildMinDiskGB)
	}
	if cfg.RebuildDockerImage == "" {
		t.Fatal("docker image default")
	}
	if cfg.RebuildAgentUseGUIProxy {
		t.Fatal("OpenAI/agent proxy egress must default OFF (direct, no GUI proxy)")
	}
	help := OpenAIRecommendedHelp()
	for _, needle := range []string{
		"https://claudegpt.org/",
		"https://claudegpt.org/v1",
		"555354813",
	} {
		if !strings.Contains(help, needle) {
			t.Fatalf("help missing %q: %s", needle, help)
		}
	}
	// QR is shown as embedded image in Settings, not as a URL in help text
	if strings.Contains(help, "qq-group.png") {
		t.Fatal("help should not point to QR URL; image is in-app")
	}
}

func TestConfig_SaveLoad_OpenAIFields(t *testing.T) {
	// Isolate config path via temp HOME/APP config is OS-specific;
	// instead marshal/unmarshal the struct which Save uses.
	cfg := DefaultConfig()
	cfg.OpenAIBaseURL = "https://claudegpt.org/v1"
	cfg.OpenAIAPIKey = "sk-test-key"
	cfg.OpenAIModel = "gpt-test"
	cfg.Proxy = "http://127.0.0.1:7890"
	cfg.RebuildMinDiskGB = 50
	cfg.RebuildUseLocalGrok = true

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	// Ensure recommended strings survive JSON round-trip fields
	raw := string(data)
	if !strings.Contains(raw, "openai_base_url") || !strings.Contains(raw, "claudegpt.org/v1") {
		t.Fatal(raw)
	}
	if !strings.Contains(raw, "openai_api_key") {
		t.Fatal(raw)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	loaded.validate()
	if loaded.OpenAIBaseURL != "https://claudegpt.org/v1" {
		t.Fatal(loaded.OpenAIBaseURL)
	}
	if loaded.OpenAIAPIKey != "sk-test-key" {
		t.Fatal(loaded.OpenAIAPIKey)
	}
	if loaded.RebuildMinDiskGB != 50 {
		t.Fatal(loaded.RebuildMinDiskGB)
	}

	// validate fills empty base URL
	empty := &Config{}
	empty.validate()
	if empty.OpenAIBaseURL != "https://claudegpt.org/v1" {
		t.Fatal(empty.OpenAIBaseURL)
	}
	if empty.RebuildMinDiskGB != 40 {
		t.Fatal(empty.RebuildMinDiskGB)
	}
}

func TestApplyMissingFieldDefaults_AgentProxyOff(t *testing.T) {
	rawJSON := []byte(`{"app_version":"4.0.3","openai_base_url":"https://claudegpt.org/v1","openai_api_key":"sk-x"}`)
	var cfg Config
	if err := json.Unmarshal(rawJSON, &cfg); err != nil {
		t.Fatal(err)
	}
	// json.Unmarshal leaves bool false anyway; force true then apply missing-key migration
	cfg.RebuildAgentUseGUIProxy = true
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON, &rawMap); err != nil {
		t.Fatal(err)
	}
	if _, ok := rawMap["rebuild_agent_use_gui_proxy"]; ok {
		t.Fatal("fixture must omit field")
	}
	ApplyMissingFieldDefaults(&cfg, rawMap)
	if cfg.RebuildAgentUseGUIProxy {
		t.Fatal("missing rebuild_agent_use_gui_proxy must become false")
	}
	// Present true must be preserved
	raw2 := []byte(`{"rebuild_agent_use_gui_proxy":true}`)
	var cfg2 Config
	_ = json.Unmarshal(raw2, &cfg2)
	var m2 map[string]json.RawMessage
	_ = json.Unmarshal(raw2, &m2)
	ApplyMissingFieldDefaults(&cfg2, m2)
	if !cfg2.RebuildAgentUseGUIProxy {
		t.Fatal("explicit true must stay")
	}
}

func TestConfig_WriteTempFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := DefaultConfig()
	cfg.OpenAIAPIKey = "sk-round"
	cfg.Proxy = "http://proxy:1"
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Config
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.OpenAIAPIKey != "sk-round" || got.OpenAIBaseURL != "https://claudegpt.org/v1" {
		t.Fatalf("%+v", got)
	}
}
