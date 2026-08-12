package rebuild

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Deep profile: source-level stealth beyond DefaultModOps, without breaking valac C ABI.
//
// Strategy:
//  1. DeepModOps — extra auto string/path markers (hyphen, paths, inject, tmp prefixes).
//  2. ReplaceFridaMarkersInStringLiterals — rewrite frida_* / Frida only inside quotes,
//     so frida_agent_main as an identifier is left alone but "frida_agent_main" in
//     detection strings / logs is rewritten (common anti-Frida scan surface).
//  3. BuildDeepDigPlan — turn scan hits into Agent-executable dig tasks for L8–L10.

// LayerDeepStringLiterals is auto under profile=deep (quoted markers only).
const LayerDeepStringLiterals StripLayerID = "L11_quoted_string_markers"

// LayerDeepPaths is auto under profile=deep (path/tmp/lib prefixes).
const LayerDeepPaths StripLayerID = "L12_path_lib_inject"

// DeepModOps returns DefaultModOps plus deeper product/path renames and server+client
// protocol/API string pairs (re.frida.* / "Frida.). Safe for Frida 17.x trees when magic
// is 5 letters. Host wheels are patched with the same pairs in BuildPatchedFridaToolsWheels.
func DeepModOps(magicName string, port int) []ModOp {
	ops := DefaultModOps(magicName, port)
	if magicName == "" || magicName == "frida" {
		return ops
	}
	pas := pascalMagic(magicName)
	// Do NOT replace bare "frida-" (would rename subprojects/frida-core and break meson).
	// Do NOT replace libfrida- globally (library sonames / pkg-config). Prefer quoted L11 strip.
	extra := []ModOp{
		{Path: "**/*", Operation: "replace", Description: "frida-inject basename", Find: "frida-inject", Replace: magicName + "-inject"},
		// Runtime tmp paths only — NEVER bare "/frida/" (would smash github.com/frida/* wrap URLs).
		{Path: "**/*", Operation: "replace", Description: "/tmp/frida path", Find: "/tmp/frida", Replace: "/tmp/" + magicName},
		{Path: "**/*", Operation: "replace", Description: "\\tmp\\frida path", Find: "\\tmp\\frida", Replace: "\\tmp\\" + magicName},
		{Path: "**/*", Operation: "replace", Description: "frida-server-main-loop", Find: "frida-server-main-loop", Replace: magicName + "-server-main-loop"},
		// quoted brand / engine id (exact quoted token)
		{Path: "**/*", Operation: "replace", Description: "G_LOG_DOMAIN \"Frida\"", Find: "\"Frida\"", Replace: "\"" + pas + "\""},
		{Path: "**/*", Operation: "replace", Description: "FridaScriptEngine", Find: "FridaScriptEngine", Replace: pas + "ScriptEngine"},
		// Server+client protocol/API sync (same length when len(magic)==5).
		// Host client must use catalog wheels (PatchClientProtocolSurface full=true).
		{Path: "**/*", Operation: "replace", Description: "protocol re.frida. → re." + magicName + ".", Find: "re.frida.", Replace: "re." + magicName + "."},
		// DBus object paths (slash form) — critical for HostSession proxy; not the same as re.frida.
		{Path: "**/*", Operation: "replace", Description: "object path /re/frida/ → /re/" + magicName + "/", Find: "/re/frida/", Replace: "/re/" + magicName + "/"},
		{Path: "**/*", Operation: "replace", Description: "API \"Frida. → \"" + pas + ".", Find: "\"Frida.", Replace: "\"" + pas + "."},
		{Path: "**/*", Operation: "replace", Description: "API 'Frida. → '" + pas + ".", Find: "'Frida.", Replace: "'" + pas + "."},
	}
	return append(ops, extra...)
}

