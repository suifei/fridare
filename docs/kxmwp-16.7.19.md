# Frida 16.7.19 深度魔改产物（tag `kxmwp-16.7.19`）

官方 Frida **16.7.19**，magic=`kxmwp`。**不是免杀包。** 端口用 `-l`。客户端必须同一 magic 的 host wheel。

Release：<https://github.com/suifei/fridare/releases/tag/kxmwp-16.7.19>

本 tag **不是** GUI `latest`（GUI latest 仍是 v4.0.8）。

## 包内容（8 平台全部 GUI 源码重编译）

Builder 镜像 `fridare/frida-builder:latest` feature **`toolchain-v5-ndk25-ndk29-node20-go124-mingw-aarch64`**：默认 NDK **r29**（17.x），另备 **r25**（`/opt/android-ndk-r25`）。16.7.19 的 `releng/env_android.py` 写了 `NDK_REQUIRED=25`，流水线会自动切到 r25。

| 资产 | 来源 |
|------|------|
| `frida-kxmwp-16.7.19-linux-x86_64-server.zip` | GUI 源码重编译 |
| `frida-kxmwp-16.7.19-linux-arm64-server.zip` | GUI 源码重编译 |
| `frida-kxmwp-16.7.19-android-arm64-server.zip` | GUI 源码重编译（NDK r25） |
| `frida-kxmwp-16.7.19-android-arm-server.zip` | GUI 源码重编译（NDK r25） |
| `frida-kxmwp-16.7.19-android-x86_64-server.zip` | GUI 源码重编译（NDK r25） |
| `frida-kxmwp-16.7.19-android-x86-server.zip` | GUI 源码重编译（NDK r25） |
| `frida-kxmwp-16.7.19-windows-x86_64-server.zip` | GUI 源码重编译（MinGW + toolchain ninja） |
| `frida-kxmwp-16.7.19-windows-x86-server.zip` | GUI 源码重编译 |
| `frida-kxmwp-16.7.19-host-wheels.zip` | pip `frida==16.7.19` 协议补丁 |

zip 内 `ORIGIN.txt` 写 `origin=GUI-path source rebuild`。没有 iOS / macOS server（需要 Apple 宿主）。

## 扫描（`kxmwp-server` / `kxmwp-server.exe`）

| 平台 | `kxmwp:rpc` | `re.kxmwp.` | `/re/kxmwp/` | 官方协议 |
|------|-------------|-------------|--------------|----------|
| linux-x86_64 | 12 | 48 | 21 | 0 |
| linux-arm64 | 12 | 48 | 21 | 0 |
| android-arm64 | 20 | 79 | 29 | 0 |
| android-arm | 11 | 45 | 19 | 0 |
| android-x86_64 | 38 | 119 | 47 | 0 |
| android-x86 | 20 | 65 | 28 | 0 |
| windows-x86_64 | 11 | 165 | 63 | 0 |
| windows-x86 | 5 | 97 | 39 | 0 |

`frida:rpc` / `re.frida.` / `/re/frida/` 均为 **0**。android-x86_64 计数偏高是因为 fat agent 里嵌了多 ABI。

## 用法

```powershell
# Windows
.\kxmwp-server.exe -l 0.0.0.0:28888
# Linux / Android
./kxmwp-server -l 0.0.0.0:28888
frida -H 127.0.0.1:28888 -n <进程名>
```

默认端口仍是 **27042**。换端口用 `-l`。装本 tag `host-wheels` 里同一 magic 的 wheel。

## 自己再编

GUI v4.0.8+ 源码重编译 → 版本 `16.7.19` → deep。Android 会按源码选用 NDK r25。Windows 交叉必须用 Frida `deps/toolchain-linux-x86_64/bin/ninja`（apt ninja 1.10 没有 `ninja -t inputs`）。

## 合规

仅供授权测试。去掉字样 ≠ 免杀。
