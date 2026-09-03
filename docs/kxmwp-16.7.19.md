# Frida 16.7.19 深度魔改产物（tag `kxmwp-16.7.19`）

官方 Frida **16.7.19**，magic=`kxmwp`。**不是免杀包。** 端口用 `-l`。客户端必须同一 magic 的 host wheel。

Release：<https://github.com/suifei/fridare/releases/tag/kxmwp-16.7.19>

本 tag **不是** GUI `latest`（GUI latest 仍是 v4.0.8）。

## 魔改面（术语）

`deep` + stealth，magic=`kxmwp`。**不是免杀。**

- basename：`frida-server` / `frida-agent` / `frida-gadget` / `frida-helper` / `frida-inject` / `frida-policyd`
- blob getter：`get_frida_agent` 等
- RPC：`frida:rpc` → `kxmwp:rpc`
- D-Bus：`re.frida.` → `re.kxmwp.`，`/re/frida/` → `/re/kxmwp/`
- 公开 API 字面量：`"Frida.` → `"Kxmwp.`
- 线程 / 池：`gum-js-loop`、`frida-main-loop`、`frida-server-main-loop`、`pool-frida`、`"gmain"`、`"gdbus"`
- maps：`"linjector"` / `"winjector"`
- socket / pipe：`unix:frida`、`abstract/frida`、named pipe 前缀、`/frida-zymbiote-`
- SELinux / memfd：`frida_file`、`frida_memfd`、`memfd:frida`
- 资源文件名：`frida-*.version` / `.symbols`
- 产物：strip、dynsym `frida_` 同长改写、注入 TU 花指令
- host wheel：同一协议面（须成对）
- 默认端口仍 **27042**（用 `-l`）

未改：C ABI 标识符（如 `frida_agent_main`）、`github.com/frida/` wrap。

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

默认端口仍是 **27042**。换端口用 `-l`。装本 tag `host-wheels` 里同一 magic 的 wheel。PC 客户端必须是 **frida==16.7.19** 的魔改 wheel；用 17.x 的 `frida` 模块会报 major versions mismatch。

### `frida-ps`：`undefined symbol: kxmwp_agent_main`

16.x 列举进程会往 helper 注入 agent。静态改名把 dynstr 里的 `frida_agent_main` 改成 `kxmwp_agent_main` 后 **GNU hash 仍按旧名计算**，`frida-ps -H` 就会报这个错。修过的 `android-arm64` zip 已重传：只修导出 `*_agent_main` 的嵌入 agent ELF 的 hash，并把 agent 里残留的 GumJS 全局名 `Frida` 改成 `Kxmwp`（**不会**把 `Kxmwp.` 写回 `Frida.`，避免特征）。

```bash
adb push kxmwp-server /data/local/tmp/kxmwp-server
chmod 755 /data/local/tmp/kxmwp-server
./kxmwp-server -l 0.0.0.0:27042
adb forward tcp:27042 tcp:27042
frida-ps -H 127.0.0.1:27042
```

请重新下载本 tag 的 `frida-kxmwp-16.7.19-android-arm64-server.zip`。真机（Redmi Note 8 / Android 11 arm64）已验证：`frida-ps` 列举进程、attach/spawn 星巴克、`Java.perform`、`Activity.onResume`。

## 自己再编

GUI v4.0.8+ 源码重编译 → 版本 `16.7.19` → deep。Android 会按源码选用 NDK r25。Windows 交叉必须用 Frida `deps/toolchain-linux-x86_64/bin/ninja`（apt ninja 1.10 没有 `ninja -t inputs`）。

## 合规

仅供授权测试。去掉字样 ≠ 免杀。
