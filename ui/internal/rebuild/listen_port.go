package rebuild

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// OfficialListenPort is Frida's stock DEFAULT_CONTROL_PORT (frida-core lib/base/socket.vala).
const OfficialListenPort = 27042

// OfficialListenPortASCII is the decimal spelling of OfficialListenPort (always 5 digits).
const OfficialListenPortASCII = "27042"

// listenPortVendorDirs are third-party trees whose "27042" hits are numeric tables, not the listen port.
var listenPortVendorDirs = map[string]bool{
	"brotli": true, "capstone": true, "xz": true, "lzma": true,
	"openssl": true, "zlib": true, "nghttp2": true,
}

// ListenPortSourceOps returns surgical source replacements for DEFAULT_CONTROL_PORT.
//
// Official Frida 17.x stores the listen port as a uint16 constant, not a user-facing
// ASCII banner. A tree-wide "27042" replace hits brotli/capstone/xz/openssl tables.
//
// Rules:
//   - port==0 or port==27042 → nil (keep stock; do not emit a no-op replace)
//   - always rewrite `DEFAULT_CONTROL_PORT = 27042` (digit length may differ)
//   - same-length ASCII 27042 only under subprojects/frida-core (not **/*)
func ListenPortSourceOps(port int) []ModOp {
	if port <= 0 || port == OfficialListenPort {
		return nil
	}
	if port > 65535 {
		return nil
	}
	oldS := OfficialListenPortASCII
	newS := strconv.Itoa(port)
	ops := []ModOp{
		{
			Path:        "**/socket.vala",
			Operation:   "replace",
			Description: fmt.Sprintf("DEFAULT_CONTROL_PORT %s → %d", oldS, port),
			Find:        "DEFAULT_CONTROL_PORT = " + oldS,
			Replace:     "DEFAULT_CONTROL_PORT = " + newS,
		},
	}
	if len(newS) == len(oldS) {
		ops = append(ops, ModOp{
			Path:        "subprojects/frida-core/**",
			Operation:   "replace",
			Description: fmt.Sprintf("listen port ASCII %s → %d (frida-core only)", oldS, port),
			Find:        oldS,
			Replace:     newS,
		})
	}
	return ops
}

// NormalizeListenPort returns a usable listen port (stock default when unset).
func NormalizeListenPort(port int) int {
	if port <= 0 {
		return OfficialListenPort
	}
	return port
}

// skipNumericTableVendorPath reports third-party paths whose 27042 must not be rewritten.
func skipNumericTableVendorPath(relSlash string) bool {
	rel := strings.ToLower(filepath.ToSlash(relSlash))
	for _, seg := range strings.Split(rel, "/") {
		if listenPortVendorDirs[seg] {
			return true
		}
	}
	return false
}

// isListenPortFind is true when this op is rewriting the official ASCII listen port.
func isListenPortFind(find string) bool {
	return find == OfficialListenPortASCII
}

// ListenPortAgentGuidance is the required Agent prompt block for the listen port.
func ListenPortAgentGuidance(port int) string {
	port = NormalizeListenPort(port)
	var b strings.Builder
	b.WriteString("\n## 监听端口（必做，L2）\n")
	b.WriteString("官方默认监听端口是 **27042**，定义在 `subprojects/frida-core/lib/base/socket.vala`：\n")
	b.WriteString("  `public const uint16 DEFAULT_CONTROL_PORT = 27042;`\n")
	b.WriteString(fmt.Sprintf("本次用户配置端口: **%d**\n", port))
	b.WriteString("规则:\n")
	b.WriteString("- find 必须是官方默认 `27042` / `DEFAULT_CONTROL_PORT = 27042`。\n")
	b.WriteString("- **禁止**把用户端口当成 find（例如把 27142 当成原文再换成 27142，那是空操作）。\n")
	b.WriteString("- **禁止**全树替换裸数字 27042（brotli / capstone / xz / openssl 等是假阳性数字表）。\n")
	b.WriteString("- **不要改** `DEFAULT_CLUSTER_PORT`（27052），除非用户明确要求。\n")
	if port == OfficialListenPort {
		b.WriteString("- 本次端口仍是 27042：不要改 DEFAULT_CONTROL_PORT，也不要写 27042→27042。\n")
	} else {
		b.WriteString(fmt.Sprintf("- 必须改：`DEFAULT_CONTROL_PORT = 27042` → `DEFAULT_CONTROL_PORT = %d`。\n", port))
		if len(strconv.Itoa(port)) == len(OfficialListenPortASCII) {
			b.WriteString(fmt.Sprintf("- 可在 **frida-core** 内把同长度 ASCII `27042` 换成 `%d`（不要出 frida-core）。\n", port))
		} else {
			b.WriteString("- 新端口与 27042 位数不同：只改 DEFAULT_CONTROL_PORT 赋值，不要做裸数字替换。\n")
		}
	}
	return b.String()
}
