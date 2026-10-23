# Fridare GUI v4.1.0

下载 Windows 包：解压后运行 `fridare-gui.exe`（无控制台黑窗）。zip 内 `docs/` 含本说明。

界面可以在中文和 English 之间切换（[issue #37](https://github.com/suifei/fridare/issues/37)，[#40](https://github.com/suifei/fridare/pull/40)）。**不是免杀包。**

## 语言

打开 **设置 → 界面配置 → 语言**。选项是 **中文** 和 **English**，默认中文。切换后窗口立即按新语言重建，并把 `language`（`zh` 或 `en`）写入 `config.json`。`fridare-create` 和 `fridare-patch` 下次启动时使用同一设置。

Open **Settings → Interface configuration → Language** and choose **中文** or **English**. Chinese stays the default. The window rebuilds immediately.

## 包内容

| 文件 | 说明 |
|------|------|
| `fridare-v4.1.0-windows-amd64.zip` | **GUI** + create/patch/hexreplace |
| `fridare-v4.1.0-windows-arm64.zip` | CLI only |
| `fridare-v4.1.0-linux-amd64.zip` / `linux-arm64.zip` | CLI only |
| `fridare-v4.1.0-darwin-amd64.zip` / `darwin-arm64.zip` | CLI only |

Linux / macOS GUI 请本机 `cd ui && ./build.sh`（Fyne 需要本机 CGO）。本机编不了的平台 zip 不会用空文件冒充。

## 预编译 Frida 产物（独立 tag，不是本 GUI）

- [kxmwp-16.7.19](https://github.com/suifei/fridare/releases/tag/kxmwp-16.7.19)
- [kxmwp-17.17.1](https://github.com/suifei/fridare/releases/tag/kxmwp-17.17.1) — 官方 17.17.0，产品标签 17.17.1，pip `frida==17.17.0`

这些 zip **没有**随 v4.1.0 重编。
