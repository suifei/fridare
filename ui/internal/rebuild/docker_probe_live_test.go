package rebuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestLiveDockerProbe records real docker availability without failing when absent.
// Output is written by the test harness caller; here we assert honest detection.
func TestLiveDockerProbe(t *testing.T) {
	report := ProbeDeps(ProbeOptions{
		WorkDir:       os.TempDir(),
		Proxy:         "http://127.0.0.1:9", // non-empty to isolate docker result
		OpenAIBaseURL: RecommendedAPIBase,
		OpenAIAPIKey:  "test",
		MinDiskGB:     0.001, // tiny threshold so disk usually passes
	})

	_, lookErr := exec.LookPath("docker")
	if lookErr != nil {
		if report.Docker.Available {
			t.Fatal("LookPath failed but probe says docker available")
		}
		if report.Docker.Detail == "" {
			t.Fatal("expected detail when docker missing")
		}
		t.Logf("docker not on PATH: %s", report.Docker.Detail)
		return
	}

	// docker binary exists — Available depends on daemon
	if report.Docker.Detail == "" {
		t.Fatal("empty docker detail")
	}
	t.Logf("docker probe: available=%v detail=%s", report.Docker.Available, report.Docker.Detail)

	// Free disk should be a real number
	if report.FreeDiskGB <= 0 && report.Disk.Available {
		t.Fatalf("inconsistent disk: %+v", report.Disk)
	}
	t.Logf("disk free=%.2f GB available=%v", report.FreeDiskGB, report.Disk.Available)

	// Grok may or may not be present
	t.Logf("grok_build available=%v detail=%s", report.GrokBuild.Available, report.GrokBuild.Detail)
}

// TestDefaultPathsUnderWorkDir ensures artifact/source paths nest under workdir/rebuild.
func TestDefaultPathsUnderWorkDir(t *testing.T) {
	root := t.TempDir()
	arts := DefaultArtifactDir(root)
	src := DefaultSourceWorkDir(root)
	if arts != filepath.Join(root, "rebuild", "artifacts") {
		t.Fatalf("artifacts path: %s", arts)
	}
	if src != filepath.Join(root, "rebuild", "src") {
		t.Fatalf("src path: %s", src)
	}
	if err := os.MkdirAll(arts, 0755); err != nil {
		t.Fatal(err)
	}
}
