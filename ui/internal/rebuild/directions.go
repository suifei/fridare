package rebuild

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// StripMode says how a layer may be applied.
type StripMode string

const (
	// StripModeAuto: safe for DefaultModOps / naive apply without AI.
	StripModeAuto StripMode = "auto"
	// StripModePostBuild: same-length binary rewrite after make (not source).
	StripModePostBuild StripMode = "post_build"
	// StripModeAIExplore: AI may propose changes; not applied by default naive path.
	StripModeAIExplore StripMode = "ai_explore"
	// StripModeForbidden: must not auto-replace (breaks compile or protocol).
	StripModeForbidden StripMode = "forbidden"
)

// StripLayerID is a stable layer name for tools + Agent direction lists.
type StripLayerID string

const (
	LayerProductBasename StripLayerID = "L1_product_basename" // frida-server → magic-server
	LayerRPCThreadPort   StripLayerID = "L2_rpc_thread_port"  // frida:rpc, gum-js-loop, port
	LayerGResourceGetter StripLayerID = "L3_gresource_getter" // get_frida_agent_*
	LayerAssetBasename   StripLayerID = "L4_asset_basename"   // .version/.symbols rename
	LayerBinaryMarkers   StripLayerID = "L5_binary_markers"   // post-build frida_agent etc.
	LayerUnderscoreCABI  StripLayerID = "L6_underscore_c_abi" // frida_agent_main (valac)
	LayerValaNamespace   StripLayerID = "L7_vala_namespace"   // namespace Frida.Agent
	LayerProtocolReFrida StripLayerID = "L8_protocol_re_frida" // re.frida.*
	LayerPublicJSAPI     StripLayerID = "L9_public_js_api"     // JS Frida global
	LayerGlobalWord      StripLayerID = "L10_global_frida_word" // bare "frida" everywhere
)

