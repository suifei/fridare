# Fridare GUI v4.0.8

下载 Windows 包：解压后运行 `fridare-gui.exe`（无控制台黑窗）。zip 内 `docs/` 含双轨说明与 16.7.19 / 17.17.1 产物用法。

## 相对 v4.0.7

- 源码重编译流水线：`RenameMagicAssetFiles` 含 `frida-inject`（修 meson `kxmwp-inject.version`）
- ninja 花指令只写进 `ARGS=`，不再把 `-DFRIDARE_JUNK_SEED` 当成 ninja 依赖
- Frida 16.x 自动关闭 `compiler_snapshot`（官方 SDK `v8-mksnapshot` 在本 builder 上不可用）
- `--disable-frida-tools`，避免 native linux 去编缺头文件的 frida-python
- 产品标签 `17.17.1` 的 host wheel 走 PyPI `frida==17.17.0`（`EffectivePipVersion`）
- 一键深度定制仍强制 `deep`；stealth 默认开

## 预编译 Frida 产物（独立 tag，不是 GUI latest）

- [kxmwp-16.7.19](https://github.com/suifei/fridare/releases/tag/kxmwp-16.7.19) — 官方 16.7.19 GUI 路径 linux-x86_64
- [kxmwp-17.17.1](https://github.com/suifei/fridare/releases/tag/kxmwp-17.17.1) — 官方 17.17.0，产品标签 17.17.1

**不是免杀包。** 端口用 `-l`。客户端必须同一 magic 的 host wheel。
