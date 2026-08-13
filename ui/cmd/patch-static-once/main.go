// patch-static-once applies catalog binary marker rewrite + symbol strip
// to a directory of already hexreplaced product blobs.
package main

import (
	"fmt"
	"fridare-gui/internal/rebuild"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: patch-static-once <dir> <magic>\n")
		os.Exit(2)
	}
	dir, magic := os.Args[1], os.Args[2]
	n, err := rebuild.PatchArtifactBinaryMarkers(dir, magic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "markers: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("markers patched files=%d\n", n)
	s, err := rebuild.StripProductBinariesDir(dir, magic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "strip: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("stripped files=%d\n", s)
}
