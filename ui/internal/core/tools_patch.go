package core

import (
	"fmt"
	"strings"
)

// ValidateMagicName enforces the shipped hexreplace / frida-tools contract:
// exactly 5 lowercase English letters a-z (same as hexreplace isStringAlpha).
func ValidateMagicName(name string) error {
	if len(name) != 5 {
		return fmt.Errorf("魔改名称必须是恰好 5 个小写字母 a-z，当前长度 %d: %q", len(name), name)
	}
	for i, c := range name {
		if c < 'a' || c > 'z' {
			return fmt.Errorf("魔改名称第 %d 个字符 %q 无效，只能是小写字母 a-z", i+1, c)
		}
	}
	return nil
}

// PatchCorePyRPC replaces the frida RPC channel name in core.py content.
// Only "frida:rpc" should be changed — never bare "frida", which would break
// imports like `import _frida` if applied to package sources.
//
// magicName must pass ValidateMagicName; invalid names return an error (never a silent no-op success).
func PatchCorePyRPC(content, magicName string) (patched string, replacements int, err error) {
	if err := ValidateMagicName(magicName); err != nil {
		return content, 0, err
	}
	old := "frida:rpc"
	new := magicName + ":rpc"
	if !strings.Contains(content, old) {
		return content, 0, nil
	}
	replacements = strings.Count(content, old)
	return strings.ReplaceAll(content, old, new), replacements, nil
}

// WouldBreakFridaImport reports whether a naive global replace of "frida" with
// magicName would corrupt a typical frida package __init__/import line.
// Used as a regression guard for the GUI tools tab behavior.
func WouldBreakFridaImport(content, magicName string) bool {
	naively := strings.ReplaceAll(content, "frida", magicName)
	// e.g. import _frida -> import _abcde
	return strings.Contains(content, "_frida") && strings.Contains(naively, "_"+magicName) && !strings.Contains(content, "_"+magicName)
}
