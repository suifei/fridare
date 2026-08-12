package rebuild

// Recommended OpenAI-compatible endpoint guidance (shown in Settings / Rebuild UI).
const (
	RecommendedSiteURL   = "https://claudegpt.org/"
	RecommendedAPIBase   = "https://claudegpt.org/v1"
	RecommendedQQGroup   = "555354813"
	RecommendedQRCodeURL = "https://github.com/suifei/artificer-deck/blob/main/public/assets/ui/qq-group.png"

	// Frida upstream repository used for shallow clones.
	FridaGitURL = "https://github.com/frida/frida.git"

	// Default minimum free disk (GB) recommended before source rebuild.
	DefaultMinDiskGB = 40

	// Default Docker image used for Frida multi-platform builds when user has not overridden.
	// Built locally from mirrored ubuntu base; not expected to exist on Docker Hub.
	DefaultBuildImage = "fridare/frida-builder:latest"

	// DualTrackStatic / DualTrackSource short labels for UI and help.
	DualTrackStatic = "静态二进制补丁（默认，hex/字符串替换，无需 Docker）"
	DualTrackSource = "源码重编译+魔改（可选，Docker + AI agent，支持任意官方版本）"
)

// RecommendedEndpointHelp is multi-line guidance shown next to OpenAI settings.
// QR image is embedded in the Settings UI (not linked as plain text).
const RecommendedEndpointHelp = `推荐（OpenAI 兼容）：
站点：https://claudegpt.org/
API 端点：https://claudegpt.org/v1
体验额度：设置页扫码加入 QQ 群 555354813 后可申请一次性体验额度。`

// DualTrackHelpMarkdown is short dual-line copy for the Rebuild tab / Help.
const DualTrackHelpMarkdown = `**A 静态补丁（默认）** hex 替换预编译二进制，无需 Docker。  
**B 源码重编译（本页）** Docker 浅克隆 → Host AI 改源码 → **仅容器内** configure/make。  
Host 支持 Win/macOS/Linux；不装本机 Frida 工具链。依赖：Docker、磁盘、代理、OpenAI（可选 grok）。
`
