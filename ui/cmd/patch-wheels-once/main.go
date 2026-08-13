// patch-wheels-once downloads official pip frida wheels (EffectivePipVersion)
// and writes patched host packages into an existing catalog root.
package main

import (
	"fmt"
	"fridare-gui/internal/rebuild"
	"os"
)

func main() {
	work := "D:\\works\\fridare-rebuild-17.17.1"
	ver := "17.17.1"
	if len(os.Args) > 1 {
		work = os.Args[1]
	}
	if len(os.Args) > 2 {
		ver = os.Args[2]
	}
	cfg := rebuild.JobConfig{
		FridaVersion: ver,
		MagicName:    "kxmwp",
		ListenPort:   rebuild.OfficialListenPort,
		WorkDir:      work,
		Proxy:        envOr("HTTP_PROXY", "http://localhost:8080"),
	}
	cat := rebuild.CatalogRoot(work)
	entry := rebuild.CatalogEntryDir(cat, ver, "linux-x86_64", "kxmwp")
	fmt.Printf("catalog=%s pip=%s product=%s\n", cat, rebuild.EffectivePipVersion(cfg), ver)
	wheels, err := rebuild.BuildPatchedFridaToolsWheels(cfg, cat, []string{entry}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wheels: %v\n", err)
		os.Exit(1)
	}
	for _, w := range wheels {
		fmt.Println(w)
	}
	fmt.Println("WHEELS_OK", len(wheels))
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
