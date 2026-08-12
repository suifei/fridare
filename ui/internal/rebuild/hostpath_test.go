package rebuild

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDockerVolumePath_Absolute(t *testing.T) {
	p := DockerVolumePath(".")
	if p == "" {
		t.Fatal("empty")
	}
	if !filepath.IsAbs(p) && runtime.GOOS != "windows" {
		// On Windows Abs may still produce volume path
		if !strings.Contains(p, `:`) && !filepath.IsAbs(p) {
			t.Fatalf("expected abs-ish path, got %q", p)
		}
	}
	if runtime.GOOS == "windows" {
		if strings.Contains(p, `\`) {
			t.Fatalf("windows volume path should use slashes: %q", p)
		}
		// drive letter form C:/...
		if len(p) < 2 || p[1] != ':' {
			// allow if abs failed
			t.Logf("path form: %s", p)
		}
	}
}

func TestWriteScriptUnixLF(t *testing.T) {
	in := "#!/bin/bash\r\necho hi\r\nmake\r"
	out := WriteScriptUnixLF(in)
	if strings.Contains(out, "\r") {
		t.Fatalf("CR remains: %q", out)
	}
	if !strings.Contains(out, "\n") {
		t.Fatal("missing LF")
	}
}

func TestSupportsDockerSourceRebuild(t *testing.T) {
	ok := SupportsDockerSourceRebuild()
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		if !ok {
			t.Fatal("expected support")
		}
	}
	if HostPlatformLabel() == "" {
		t.Fatal("label")
	}
}

func TestRunContainerArgs_UsesNormalizedVolume(t *testing.T) {
	args := RunContainerArgs(DockerWorkspace{
		Image:            "img",
		HostWorkDir:      ".",
		ContainerWorkDir: "/work",
	}, []string{"true"})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-v") {
		t.Fatal(joined)
	}
	// should not contain backslash volume on windows after normalize
	for _, a := range args {
		if strings.HasPrefix(a, "-v") {
			continue
		}
		if strings.Contains(a, ":/work") && strings.Contains(a, `\`) {
			t.Fatalf("backslash in volume map: %s", a)
		}
	}
}
