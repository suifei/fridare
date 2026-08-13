package main

import (
	"fmt"
	"fridare-gui/internal/rebuild"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: write-compile-script <out.sh> <target,target,...>\n")
		os.Exit(2)
	}
	out := os.Args[1]
	ids := strings.Split(os.Args[2], ",")
	for i := range ids {
		ids[i] = strings.TrimSpace(ids[i])
	}
	script, err := rebuild.BuildOnlyPipelineScript(rebuild.PipelineScriptOptions{
		SourceDir:   "frida",
		ArtifactDir: "artifacts",
		TargetIDs:   ids,
		StealthSeed: "kxmwp16",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, []byte(rebuild.WriteScriptUnixLF(script)), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote", out)
}
