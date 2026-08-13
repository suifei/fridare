# Fridare v4.0.7

当前 **GUI / CLI** 正式版。把 stealth 能力接到界面上，不再只藏在 Docker 流水线里。

下载 Windows 包：解压后运行 `fridare-gui.exe`（无控制台黑窗）。

**这不是免杀包。** 去掉字样 ≠ 行为隐身。

## 相对 v4.0.6

- **源码重编译**步骤②：去符号 / 花指令 / 行为标记 / 随机 agent 落盘，可单独开关，写入配置
- **静态「frida 魔改」与 frida-tools**：导出后去符号（ELF/PE `frida_` / `_frida_`）
- 一键深度定制强制 `deep`；默认 Frida 版本 **17.17.0**
- 安装包附 `docs/kxmwp-17.17.1.md`

## 预编译 Frida（另 tag）

[kxmwp-17.17.1](https://github.com/suifei/fridare/releases/tag/kxmwp-17.17.1) 是 **server/wheel 产物**，不是本 GUI。GUI 做静态/源码魔改；kxmwp zip 可直接部署。

## 包内容

| 文件 | 说明 |
|------|------|
| `fridare-v4.0.7-windows-amd64.zip` | **GUI** + create/patch/hexreplace（推荐） |
| `fridare-v4.0.7-windows-arm64.zip` | CLI only |
| `fridare-v4.0.7-linux-amd64.zip` / `linux-arm64.zip` | CLI only |
| `fridare-v4.0.7-darwin-amd64.zip` / `darwin-arm64.zip` | CLI only |

Linux / macOS GUI 请本机 `cd ui && ./build.sh`（Fyne 需要本机 CGO）。
