# Fridare 双技术路线说明

Fridare 同时提供两条独立的 Frida 魔改技术线。**默认路径不依赖 Docker / AI / 源码编译**；源码重编译路径为可选增强，仅在用户主动启用时使用。

## 路线 A：静态二进制补丁（默认）

**现状 / 默认路径**，对应 GUI 中：

- `🔧 frida 魔改` — 对已下载的 frida-server / agent / gadget 等预编译二进制做特征字符串替换
- `🛠️ frida-tools 魔改` — 对主机侧 Python `frida-tools` 做 RPC/标识对齐
- `📦 iOS DEB 魔改` / `🆕 iOS DEB 打包` — 在 deb 包内完成上述二进制替换并重新打包

### 技术特征

| 项目 | 说明 |
|------|------|
| 输入 | 官方 release 预编译二进制 |
| 手段 | 定长/特征 hex 与字符串替换（hexreplace） |
| 依赖 | 无 Docker、无需编译工具链 |
| 速度 | 秒级 |
| 局限 | 只能改已存在于二进制中的可见特征；无法改逻辑、线程名策略、协议实现等 |

这是 Fridare 项目传统且成熟的路径，适合大多数快速魔改场景。

## 路线 B：源码级重编译 + 魔改（可选）

**可选路径**，对应 GUI 中：`🧬 源码重编译` 标签页。

行业参考（设计借鉴，非整库搬运）：