// RepairFridaGitWraps restores github.com/frida/ URLs smashed by accidental
// github.com/frida/ → github.com/{magic}/ replacements in meson wraps AND go.mod.
// Safe no-op when clean.
func RepairFridaGitWraps(sourceDir, magic string) (int, error) {
	if sourceDir == "" || magic == "" || magic == "frida" {
		return 0, nil
	}
	bad := "github.com/" + magic + "/"
	good := "github.com/frida/"
	n := 0
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		base := strings.ToLower(info.Name())
		// meson wraps + Go modules/sources + npm/lock that may embed github.com/frida/*
		if !(strings.HasSuffix(base, ".wrap") || base == "go.mod" || base == "go.sum" ||
			strings.HasSuffix(base, ".go") ||
			base == "package.json" || base == "package-lock.json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Contains(data, []byte(bad)) {
			return nil
		}
		out := bytes.ReplaceAll(data, []byte(bad), []byte(good))
		if err := os.WriteFile(path, out, info.Mode()); err != nil {
			return err
		}
		n++
		return nil
	})
	return n, err
}

func pascalMagic(magic string) string {
	if magic == "" {
		return "Frida"
	}
	return strings.ToUpper(magic[:1]) + magic[1:]
}

// DeepDirectionLayers appends L11–L13 for deep profile documentation/scan.
func DeepDirectionLayers() []StripLayer {
	return []StripLayer{
		{
			ID: LayerDeepStringLiterals, Mode: StripModeAuto,
			Title:       "引号内 frida 标记（字面量）",
			Description: "改产品/RPC 字符串；deep 同时改 re.frida.* → re.{magic}.* 与 \"Frida. → \"{Magic}.（须配套 host 客户端）。",
			Patterns:    []string{`"frida_`, `"frida:`, `re.frida.`, `"Frida.`},
		},
		{
			ID: LayerDeepPaths, Mode: StripModeAuto,
			Title:       "路径 / inject 前缀",
			Description: "/frida/、frida-inject、server-main-loop 等（不全局替换 frida- 以免毁掉 frida-core 目录名）。",
			Patterns:    []string{"/frida/", "frida-inject"},
		},
		{
			ID: LayerIdentifiers, Mode: StripModeAuto,
			Title:       "函数 / 类 / 命名空间标识符",
			Description: "token 级重命名 frida_*、Frida*、FRIDA_*、namespace Frida → magic；声明与引用一并改。profile=deep|abi|explore|full。",
			Patterns:    []string{"frida_agent_main", "namespace Frida", "Frida."},
		},
	}
}

// OpsForProfile returns replace ops for safe | deep | explore | abi | full.
func OpsForProfile(profile, magic string, port int) []ModOp {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "deep", "explore", "abi", "full":
		return DeepModOps(magic, port)
	default:
		return DefaultModOps(magic, port)
	}
}

// StringRewriteOpts controls which string markers structure-aware rewrite may touch.
type StringRewriteOpts struct {
	// ProtocolAPI: rewrite re.frida.* DBus/protocol and "Frida.*" public API string prefixes.
	// Requires matching host client (tools/wheel) rebuild — stock frida-tools will not connect.
	ProtocolAPI bool
}

// quotedFridaReplacements builds pairs for inside string literals.
// magic len 5 keeps re.frida. / re.{magic}. same length for binary patch compatibility.
func quotedFridaReplacements(magic string) [][2]string {
	return quotedFridaReplacementsOpts(magic, StringRewriteOpts{})
}

