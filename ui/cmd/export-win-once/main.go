package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fridare-gui/internal/rebuild"
)

func main() {
	work := `D:\works\fridare-rebuild-17.17.0`
	if len(os.Args) > 1 {
		work = os.Args[1]
	}
	cfg := rebuild.JobConfig{
		FridaVersion:     "17.17.0",
		MagicName:        "kxmwp",
		ListenPort:       27142,
		WorkDir:          work,
		TargetIDs:        []string{"windows-x86_64"},
		ArtifactDir:      filepath.Join(work, "artifacts"),
		Proxy:            "http://localhost:8080",
		DirectionProfile: "deep",
	}
	cat := rebuild.CatalogRoot(work)
	primary := filepath.Join(cat, "17.17.0", "windows-x86_64", "kxmwp")
	entries := []string{primary}

	fmt.Println("building host wheels magic=kxmwp version=17.17.0 …")
	wheels, err := rebuild.BuildPatchedFridaToolsWheels(cfg, cat, entries, nil)
	if err != nil {
		fmt.Println("WHEELS_ERR", err)
		// continue to crosscheck if partial
	} else {
		fmt.Println("wheels", len(wheels))
		for _, w := range wheels {
			fmt.Println(" ", w)
		}
	}

	report, cerr := rebuild.RunCatalogProtocolCrossCheck(cat, cfg, entries, wheels)
	if cerr != nil {
		fmt.Println("CROSS_ERR", cerr)
	} else {
		out := filepath.Join(primary, "PROTOCOL-SYNC.json")
		_ = rebuild.WriteProtocolCrossCheckReport(out, report)
		fmt.Printf("protocol matched=%v issues=%v path=%s\n", report.Matched, report.Issues, out)
	}
	if err != nil {
		os.Exit(1)
	}
	fmt.Println("EXPORT_OK")
}
