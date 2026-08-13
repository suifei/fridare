# Frida 16.7.19 深度魔改产物（tag `kxmwp-16.7.19`）

官方 Frida **16.7.19**，magic=`kxmwp`。**不是免杀包。** 端口用 `-l`。客户端必须同一 magic 的 host wheel。

Release：<https://github.com/suifei/fridare/releases/tag/kxmwp-16.7.19>

## 包内容（与 17.17.1 同矩阵）

| 资产 | 来源 |
|------|------|
| `frida-kxmwp-16.7.19-linux-x86_64-server.zip` | GUI 源码重编译 |
| `frida-kxmwp-16.7.19-linux-arm64-server.zip` | GUI 源码重编译 |
| `frida-kxmwp-16.7.19-android-arm64-server.zip` | 静态 hexreplace 官方包 |
| `frida-kxmwp-16.7.19-android-arm-server.zip` | 静态 hexreplace 官方包 |
| `frida-kxmwp-16.7.19-android-x86_64-server.zip` | 静态 hexreplace 官方包 |
| `frida-kxmwp-16.7.19-android-x86-server.zip` | 静态 hexreplace 官方包 |
| `frida-kxmwp-16.7.19-windows-x86_64-server.zip` | 静态 hexreplace 官方包 |
| `frida-kxmwp-16.7.19-windows-x86-server.zip` | 静态 hexreplace 官方包 |
| `frida-kxmwp-16.7.19-host-wheels.zip` | pip `frida==16.7.19` 协议补丁 |

Android / Windows 走静态轨，因为 16.7.19 源码要求 **NDK r25**，当前 builder 镜像是 **r29**（17.x）。zip 内 `ORIGIN.txt` 写了来源。

没有 iOS / macOS server（需要 Apple 宿主）。

## 扫描（`kxmwp-server`）

| 平台 | `kxmwp:rpc` | `re.kxmwp.` | `/re/kxmwp/` | 官方协议 |
|------|-------------|-------------|--------------|----------|
| linux-x86_64（源码） | 12 | 48 | 21 | 0 |
| linux-arm64（源码） | 12 | 48 | 21 | 0 |
| android-arm64（静态） | 20 | 79 | 29 | 0 |
| android-arm（静态） | 11 | 45 | 19 | 0 |
| windows-x86_64（静态） | 23 | 127 | 51 | 0 |

`frida:rpc` / `re.frida.` / `/re/frida/` 均为 **0**。

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

GUI v4.0.8+ 源码重编译 → 版本 `16.7.19` → deep。Android 源码编需要 NDK r25。

## 合规

仅供授权测试。去掉字样 ≠ 免杀。
