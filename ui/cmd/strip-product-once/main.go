package main

import (
	"fmt"
	"fridare-gui/internal/rebuild"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: strip-product-once <dir-or-file> <magic>")
		os.Exit(2)
	}
	target, magic := os.Args[1], os.Args[2]
	st, err := os.Stat(target)
	if err != nil {
		panic(err)
	}
	if st.IsDir() {
		pn, perr := rebuild.PatchArtifactBinaryMarkers(target, magic)
		fmt.Printf("patched_files=%d err=%v dir=%s\n", pn, perr, target)
		n, err := rebuild.StripProductBinariesDir(target, magic)
		fmt.Printf("stripped_files=%d err=%v dir=%s\n", n, err, target)
		return
	}
	res, err := rebuild.StripProductBinary(target, magic)
	fmt.Printf("%s format=%s exports=%d debug=%v in=%d out=%d err=%v\n",
		filepath.Base(target), res.Format, res.ExportsRewritten, res.DebugStripped, res.BytesIn, res.BytesOut, err)
	data, _ := os.ReadFile(target)
	fmt.Printf("has_frida_=%v has_magic_=%v\n",
		strings.Contains(string(data), "frida_"),
		strings.Contains(string(data), magic+"_"))
}
