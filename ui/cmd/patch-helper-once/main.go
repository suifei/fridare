// patch-helper-once repairs already-magic Android product binaries:
// helper dex descriptors/checksums, *_agent_main GNU/sysv hashes, and
// renaming leftover GumJS token "Frida" to PascalCase(magic) inside the
// embedded agent ELF (16.x frida-ps; does not write Frida. back; skips
// Friday / FridaXxx).
package main

import (
	"fmt"
	"fridare-gui/internal/rebuild"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: patch-helper-once <file-or-dir> <magic>\n")
		os.Exit(2)
	}
	root, magic := os.Args[1], os.Args[2]
	n := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if info.Size() < rebuild.MinArtifactBytes || info.Size() > 200*1024*1024 {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		hits := rebuild.PatchAndroidHelperInBinary(data, magic)
		if hits == 0 {
			return nil
		}
		if werr := os.WriteFile(path, data, info.Mode()); werr != nil {
			return werr
		}
		fmt.Printf("%s helper-hits=%d\n", path, hits)
		n++
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("patched files=%d\n", n)
}
