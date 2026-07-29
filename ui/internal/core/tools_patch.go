package core

import "strings"

// PatchCorePyRPC replaces the frida RPC channel name in core.py content.
// Only "frida:rpc" (or already-patched "xxxxx:rpc" when re-patching from a
// known original) should be changed — never bare "frida", which would break
// imports like `import _frida` if applied to package sources.
//
// magicName must be the 5-letter name used for the server binary.
func PatchCorePyRPC(content, magicName string) (patched string, replacements int) {
	if len(magicName) != 5 {
		return content, 0
	}
	old := "frida:rpc"
	new := magicName + ":rpc"
	if !strings.Contains(content, old) {
		return content, 0
	}
	replacements = strings.Count(content, old)
	return strings.ReplaceAll(content, old, new), replacements
}

// WouldBreakFridaImport reports whether a naive global replace of "frida" with
// magicName would corrupt a typical frida package __init__/import line.
// Used as a regression guard for the GUI tools tab behavior.
func WouldBreakFridaImport(content, magicName string) bool {
	naively := strings.ReplaceAll(content, "frida", magicName)
	// e.g. import _frida -> import _abcde
	return strings.Contains(content, "_frida") && strings.Contains(naively, "_"+magicName) && !strings.Contains(content, "_"+magicName)
}
