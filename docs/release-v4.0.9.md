# Fridare GUI v4.0.9

下载 Windows 包：解压后运行 `fridare-gui.exe`（无控制台黑窗）。zip 内 `docs/` 含本说明。

修 [issue #38](https://github.com/suifei/fridare/issues/38) 的三项，并带上 [#39](https://github.com/suifei/fridare/pull/39)。**不是免杀包。**

## 相对 v4.0.8

- 源码重编译同时勾选 arm64 与 x86_64 时，catalog 不再把两组产物都压成 x86-64。每个目标只收自己的 `artifacts/<target>/`。`CC=aarch64-linux-gnu-gcc` 只留在 `linux-arm64` 子 shell。
- Android server 启动 `backend_class != null`：静态魔改只改段内字符串，嵌入 helper dex 仍是 `Lre/frida/...`，dex SHA-1/Adler32 也是旧的。导出和魔改完成都会走 `PatchProductFile`（含 `PatchAndroidHelperInBinary`）。请用本版 GUI **重新导出或重新魔改** Android 产物。本仓库没有重编已发布的 `kxmwp-*` Frida zip。
- pip 名称：PyPI 发行名是 `frida-tools`，wheel 文件名和 import 包是 `frida_tools`，原生绑定是 `frida`。安装 catalog 里的 wheel 文件，不要 `pip install frida-tools`。产品标签 `17.17.1` 的 pip 版本仍是 `EffectivePipVersion` → `17.17.0`。

## 包内容

| 文件 | 说明 |
|------|------|
| `fridare-v4.0.9-windows-amd64.zip` | **GUI** + create/patch/hexreplace |
| `fridare-v4.0.9-windows-arm64.zip` | CLI only |
| `fridare-v4.0.9-linux-amd64.zip` / `linux-arm64.zip` | CLI only |
| `fridare-v4.0.9-darwin-amd64.zip` / `darwin-arm64.zip` | CLI only |

Linux / macOS GUI 请本机 `cd ui && ./build.sh`（Fyne 需要本机 CGO）。本机编不了的平台 zip 不会用空文件冒充。

## 预编译 Frida 产物（独立 tag，不是本 GUI）

- [kxmwp-16.7.19](https://github.com/suifei/fridare/releases/tag/kxmwp-16.7.19)
- [kxmwp-17.17.1](https://github.com/suifei/fridare/releases/tag/kxmwp-17.17.1) — 官方 17.17.0，产品标签 17.17.1，pip `frida==17.17.0`

这些 zip **没有**随 v4.0.9 重编。Android 上仍报 `backend_class != null` 时，用本版 GUI 对 server 再跑一遍静态魔改，或重新做源码导出。
