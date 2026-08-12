// frida-strip-scan: batch classify "frida" markers into strip directions.
// Used by tools and as dig direction for the rebuild AI Agent.
//
//	go run ./cmd/frida-strip-scan -root path/to/frida -magic abcde -o report.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"fridare-gui/internal/rebuild"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", "", "Frida source tree root (required)")
	magic := flag.String("magic", "abcde", "5-letter magic for report context")
	out := flag.String("o", "", "write ScanReport JSON (default stdout)")
	dirOut := flag.String("directions", "", "write DirectionManifest JSON (safe|explore)")
	profile := flag.String("profile", "safe", "direction profile: safe | deep | explore")
	applyDeep := flag.Bool("apply-deep", false, "apply deep source strip (DeepModOps + quoted literals) in-place on -root")
	digOut := flag.String("dig", "", "write deep dig brief markdown")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "usage: frida-strip-scan -root <src> [-magic abcde] [-profile safe|deep|explore] [-apply-deep] [-o report.json] [-directions d.json] [-dig brief.md]")
		os.Exit(2)
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rep, err := rebuild.ScanFridaMarkers(abs, *magic)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan:", err)
		os.Exit(1)
	}
	data, _ := json.MarshalIndent(rep, "", "  ")
	if *out == "" {
		fmt.Println(string(data))
	} else {
		if err := os.WriteFile(*out, append(data, '\n'), 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "wrote", *out, "hits=", len(rep.Hits))
	}
	if *applyDeep {
		cfg := rebuild.JobConfig{MagicName: *magic, ListenPort: 27142, DirectionProfile: "deep"}
		plan, err := rebuild.PlanModsFromTree(abs, cfg, "deep-strip")
		if err != nil {
			fmt.Fprintln(os.Stderr, "plan:", err)
			os.Exit(1)
		}
		// Re-plan uses deep ops via profile
		cfg.DirectionProfile = "deep"
		agent := &rebuild.StubAgent{}
		if err := agent.ApplyMods(context.Background(), cfg, plan, abs); err != nil {
			fmt.Fprintln(os.Stderr, "apply-deep:", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "apply-deep: OK (content + assets + quoted literals + dig brief)")
		// refresh scan after apply
		rep, _ = rebuild.ScanFridaMarkers(abs, *magic)
	}
	if *dirOut != "" {
		var m rebuild.DirectionManifest
		switch *profile {
		case "explore":
			m = rebuild.ExploreDirectionManifest(*magic, 27142)
		case "deep":
			m = rebuild.DeepDirectionManifest(*magic, 27142)
		default:
			m = rebuild.SafeDirectionManifest(*magic, 27142)
		}
		if err := rebuild.WriteDirectionManifest(*dirOut, m); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "wrote directions", *dirOut, "profile=", m.Profile)
	}
	if *digOut != "" {
		tasks := rebuild.BuildDeepDigTasks(rep, *magic)
		if err := os.WriteFile(*digOut, []byte(rebuild.FormatDeepDigBrief(tasks, *magic)), 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "wrote dig brief", *digOut, "tasks=", len(tasks))
	}
}
