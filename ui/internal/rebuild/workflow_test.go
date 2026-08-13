package rebuild

import (
	"strings"
	"testing"
	"time"
)

func TestShallowCloneArgs(t *testing.T) {
	args, err := ShallowCloneArgs(CloneSpec{Version: "16.5.9", DestDir: "frida"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--depth=1") {
		t.Fatalf("missing depth=1: %s", joined)
	}
	if !strings.Contains(joined, "--branch") || !strings.Contains(joined, "16.5.9") {
		t.Fatalf("missing branch/version: %s", joined)
	}
	if !strings.Contains(joined, FridaGitURL) {
		t.Fatalf("missing repo: %s", joined)
	}
	// order: git clone --depth=1 --branch 16.5.9 ...
	if args[0] != "git" || args[1] != "clone" {
		t.Fatalf("prefix: %v", args)
	}
	foundDepth := false
	for _, a := range args {
		if a == "--depth=1" {
			foundDepth = true
		}
	}
	if !foundDepth {
		t.Fatal("depth flag")
	}
}

func TestShallowCloneArgs_RejectEmpty(t *testing.T) {
	_, err := ShallowCloneArgs(CloneSpec{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestShallowCloneArgs_RejectUnsafeVersion(t *testing.T) {
	_, err := ShallowCloneArgs(CloneSpec{Version: "16.5.9; rm -rf /"})
	if err == nil {
		t.Fatal("expected reject")
	}
}

func TestManagementBranchName(t *testing.T) {
	ts := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	name := ManagementBranchName(ts)
	if !strings.HasPrefix(name, "fridare/mod-") {
		t.Fatal(name)
	}
	if !strings.Contains(name, "20260811") {
		t.Fatal(name)
	}
	args := CreateBranchArgs(name)
	if strings.Join(args, " ") != "git checkout -b "+name {
		t.Fatalf("%v", args)
	}
}

func TestBuildPipelineScript_IncludesCloneBranchTargets(t *testing.T) {
	script, err := BuildPipelineScript(PipelineScriptOptions{
		Clone:     CloneSpec{Version: "17.16.4"},
		Branch:    "fridare/mod-test",
		SourceDir: "frida",
		TargetIDs: []string{"android-arm64", "windows-x86_64"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "--depth=1") {
		t.Fatal("depth")
	}
	if !strings.Contains(script, "git checkout -B fridare/mod-test") {
		t.Fatal("branch")
	}
	if !strings.Contains(script, "--host=android-arm64") {
		t.Fatal("android host")
	}
	if !strings.Contains(script, "--host=x86_64-w64-mingw32") {
		t.Fatal("windows MinGW host")
	}
	if !strings.Contains(script, "--without-prebuilds=sdk:host") {
		t.Fatal("windows MinGW should skip missing mingw SDK prebuild")
	}
	if !strings.Contains(script, "17.16.4") {
		t.Fatal("version")
	}
}

func TestBuildPipelineScript_RejectsNoTargets(t *testing.T) {
	_, err := BuildPipelineScript(PipelineScriptOptions{
		Clone: CloneSpec{Version: "16.0.0"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSupportedBuildTargets_MainstreamPlatforms(t *testing.T) {
	platforms := map[string]bool{}
	for _, tgt := range SupportedBuildTargets() {
		platforms[tgt.Platform] = true
		if tgt.Host == "" || tgt.ID == "" {
			t.Fatalf("incomplete: %+v", tgt)
		}
	}
	for _, p := range []string{"android", "ios", "windows", "macos"} {
		if !platforms[p] {
			t.Fatalf("missing platform %s", p)
		}
	}
}

func TestConfigureHostArgs(t *testing.T) {
	args := ConfigureHostArgs("..", "android-arm64")
	if args[1] != "--host=android-arm64" {
		t.Fatal(args)
	}
}

func TestDockerRunContainsProxyAndMount(t *testing.T) {
	args := RunContainerArgs(DockerWorkspace{
		Image:            "fridare/frida-builder:latest",
		HostWorkDir:      "/host/work",
		ContainerWorkDir: "/work",
		HTTPProxy:        "http://p:1",
		HTTPSProxy:       "http://p:1",
	}, []string{"bash", "-lc", "true"})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-v") || !strings.Contains(joined, "/host/work:/work") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "HTTP_PROXY=http://p:1") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "fridare/frida-builder:latest") {
		t.Fatal(joined)
	}
}

func TestDockerfileSkeleton(t *testing.T) {
	// default var may be empty-mirror skeleton
	df := DockerfileSkeletonForMirror("")
	if !strings.Contains(df, "ubuntu") {
		t.Fatal("dockerfile")
	}
}

func TestCompileIsolationPolicy(t *testing.T) {
	if !strings.Contains(CompileIsolationPolicy, "Docker") {
		t.Fatal(CompileIsolationPolicy)
	}
	if !strings.Contains(CompileIsolationPolicy, "Windows") {
		t.Fatal("should mention Windows host avoidance")
	}
}

func TestBuildOnlyPipelineScript_DockerOnlyMake(t *testing.T) {
	script, err := BuildOnlyPipelineScript(PipelineScriptOptions{
		SourceDir:   "frida",
		ArtifactDir: "artifacts",
		TargetIDs:   []string{"android-arm64"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, DockerBuildStageMarker) {
		t.Fatal("missing docker-only marker")
	}
	if !strings.Contains(script, "configure --host=android-arm64") {
		t.Fatal(script)
	}
	if !strings.Contains(script, "--enable-server") || !strings.Contains(script, "--disable-frida-python") ||
		!strings.Contains(script, "--disable-frida-tools") {
		t.Fatalf("product opts missing in configure: %s", script)
	}
	if !strings.Contains(script, "make") {
		t.Fatal(script)
	}
	// Build dir must stay under /work (sibling of frida), not ../build-* which leaves the volume
	if strings.Contains(script, "../build-android-arm64") {
		t.Fatalf("build dir escapes work volume: %s", script)
	}
	if !strings.Contains(script, "build-android-arm64") {
		t.Fatal(script)
	}
	// Runtime only verifies image toolchain (Node/NDK baked at docker build)
	if !strings.Contains(script, "ANDROID_NDK_ROOT") {
		t.Fatalf("missing NDK verify: %s", script)
	}
	if !strings.Contains(script, "node") {
		t.Fatalf("missing Node verify: %s", script)
	}
	if strings.Contains(script, "android-ndk.zip") || strings.Contains(script, "emergency") {
		t.Fatal("compile script must not download NDK; deps belong in Dockerfile")
	}
	if !strings.Contains(script, "verify build environment") && !strings.Contains(script, "NDK OK") && !strings.Contains(script, "fridare] verify") {
		if !strings.Contains(script, "ensure") && !strings.Contains(script, "NDK") {
			t.Fatalf("missing env verify: %s", script)
		}
	}
	// Must not suggest host windows paths
	if strings.Contains(script, "C:\\") || strings.Contains(script, "cmd.exe") {
		t.Fatal("host windows paths in container script")
	}
}

func TestNeedsAndroidNDK(t *testing.T) {
	if !NeedsAndroidNDK([]string{"android-arm64"}) {
		t.Fatal("android should need NDK")
	}
	if NeedsAndroidNDK([]string{"linux-x86_64"}) {
		t.Fatal("linux host should not force NDK")
	}
}

func TestNeedsLinuxArm64Cross(t *testing.T) {
	if !NeedsLinuxArm64Cross([]string{"linux-arm64"}) {
		t.Fatal("linux-arm64 needs aarch64 cross")
	}
	if NeedsLinuxArm64Cross([]string{"linux-x86_64", "android-arm64"}) {
		t.Fatal("other targets should not force aarch64 host cross")
	}
	sh := VerifyBuildEnvShellEx(false, true)
	if !strings.Contains(sh, "aarch64-linux-gnu-gcc") {
		t.Fatal(sh)
	}
	script, err := BuildOnlyPipelineScript(PipelineScriptOptions{
		SourceDir: "frida", ArtifactDir: "artifacts", TargetIDs: []string{"linux-arm64"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "aarch64-linux-gnu-gcc") {
		t.Fatal("linux-arm64 pipeline must verify aarch64 cross compiler")
	}
	if !strings.Contains(script, "--host=aarch64-linux-gnu") {
		t.Fatal("linux-arm64 must configure with GNU triplet host, not bare linux-arm64")
	}
}

func TestBuildPipelineScript_HasDockerOnlyMarker(t *testing.T) {
	script, err := BuildPipelineScript(PipelineScriptOptions{
		Clone:     CloneSpec{Version: "17.16.4"},
		Branch:    "fridare/mod-x",
		SourceDir: "frida",
		TargetIDs: []string{"android-arm64"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, DockerBuildStageMarker) {
		t.Fatal(script)
	}
}

func TestIsDockerCompileCommand(t *testing.T) {
	args := RunContainerArgs(DockerWorkspace{
		Image:            DefaultBuildImage,
		HostWorkDir:      "/host",
		ContainerWorkDir: "/work",
	}, []string{"bash", "/work/build-only.sh"})
	if !IsDockerCompileCommand(args) {
		t.Fatalf("%v", args)
	}
	if IsDockerCompileCommand([]string{"make", "-C", "frida"}) {
		t.Fatal("bare host make must not count as docker compile")
	}
	if IsDockerCompileCommand([]string{"git", "clone", "x"}) {
		t.Fatal("host git is not compile")
	}
}

func TestDockerCloneShell(t *testing.T) {
	sh, err := DockerCloneShell("16.5.9", "frida")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sh, "--depth=1") || !strings.Contains(sh, "16.5.9") {
		t.Fatal(sh)
	}
}
