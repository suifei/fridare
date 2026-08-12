# Frida 17.17.0 深度源码魔改产物（magic=`kxmwp`）

本页说明：**这批预编译包是什么、怎么用、怎么在 Fridare GUI 里自己再编一遍，以及为什么没有 iOS / macOS。**

对应 GitHub Release：<https://github.com/suifei/fridare/releases/tag/kxmwp-17.17.0>

---

## 1. 这批产物是什么

| 项 | 值 |
|----|-----|
| 上游 | 官方 Frida **17.17.0**（`git clone --depth=1 --branch 17.17.0`） |
| 路线 | Fridare **路线 B：源码重编译 + deep 魔改**（Docker-only 编译） |
| Magic | **`kxmwp`**（必须 **5 个小写字母**，与 `frida` 同长） |
| 协议面 | `re.kxmwp.` · `/re/kxmwp/` · `kxmwp:rpc` · `Kxmwp.*` API |
| 编译环境 | `fridare/frida-builder`（Ubuntu + NDK r29 + Node 20 + Go 1.24 + MinGW + **aarch64 交叉 GCC**） |
| 本机连通 | Windows `kxmwp-server` + 同 magic host wheel：`query_ok` + `enumerate_ok` |

**服务端和客户端必须成对、同一 magic。** 不要用官方 pip 的 `frida` 去连这批 server。

---

## 2. 下载清单

目录约定：解压后用 **`kxmwp-server` / `kxmwp-server.exe`**（已 strip 的产品），`*-raw` 为未打包中间体，体积更大。

| 资产 | 用途 |
|------|------|
| `frida-kxmwp-17.17.0-windows-x86_64-server.zip` | Windows 64 位 server / agent / gadget / helper / inject |
| `frida-kxmwp-17.17.0-windows-x86-server.zip` | Windows 32 位（i686 MinGW） |
| `frida-kxmwp-17.17.0-linux-x86_64-server.zip` | Linux x86_64 |
| `frida-kxmwp-17.17.0-linux-arm64-server.zip` | Linux aarch64（Docker 内交叉） |
| `frida-kxmwp-17.17.0-android-arm64-server.zip` | Android arm64（真机主力） |
| `frida-kxmwp-17.17.0-android-arm-server.zip` | Android armeabi-v7a |
| `frida-kxmwp-17.17.0-android-x86_64-server.zip` | Android 模拟器 x86_64 |
| `frida-kxmwp-17.17.0-android-x86-server.zip` | Android 模拟器 x86 |
| `frida-kxmwp-17.17.0-host-wheels.zip` | **PC 侧客户端**：多平台 `frida==17.17.0` 魔改 wheel + `frida_tools` |

**没有** iOS / macOS server 包（见第 6 节）。host-wheels 里仍含 **macOS 客户端 wheel**，供你在 Mac 上连 **Android / Linux / Windows 设备上的魔改 server**。

---

## 3. 如何使用这些魔改版

### 3.1 安装客户端（电脑上）

解压 `frida-kxmwp-17.17.0-host-wheels.zip`，按本机选目录：

```
python/host/windows-amd64/frida-17.17.0-*.whl
python/host/windows-arm64/...
python/host/linux-x86_64/...
python/host/linux-arm64/...
python/host/macos-x86_64/...
python/host/macos-arm64/...
python/frida_tools-*.fridare.kxmwp-*.whl
```

```powershell
python -m venv .venv
.\.venv\Scripts\pip install --force-reinstall path\to\frida-17.17.0-cp37-abi3-win_amd64.whl
.\.venv\Scripts\pip install --force-reinstall path\to\frida_tools-*.whl
```

Linux / macOS 把 `pip` 换成 `bin/pip`。

### 3.2 启动服务端

**Windows（本机或目标机）：**

```powershell
.\kxmwp-server.exe -l 0.0.0.0:27042
```

**Linux：**

```bash
chmod +x kxmwp-server
./kxmwp-server -l 0.0.0.0:27042
```

