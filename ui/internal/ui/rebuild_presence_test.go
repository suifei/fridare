package ui

import (
	"fridare-gui/internal/config"
	"fridare-gui/internal/rebuild"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestShippedSourceContainsOpenAIRecommendedStrings asserts the shipped UI/settings
// source embeds the recommended endpoint guidance (static presence for GUI environments
// that cannot open a display).
func TestShippedSourceContainsOpenAIRecommendedStrings(t *testing.T) {
	// Locate package source next to this test file
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(file)
	checks := []struct {
		path    string
		needles []string
	}{
		{
			path: filepath.Join(dir, "tabs.go"),
			needles: []string{
				"https://claudegpt.org/v1",
				"https://claudegpt.org/",
				"555354813",
				"QQGroupQR",
				"OpenAI",
				"源码重编译",
			},
		},
		{
			path: filepath.Join(dir, "rebuild_tab.go"),
			needles: []string{
				"源码级魔改",
				"检查依赖",
				"一键深度定制",
				"基础镜像",
				"魔改开发",
				"depth=1",
				"GUI 代理",
				"默认不走",
				"useGUIProxyCheck",
				"jobLog",
				"agentLog",
				"DefaultDockerHubMirror",
				"JobModeBootstrap",
				"JobModeDevelop",
				"历史产物库",
				"refreshCatalog",
				"deep",
				"/re/frida/",
				"PROTOCOL-SYNC",
			},
		},
		{
			path: filepath.Join(dir, "main_window.go"),
			needles: []string{
				"源码重编译",
				"RebuildTab",
			},
		},
	}
	for _, c := range checks {
		data, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatalf("read %s: %v", c.path, err)
		}
		body := string(data)
		for _, n := range c.needles {
			if !strings.Contains(body, n) {
				t.Errorf("%s missing %q", filepath.Base(c.path), n)
			}
		}
	}
}

func TestDualTrackDocExists(t *testing.T) {
	// ui/internal/ui -> repo root is ../../../
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	doc := filepath.Join(root, "docs", "dual-track.md")
	data, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, n := range []string{"静态二进制补丁", "源码", "Docker", "hexreplace", "phantom", "strongR"} {
		if !strings.Contains(s, n) {
			t.Errorf("dual-track.md missing %q", n)
		}
	}
}

func TestDefaultConfigMatchesRecommendedAPI(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.OpenAIBaseURL != rebuild.RecommendedAPIBase {
		t.Fatalf("%s vs %s", cfg.OpenAIBaseURL, rebuild.RecommendedAPIBase)
	}
	help := config.OpenAIRecommendedHelp()
	if !strings.Contains(help, rebuild.RecommendedQQGroup) {
		t.Fatal(help)
	}
}

func TestHelpSectionsIncludeDualTrack(t *testing.T) {
	ht := &HelpTab{}
	ht.setupHelpData()
	found := false
	for _, s := range ht.helpSections {
		if strings.Contains(s.Title, "双技术路线") {
			found = true
			if !strings.Contains(s.Content, "claudegpt.org") {
				t.Error("help missing recommended site")
			}
			if !strings.Contains(s.Content, "静态") {
				t.Error("help missing static track")
			}
		}
	}
	if !found {
		t.Fatal("dual track help section missing")
	}
}
