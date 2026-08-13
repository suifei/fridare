package rebuild

import "strings"

// productOnlyCloneRefs maps Fridare product version labels that are not
// official Frida git tags onto the upstream tag to clone.
var productOnlyCloneRefs = map[string]string{
	"17.17.1": "17.17.0",
}

// EffectiveCloneRef is the git --branch used by the GUI rebuild clone step.
func EffectiveCloneRef(cfg JobConfig) string {
	if s := strings.TrimSpace(cfg.CloneRef); s != "" {
		return s
	}
	v := strings.TrimSpace(cfg.FridaVersion)
	if alt, ok := productOnlyCloneRefs[v]; ok {
		return alt
	}
	return v
}

// ApplyGUIRebuildStealthDefaults applies the source-rebuild tab defaults:
// deep profile, strip/junk/markers on. RandomAgentPrefix is left to the caller.
func ApplyGUIRebuildStealthDefaults(cfg *JobConfig) {
	if cfg == nil {
		return
	}
	if strings.TrimSpace(cfg.DirectionProfile) == "" {
		cfg.DirectionProfile = "deep"
	}
	cfg.StripSymbols = true
	cfg.DisableSymbolStrip = false
	cfg.DisableJunk = false
	cfg.DisableStealthMarkers = false
	if cfg.ListenPort <= 0 {
		cfg.ListenPort = OfficialListenPort
	}
	if cfg.MinDiskGB <= 0 {
		cfg.MinDiskGB = 10
	}
	if strings.TrimSpace(cfg.DockerImage) == "" {
		cfg.DockerImage = DefaultBuildImage
	}
	if strings.TrimSpace(cfg.DockerMirror) == "" {
		cfg.DockerMirror = DefaultDockerHubMirror
	}
	cfg.DockerPullDirect = true
}