func quotedFridaReplacementsOpts(magic string, opts StringRewriteOpts) [][2]string {
	if magic == "" {
		magic = "xxxxx"
	}
	pas := pascalMagic(magic)
	// Product/RPC markers first (longer / more specific).
	pairs := [][2]string{
		{"frida:rpc", magic + ":rpc"},
		{"frida_agent_main", magic + "_agent_main"},
		{"frida_server_main", magic + "_server_main"},
		{"frida_agent", magic + "_agent"},
		{"frida_server", magic + "_server"},
		{"frida_gadget", magic + "_gadget"},
		{"frida_helper", magic + "_helper"},
		{"frida-server", magic + "-server"},
		{"frida-agent", magic + "-agent"},
		{"frida-gadget", magic + "-gadget"},
		{"frida-helper", magic + "-helper"},
		{"frida-inject", magic + "-inject"},
		{"gum-js-loop", magic + "-js-loop"},
		{"pool-frida", "pool-" + magic},
	}
	if opts.ProtocolAPI {
		// Server+client must both use these. Same length when len(magic)==5.
		// Dotted: DBus interface names. Slashed: DBus object paths (/re/frida/HostSession).
		pairs = append([][2]string{
			{"re.frida.", "re." + magic + "."},
			{"/re/frida/", "/re/" + magic + "/"},
			{"re.frida", "re." + magic}, // rare bare suffix
			{"\"Frida.", "\"" + pas + "."},
			{"'Frida.", "'" + pas + "."},
		}, pairs...)
	}
	return pairs
}

// ReplaceFridaMarkersInStringLiterals is the legacy name for C-like structure-aware
// rewrite (comments + string/char literals only). Prefer StructureAwareRewrite by path.
func ReplaceFridaMarkersInStringLiterals(content, magic string) (string, int) {
	return RewriteCSource(content, magic)
}

func replacePairsInSegment(seg string, pairs [][2]string) (string, int) {
	n := 0
	for _, p := range pairs {
		if p[0] == p[1] || p[0] == "" {
			continue
		}
		if !strings.Contains(seg, p[0]) {
			continue
		}
		// Never apply bare brand tokens (would smash every "frida" substring).
		// Protocol/API are rewritten only via explicit re.frida. / "Frida. pairs when enabled.
		if p[0] == "frida" || p[0] == "Frida" || p[0] == "FRIDA" {
			continue
		}
		c := strings.Count(seg, p[0])
		seg = strings.ReplaceAll(seg, p[0], p[1])
		n += c
	}
	return seg, n
}

// ApplyDeepSourceExtras runs after Default/DeepModOps content replace + asset renames:
// structure-aware string rewrite + identifier/namespace rename (deep/explore/abi/full) + dig brief.
// No-op for profile=safe (except LF normalize is always cheap/safe when called from Stub/Grok).
func ApplyDeepSourceExtras(sourceDir string, cfg JobConfig) error {
	if sourceDir == "" {
		return nil
	}
	// Always strip CRLF from text sources after Host (Windows) edits so Linux Docker shebangs work.
	if n, err := NormalizeSourceTreeLF(sourceDir); err == nil && n > 0 {
		_ = os.WriteFile(filepath.Join(sourceDir, "fridare-lf-normalize.txt"),
			[]byte(fmt.Sprintf("crlf_files_fixed=%d\n", n)), 0644)
	}
	// Restore meson wrap clone URLs if an older path-op smashed github.com/frida/ → github.com/{magic}/.
	if n, err := RepairFridaGitWraps(sourceDir, cfg.MagicName); err == nil && n > 0 {
		_ = os.WriteFile(filepath.Join(sourceDir, "fridare-wrap-repair.txt"),
			[]byte(fmt.Sprintf("wrap_files_repaired=%d magic=%s\n", n, cfg.MagicName)), 0644)
	}
	// MinGW Windows cross: old headers / inject.xcent basename mismatches.
	_, _ = ApplyMinGWCompatPatches(sourceDir)
	prof := strings.ToLower(strings.TrimSpace(cfg.DirectionProfile))
	if prof != "deep" && prof != "explore" && prof != "abi" && prof != "full" {
		return nil
	}
	var ft, n int
	var err error
	if ProfileRenamesIdentifiers(prof) {
		// abi/full/explore: strings + function/class/namespace identifiers (token-level)
		ft, n, err = ApplyIdentifierRenameStrip(sourceDir, cfg.MagicName)
		if err != nil {
			return fmt.Errorf("identifier/namespace rename: %w", err)
		}
	} else {
		// deep (default): product/RPC/protocol/API string surface only — keeps compile tree stable.
		// Server+client protocol sync (re.frida.* / Frida.* / frida:rpc) without renaming C ABI / meson vars.
		ft, n, err = ApplyStructureAwareStripOpts(sourceDir, cfg.MagicName, StringRewriteOpts{ProtocolAPI: true})
		if err != nil {
			return fmt.Errorf("deep string-literal strip: %w", err)
		}
	}
	// Re-normalize: rewrite may re-read/write; keep shebangs LF for Docker.
	_, _ = NormalizeSourceTreeLF(sourceDir)
	if n > 0 || ft > 0 {
		_ = os.WriteFile(filepath.Join(sourceDir, "fridare-deep-strip.txt"),
			[]byte(fmt.Sprintf("files=%d replacements=%d magic=%s profile=%s idents=%v protocol_api=1\n",
				ft, n, cfg.MagicName, prof, ProfileRenamesIdentifiers(prof))), 0644)
	}
	if rep, err := ScanFridaMarkers(sourceDir, cfg.MagicName); err == nil {
		tasks := BuildDeepDigTasks(rep, cfg.MagicName)
		_ = os.WriteFile(filepath.Join(sourceDir, "fridare-deep-dig.md"),
			[]byte(FormatDeepDigBrief(tasks, cfg.MagicName)), 0644)
		if residual, err := ResidualFridaWordCount(sourceDir); err == nil {
			_ = os.WriteFile(filepath.Join(sourceDir, "fridare-residual-frida.txt"),
				[]byte(fmt.Sprintf("residual_word_frida=%d\n(after deep auto layers; AI dig L8-L10 for remainder)\n", residual)), 0644)
		}
	}
	return nil
}

