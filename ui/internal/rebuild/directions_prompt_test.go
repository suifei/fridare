package rebuild

import (
	"strings"
	"testing"
)

func TestBuildAgentPrompt_EmbedsDirections(t *testing.T) {
	p := BuildAgentPrompt(JobConfig{MagicName: "abcde", ListenPort: 27142, FridaVersion: "17.16.4", DirectionProfile: "safe", Goals: "test"}, "br", "/src")
	for _, want := range []string{"L1_product_basename", "forbidden", "post_build", "方向清单", "profile=safe", "strongR",
		"DEFAULT_CONTROL_PORT", "27042", "27142", "port=27142"} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
	if strings.Contains(p, "re.abcde.") {
		t.Fatal("safe prompt should not force protocol rewrite pairs")
	}
}

func TestBuildAgentPrompt_DeepServerClientSync(t *testing.T) {
	p := BuildAgentPrompt(JobConfig{MagicName: "abcde", ListenPort: 27142, FridaVersion: "17.16.4", DirectionProfile: "deep", Goals: "hide"}, "br", "/src")
	for _, want := range []string{"profile=deep", "re.abcde.", "abcde:rpc", "Abcde.", "host wheel", "strongR"} {
		if !strings.Contains(p, want) {
			t.Fatalf("deep prompt missing %q\n%s", want, p)
		}
	}
}
