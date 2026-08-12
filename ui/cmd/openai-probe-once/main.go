// openai-probe-once loads the app config and runs a real OpenAI-compatible
// endpoint probe (redacted output). Usage:
//
//	go run ./cmd/openai-probe-once [times]
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"fridare-gui/internal/config"
	"fridare-gui/internal/rebuild"
)

func main() {
	times := 2
	if len(os.Args) > 1 {
		if n, err := strconv.Atoi(os.Args[1]); err == nil && n > 0 && n <= 10 {
			times = n
		}
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println("LOAD_CONFIG_ERR", err)
		os.Exit(2)
	}
	key := strings.TrimSpace(cfg.OpenAIAPIKey)
	base := strings.TrimSpace(cfg.OpenAIBaseURL)
	fmt.Printf("config_path_ok=true base=%s model=%s key_len=%d use_gui_proxy=%v proxy=%s\n",
		base, cfg.OpenAIModel, len(key), cfg.RebuildAgentUseGUIProxy, cfg.Proxy)

	var lastOK bool
	for i := 1; i <= times; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		res := rebuild.ProbeOpenAIEndpoint(ctx, rebuild.OpenAIProbeOptions{
			BaseURL:     base,
			APIKey:      key,
			Model:       cfg.OpenAIModel,
			UseGUIProxy: cfg.RebuildAgentUseGUIProxy,
			Proxy:       cfg.Proxy,
			Timeout:     25 * time.Second,
		})
		cancel()
		lastOK = res.OK
		// Redact any accidental key leak
		report := rebuild.FormatOpenAIProbeReport(res)
		report = rebuild.RedactSecret(report, key)
		fmt.Printf("--- probe %d/%d ---\n%s", i, times, report)
	}
	if !lastOK {
		os.Exit(1)
	}
}