// NormalizeSourceTreeLF converts CRLF → LF for text sources so Linux Docker shebangs
// (`#!/usr/bin/env python3`) do not become `python3\r` (exit 127).
func NormalizeSourceTreeLF(sourceDir string) (filesFixed int, err error) {
	if sourceDir == "" {
		return 0, fmt.Errorf("sourceDir empty")
	}
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if info.Size() == 0 || info.Size() > 4*1024*1024 {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return nil
		}
		base := filepath.Base(path)
		ext := strings.ToLower(filepath.Ext(path))
		if !textFileExts[ext] && base != "meson.build" && base != "configure" && base != "makefile" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !bytes.Contains(data, []byte("\r\n")) && !bytes.Contains(data, []byte("\r")) {
			return nil
		}
		// Prefer CRLF→LF then bare CR→LF
		out := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
		out = bytes.ReplaceAll(out, []byte("\r"), []byte("\n"))
		if bytes.Equal(out, data) {
			return nil
		}
		if err := os.WriteFile(path, out, info.Mode()); err != nil {
			return err
		}
		filesFixed++
		return nil
	})
	return filesFixed, err
}

// ApplyDeepStringLiteralStrip walks sourceDir and applies per-language structure-aware
// rewriters (C lex, Python lex/AST, Vala-as-C-lex). Mechanical safety layer for deep profile.
func ApplyDeepStringLiteralStrip(sourceDir, magic string) (filesTouched, replacements int, err error) {
	return ApplyStructureAwareStrip(sourceDir, magic)
}

// DeepDigTask is one AI/file-level dig unit derived from scan.
type DeepDigTask struct {
	Layer       StripLayerID `json:"layer"`
	Mode        StripMode    `json:"mode"`
	Path        string       `json:"path"`
	Pattern     string       `json:"pattern"`
	Count       int          `json:"count"`
	Suggestion  string       `json:"suggestion"`
	AutoApply   bool         `json:"auto_apply"`
	Risk        string       `json:"risk"`
}

