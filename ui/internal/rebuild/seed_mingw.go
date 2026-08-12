package rebuild

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed seed_mingw_wraps.sh
var seedMinGWWrapsScript string

//go:embed mingw_dns_stub.h
var mingwDNSStubHeader string

// SeedMinGWWrapsFileName is staged next to build-only.sh under the Docker work dir (/work).
const SeedMinGWWrapsFileName = "seed-mingw-wraps.sh"

// MinGWDNSStubHeaderFileName is force-included for MinGW compiles (Vala-generated C lacks source stubs).
const MinGWDNSStubHeaderFileName = "fridare-mingw-dns.h"

// StageSeedMinGWWraps writes the embedded MinGW wrap seed script and DNS stub header into workDir
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
	hdr := filepath.Join(workDir, MinGWDNSStubHeaderFileName)
	if err := os.WriteFile(hdr, []byte(WriteScriptUnixLF(mingwDNSStubHeader)), 0644); err != nil {
		return fmt.Errorf("stage %s: %w", MinGWDNSStubHeaderFileName, err)
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

// MinGWCrossFileDNSIncludeShell returns a bash fragment (run from build dir after configure)
// that injects -include fridare-mingw-dns.h into the *cross* machine file only.
// Must not use env CFLAGS — that breaks Linux build-machine compiler detection (windows-x86).
func MinGWCrossFileDNSIncludeShell(headerFileName string) string {
	if headerFileName == "" {
		headerFileName = MinGWDNSStubHeaderFileName
	}
	// Embed a short Python script via bash heredoc (no Go/Python quote wars).
	return fmt.Sprintf(`python3 - <<'PY'
import pathlib, re
inc = "/work/%s"
ps = list(pathlib.Path(".").glob("frida-*-mingw.txt"))
if not ps:
    raise SystemExit("no frida-*-mingw.txt after configure")
def fix(m):
    key, val = m.group(1), m.group(2)
    if inc in val:
        return m.group(0)
    return f"{key} = ['-include', {inc!r}] + ({val})"
for p in ps:
    s = p.read_text()
    s2 = re.sub(r"^(c_args|cpp_args)\s*=\s*(.+)$", fix, s, flags=re.M)
    p.write_text(s2)
    print("[fridare] MinGW cross-file DNS include:", p)
PY`, headerFileName)
}
