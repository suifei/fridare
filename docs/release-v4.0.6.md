# Fridare v4.0.6

当前 **GUI / CLI** 正式版。把 Docker **toolchain-v4** 与监听端口说明打进安装包。

下载 Windows 包：解压后运行 `fridare-gui.exe`（无控制台黑窗）。zip 内 `docs/` 含双轨说明与 17.17.0 产物用法。

## 相对 v4.0.5

- **Builder v4**：镜像预装 MinGW + `gcc-aarch64-linux-gnu`（`linux-arm64` 用 `--host=aarch64-linux-gnu`）
- **监听端口**：改不改二进制默认值都行。启动 `*-server -l 0.0.0.0:28888`，客户机 `frida -H 127.0.0.1:28888`
- **安装包附带**：`docs/dual-track.md`、`docs/kxmwp-17.17.0.md`、CHANGELOG
- MinGW DNS 只写 meson cross file（不再全局 `CFLAGS=-include`）
- 编译强制 `--enable-server`；deep 时 `--disable-frida-python`

## 预编译 Frida 17.17.0（另 tag）

[kxmwp-17.17.0](https://github.com/suifei/fridare/releases/tag/kxmwp-17.17.0) 是 **server/wheel 产物**，不是本 GUI。两边配合用：GUI 做静态/源码魔改，kxmwp zip 可直接部署。

## 包内容

| 文件 | 说明 |
|------|------|
| `fridare-v4.0.6-windows-amd64.zip` | **GUI** + create/patch/hexreplace（推荐） |
| `fridare-v4.0.6-windows-arm64.zip` | CLI only |
| `fridare-v4.0.6-linux-amd64.zip` / `linux-arm64.zip` | CLI only |
| `fridare-v4.0.6-darwin-amd64.zip` / `darwin-arm64.zip` | CLI only |

Linux / macOS GUI 请本机 `cd ui && ./build.sh`（Fyne 需要本机 CGO）。
