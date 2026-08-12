package main

import (
	"fmt"
	"fridare-gui/internal/rebuild"
	"os"
	"path/filepath"
)

func main() {
	root := os.Args[1]
	magic := os.Args[2]
	ft, n, err := rebuild.ApplyStructureAwareStrip(root, magic)
	if err != nil {
		panic(err)
	}
	fmt.Printf("files=%d replacements=%d\n", ft, n)
	c, _ := os.ReadFile(filepath.Join(root, "agent.c"))
	p, _ := os.ReadFile(filepath.Join(root, "tool.py"))
	fmt.Println("=== AFTER agent.c ===")
	fmt.Print(string(c))
	fmt.Println("=== AFTER tool.py ===")
	fmt.Print(string(p))
}
