package main

import (
	"fmt"
	"fridare-gui/internal/rebuild"
	"os"
	"path/filepath"
)

func main() {
	work := os.Args[1]
	cfg := rebuild.JobConfig{
		FridaVersion: "17.16.4",
		MagicName:    "abcde",
		ListenPort:   27142,
		WorkDir:      work,
		TargetIDs:    []string{"android-arm64"},
		ArtifactDir:  filepath.Join(work, "artifacts"),
		DockerMirror: "docker.1ms.run",
		Proxy:        "http://localhost:8080",
	}
	cat := rebuild.CatalogRoot(work)
	dockerArts := filepath.Join(work, "src", "artifacts")
	flat := filepath.Join(work, "artifacts")
	primary, entries, err := rebuild.OrganizeExportToCatalog(cat, cfg, dockerArts, flat)
	if err != nil {
		panic(err)
	}
	fmt.Println("primary", primary)
	fmt.Println("entries", entries)

	binDir := filepath.Join(primary, "binaries")
	ents, _ := os.ReadDir(binDir)
	for _, e := range ents {
		info, _ := e.Info()
		if info != nil {
			fmt.Printf("  %s %d\n", e.Name(), info.Size())
		}
	}

	wheels, werr := rebuild.BuildPatchedFridaToolsWheels(cfg, cat, entries, nil)
	if werr != nil {
		fmt.Println("wheels warn:", werr)
	} else {
		fmt.Println("wheels", len(wheels))
		for _, w := range wheels {
			fmt.Println(" ", filepath.Base(w))
		}
	}
	fmt.Println("EXPORT_OK")
}
