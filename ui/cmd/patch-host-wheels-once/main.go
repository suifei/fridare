package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fridare-gui/internal/rebuild"
)

func main() {
	magic := os.Args[1]
	root := os.Args[2]
	nOk := 0
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if !strings.HasPrefix(name, "frida-") || !strings.HasSuffix(name, ".whl") {
			return nil
		}
		tmp := p + ".new"
		n, err := rebuild.PatchFridaRPCInWheel(p, tmp, magic)
		if err != nil {
			fmt.Println("skip", p, err)
			return nil
		}
		_ = os.Remove(p)
		if err := os.Rename(tmp, p); err != nil {
			fmt.Println("rename", err)
			return nil
		}
		fmt.Printf("%s replacements=%d\n", filepath.Base(filepath.Dir(p))+"/"+name, n)
		nOk++
		return nil
	})
	fmt.Println("patched_wheels", nOk)
}
