package main

import (
	"fmt"
	"os"

	"fridare-gui/internal/rebuild"
)

func main() {
	n, err := rebuild.RepairFridaGitWraps(os.Args[1], os.Args[2])
	fmt.Println("repaired", n, err)
	if err != nil {
		os.Exit(1)
	}
}
