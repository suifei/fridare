package main

import (
	"fmt"
	"os"

	"fridare-gui/internal/rebuild"
)

func main() {
	src := ""
	if len(os.Args) > 1 {
		src = os.Args[1]
	} else {
		// Convenience default for local e2e workspace only when present.
		const e2e = `D:\works\fridare-rebuild-e2e\src\frida`
		if st, err := os.Stat(e2e); err == nil && st.IsDir() {
			src = e2e
		}
	}
	if src == "" {
		fmt.Fprintln(os.Stderr, "usage: mingw-fix-once <frida-source-dir>")
		os.Exit(2)
	}
	n, err := rebuild.ApplyMinGWCompatPatches(src)
	fmt.Println("src", src)
	fmt.Println("patches", n, "err", err)
	if err != nil {
		os.Exit(1)
	}
}
