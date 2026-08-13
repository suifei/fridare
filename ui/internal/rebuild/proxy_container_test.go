package rebuild

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestRewriteProxyForContainer_Localhost(t *testing.T) {
	cases := []struct {
		in, wantSub string
	}{
		{"http://localhost:8080", "host.docker.internal:8080"},
		{"http://127.0.0.1:7890", "host.docker.internal:7890"},
		{"https://127.0.0.1:7890", "host.docker.internal:7890"},
		{"http://LOCALHOST:1080", "host.docker.internal:1080"},
		{"http://proxy.example.com:8080", "proxy.example.com:8080"},
		{"", ""},
	}
	for _, c := range cases {
		got := RewriteProxyForContainer(c.in)
		if c.wantSub == "" {
			if got != "" {
				t.Fatalf("in %q got %q", c.in, got)
			}
			continue
		}
		if !strings.Contains(got, c.wantSub) {
			t.Fatalf("in %q got %q want contain %q", c.in, got, c.wantSub)
		}
		if strings.Contains(got, "localhost") || strings.Contains(got, "127.0.0.1") {
			t.Fatalf("still has loopback: %q", got)
		}
	}
}

func TestRunContainerArgs_RewritesProxyAndAddHost(t *testing.T) {
	args := RunContainerArgs(DockerWorkspace{
		Image:            "img:latest",
		HostWorkDir:      ".",
		ContainerWorkDir: "/work",
		HTTPProxy:        "http://localhost:8080",
		HTTPSProxy:       "http://127.0.0.1:8080",
	}, []string{"bash", "-lc", "true"})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "host.docker.internal") {
		t.Fatalf("missing host.docker.internal rewrite: %s", joined)
	}
	if strings.Contains(joined, "HTTP_PROXY=http://localhost") {
		t.Fatalf("localhost proxy leaked into container env: %s", joined)
	}
	if !strings.Contains(joined, "--add-host=host.docker.internal:host-gateway") {
		t.Fatalf("missing add-host: %s", joined)
	}
	if !strings.Contains(joined, "HTTP_PROXY=http://host.docker.internal:8080") {
		t.Fatalf("expected rewritten HTTP_PROXY: %s", joined)
	}
}

func TestDockerCloneShell_SingleLineAndDepth(t *testing.T) {
	sh, err := DockerCloneShell("17.16.4", "frida")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sh, "\n") {
		// allow no multiline for windows docker -lc reliability
		t.Fatalf("expected single-line script, got newlines: %q", sh)
	}
	if !strings.Contains(sh, "--depth=1") || !strings.Contains(sh, "17.16.4") {
		t.Fatal(sh)
	}
	if !strings.Contains(sh, "host.docker.internal") && !strings.Contains(sh, "git clone") {
		// script mentions clone
		if !strings.Contains(sh, "git clone") {
			t.Fatal(sh)
		}
	}
}

func TestFormatDockerRunError_LocalhostHint(t *testing.T) {
	err := FormatDockerRunError("clone", "fatal: Failed to connect to localhost port 8080: Connection refused", context.DeadlineExceeded)
	if err == nil {
		t.Fatal("expected error")
	}
	s := err.Error()
	if !strings.Contains(s, "host.docker.internal") && !strings.Contains(s, "localhost") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "Connection refused") {
		t.Fatal(s)
	}
}

func TestFormatDockerRunError_NoFalseVersionHint(t *testing.T) {
	// ninja logs mention "file" and ".version" without a meson missing-file error
	err := FormatDockerRunError("compile",
		"[174/336] Generating compiler/agent.js\nv8-mksnapshot died with <Signals.SIGTRAP: 5>\nFile compiler.version used as input",
		fmt.Errorf("exit status 1"))
	if err == nil {
		t.Fatal("expected error")
	}
	s := err.Error()
	if strings.Contains(s, "缺 version/symbols") {
		t.Fatalf("false version/symbols hint: %s", s)
	}
	if !strings.Contains(s, "compiler_snapshot") && !strings.Contains(s, "mksnapshot") {
		t.Fatalf("want snapshot-tool hint: %s", s)
	}
}

func TestContainerNetworkHint(t *testing.T) {
	h := ContainerNetworkHint("http://localhost:8080", "http://host.docker.internal:8080")
	if !strings.Contains(h, "host.docker.internal") {
		t.Fatal(h)
	}
}