- [strongR-frida / strongR-frida-android](https://github.com/hzzheyang/strongR-frida-android) — 上游跟进 + git patch 集 + 自动构建 anti-detection frida-server
- [phantom-frida](https://github.com/TheQmaks/phantom-frida) — clone → 源码级标识变换 → configure/make → 产物校验（Android）
- [官方 Frida Building](https://frida.re/docs/building/) — `configure --host=…` + `make` 多宿主 out-of-tree 构建

### 技术特征

| 项目 | 说明 |
|------|------|
| 输入 | 用户指定的官方 Frida git 版本（`git clone --depth=1`） |
| 手段 | Host AI 改文件 + **Docker/Linux 内** configure/make → 导出产物 |
| 依赖 | Docker（可选探测）、磁盘空间、上游代理、OpenAI 兼容端点、grok-build（可选本机已有） |
| 速度 | 小时级；磁盘常需数十 GB |
| 优势 | 可改源码级标识、线程名、路径、部分行为；兼容任意官方版本号/tag |

### 编译隔离（重要）

**Frida 的 `configure` / `make` / 交叉工具链只在 Docker（Linux）里跑，绝不在 Host 上装完整编译环境。**

原因：

- Windows 下 Frida 工具链极重、易把本机环境搞乱  
- Docker + Ubuntu 镜像可复现、可丢弃、与 GUI/AI 进程隔离  
- Host 只跑：GUI、依赖探测、AI agent（改 bind-mount 上的源码）、`docker` 客户端  

分工一览：

| 步骤 | 位置 |
|------|------|
| 浅克隆 depth=1 | Docker（写回 Host 挂载目录） |
| 管理分支 | 优先 Docker git |
| AI / 朴素源码魔改 | Host（改挂载树，不编译） |
| configure + make | **仅 Docker/Linux** |
| 产物导出 | 容器写 artifacts → Host 可读 |

### Host 客户端多系统（Windows / macOS / Linux）

GUI 与 Docker **客户端**在 Windows、macOS、Linux 上均可使用：

- **Docker Desktop**（Win/mac）或 **Docker Engine**（Linux）
- 卷挂载路径自动规范化（Windows 下 `C:/...` 形式，避免 bash 脚本 CRLF）
- 写入容器的 `pipeline.sh` / `build-only.sh` 强制 **LF** 换行
- 不在 Host 安装 Frida 编译工具链；本机只需 docker +（可选）grok-build

### 国内 Docker Hub 镜像源

访问 Hub 失败时，在 **设置 → 源码重编译 / Docker** 配置：

| 项 | 默认 | 说明 |
|----|------|------|
| Hub 镜像源 | `docker.1ms.run` | `ubuntu:22.04` → `docker.1ms.run/library/ubuntu:22.04` |
| 镜像源直连 | 开 | **不走 GUI 代理**（`docker.1ms.run` 需直连） |

本地 builder 标签仍为 `fridare/frida-builder:latest`（本机构建）；`FROM` 基座经镜像源拉取。  
容器内 `git clone` Frida 仍可使用 GUI 配置的上游代理。

### Builder 镜像：依赖在 Dockerfile 阶段准备

**原则**：重依赖只在 `docker build` 时安装一次；AI agent 魔改阶段只做环境确认，再按 Frida 版本拉 subprojects，不在编译任务里现装 NDK/Node/Go。

`fridare/frida-builder` 在 **docker build** 时预装：

- apt：git / make / python3 / ninja / glib / cmake / flex / bison 等  
- **Node.js 20**（Frida 硬性要求 ≥18；Ubuntu 22.04 自带 apt node 太旧）  
- **Go 1.24**（Compiler backend / ESBuild）  
- **Android NDK r29** → `/opt/android-ndk-r29`（`ANDROID_NDK_ROOT`）

**国内 Hub 镜像源**默认 `docker.1ms.run`（设置页与「源码重编译」页均可改；pull/build 默认直连、不走 GUI 代理）。

### 服务端 + 客户端协议同步（deep 默认）

源码重编译在 **deep / abi / explore / full** 下会**同时**改服务端与 host 客户端协议面，避免只改一边连不上：

| 面 | 服务端 | 客户端（catalog wheel） |
|----|--------|-------------------------|
| RPC 通道 | `frida:rpc` → `{magic}:rpc` | 同左（`.py` + `.pyd/.so`） |
| DBus 接口名 | `re.frida.*` → `re.{magic}.*` | 同左 |
| DBus 对象路径 | `/re/frida/` → `/re/{magic}/` | 同左（缺则 `UNKNOWN_METHOD`） |
| 公开 API 字面量 | `"Frida.` → `"{Magic}.` | 同左 |

- magic 必须 **5 个小写字母**（与 `frida` 同长，二进制补丁安全）
- 产物目录含 `python/INSTALL.txt` 与 `PROTOCOL-SYNC.json`（导出后自动交叉核对）
- **务必**安装 catalog 内 `python/host/<os-arch>/frida-*.whl` + `frida_tools`，勿用官方 pip
- **safe** 模式少改源码协议面，尽量兼容 stock 客户端；导出仍会补丁 rpc 等标记并打包对齐 wheel

静态路线（hexreplace / frida-tools 标签）同样按 full 协议面同步替换，服务端二进制与 tools 成对使用。

### GUI 两步流程

| 步骤 | 做什么 | 是否要 AI |
|------|--------|-----------|
| ① 基础镜像 | docker build 工具链；稳定标签留档；可选 `docker save` 到 `~/.fridare/rebuild/images/` | 否 |
| ② 魔改开发 | 复用本地镜像 → 浅克隆版本 → AI 魔改 → configure/make | 是 |

后续「AI 魔改 + 编译」任务只做：

1. 确认镜像带 `FRIDARE_BUILDER_FEATURES`（含 NDK / Node / Go 自检）  
2. 浅克隆指定 Frida 版本  
3. 源码魔改（agent 改文件，不装工具链）  
4. `configure`/`make`（Frida 拉该版本 subprojects，不再下 NDK/Node/Go）

旧镜像 feature 标记不匹配时会自动 **重建 builder**（首次慢，一次即可）。本地 `latest` + feature 标签 + tar 档案均可复用。

### 流水线阶段

1. **环境探测** — Docker / 空闲磁盘 / 代理 / grok-build / OpenAI 端点  
2. **步骤① 基础镜像** — 工具链镜像构建与本地留档  
3. **浅克隆** — 容器内 `git clone --depth=1 --branch <version>`  
4. **管理分支** — 容器内 `fridare/mod-<timestamp>`  
5. **AI 魔改** — Host agent 对话目标 + 全树文件操作（GUI 右侧 Agent 栏固定可见）  
6. **多平台编译** — 容器内 `configure --host=… && make`  
7. **导出产物** — 分类目录 `catalog/{version}/{platform}/{magic}/`（binaries + python wheel）  
8. **主机客户端包** — 多平台 `frida=={源码 tag}` wheel + `frida-tools` 纯 wheel，一并写入 catalog；GUI 可浏览历史条目无需重编  

### 源码魔改策略（与编译兼容）

详见 **[community-align.md](./community-align.md)**（社区对齐 + 去 frida 分层 + Agent 方向）。

| 层级 | 做什么 |
|------|--------|
| 源码 | 连字符 basename（`frida-server`→`{magic}-server` 等）；`get_frida_agent_*` 等 GResource getter；`frida:rpc` / 线程名 / 端口；资源文件 basename 重命名（`.version`/`.symbols`） |
| 不改（否则 valac 失败） | Vala `namespace Frida.Agent`；裸 `frida_agent_main` 等 C 前缀（由 valac 从命名空间生成） |
| 编译后 | 同长度二进制补丁：`frida_agent`/`frida_server`/`frida:rpc` 等 → magic（探测器扫二进制用） |
| 批量工具 | `go run ./cmd/frida-strip-scan` 分类命中；方向 JSON 供 Agent 深挖 |
| **deep 源码隐藏** | `-profile deep`：DeepModOps + 字符串结构改写 + **函数/类/命名空间标识符** token 重命名 + dig |
| **AST/词法层** | 见 [ast-rewrite-layer.md](./ast-rewrite-layer.md)：分语言，非 monorepo 单 AST |
| 一键 StageDone | `ui/scripts/one-shot-rebuild.ps1` / `.sh` → `cmd/e2e-rebuild` |

### Catalog 布局

```text
catalog/{fridaVersion}/{targetPlatform}/{magic}/
  binaries/          # abcde-server, abcde-agent*.so …（非空，≥1KB）
  MANIFEST.json      # version / platform / magic / binary_files
  python/
    frida_tools-*.whl
    host/
      windows-amd64/frida-*.whl   # 已 patch frida:rpc
      windows-arm64/
      macos-x86_64/
      macos-arm64/
      linux-x86_64/
      linux-arm64/
    INSTALL.txt
catalog/{fridaVersion}/_host-tools/{magic}/python/…  # 跨 target 共享的 host 包
```

宿主机按本机 OS/CPU 选 `python/host/<id>/` 安装，再装 `frida_tools-*.whl`，版本须等于源码 tag。

> **诚实限制**：iOS/macOS 官方工具链偏 Apple 宿主；Linux Docker 上可能仅能完整交叉编译 Android/Linux/Windows 子集。GUI 会列出全部目标，但对不可用工具链给出明确提示，而不是静默声称「一定能编过」。

## 系统不予置（不强制改系统）

- 不强制安装 Docker / grok-build  
- 不强制改系统 PATH 或全局代理  
- 源码重编译标签页可随时打开做探测；未启用时不影响下载/静态魔改/tools 魔改  
- 用户可随时检查依赖、终止任务、重来  

## 与 frida-tools / host frida 的关系

两条路线在服务端魔改后，通常都需要对齐主机侧客户端：

1. 原生绑定 `frida=={version}`（按宿主机平台选 wheel）  
2. 纯 Python `frida-tools`（任意宿主机）  

源码重编译路径在导出阶段自动下载多平台 wheel，并按 **deep 协议面** 同步改写 host 绑定（`re.frida.*`、`/re/frida/` 对象路径、`frida:rpc`、`Frida.*` API，写入 `.py` + `_frida.pyd/.so`），pin `frida-tools` 到同一 Frida 版本，并写出 `PROTOCOL-SYNC.json`。  
静态路径仍可使用 GUI 的 `🛠️ frida-tools 魔改` 做同面补丁。  
「frida-tools 仅跑在容器内 + 主机设备网桥」明确延后，当前不做。
