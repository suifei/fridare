package rebuild

import (
	"strings"
	"testing"
)

func TestResolveDockerImage_Mirror1ms(t *testing.T) {
	got := ResolveDockerImage("ubuntu:22.04", "docker.1ms.run")
	want := "docker.1ms.run/library/ubuntu:22.04"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// redis example from user
	got = ResolveDockerImage("redis:7-alpine", "docker.1ms.run")
	if got != "docker.1ms.run/library/redis:7-alpine" {
		t.Fatal(got)
	}
	got = ResolveDockerImage("library/redis:7-alpine", "docker.1ms.run")
	if got != "docker.1ms.run/library/redis:7-alpine" {
		t.Fatal(got)
	}
}

func TestResolveDockerImage_Idempotent(t *testing.T) {
	in := "docker.1ms.run/library/ubuntu:22.04"
	if ResolveDockerImage(in, "docker.1ms.run") != in {
		t.Fatal("should not double-prefix")
	}
}

func TestResolveDockerImage_NoMirror(t *testing.T) {
	if ResolveDockerImage("ubuntu:22.04", "") != "ubuntu:22.04" {
		t.Fatal("no mirror should pass through")
	}
}

func TestResolveDockerImage_ForeignRegistry(t *testing.T) {
	in := "ghcr.io/foo/bar:latest"
	if ResolveDockerImage(in, "docker.1ms.run") != in {
		t.Fatal("should not prefix ghcr")
	}
}

func TestDockerfileSkeletonForMirror(t *testing.T) {
	df := DockerfileSkeletonForMirror("docker.1ms.run")
	if !strings.Contains(df, "FROM docker.1ms.run/library/ubuntu:22.04") {
		t.Fatal(df)
	}
	// Heavy toolchain baked at image build (not per compile job)
	if !strings.Contains(df, "ANDROID_NDK_ROOT") || !strings.Contains(df, "android-ndk-r29-linux.zip") {
		t.Fatal("Dockerfile must preinstall NDK r29")
	}
	if !strings.Contains(df, "node-v20") && !strings.Contains(df, "nodejs.org") {
		t.Fatal("Dockerfile must preinstall Node.js 20 (not apt node 12)")
	}
	if !strings.Contains(df, "go1.24") && !strings.Contains(df, "go.dev/dl") {
		t.Fatal("Dockerfile must preinstall Go 1.24")
	}
	// must NOT install outdated apt nodejs as the primary runtime
	if strings.Contains(df, "nodejs npm \\") || strings.Contains(df, "nodejs npm\n") {
		t.Fatal("should not apt-install nodejs (too old on Ubuntu 22.04)")
	}
	if !strings.Contains(df, BuilderImageFeatureTag) {
		t.Fatal("missing feature tag")
	}
	if !strings.Contains(df, "build-essential") || !strings.Contains(df, "git") {
		t.Fatal("missing apt toolchain")
	}
	df0 := DockerfileSkeletonForMirror("")
	if !strings.Contains(df0, "FROM ubuntu:22.04") {
		t.Fatal(df0)
	}
}

func TestImageHasBuilderFeaturesShell(t *testing.T) {
	sh := ImageHasBuilderFeaturesShell()
	if !strings.Contains(sh, BuilderImageFeatureTag) || !strings.Contains(sh, BuilderImageNDKPath) {
		t.Fatal(sh)
	}
}

func TestDockerPullEnv_StripsProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy:1")
	t.Setenv("HTTPS_PROXY", "http://proxy:1")
	env := DockerPullEnv(true)
	joined := strings.Join(env, "\n")
	// explicit empty overrides should be present; no non-empty proxy values
	for _, e := range env {
		if strings.HasPrefix(e, "HTTP_PROXY=") && len(e) > len("HTTP_PROXY=") {
			// allow empty assignment HTTP_PROXY=
			if e != "HTTP_PROXY=" {
				t.Fatalf("proxy not stripped: %s", e)
			}
		}
	}
	_ = joined
	env2 := DockerPullEnv(false)
	// may still contain system proxy — just ensure function returns
	if len(env2) == 0 {
		t.Fatal("empty env")
	}
}