// StripLayer describes one direction for batch tools + AI Agent.
type StripLayer struct {
	ID          StripLayerID `json:"id"`
	Mode        StripMode    `json:"mode"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	// Patterns are illustrative scan needles (not all auto-applied).
	Patterns []string `json:"patterns,omitempty"`
}

// DefaultStripLayers is the canonical direction map (community-aligned + compile-safe).
func DefaultStripLayers() []StripLayer {
	return []StripLayer{
		{
			ID: LayerProductBasename, Mode: StripModeAuto,
			Title:       "产品 basename（连字符）",
			Description: "meson 目标/文件名：frida-server|agent|gadget|helper → {magic}-*。与 strongR/phantom 类「改可观测名」重合。",
			Patterns:    []string{"frida-server", "frida-agent", "frida-gadget", "frida-helper"},
		},
		{
			ID: LayerRPCThreadPort, Mode: StripModeAuto,
			Title:       "RPC / 线程 / 端口",
			Description: "frida:rpc、gum-js-loop、frida-main-loop、pool-frida、默认端口 27042。保持客户端需对齐 magic:rpc。",
			Patterns:    []string{"frida:rpc", "gum-js-loop", "frida-main-loop", "pool-frida", "27042"},
		},
		{
			ID: LayerGResourceGetter, Mode: StripModeAuto,
			Title:       "GResource blob getter",
			Description: "get_frida_agent_* / get_frida_helper_* 等与资源 basename 对齐。勿扩展为裸 frida_agent_。",
			Patterns:    []string{"get_frida_agent", "get_frida_helper", "get_frida_gadget", "get_frida_server"},
		},
		{
			ID: LayerAssetBasename, Mode: StripModeAuto,
			Title:       "资源文件 basename",
			Description: "RenameMagicAssetFiles：frida-agent.version/.symbols 等 → magic 前缀，避免 meson 缺文件。",
			Patterns:    []string{"frida-agent.", "frida-server.", "frida-gadget."},
		},
		{
			ID: LayerBinaryMarkers, Mode: StripModePostBuild,
			Title:       "编译后二进制同长度标记",
			Description: "PatchArtifactBinaryMarkers：frida_agent/frida_server/frida:rpc 等写入产物（valac 仍发 frida_agent_main）。",
			Patterns:    []string{"frida_agent", "frida_server", "frida-server", "frida:rpc"},
		},
		{
			ID: LayerUnderscoreCABI, Mode: StripModeForbidden,
			Title:       "下划线 C ABI（valac 符号）",
			Description: "裸 frida_agent_* / frida_server_* 与 namespace Frida.Agent 生成符号绑定；全量源码替换会 link 失败。仅 post_build 或 AI+编译门禁。",
			Patterns:    []string{"frida_agent_main", "frida_agent_", "frida_server_"},
		},
		{
			ID: LayerValaNamespace, Mode: StripModeForbidden,
			Title:       "Vala 命名空间",
			Description: "namespace Frida.Agent → X.Agent 会丢父命名空间类型；using Frida 又导致 Error 歧义。禁止 auto。",
			Patterns:    []string{"namespace Frida.Agent", "namespace Frida.Gadget", "Frida.Agent."},
		},
		{
			ID: LayerProtocolReFrida, Mode: StripModeAuto,
			Title:       "协议串 re.frida.*",
			Description: "可改：re.frida. → re.{magic}.（同长度）。须同时魔改 host frida/frida-tools；stock 客户端将无法连接。safe 默认关，deep 开。",
			Patterns:    []string{"re.frida.", "re.frida.server"},
		},
		{
			ID: LayerPublicJSAPI, Mode: StripModeAuto,
			Title:       "公开 Frida.* API 字面量",
			Description: "可改：字符串 \"Frida.xxx\" → \"{Magic}.xxx\"。自有脚本需跟新 API 名；Java.perform 等非 Frida 前缀不动。safe 关，deep 开。",
			Patterns:    []string{"Frida.", "Frida.version"},
		},
		{
			ID: LayerGlobalWord, Mode: StripModeAIExplore,
			Title:       "全局单词 frida（全树）",
			Description: "「完全去掉 frida 字样」的极限目标。不可 auto；需批量扫描分类 + AI 按文件深挖 + 每层编译验证。",
			Patterns:    []string{"frida", "Frida", "FRIDA"},
		},
	}
}

// DirectionManifest is machine-readable direction for Agent + batch tools.
type DirectionManifest struct {
	Magic       string       `json:"magic"`
	Port        int          `json:"port"`
	Profile     string       `json:"profile"` // safe | explore
	Layers      []StripLayer `json:"layers"`
	AgentGoals  string       `json:"agent_goals"`
	Notes       string       `json:"notes,omitempty"`
	GeneratedBy string       `json:"generated_by"`
}

// SafeDirectionManifest builds the default auto-only profile (what DefaultModOps implements).
// Excludes L8 protocol / L9 public API source rewrites — those require matching host client
// and are enabled only under deep|abi|explore|full (DeepDirectionManifest).
func SafeDirectionManifest(magic string, port int) DirectionManifest {
	if magic == "" {
		magic = "frida"
	}
	if port <= 0 {
		port = 27042
	}
	var layers []StripLayer
	for _, L := range DefaultStripLayers() {
		// L8/L9 are Auto in the full map for deep; safe must not list them as auto.
		if L.ID == LayerProtocolReFrida || L.ID == LayerPublicJSAPI {
			continue
		}
		if L.Mode == StripModeAuto || L.Mode == StripModePostBuild {
			layers = append(layers, L)
		}
	}
	return DirectionManifest{
		Magic:   magic,
		Port:    port,
		Profile: "safe",
		Layers:  layers,
		AgentGoals: fmt.Sprintf(
			"方向=safe：应用 L1–L5（basename/rpc/getter/资源/编译后标记）。magic=%s port=%d。禁止 L6/L7 auto；L8 协议 / L9 API / L10 全局词 不改源码（尽量兼容 stock 客户端）。导出时仍会同长度补丁 frida:rpc 等，并打包对齐的 host wheel。",
			magic, port),
		Notes:       "对齐社区「改标识+编 server」；不下沉 Vala 命名空间/裸 C ABI；不源码改 re.frida./Frida.*。",
		GeneratedBy: "fridare/rebuild.SafeDirectionManifest",
	}
}

// DeepDirectionManifest is source-level stealth: DeepModOps + quoted-string strip (L11–L12)
// plus full layer map for AI dig on L8–L10. Forbidden L6/L7 still not auto identifier rewrite.
func DeepDirectionManifest(magic string, port int) DirectionManifest {
	if magic == "" {
		magic = "frida"
	}
	if port <= 0 {
		port = 27042
	}
	layers := append([]StripLayer{}, DefaultStripLayers()...)
	layers = append(layers, DeepDirectionLayers()...)
	return DirectionManifest{
		Magic:   magic,
		Port:    port,
		Profile: "deep",
		Layers:  layers,
		AgentGoals: fmt.Sprintf(
			"方向=deep 服务端+客户端协议同步：\n"+
				"1) DeepModOps（L1–L3 + inject/路径 + re.frida./\"Frida.）\n"+
				"2) L11 结构感知字符串：产品标记 + re.frida.* + Frida.* API + rpc\n"+
				"3) 不自动做 L13 全量标识符重命名（避免破坏 meson/C ABI；需 L13 请用 profile=abi）\n"+
				"4) L4 资源改名 + L5 二进制补丁 + host wheel 全协议面\n"+
				"5) stock pip 客户端将不兼容；必须用 catalog wheel。magic=%s port=%d",
			magic, port),
		Notes:       "deep=服务端+客户端同步协议/API/rpc；标识符全量改名留给 abi/full。目录名 frida-core 不改。",
		GeneratedBy: "fridare/rebuild.DeepDirectionManifest",
	}
}

// ExploreDirectionManifest includes AI-explore layers for prompt guidance (still not auto-applied).
func ExploreDirectionManifest(magic string, port int) DirectionManifest {
	m := DeepDirectionManifest(magic, port)
	m.Profile = "explore"
	m.AgentGoals = fmt.Sprintf(
		"方向=explore：在 deep 基线之上，对 L8–L10 提出可选补丁（须可编译、可回滚）；禁止直接改 L6/L7 标识符。magic=%s。",
		magic)
	m.Notes = "AI 深挖须写出 fridare-agent-ops.tsv；forbidden 层不得写入可执行 ops。"
	m.GeneratedBy = "fridare/rebuild.ExploreDirectionManifest"
	return m
}

// WriteDirectionManifest writes JSON next to source or work dir.
func WriteDirectionManifest(path string, m DirectionManifest) error {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// LoadDirectionManifest reads a direction JSON file.
func LoadDirectionManifest(path string) (DirectionManifest, error) {
	var m DirectionManifest
	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	return m, nil
}

// StripHit is one scan finding classified into a layer.
type StripHit struct {
	Layer   StripLayerID `json:"layer"`
	Mode    StripMode    `json:"mode"`
	Path    string       `json:"path"` // relative
	Pattern string       `json:"pattern"`
	Count   int          `json:"count"`
}

// ScanReport is batch scan output for tools + Agent.
type ScanReport struct {
	Root       string     `json:"root"`
	Magic      string     `json:"magic"`
	Hits       []StripHit `json:"hits"`
	ByLayer    map[string]int `json:"by_layer"`
	AutoOps    int        `json:"auto_ops_available"` // from DefaultModOps if applied
	Notes      string     `json:"notes"`
}

// ClassifyPattern maps a needle to the best strip layer.
func ClassifyPattern(pattern string) (StripLayerID, StripMode) {
	p := pattern
	switch {
	case strings.HasPrefix(p, "get_frida_"):
		return LayerGResourceGetter, StripModeAuto
	case p == "frida:rpc" || p == "gum-js-loop" || p == "frida-main-loop" || p == "pool-frida" || p == "27042":
		return LayerRPCThreadPort, StripModeAuto
	case p == "frida-server" || p == "frida-agent" || p == "frida-gadget" || p == "frida-helper":
		return LayerProductBasename, StripModeAuto
	case strings.HasPrefix(p, "namespace Frida.") || strings.HasPrefix(p, "Frida.Agent.") || strings.HasPrefix(p, "Frida.Gadget."):
		return LayerValaNamespace, StripModeForbidden
	case strings.Contains(p, "re.frida"):
		return LayerProtocolReFrida, StripModeAuto // deep rewrites; safe path may skip
	case strings.HasPrefix(p, "Frida.") || p == "Frida.version":
		return LayerPublicJSAPI, StripModeAuto
	case p == "frida_agent_main" || strings.HasPrefix(p, "frida_agent_") || strings.HasPrefix(p, "frida_server_"):
		return LayerUnderscoreCABI, StripModeForbidden // still forbidden as *global text* auto; token idents OK via L13
	case p == "frida" || p == "Frida" || p == "FRIDA":
		return LayerGlobalWord, StripModeAIExplore
	default:
		if strings.Contains(p, "frida-") {
			return LayerProductBasename, StripModeAuto
		}
		if strings.Contains(p, "frida_") {
			return LayerUnderscoreCABI, StripModeForbidden
		}
		return LayerGlobalWord, StripModeAIExplore
	}
}

// ScanFridaMarkers walks a source tree for direction patterns (text files only).
// Does not modify files. Used by CLI + Agent planning.
func ScanFridaMarkers(sourceDir, magic string) (ScanReport, error) {
	rep := ScanReport{
		Root:    sourceDir,
		Magic:   magic,
		ByLayer: map[string]int{},
		Notes:   "分类命中；auto 层对应 DefaultModOps；forbidden/ai_explore 默认不自动替换",
	}
	if sourceDir == "" {
		return rep, fmt.Errorf("sourceDir empty")
	}
	// Patterns to search (ordered: more specific first for counting)
	needles := []string{
		"get_frida_agent", "get_frida_helper", "get_frida_gadget", "get_frida_server",
		"namespace Frida.Agent", "namespace Frida.Gadget", "Frida.Agent.",
		"frida:rpc", "gum-js-loop", "frida-main-loop", "pool-frida",
		"frida_agent_main", "frida-server", "frida-agent", "frida-gadget", "frida-helper",
		"re.frida.",
	}
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if shouldSkipModDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		// skip huge / binary
		if info.Size() > 4*1024*1024 || info.Size() == 0 {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		base := strings.ToLower(filepath.Base(path))
		// skip Fridare bookkeeping (directions/scan/dig contain pattern strings as metadata)
		if strings.HasPrefix(base, "fridare-") {
			return nil
		}
		if !textFileExts[ext] && base != "meson.build" && base != "makefile" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		// skip obvious binary
		headN := 512
		if len(data) < headN {
			headN = len(data)
		}
		if strings.IndexByte(string(data[:headN]), 0) >= 0 {
			return nil
		}
		s := string(data)
		rel, _ := filepath.Rel(sourceDir, path)
		rel = filepath.ToSlash(rel)
		for _, n := range needles {
			c := strings.Count(s, n)
			if c == 0 {
				continue
			}
			lid, mode := ClassifyPattern(n)
			rep.Hits = append(rep.Hits, StripHit{
				Layer: lid, Mode: mode, Path: rel, Pattern: n, Count: c,
			})
			rep.ByLayer[string(lid)] += c
		}
		return nil
	})
	if err != nil {
		return rep, err
	}
	sort.Slice(rep.Hits, func(i, j int) bool {
		if rep.Hits[i].Layer != rep.Hits[j].Layer {
			return rep.Hits[i].Layer < rep.Hits[j].Layer
		}
		return rep.Hits[i].Path < rep.Hits[j].Path
	})
	rep.AutoOps = len(DefaultModOps(magic, 27042))
	return rep, nil
}

// OpsFromDirectionManifest returns auto source replace ops for the magic/port.
// Profile deep|explore → DeepModOps; else DefaultModOps.
// Forbidden layers never become ops here — Agent must propose those explicitly.
func OpsFromDirectionManifest(m DirectionManifest) []ModOp {
	magic := m.Magic
	if magic == "" {
		magic = "frida"
	}
	port := m.Port
	if port <= 0 {
		port = 27042
	}
	return OpsForProfile(m.Profile, magic, port)
}

// DirectionGoalsPrompt appends layer guidance for BuildAgentPrompt / Grok refine.
func DirectionGoalsPrompt(m DirectionManifest) string {
	var b strings.Builder
	b.WriteString("\n## 魔改方向清单（Fridare Strip Directions）\n")
	b.WriteString(fmt.Sprintf("profile=%s magic=%s\n", m.Profile, m.Magic))
	if m.AgentGoals != "" {
		b.WriteString("目标: " + m.AgentGoals + "\n")
	}
	b.WriteString("\n| 层 | 模式 | 说明 |\n|----|------|------|\n")
	for _, L := range m.Layers {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", L.ID, L.Mode, L.Title))
	}
	b.WriteString("\n规则:\n")
	b.WriteString("- mode=auto：必须执行（与 DefaultModOps 一致）\n")
	b.WriteString("- mode=post_build：不要改源码 ABI；编译后由导出管线补丁\n")
	b.WriteString("- mode=forbidden：禁止写入可执行 replace（会破坏 valac/link）\n")
	b.WriteString("- mode=ai_explore：仅当用户 profile=explore 时可选；写出补丁理由与回滚点\n")
	return b.String()
}

// MergeGoalsWithDirections combines user goals + direction agent goals for JobConfig.Goals.
func MergeGoalsWithDirections(userGoals string, m DirectionManifest) string {
	userGoals = strings.TrimSpace(userGoals)
	ag := strings.TrimSpace(m.AgentGoals)
	if userGoals == "" {
		return ag
	}
	if ag == "" {
		return userGoals
	}
	return userGoals + "\n\n" + ag
}
