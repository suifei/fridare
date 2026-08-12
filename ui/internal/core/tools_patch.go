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
	return PatchClientProtocolSurface(content, magicName, false)
}

// PatchClientProtocolSurface rewrites host-client Python (or any text) so it matches
// a magic-renamed server. When full is false: only frida:rpc.
// When full is true (server+client sync): also re.frida.* interface names,
// /re/frida/ object paths, and Frida.* API prefixes.
// Never applies bare "frida" substring (would break import _frida).
func PatchClientProtocolSurface(content, magicName string, full bool) (patched string, replacements int, err error) {
	if err := ValidateMagicName(magicName); err != nil {
		return content, 0, err
	}
	out := content
	n := 0
	add := func(old, neu string) {
		if old == "" || old == neu || !strings.Contains(out, old) {
			return
		}
		c := strings.Count(out, old)
		out = strings.ReplaceAll(out, old, neu)
		n += c
	}
	add("frida:rpc", magicName+":rpc")
	if full {
		pas := strings.ToUpper(magicName[:1]) + magicName[1:]
		// DBus interface names: re.frida.HostSession17 → re.{magic}.HostSession17
		add("re.frida.", "re."+magicName+".")
		add("re.frida", "re."+magicName) // rare bare
		// DBus object paths: /re/frida/HostSession → /re/{magic}/HostSession
		// (slash form; NOT the same as re.frida. dotted interfaces)
		add("/re/frida/", "/re/"+magicName+"/")
		add("Frida.", pas+".")
	}
	return out, n, nil
}

// ClientProtocolBinaryPairs returns same-length byte pairs for native client modules
// (.pyd/.so) and server binaries when len(magic)==5.
func ClientProtocolBinaryPairs(magicName string) ([][2][]byte, error) {
	if err := ValidateMagicName(magicName); err != nil {
		return nil, err
	}
	pas := strings.ToUpper(magicName[:1]) + magicName[1:]
	pairs := [][2][]byte{
		{[]byte("frida:rpc"), []byte(magicName + ":rpc")},
		{[]byte("re.frida."), []byte("re." + magicName + ".")},
		// object path segment (same length as re.frida. pair family when magic len==5)
		{[]byte("/re/frida/"), []byte("/re/" + magicName + "/")},
		{[]byte("Frida."), []byte(pas + ".")},
	}
	for _, p := range pairs {
		if len(p[0]) != len(p[1]) {
			return nil, fmt.Errorf("magic %q length mismatch for pair %q", magicName, p[0])
		}
	}
	return pairs, nil
}

// WouldBreakFridaImport reports whether a naive global replace of "frida" with
// magicName would corrupt a typical frida package __init__/import line.
// Used as a regression guard for the GUI tools tab behavior.
func WouldBreakFridaImport(content, magicName string) bool {
	naively := strings.ReplaceAll(content, "frida", magicName)
	// e.g. import _frida -> import _abcde
	return strings.Contains(content, "_frida") && strings.Contains(naively, "_"+magicName) && !strings.Contains(content, "_"+magicName)
}
