package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"fridare-gui/internal/rebuild"
)

func main() {
	src := ""
	if len(os.Args) > 1 {
		src = os.Args[1]
	} else {
		const e2e = `D:\works\fridare-rebuild-e2e\src\frida`
		if st, err := os.Stat(e2e); err == nil && st.IsDir() {
			src = e2e
		}
	}
	if src == "" {
		fmt.Fprintln(os.Stderr, "usage: plan-tree-once <frida-source-dir>")
		os.Exit(2)
	}
	cfg := rebuild.JobConfig{
		MagicName: "abcde", ListenPort: 27142, FridaVersion: "17.16.4",
		DirectionProfile: "deep",
	}
	fmt.Println("start", time.Now().Format(time.RFC3339), "src", src)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("mem_before=%.1fMB\n", float64(m.Alloc)/1e6)
	t0 := time.Now()
	plan, err := rebuild.PlanModsFromTree(src, cfg, "br")
	runtime.ReadMemStats(&m)
	n := 0
	if plan != nil {
		n = len(plan.Operations)
	}
	fmt.Printf("err=%v ops=%d elapsed=%s alloc=%.1fMB sys=%.1fMB\n",
		err, n, time.Since(t0), float64(m.Alloc)/1e6, float64(m.Sys)/1e6)
}
