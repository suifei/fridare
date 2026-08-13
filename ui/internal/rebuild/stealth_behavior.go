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
		// Quoted maps token only — never meson 'linjector.vala' / linjector-glue.c / identifiers.
		{Path: "**/*", Operation: "replace", Description: "maps \"linjector\"", Find: `"linjector"`, Replace: `"` + magic + `ctor"`},
		// unix socket brand
		{Path: "**/*", Operation: "replace", Description: "unix:frida socket", Find: "unix:frida", Replace: "unix:" + magic},
		{Path: "**/*", Operation: "replace", Description: "abstract frida socket", Find: "abstract/frida", Replace: "abstract/" + magic},
		// Quoted /frida- socket/maps prefixes only (bare /frida- smashes github.com/frida/frida-core).
		{Path: "**/*", Operation: "replace", Description: "quoted /frida- path", Find: `"/frida-"`, Replace: `"/` + magic + `-"`},
		{Path: "**/*", Operation: "replace", Description: "frida-zymbiote socket", Find: "/frida-zymbiote-", Replace: "/" + magic + "-zymbiote-"},
		// SELinux type names (same length when len(magic)==5)
		{Path: "**/*", Operation: "replace", Description: "SELinux u:object_r:frida", Find: "u:object_r:frida", Replace: "u:object_r:" + magic},
		{Path: "**/*", Operation: "replace", Description: "SELinux u:r:frida", Find: "u:r:frida", Replace: "u:r:" + magic},
		{Path: "**/*", Operation: "replace", Description: "SELinux \"frida_file\"", Find: `"frida_file"`, Replace: `"` + magic + `_file"`},
		{Path: "**/*", Operation: "replace", Description: "SELinux \"frida_memfd\"", Find: `"frida_memfd"`, Replace: `"` + magic + `_memfd"`},
		// memfd name prefix
		{Path: "**/*", Operation: "replace", Description: "memfd:frida", Find: "memfd:frida", Replace: "memfd:" + magic},
	}
	if cfg.RandomAgentPrefix {
		ops = append(ops, randomAgentDiskOps(magic, cfg.BuildID)...)
	}
	return ops
}

// agentDumpSOSuffixes are runtime dump / memfd SO names only.
// Do NOT use a bare "frida-agent-" find — that rewrites meson
// frida-agent-android.version while RenameMagicAssetFiles still uses magic.
var agentDumpSOSuffixes = []string{
	"-64.so", "-32.so", "-arm.so", "-arm64.so", "-armhf.so",
	"-64.dylib", "-32.dylib",
}

// randomAgentDiskOps must run *before* DefaultModOps (frida-agent → {magic}-agent).
// PlanModsFromTree prepends these so expand+apply still see stock dump SO names.
func randomAgentDiskOps(magic, buildID string) []ModOp {
	pfx := AgentDiskPrefix(magic, buildID, true)
	if pfx == "" || pfx == magic {
		return nil
	}
	var ops []ModOp
	for _, suf := range agentDumpSOSuffixes {
		stock := "frida-agent" + suf
		mag := magic + "-agent" + suf
		neu := pfx + "-agent" + suf
		ops = append(ops,
			ModOp{Path: "**/*", Operation: "replace", Description: "random agent dump " + stock, Find: stock, Replace: neu},
			ModOp{Path: "**/*", Operation: "replace", Description: "random agent dump after magic " + mag, Find: mag, Replace: neu},
		)
	}
	ops = append(ops, ModOp{
		Path: "**/*", Operation: "replace", Description: "random memfd agent so",
		Find: "memfd:" + magic + "-agent-", Replace: "memfd:" + pfx + "-agent-",
	})
	return ops
}

// prependRandomAgentOps puts dump-prefix replacements ahead of DefaultModOps.
func prependRandomAgentOps(cfg JobConfig, rest []ModOp) []ModOp {
	if !cfg.RandomAgentPrefix {
		return rest
	}
	magic := strings.TrimSpace(cfg.MagicName)
	if magic == "" || magic == "frida" {
		return rest
	}
	head := randomAgentDiskOps(magic, cfg.BuildID)
	if len(head) == 0 {
		return rest
	}
	return append(head, rest...)
}
