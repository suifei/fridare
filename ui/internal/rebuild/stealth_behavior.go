package rebuild

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// StealthSeedHex is SHA-256(magic+"|"+buildID) hex; empty buildID → "0".
func StealthSeedHex(magic, buildID string) string {
	if strings.TrimSpace(buildID) == "" {
		buildID = "0"
	}
	if magic == "" {
		magic = "frida"
	}
	sum := sha256.Sum256([]byte(magic + "|" + buildID))
	return hex.EncodeToString(sum[:8])
}

// AgentDiskPrefix is the on-disk agent basename prefix.
// When random is false it is the magic name; when true it is 5 a-z from the seed.
func AgentDiskPrefix(magic, buildID string, random bool) string {
	if !random {
		if magic == "" {
			return "frida"
		}
		return magic
	}
	seed := StealthSeedHex(magic, buildID)
	var b strings.Builder
	for i := 0; i < 5; i++ {
		// map hex nibble pairs into a-z
		v := int(seed[i*2]) + int(seed[i*2+1])
		b.WriteByte(byte('a' + (v % 26)))
	}
	return b.String()
}

// StealthBehaviorOps returns extra source replacements for stealth markers.
// Random agent on-disk prefix is included only when cfg.RandomAgentPrefix.
// Does not emit protocol re.frida. pairs. Tree apply still skips vendor dirs.
func StealthBehaviorOps(cfg JobConfig) []ModOp {
	magic := strings.TrimSpace(cfg.MagicName)
	if magic == "" || magic == "frida" {
		return nil
	}
	ops := []ModOp{
		// maps / injector (same length: linjector = 9, magic+ctor = 9 when len(magic)==5)
		{Path: "**/*", Operation: "replace", Description: "maps linjector", Find: "linjector", Replace: magic + "ctor"},
		// unix socket brand
		{Path: "**/*", Operation: "replace", Description: "unix:frida socket", Find: "unix:frida", Replace: "unix:" + magic},
		{Path: "**/*", Operation: "replace", Description: "abstract frida socket", Find: "abstract/frida", Replace: "abstract/" + magic},
		// /proc maps path crumbs
		{Path: "**/*", Operation: "replace", Description: "maps /frida-", Find: "/frida-", Replace: "/" + magic + "-"},
		// SELinux
		{Path: "**/*", Operation: "replace", Description: "SELinux u:object_r:frida", Find: "u:object_r:frida", Replace: "u:object_r:" + magic},
		{Path: "**/*", Operation: "replace", Description: "SELinux u:r:frida", Find: "u:r:frida", Replace: "u:r:" + magic},
		{Path: "**/*", Operation: "replace", Description: "SELinux frida_file", Find: "frida_file", Replace: magic + "_file"},
		// memfd
		{Path: "**/*", Operation: "replace", Description: "memfd:frida", Find: "memfd:frida", Replace: "memfd:" + magic},
	}
	if cfg.RandomAgentPrefix {
		pfx := AgentDiskPrefix(magic, cfg.BuildID, true)
		// on-disk dump names only — not the product basename meson target (already magic)
		ops = append(ops,
			ModOp{Path: "**/*", Operation: "replace", Description: "random agent dump prefix", Find: "frida-agent-", Replace: pfx + "-agent-"},
			ModOp{Path: "**/*", Operation: "replace", Description: "random memfd agent", Find: "memfd:" + magic + "-agent", Replace: "memfd:" + pfx + "-agent"},
		)
	}
	return ops
}
