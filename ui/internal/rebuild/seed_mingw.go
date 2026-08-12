package rebuild

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed seed_mingw_wraps.sh
var seedMinGWWrapsScript string

// SeedMinGWWrapsFileName is staged next to build-only.sh under the Docker work dir (/work).
const SeedMinGWWrapsFileName = "seed-mingw-wraps.sh"

// StageSeedMinGWWraps writes the embedded MinGW wrap seed script into workDir
// (host path that mounts as /work). Always LF-normalized for Linux bash.
func StageSeedMinGWWraps(workDir string) error {
	if workDir == "" {
		return fmt.Errorf("StageSeedMinGWWraps: empty workDir")
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(workDir, SeedMinGWWrapsFileName)
	body := WriteScriptUnixLF(seedMinGWWrapsScript)
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		return fmt.Errorf("stage %s: %w", SeedMinGWWrapsFileName, err)
	}
	return nil
}

// SeedMinGWWrapsShellSnippet returns the bash snippet that runs the staged seed
// script for a MinGW target. Prefer -f (Windows bind mounts often lack +x).
func SeedMinGWWrapsShellSnippet(srcDir string) string {
	if srcDir == "" {
		srcDir = "frida"
	}
	// Always /work/seed-mingw-wraps.sh (orchestrator stages there). Fallback: sibling of source.
	return fmt.Sprintf(
		"if [ -f /work/%s ]; then bash /work/%s %s || echo '[fridare] WARN: seed-mingw-wraps non-zero (continuing)'; "+
			"elif [ -f %s/../%s ]; then bash %s/../%s %s || echo '[fridare] WARN: seed-mingw-wraps non-zero (continuing)'; "+
			"else echo '[fridare] WARN: %s missing (MinGW wraps not pre-seeded)'; fi\n",
		SeedMinGWWrapsFileName, SeedMinGWWrapsFileName, shellQuote(srcDir),
		shellQuote(srcDir), SeedMinGWWrapsFileName, shellQuote(srcDir), SeedMinGWWrapsFileName, shellQuote(srcDir),
		SeedMinGWWrapsFileName,
	)
}
