package rebuild

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOrchestratorBootstrapOnly_DryRun(t *testing.T) {
	dir := t.TempDir()
	o := NewOrchestrator(nil, &StubAgent{})
	cfg := JobConfig{
		WorkDir:      dir,
		ArtifactDir:  filepath.Join(dir, "arts"),
		Proxy:        "http://127.0.0.1:9",
		DockerImage:  "fridare/frida-builder:latest",
		DockerMirror: DefaultDockerHubMirror,
		Mode:         JobModeBootstrap,
		ArchiveImage: false,
		DryRun:       true,
		MinDiskGB:    1,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := o.RunSync(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	st := o.State()
	if st.Stage != StageDone {
		t.Fatalf("stage %s", st.Stage)
	}
	// Should not have required version for bootstrap
	if st.Artifact == "" {
		// bootstrap may still set work paths; message should mention step 1
	}
	if st.Message == "" {
		t.Fatal("empty message")
	}
}

func TestJobConfigEffectiveMode(t *testing.T) {
	if (JobConfig{}).EffectiveMode() != JobModeFull {
		t.Fatal("default full")
	}
	if (JobConfig{Mode: JobModeBootstrap}).EffectiveMode() != JobModeBootstrap {
		t.Fatal("bootstrap")
	}
}
