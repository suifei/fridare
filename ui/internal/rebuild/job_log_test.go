package rebuild

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJobFileLogger_WritesAndLatest(t *testing.T) {
	root := t.TempDir()
	l, err := NewJobFileLogger(root, "job-test-123")
	if err != nil {
		t.Fatal(err)
	}
	l.Log("hello %s", "world")
	l.LogBlock("docker", "line1\nline2")
	path := l.Path()
	l.Close("OK done")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "hello world") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "line1") || !strings.Contains(s, "docker") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "OK done") {
		t.Fatal(s)
	}
	latest := filepath.Join(root, "logs", "latest.log")
	ld, err := os.ReadFile(latest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ld), "hello world") {
		t.Fatal(string(ld))
	}
}

func TestLoggingRunner_CapturesOutput(t *testing.T) {
	root := t.TempDir()
	l, err := NewJobFileLogger(root, "job-x")
	if err != nil {
		t.Fatal(err)
	}
	inner := &captureRunner{onRun: func(ctx context.Context, env []string, name string, args ...string) (string, error) {
		return "stdout-from-docker\nerror bits", nil
	}}
	r := LoggingRunner{Inner: inner, Logger: l}
	out, err := r.Run(context.Background(), nil, "docker", "run", "x")
	if err != nil || !strings.Contains(out, "stdout-from-docker") {
		t.Fatalf("%v %q", err, out)
	}
	l.Close("")
	data, _ := os.ReadFile(l.Path())
	if !strings.Contains(string(data), "EXEC docker") {
		t.Fatal(string(data))
	}
	if !strings.Contains(string(data), "stdout-from-docker") {
		t.Fatal(string(data))
	}
}

func TestOrchestratorDryRun_WritesJobLogFile(t *testing.T) {
	tmp := t.TempDir()
	orch := NewOrchestrator(&fakeRunner{}, &StubAgent{})
	cfg := JobConfig{
		FridaVersion:  "17.16.4",
		TargetIDs:     []string{"android-arm64"},
		MagicName:     "abcde",
		ListenPort:    27042,
		WorkDir:       tmp,
		ArtifactDir:   filepath.Join(tmp, "out"),
		Proxy:         "http://127.0.0.1:7890",
		OpenAIBaseURL: RecommendedAPIBase,
		OpenAIAPIKey:  "sk",
		DryRun:        true,
	}
	if err := orch.RunSync(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	st := orch.State()
	if st.LogPath == "" {
		t.Fatal("expected LogPath")
	}
	if _, err := os.Stat(st.LogPath); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(st.LogPath)
	s := string(data)
	if !strings.Contains(s, "Fridare rebuild log") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "[probe]") && !strings.Contains(s, "probe") {
		// stages logged
		if !strings.Contains(s, "bootstrap") && !strings.Contains(s, "dry-run") {
			t.Fatalf("log missing stages: %s", s)
		}
	}
	// latest.log exists
	if st.LatestLog == "" {
		t.Fatal("latest")
	}
	if _, err := os.Stat(st.LatestLog); err != nil {
		t.Fatal(err)
	}
}