// BuildDeepDigTasks converts a ScanReport into prioritized dig tasks for the Agent.
func BuildDeepDigTasks(rep ScanReport, magic string) []DeepDigTask {
	var tasks []DeepDigTask
	for _, h := range rep.Hits {
		t := DeepDigTask{
			Layer: h.Layer, Mode: h.Mode, Path: h.Path, Pattern: h.Pattern, Count: h.Count,
		}
		switch h.Mode {
		case StripModeAuto, StripModePostBuild:
			t.AutoApply = true
			t.Suggestion = fmt.Sprintf("由 Default/DeepModOps 或 post-build 处理: %s → magic=%s", h.Pattern, magic)
			t.Risk = "low"
		case StripModeForbidden:
			t.AutoApply = false
			t.Suggestion = "禁止源码标识符级替换；依赖引号内替换(L11)或 post-build 二进制补丁(L5)"
			t.Risk = "compile_break"
		case StripModeAIExplore:
			t.AutoApply = false
			t.Suggestion = fmt.Sprintf("AI 深挖：评估是否在仅本文件上下文替换 %q，并保证编译与客户端协议", h.Pattern)
			t.Risk = "protocol_or_api"
		}
		tasks = append(tasks, t)
	}
	return tasks
}

// FormatDeepDigBrief is a human/agent readable dig brief.
func FormatDeepDigBrief(tasks []DeepDigTask, magic string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Fridare deep dig brief (magic=%s)\n\n", magic))
	b.WriteString("## 目标\n在源码层尽量削弱 frida 可指纹面；标识符 ABI 不动，字符串/路径/产物名优先。\n\n")
	auto, forb, explore := 0, 0, 0
	for _, t := range tasks {
		switch t.Mode {
		case StripModeAuto, StripModePostBuild:
			auto++
		case StripModeForbidden:
			forb++
		case StripModeAIExplore:
			explore++
		}
	}
	b.WriteString(fmt.Sprintf("任务统计: auto/post=%d forbidden=%d ai_explore=%d total=%d\n\n", auto, forb, explore, len(tasks)))
	b.WriteString("## 优先执行（auto）\n")
	n := 0
	for _, t := range tasks {
		if !t.AutoApply {
			continue
		}
		if n >= 40 {
			b.WriteString("…\n")
			break
		}
		b.WriteString(fmt.Sprintf("- [%s] %s %q ×%d — %s\n", t.Layer, t.Path, t.Pattern, t.Count, t.Suggestion))
		n++
	}
	b.WriteString("\n## AI 深挖候选（explore）\n")
	n = 0
	for _, t := range tasks {
		if t.Mode != StripModeAIExplore {
			continue
		}
		if n >= 30 {
			b.WriteString("…\n")
			break
		}
		b.WriteString(fmt.Sprintf("- [%s] %s %q ×%d risk=%s — %s\n", t.Layer, t.Path, t.Pattern, t.Count, t.Risk, t.Suggestion))
		n++
	}
	b.WriteString("\n## 禁止 auto（forbidden）\n")
	n = 0
	for _, t := range tasks {
		if t.Mode != StripModeForbidden {
			continue
		}
		if n >= 20 {
			b.WriteString("…\n")
			break
		}
		b.WriteString(fmt.Sprintf("- [%s] %s %q — %s\n", t.Layer, t.Path, t.Pattern, t.Suggestion))
		n++
	}
	return b.String()
}

// ResidualFridaWordCount counts bare word "frida"/"Frida"/"FRIDA" outside our bookkeeping files.
// Used after deep strip to measure remaining surface for AI dig.
func ResidualFridaWordCount(sourceDir string) (int, error) {
	re := regexp.MustCompile(`(?i)\bfrida\b`)
	total := 0
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.Contains(path, ".git") || strings.HasPrefix(filepath.Base(path), "fridare-") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !textFileExts[ext] {
			return nil
		}
		if info.Size() > 2*1024*1024 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		total += len(re.FindAll(data, -1))
		return nil
	})
	return total, err
}

// LooksLikeIdentifier reports whether a match position is likely a C-like identifier
// (not used externally yet; kept for future gated L6 experiments).
func LooksLikeIdentifier(s string, start, end int) bool {
	if start > 0 {
		r := rune(s[start-1])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return true
		}
	}
	if end < len(s) {
		r := rune(s[end])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return true
		}
	}
	return false
}
