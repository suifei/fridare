# 源码魔改：社区对齐 vs Fridare 可编译策略

本文说明 Fridare **源码重编译路径**与社区常见方案（如 [strongR-frida-android](https://github.com/hzzheyang/strongR-frida-android)、[phantom-frida](https://github.com/TheQmaks/phantom-frida) 一类「改标识 + 编 server」）的**重合点与刻意差异**。  
以仓库内真实实现为准：`ui/internal/rebuild` 的 `DefaultModOps`、`DefaultStripLayers`、`PatchArtifactBinaryMarkers`。

## 重合点（对标社区「改可观测特征」）

| 社区常见做法 | Fridare 实现 | 代码位置 |
|--------------|--------------|----------|
| 改 server/agent/gadget 文件名与 meson 目标 | 连字符 basename：`frida-server`→`{magic}-server` 等 | `DefaultModOps` L1 |
| 改 RPC 通道名 | `frida:rpc`→`{magic}:rpc` | `DefaultModOps` L2 |
| 改线程/循环名 | `gum-js-loop`、`frida-main-loop`、`pool-frida` | `DefaultModOps` L2 |
| 改默认端口 | `27042`→配置端口 | `DefaultModOps` L2 |
| 编出可部署 android/linux server | Docker 内 `configure --host=… && make` | orchestrator build |
| 主机侧 tools 对齐 | 多平台 `frida` wheel + `frida-tools` 写 catalog | `BuildPatchedFridaToolsWheels` |

社区 patch 集随 Frida 大版本变化；Fridare **不**保证与某一 fork 的 patch 文件逐行 1:1，而是对齐**策略**：可观测标识 + 可编译 + 客户端可连。

## 刻意差异（为何不全量「去掉 frida 单词」）

| 社区/直觉做法 | Fridare 默认 | 原因 |
|---------------|--------------|------|
| 全局替换 `frida_agent` / `frida_server` | **禁止 auto**（L6 forbidden） | valac 从 `namespace Frida.Agent` 生成 `frida_agent_*`；源码 glue 与导出符号必须一致，硬改会 link 失败 |
| 改 `namespace Frida.Agent` → 自定义命名空间 | **禁止 auto**（L7 forbidden） | 丢失父命名空间类型；`using Frida` 又导致 `Error` 与 `GLib.Error` 歧义（17.x 实测） |
| 改 `re.frida.*` 协议串 | **safe 不改；deep 改** → `re.{magic}.*` | 见下「协议/API 能否改」 |
| 改公开 `Frida.*` API 字面量 | **safe 不改；deep 改** → `{Magic}.*` | 见下 |
| 全树删掉单词 `frida` | **仅方向层 L10** | 极限对抗目标：需扫描分类 + AI 深挖 + 编译门禁，不能一键 auto |

**编译后补齐**：`PatchArtifactBinaryMarkers` 在产物二进制里做同长度替换（`frida_agent`→`abcde_agent`、`frida:rpc` 等），使探测器扫 **server/agent 文件**时看到 magic，而不破坏源码 ABI。

## 方向分层（Agent 深挖地图）

机读定义：`DefaultStripLayers()` / `SafeDirectionManifest` / `ExploreDirectionManifest`。

| 层 | 模式 | 谁执行 |
|----|------|--------|
| L1 产品 basename | auto | DefaultModOps / StubAgent / Grok 基线 |
| L2 RPC/线程/端口 | auto | 同上 |
| L3 GResource getter | auto | 同上 |
| L4 资源文件名 | auto | `RenameMagicAssetFiles` |
| L5 二进制标记 | post_build | 导出管线 |
| L6 下划线 C ABI | forbidden | 不自动；仅 post_build 间接覆盖字符串 |
| L7 Vala 命名空间 | forbidden | 不自动 |
| L8 协议 re.frida | ai_explore | 仅 `-profile explore` + Agent 可选 |
| L9 公开 JS API | ai_explore | 可选 |
| L10 全局单词 frida | ai_explore | 批量扫描 + AI 按文件深挖 |
| L11 引号内标记 | **deep auto** | 只改 `"frida_agent"` 等字面量，**不碰** `void frida_agent_main` 标识符 |
| L12 路径/inject | **deep auto** | `/frida/`、`frida-inject`、server-main-loop 等（**不**全局 `frida-`，以免毁掉 `frida-core` 子工程名） |

### 协议 `re.frida.*` 与 `Frida.version` 类 API：能不能改？

**能改。** 之前默认不改，不是技术不能，而是**兼容策略**：

| 条件 | 结果 |
|------|------|
| 只改 server/agent，host 仍用官方 `frida`/`frida-tools` | **连不上**（DBus/接口名对不上） |
| server + catalog 内魔改 wheel/tools **同一 magic** | **可以改**，且更利于隐藏 |
| 仍要跑社区现成脚本里的 `Frida.xxx` | 脚本也要改成 `{Magic}.xxx`，或保留 API 字面量 |

因此：

- **`profile=safe`**：不改协议/API 字面量 → 仍可能用 stock 客户端（仅产品名/RPC 对齐时视情况）。  
- **`profile=deep`**：自动 `re.frida.`→`re.{magic}.`、`"Frida.`→`"{Magic}.`，并在产物二进制里补丁；**必须**用本 catalog 的 host wheel，不要用未魔改的官方 pip 包。

同长度：`frida` 与 5 字母 magic 使 `re.frida.` 与 `re.abcde.` 等长，二进制补丁安全。

### 机械安全层（按语言 AST/词法）

详见 **[ast-rewrite-layer.md](./ast-rewrite-layer.md)**。deep 路径在引号/注释级改写时走 `StructureAwareRewrite`（C 词法、Python AST/词法），**不是**全文件傻替换。

### profile=deep：源码级深挖隐藏

```text
go run ./cmd/frida-strip-scan -root <src> -magic abcde -profile deep -apply-deep -dig dig.md -o after.json
# 或 e2e / one-shot：
go run ./cmd/e2e-rebuild -profile deep ...
```

流水线：

1. `DeepModOps` = DefaultModOps + inject/路径/主循环/引号品牌名  
2. `RenameMagicAssetFiles`  
3. `ApplyDeepStringLiteralStrip` — 引号与注释内 frida_* / Frida  
4. 写出 `fridare-deep-dig.md`（AI 深挖任务：L8–L10）+ `fridare-residual-frida.txt`  
5. 编译后 `PatchArtifactBinaryMarkers` 再补二进制  

**标识符级** `frida_agent_main` 仍禁止源码 auto；靠 L11 字符串 + L5 二进制双重隐藏探测器扫描面。

批量扫描：`go run ./cmd/frida-strip-scan -root <src> -o report.json -directions directions.json`。  
Agent：`BuildAgentPrompt` 嵌入方向表；`PlanModsFromTree` 写 `fridare-directions.json` + `fridare-strip-scan.json`；e2e 默认 `StubAgent`，`-agent` 启用 `GrokAgent`；`-profile deep` 启用源码深挖。

## 一键入口

```text
# Windows
.\ui\scripts\one-shot-rebuild.ps1 -Work D:\works\fridare-rebuild-e2e

# 或
cd ui && go run ./cmd/e2e-rebuild -work ... -profile safe

# 仅接线验证
go run ./cmd/e2e-rebuild -dry-run
```

成功判定：退出码 0，stdout 含 `E2E OK` / `StageDone`。  
失败：非 0，打印 `stage=` 与 error（不伪造 dry-run catalog）。

## 与 GUI 的关系

| 路径 | AgentDriver | 方向 |
|------|-------------|------|
| GUI 步骤② | GrokAgent（可 naive 回退） | 同 DirectionProfile |
| e2e / one-shot 默认 | StubAgent | safe 方向 = DefaultModOps |
| e2e `-agent` | GrokAgent | safe/explore 写入 Goals + prompt |

**结论**：魔改流程已产品化为软件内 **orchestrator + AgentDriver + 方向清单 + 批量扫描**；不是旁路临时脚本。完全去掉源码中所有 `frida` 字样在工程上「有方向、可工具化、需 AI 分层深挖」，**不是**当前默认 auto 目标。
