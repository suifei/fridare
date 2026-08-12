package rebuild

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuilderStableImageTag(t *testing.T) {
	got := BuilderStableImageTag("fridare/frida-builder:latest")
	want := "fridare/frida-builder:" + BuilderImageFeatureTag
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	got2 := BuilderStableImageTag("fridare/frida-builder")
	if got2 != want {
		t.Fatal(got2)
	}
}

func TestDefaultDockerHubMirror(t *testing.T) {
	if DefaultDockerHubMirror != "docker.1ms.run" {
		t.Fatal(DefaultDockerHubMirror)
	}
}

func TestImageArchiveDir(t *testing.T) {
	d := ImageArchiveDir(filepath.Join("tmp", "work"))
	if !strings.HasSuffix(d, "images") {
		t.Fatal(d)
	}
}