**Android：**

```bash
adb push kxmwp-server /data/local/tmp/
adb shell "chmod 755 /data/local/tmp/kxmwp-server && /data/local/tmp/kxmwp-server -l 0.0.0.0:27042"
adb forward tcp:27042 tcp:27042
```

### 3.3 连接

```python
import frida
d = frida.get_device_manager().add_remote_device("127.0.0.1:27042")
print(d.query_system_parameters())
print(len(d.enumerate_processes()))
```

或：

```bash
frida -H 127.0.0.1:27042 -n <进程名>
```

### 3.4 常见坑

| 现象 | 原因 | 处理 |
|------|------|------|
| `UNKNOWN_METHOD` | 只用了官方 frida，或只改了 `re.frida.` 没改 `/re/frida/` | 装 catalog 里 **同一 magic** 的 host wheel |
| 连上但 enumerate 失败 | server / client magic 不一致 | 成对使用 `kxmwp` 包 |
| Android 秒退 | 权限 / SELinux / 未 root | 用 `/data/local/tmp` + root |
| Windows 杀软拦截 | 未签名自定义 server | 加白名单 |

---

## 4. 本批构建过程（发生了什么）

```text
官方 Frida 17.17.0
        │
        ▼
 Host：AI / 确定性 deep 魔改（改挂载源码，不编译）
        │  magic=kxmwp
        │  re.frida. → re.kxmwp. ；/re/frida/ → /re/kxmwp/ ；frida:rpc → kxmwp:rpc
        │  basename：frida-server → kxmwp-server …
        ▼
 Docker：fridare/frida-builder
        │  configure --host=<triplet> --enable-server --enable-gadget --enable-inject
        │           --disable-frida-python
        │  make（MinGW / NDK / aarch64-linux-gnu）
        ▼
 catalog/17.17.0/<target>/kxmwp/binaries
        +
 host wheels（pip 官方 17.17.0 再打协议补丁）
        ▼
 PROTOCOL-SYNC 交叉校验 + Windows 本机 smoke
```

本批实际踩过并已写回 Fridare 代码的点：

1. **MinGW DNS**：Vala 生成的 `device-monitor-windows.c` 缺类型 → 嵌入 `fridare-mingw-dns.h`，**只注入 meson cross file**（禁止全局 `CFLAGS=-include`，否则 i686 会把 Linux 本机编译器弄坏）。
2. **原生 Linux 默认关 server**：meson `auto` 在非交叉时关掉 server/gadget → 强制 `--enable-server` 等。
3. **deep 魔改后 generate.py / bindgen**：`--disable-frida-python`；`generate.py` 容忍空 enum。
4. **linux-arm64**：镜像预装 `gcc-aarch64-linux-gnu`；`--host=aarch64-linux-gnu`（不能只写 `linux-arm64`）。
5. **windows-x86**：见第 1 条 CFLAGS 污染。

Builder 镜像 feature：`toolchain-v4-ndk29-node20-go124-mingw-aarch64`。

---

## 5. 在 Fridare GUI 里自己再做一遍

需要：**Docker Desktop / Engine**、足够磁盘（建议 40GB+）、可选本机代理、可选 OpenAI 兼容端点 / grok。

### 5.1 安装 GUI

