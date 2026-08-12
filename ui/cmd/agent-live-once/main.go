// agent-live-once drives the shipped GrokAgent with real app config against a
// disposable fixture tree. Prints redacted evidence to stdout.
//
//	go run ./cmd/agent-live-once [rounds]
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fridare-gui/internal/config"
	"fridare-gui/internal/rebuild"
)

func main() {
	rounds := 2
	if len(os.Args) > 1 {
		if n, err := strconv.Atoi(os.Args[1]); err == nil && n > 0 && n <= 5 {
			rounds = n
		}
	}

	cfgFile, err := config.LoadConfig()
	if err != nil {
		fmt.Println("LOAD_CONFIG_ERR", err)
		os.Exit(2)
	}
	key := strings.TrimSpace(cfgFile.OpenAIAPIKey)
	fmt.Printf("config_ok base=%s model=%s key_len=%d use_local_grok=%v agent_proxy=%v proxy=%s\n",
		cfgFile.OpenAIBaseURL, cfgFile.OpenAIModel, len(key),
		cfgFile.RebuildUseLocalGrok, cfgFile.RebuildAgentUseGUIProxy, cfgFile.Proxy)

	binary, grokOK := rebuild.ResolveGrokBinary(nil)
	fmt.Printf("grok_resolve ok=%v path=%s\n", grokOK, binary)

	// Env proof (names only / redacted values)
	env := rebuild.EnvForAgent(rebuild.JobConfig{
		OpenAIAPIKey:     cfgFile.OpenAIAPIKey,
		OpenAIBaseURL:    cfgFile.OpenAIBaseURL,
		OpenAIModel:      cfgFile.OpenAIModel,
		Proxy:            cfgFile.Proxy,
		AgentUseGUIProxy: cfgFile.RebuildAgentUseGUIProxy,
	})
	fmt.Println("env_keys:")
	for _, e := range env {
		eq := strings.IndexByte(e, '=')
		if eq <= 0 {
			continue
		}
		k, v := e[:eq], e[eq+1:]
		switch k {
		case "OPENAI_API_KEY", "OPENAI_KEY":
			fmt.Printf("  %s=len:%d\n", k, len(v))
		case "OPENAI_BASE_URL", "OPENAI_API_BASE", "OPENAI_API_BASE_URL", "OPENAI_MODEL", "GROK_MODEL",
			"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy", "ALL_PROXY", "all_proxy":
			fmt.Printf("  %s=%s\n", k, v)
		}
	}

	workRoot := filepath.Join(cfgFile.WorkDir, "agent-live-eval")
	_ = os.MkdirAll(workRoot, 0755)

	var lastOK bool
	for r := 1; r <= rounds; r++ {
		fmt.Printf("\n======== ROUND %d/%d ========\n", r, rounds)
		roundDir := filepath.Join(workRoot, fmt.Sprintf("round-%d-%d", r, time.Now().Unix()))
		src := filepath.Join(roundDir, "src", "frida")
		promptDir := filepath.Join(roundDir, "agent-prompts")
		if err := os.MkdirAll(src, 0755); err != nil {
			fmt.Println("mkdir_err", err)
			os.Exit(1)
		}
		// Representative markers the deep/safe ops look for
		files := map[string]string{
			"meson.build": "# frida project\nproject('frida')\n",
			"server/main.c": `
#include <stdio.h>
// re.frida.HostSession
const char *rpc = "frida:rpc";
const char *iface = "re.frida.Server";
const char *path = "/re/frida/HostSession";
const char *name = "frida-server";
int main() { printf("Frida.version\n"); return 0; }
`,
			"lib/agent.c": `// frida-agent helper\nvoid frida_agent_main(void) {}\n`,
			"README.md":   "frida-server binary and frida:rpc channel\n",
		}
		for rel, body := range files {
			p := filepath.Join(src, filepath.FromSlash(rel))
			_ = os.MkdirAll(filepath.Dir(p), 0755)
			if err := os.WriteFile(p, []byte(body), 0644); err != nil {
				fmt.Println("write_fixture_err", err)
				os.Exit(1)
			}
		}

		job := rebuild.JobConfig{
			FridaVersion:     "17.16.4",
			MagicName:        "abcde",
			ListenPort:       27142,
			Goals:            "评估内置 AI agent：在 fixture 上执行 deep 魔改计划与 apply；输出变更摘要。",
			WorkDir:          roundDir,
			OpenAIBaseURL:    cfgFile.OpenAIBaseURL,
			OpenAIAPIKey:     cfgFile.OpenAIAPIKey,
			OpenAIModel:      cfgFile.OpenAIModel,
			Proxy:            cfgFile.Proxy,
			AgentUseGUIProxy: cfgFile.RebuildAgentUseGUIProxy,
			UseExistingGrok:  cfgFile.RebuildUseLocalGrok,
			GrokBinary:       binary, // pin resolved binary when present
			DirectionProfile: "deep",
		}
		if !grokOK {
			job.GrokBinary = ""
			job.UseExistingGrok = false
		}

		agent := rebuild.NewDefaultAgent(promptDir)
		// Plan
		ctx1, cancel1 := context.WithTimeout(context.Background(), 3*time.Minute)
		plan, err := agent.PlanMods(ctx1, job, "fridare/mod-live")
		cancel1()
		fmt.Printf("plan_err=%v\n", err)
		if plan != nil {
			fmt.Printf("plan_ops=%d goals=%q\n", len(plan.Operations), plan.Goals)
			for i, op := range plan.Operations {
				if i >= 8 {
					fmt.Printf("  ... +%d more\n", len(plan.Operations)-8)
					break
				}
				fmt.Printf("  op[%d] %s %s find=%q\n", i, op.Operation, op.Path, truncate(op.Find, 40))
			}
		}

		// Apply (invokes grok when configured, then naive apply)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Minute)
		aerr := agent.ApplyMods(ctx2, job, plan, src)
		cancel2()
		fmt.Printf("apply_err=%v\n", aerr)

		// Collect artifacts
		_ = filepath.Walk(promptDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(promptDir, path)
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			text := rebuild.RedactSecret(string(data), key)
			if len(text) > 1500 {
				text = text[:1500] + "\n…(truncated)"
			}
			fmt.Printf("--- artifact %s (%d bytes) ---\n%s\n", rel, info.Size(), text)
			return nil
		})

		// Verify naive/agent applied something on fixture
		mainC, _ := os.ReadFile(filepath.Join(src, "server", "main.c"))
		body := string(mainC)
		hasMagic := strings.Contains(body, "abcde") || strings.Contains(body, "re.abcde")
		fmt.Printf("fixture_has_magic_markers=%v\n", hasMagic)
		fmt.Printf("invoke_args_sample=%v\n", rebuild.GrokInvokeArgs(binary, "prompt.md", src))

		roundOK := aerr == nil && plan != nil && len(plan.Operations) > 0
		// If grok present, prefer seeing agent-output or error artifact
		if grokOK {
			if _, e1 := os.Stat(filepath.Join(promptDir, "agent-output.txt")); e1 == nil {
				fmt.Println("agent_output_present=true")
			} else if _, e2 := os.Stat(filepath.Join(promptDir, "agent-error.txt")); e2 == nil {
				fmt.Println("agent_error_present=true")
			} else if _, e3 := os.Stat(filepath.Join(promptDir, "plan-agent-output.txt")); e3 == nil {
				fmt.Println("plan_agent_output_present=true")
			} else {
				fmt.Println("agent_output_present=false")
			}
		}
		fmt.Printf("round_ok=%v\n", roundOK)
		lastOK = roundOK
	}

	if !lastOK {
		os.Exit(1)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
