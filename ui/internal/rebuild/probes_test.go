package rebuild

import (
	"errors"
	"strings"
	"testing"
)

func TestProxyRequiredForSourceMod(t *testing.T) {
	if err := ProxyRequiredForSourceMod(""); err == nil {
		t.Fatal("expected error when proxy empty")
	}
	if err := ProxyRequiredForSourceMod("   "); err == nil {
		t.Fatal("expected error when proxy whitespace")
	}
	if err := ProxyRequiredForSourceMod("http://127.0.0.1:7890"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProbeDeps_MissingAll(t *testing.T) {
	report := ProbeDeps(ProbeOptions{
		WorkDir:       t.TempDir(),
		Proxy:         "",
		OpenAIBaseURL: "",
		OpenAIAPIKey:  "",
		MinDiskGB:     1,
		LookPath: func(file string) (string, error) {
			return "", errors.New("not found")
		},
		RunCommand: func(name string, args ...string) (string, error) {
			return "", errors.New("no")
		},
		FreeDiskGB: func(path string) (float64, error) {
			return 0.5, nil
		},
	})
	if report.Ready {
		t.Fatal("expected not ready")
	}
	if report.Docker.Available {
		t.Fatal("docker should be unavailable")
	}
	if report.Proxy.Available {
		t.Fatal("proxy should be unavailable")
	}
	if report.OpenAI.Available {
		t.Fatal("openai should be unavailable")
	}
	if report.Disk.Available {
		t.Fatal("disk should fail min threshold")
	}
	if len(report.Messages) == 0 {
		t.Fatal("expected messages")
	}
}

func TestProbeDeps_AllGood(t *testing.T) {
	report := ProbeDeps(ProbeOptions{
		WorkDir:       t.TempDir(),
		Proxy:         "http://proxy.local:8080",
		OpenAIBaseURL: RecommendedAPIBase,
		OpenAIAPIKey:  "sk-test",
		MinDiskGB:     10,
		LookPath: func(file string) (string, error) {
			switch file {
			case "docker", "git", "grok", "grok-build":
				return "/usr/bin/" + file, nil
			default:
				return "", errors.New("no")
			}
		},
		RunCommand: func(name string, args ...string) (string, error) {
			if strings.Contains(name, "docker") || (len(args) > 0 && args[0] == "info") {
				return "29.0.0", nil
			}
			return "git version 2.40.0", nil
		},
		FreeDiskGB: func(path string) (float64, error) {
			return 100, nil
		},
	})
	if !report.Ready {
		t.Fatalf("expected ready, messages=%v", report.Messages)
	}
	if !report.Docker.Available || !report.GrokBuild.Available || !report.Git.Available {
		t.Fatalf("expected docker/grok/git available: %+v", report)
	}
	if report.FreeDiskGB != 100 {
		t.Fatalf("free disk: %v", report.FreeDiskGB)
	}
	if !strings.Contains(report.OpenAI.Detail, RecommendedAPIBase) && !strings.Contains(report.OpenAI.Detail, "claudegpt") {
		// Detail contains base URL
		if !strings.Contains(report.OpenAI.Detail, "端点") {
			t.Fatalf("openai detail: %s", report.OpenAI.Detail)
		}
	}
}

func TestCanStartSourceJob(t *testing.T) {
	ok := DepsReport{Ready: true}
	if err := CanStartSourceJob(ok, "http://p"); err != nil {
		t.Fatal(err)
	}
	if err := CanStartSourceJob(ok, ""); err == nil {
		t.Fatal("proxy required")
	}
	bad := DepsReport{Ready: false, Messages: []string{"docker missing"}}
	if err := CanStartSourceJob(bad, "http://p"); err == nil {
		t.Fatal("expected deps error")
	}
}

func TestResolveGrokBinary(t *testing.T) {
	_, ok := ResolveGrokBinary(func(s string) (string, error) {
		return "", errors.New("no")
	})
	if ok {
		t.Fatal("expected not ok")
	}
	p, ok := ResolveGrokBinary(func(s string) (string, error) {
		if s == "grok" {
			return "C:\\tools\\grok.exe", nil
		}
		return "", errors.New("no")
	})
	if !ok || p != "C:\\tools\\grok.exe" {
		t.Fatalf("got %q %v", p, ok)
	}
}

func TestRecommendedConstantsPresent(t *testing.T) {
	if RecommendedSiteURL != "https://claudegpt.org/" {
		t.Fatal(RecommendedSiteURL)
	}
	if RecommendedAPIBase != "https://claudegpt.org/v1" {
		t.Fatal(RecommendedAPIBase)
	}
	if RecommendedQQGroup != "555354813" {
		t.Fatal(RecommendedQQGroup)
	}
	if !strings.Contains(RecommendedQRCodeURL, "qq-group.png") {
		t.Fatal(RecommendedQRCodeURL)
	}
	if !strings.Contains(RecommendedEndpointHelp, "claudegpt.org") {
		t.Fatal(RecommendedEndpointHelp)
	}
	if !strings.Contains(DualTrackHelpMarkdown, "静态") {
		t.Fatal("dual track help")
	}
}
