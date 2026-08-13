package rebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InjectionABIWhitelistBasenames are injection-adjacent C/Vala sources
// (gum injector, agent entry, Windows/Linux process enumeration).
var InjectionABIWhitelistBasenames = []string{
	"gumprocess-linux.c",
	"gumprocess-windows.c",
	"gumprocess-posix.c",
	"gumprocess.c",
	"agent-glue.c",
	"linjector-glue.c",
	"winjector-glue.c",
	"linjector.vala",
	"winjector.vala",
}

// InjectionABIWhitelistFragments match relative paths that belong on the ABI list.
var InjectionABIWhitelistFragments = []string{
	"gumprocess-linux.c",
	"gumprocess-windows.c",
	"gumprocess-posix.c",
	"/gum/gumprocess.c",
	"linjector",
	"winjector",
	"agent-glue.c",
	"/lib/agent/",
}

// IsInjectionABIPath reports whether rel (slash path) is an injection-adjacent TU.
func IsInjectionABIPath(rel string) bool {
	rel = filepath.ToSlash(rel)
	low := strings.ToLower(rel)
	if strings.Contains(low, "/frida-core/") && strings.HasSuffix(low, "/meson.build") {
		return false
	}
	base := strings.ToLower(filepath.Base(rel))
	for _, b := range InjectionABIWhitelistBasenames {
		if base == strings.ToLower(b) {
			return true
		}
	}
	for _, frag := range InjectionABIWhitelistFragments {
		if strings.Contains(low, strings.ToLower(frag)) {
			return true
		}
	}
	return false
}

// ApplyInjectionABIRename token-renames frida_* / Frida* only on the injection whitelist.
// Does not rename directories. Protocol strings (re.frida.) are left to DeepModOps, not this step.
func ApplyInjectionABIRename(sourceDir, magic string) (filesTouched, replacements int, err error) {
	if sourceDir == "" {
		return 0, 0, fmt.Errorf("sourceDir empty")
	}
	if magic == "" || magic == "frida" {
		return 0, 0, nil
	}
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if info.Size() == 0 || info.Size() > 4*1024*1024 {
			return nil
		}
		rel, _ := filepath.Rel(sourceDir, path)
		rel = filepath.ToSlash(rel)
		if !IsInjectionABIPath(rel) {
			return nil
		}
		if skipIdentifierRenamePath(path) {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, "fridare-") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !textFileExts[ext] && base != "meson.build" && base != "makefile" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if len(data) > 0 {
			headN := 512
			if len(data) < headN {
				headN = len(data)
			}
			if strings.IndexByte(string(data[:headN]), 0) >= 0 {
				return nil
			}
		}
		// ProtocolAPI false: this step must not rewrite re.frida.
		res := StructureAwareRewriteAllOpts(path, string(data), magic, true, StringRewriteOpts{ProtocolAPI: false})
		if res.Replacements == 0 || res.Content == string(data) {
			return nil
		}
		if err := os.WriteFile(path, []byte(res.Content), info.Mode()); err != nil {
			return err
		}
		filesTouched++
		replacements += res.Replacements
		return nil
	})
	return filesTouched, replacements, err
}