从 [Fridare Releases](https://github.com/suifei/fridare/releases) 下载 **Windows GUI**（`fridare-v*-windows-amd64.zip` 里的 `fridare-gui.exe`），或本机：

```powershell
$env:Path = "C:\msys64\mingw64\bin;" + $env:Path   # 仅本机构建 GUI 时需要 gcc
cd ui
.\build.ps1
.\build\fridare-gui.exe
```

### 5.2 设置

1. 打开 **设置**  
2. **源码重编译 / Docker**：Hub 镜像源默认 `docker.1ms.run`，**镜像源直连**（不走 GUI 代理）  
3. 克隆 Frida 如需代理：填 HTTP 代理（容器内 `localhost` 会改成 `host.docker.internal`）  
4. 可选：OpenAI Base URL / Key；**「OpenAI 端点走 GUI 代理」默认关**；可点「测试端点连接」  
5. 魔改名：**5 位小写 a-z**（可随机）

### 5.3 一键深度（推荐）

1. 打开 **「源码重编译」** 标签  
2. Frida 版本填官方 tag，例如 `17.17.0`  
3. Profile 选 **`deep`**（默认）  
4. 勾选 Docker 可编目标，例如：  
   `windows-x86_64`、`windows-x86`、`linux-x86_64`、`linux-arm64`、`android-arm64`、`android-arm`、`android-x86_64`、`android-x86`  
   **不要指望在 Linux Docker 里勾 iOS / macOS 能编出完整 server**（见第 6 节）  
5. 点 **「一键深度定制（①+② deep）」**  

含义：

| 步骤 | 做什么 | 在哪跑 |
|------|--------|--------|
| ① 基础镜像 | 构建/复用 `fridare/frida-builder`（NDK/Node/Go/MinGW/aarch64） | Docker |
| ② 开发 | 浅克隆 → 魔改源码 → `configure/make` → catalog + host wheels + PROTOCOL-SYNC | Host 改文件 + **仅 Docker 编译** |

右侧 **Agent** 栏会打日志。全量多平台是 **小时～天级**。首次拉镜像/NDK 最慢。

也可分步：**① 只准备镜像** → **② 只魔改编译**（镜像已就绪时更快）。

### 5.4 产物在哪

```
~/.fridare/rebuild/   或你填的工作目录
  catalog/<version>/<target>/<magic>/
    binaries/          # *-server / agent / gadget …
    PROTOCOL-SYNC.json
    python/            # 或与 _host-tools/<magic>/python/host/ 共用
```

GUI 的 catalog 列表可浏览历史条目，不必每次重编。

### 5.5 只用静态路线（不 Docker）

标签 **frida 魔改** + **frida-tools 魔改**，同一 5 位 magic，秒级；深度不如源码 deep。iOS 可用 **iOS DEB 魔改** 改官方 deb（不是源码重编 iOS）。

---

## 6. 为什么没有编 iOS / macOS

在 Fridare 的 **Docker 源码重编译** 路线里，这两个平台被标成 **Docker 不友好**，不是忘了，而是 **工具链限制**：

| 平台 | 需要什么 | Linux Docker 里通常怎样 |
|------|----------|-------------------------|
| **iOS**（arm64 / arm64e / simulator） | Xcode + iOS SDK（Apple 官方） | 没有合法完整 iOS SDK，交叉编译基本做不了完整 frida-server / agent 产品链 |
| **macOS**（arm64 / x86_64） | macOS SDK + Apple 工具链 | 同样依赖 Apple 侧环境，不能指望 Ubuntu 容器里的 MinGW/NDK 那一套 |

GitHub Actions 能做 iOS / macOS 的源码构建，但和现在的 **Linux Docker 交叉不是同一条路**，复杂度也确实高。

| 路径 | 可行性 | 说明 |
|------|--------|------|
| **GitHub Actions `macos-*` runner** | 可行 | 官方提供 macOS 虚拟机，可装 Xcode、跑 `configure --host=macos-*` / `ios-*` |
| **Linux Docker 交叉编 iOS/macOS** | 基本不可行 | 与代码里 `DockerFriendly: false` 一致：要 Apple SDK / 签名 / 官方工具链 |
| **用户本机 macOS + Fridare GUI/CLI** | 最稳产品路径 | 和现在 Windows/Linux Docker 对称：Host 改源码，本机工具链编译 |

**现在就能用的 iOS 方案：** 路线 A 静态魔改官方 **iOS deb**（GUI「iOS DEB 魔改 / 打包」），不经过 Docker 源码编译。

---

## 7. 合规与范围

仅供安全研究、授权测试与自我环境定制。请遵守当地法律与目标系统授权。魔改不保证对抗所有检测面。
