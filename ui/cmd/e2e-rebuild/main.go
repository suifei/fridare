// e2e-rebuild drives the full source rebuild pipeline (clone → mod → docker build)
// without the Fyne GUI. One-shot scripts wrap this command.
//
//	go run ./cmd/e2e-rebuild -version 17.16.4 -target android-arm64
//	go run ./cmd/e2e-rebuild -dry-run   # stage wiring only
//
// Logs: {workdir}/logs/rebuild-*.log and latest.log
// Success prints "E2E OK" when stage=done.
package main

import (
	"context"
	"flag"
	"fmt"
	"fridare-gui/internal/rebuild"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	version := flag.String("version", "17.16.4", "Frida official git tag")
	target := flag.String("target", "android-arm64", "comma-separated build target ids")
	magic := flag.String("magic", "abcde", "5-letter magic name")
	port := flag.Int("port", 27142, "listen port in mod plan")
	proxy := flag.String("proxy", envOr("FRIDARE_PROXY", envOr("HTTPS_PROXY", envOr("HTTP_PROXY", "http://localhost:8080"))), "HTTP proxy for git inside container (localhost rewritten)")
	mirror := flag.String("mirror", envOr("FRIDARE_DOCKER_MIRROR", "docker.1ms.run"), "Docker Hub mirror prefix")
	image := flag.String("image", "fridare/frida-builder:latest", "local builder image tag")
	work := flag.String("work", "", "work dir (default ~/.fridare/rebuild-e2e)")
	dry := flag.Bool("dry-run", false, "orchestration only, no real docker compile")
	timeout := flag.Duration("timeout", 6*time.Hour, "max wall time")
	profile := flag.String("profile", "deep", "strip direction profile: safe | deep | abi | full | explore (deep=服务端+客户端协议/API/idents 同步)")
	agent := flag.Bool("agent", false, "use GrokAgent (UseExistingGrok) when grok on PATH; default StubAgent")
	mode := flag.String("mode", "full", "full | bootstrap | develop")
	flag.Parse()

	home, _ := os.UserHomeDir()
	workDir := *work
	if workDir == "" {
		workDir = filepath.Join(home, ".fridare", "rebuild-e2e")
	}
	arts := filepath.Join(workDir, "artifacts")
	_ = os.MkdirAll(workDir, 0755)
	dirFile := filepath.Join(workDir, "fridare-directions.json")

	targets := strings.Split(*target, ",")
	for i := range targets {
		targets[i] = strings.TrimSpace(targets[i])
	}

	jobMode := rebuild.JobModeFull
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "bootstrap":
		jobMode = rebuild.JobModeBootstrap
	case "develop":
		jobMode = rebuild.JobModeDevelop
	case "full", "":
		jobMode = rebuild.JobModeFull
	default:
		fmt.Fprintf(os.Stderr, "unknown -mode %q (want full|bootstrap|develop)\n", *mode)
		os.Exit(2)
	}

	cfg := rebuild.JobConfig{
		FridaVersion:     *version,
		TargetIDs:        targets,
		MagicName:        *magic,
		ListenPort:       *port,
		Goals:            "e2e one-shot deep: server+client sync re.frida./Frida./rpc + basename/idents + post-build markers + host wheels",
		WorkDir:          workDir,
		ArtifactDir:      arts,
		Proxy:            *proxy,
		OpenAIBaseURL:    envOr("OPENAI_BASE_URL", "https://claudegpt.org/v1"),
		OpenAIAPIKey:     envOr("OPENAI_API_KEY", "e2e-no-key"),
		DockerImage:      *image,
		DockerMirror:     *mirror,
		DockerPullDirect: true,
		UseExistingGrok:  *agent,
		AgentUseGUIProxy: true,
		MinDiskGB:        3,
		DryRun:           *dry,
		Mode:             jobMode,
		DirectionProfile: *profile,
		DirectionFile:    dirFile,
	}

	fmt.Printf("=== Fridare e2e rebuild (one-shot entry) ===\n")
	fmt.Printf("version=%s targets=%v magic=%s mode=%s profile=%s agent=%v\n",
		cfg.FridaVersion, cfg.TargetIDs, cfg.MagicName, jobMode, cfg.DirectionProfile, *agent)
	fmt.Printf("work=%s\nproxy=%s mirror=%s dry=%v\n", workDir, cfg.Proxy, cfg.DockerMirror, cfg.DryRun)
	fmt.Printf("directions=%s timeout=%s\n\n", dirFile, *timeout)

	var driver rebuild.AgentDriver = &rebuild.StubAgent{}
	if *agent {
		driver = &rebuild.GrokAgent{
			PromptDir: filepath.Join(workDir, "agent-prompts"),
		}
		fmt.Println("AgentDriver=GrokAgent (refine if grok on PATH; else naive tree plan)")
	} else {
		fmt.Println("AgentDriver=StubAgent (deterministic DefaultModOps + directions)")
	}

	orch := rebuild.NewOrchestrator(nil, driver)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\ninterrupt — cancelling…")
		orch.Cancel()
		cancel()
	}()

	progress := func(ev rebuild.ProgressEvent) {
		line := fmt.Sprintf("%s [%s] %.0f%% %s", ev.Time.Format("15:04:05"), ev.Stage, ev.Percent*100, ev.Message)
		if ev.Err != "" {
			line += " ERR=" + ev.Err
		}
		fmt.Println(line)
	}

	if err := orch.Start(cfg, progress); err != nil {
		fmt.Fprintf(os.Stderr, "start failed: %v\n", err)
		os.Exit(2)
	}

	for {
		select {
		case <-ctx.Done():
			orch.Cancel()
			st := orch.State()
			printFinal(st)
			fmt.Fprintf(os.Stderr, "timeout/cancel: %v\n", ctx.Err())
			os.Exit(3)
		case <-time.After(2 * time.Second):
			st := orch.State()
			switch st.Stage {
			case rebuild.StageDone:
				printFinal(st)
				fmt.Println("E2E OK")
				fmt.Println("StageDone")
				os.Exit(0)
			case rebuild.StageFailed, rebuild.StageCancelled:
				printFinal(st)
				fmt.Fprintf(os.Stderr, "E2E FAIL stage=%s err=%s\n", st.Stage, st.Error)
				os.Exit(1)
			}
		}
	}
}

func printFinal(st rebuild.JobState) {
	fmt.Println("\n=== final state ===")
	fmt.Printf("stage=%s percent=%.0f%%\n", st.Stage, st.Percent*100)
	fmt.Printf("branch=%s\n", st.Branch)
	fmt.Printf("artifact=%s\n", st.Artifact)
	fmt.Printf("log=%s\n", st.LogPath)
	fmt.Printf("latest_log=%s\n", st.LatestLog)
	if st.Error != "" {
		fmt.Printf("error=%s\n", st.Error)
	}
	if st.LogPath != "" {
		if b, err := os.ReadFile(st.LogPath); err == nil {
			s := string(b)
			if len(s) > 4000 {
				s = s[len(s)-4000:]
			}
			fmt.Println("\n=== log tail ===")
			fmt.Println(s)
		}
	}
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
