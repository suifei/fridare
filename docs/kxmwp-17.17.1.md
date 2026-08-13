# Frida 17.17.0 深度魔改产物（tag `kxmwp-17.17.1`）

上游仍是官方 Frida **17.17.0**；`17.17.1` 是本仓库产品标签（`EffectiveCloneRef` / `EffectivePipVersion` → 17.17.0）。  
本轮 **linux-x86_64** 经 **GUI JobConfig + Orchestrator** 全量重编（deep + stealth）。其它平台 zip 仍是先前 r2 去符号/补丁包。

Release：<https://github.com/suifei/fridare/releases/tag/kxmwp-17.17.1>

**不是免杀包。** 端口用 `-l`。客户端必须用 **同一 magic** 的 host wheel（pip 上是 `frida==17.17.0` 再打协议补丁）。

## 本轮 GUI 路径 linux-x86_64 扫描

| 串 | `kxmwp-server` |
|----|----------------|
| `kxmwp:rpc` | 13 |
| `re.kxmwp.` | 50 |
| `/re/kxmwp/` | 22 |
| `frida:rpc` / `re.frida.` / `/re/frida/` | **0** |

## 用法

```powershell
# Windows 包仍是 kxmwp-server.exe；Linux 本轮是 ELF kxmwp-server
.\kxmwp-server.exe -l 0.0.0.0:28888
# 装 host-wheels 里同一 magic 的 frida + frida_tools
frida -H 127.0.0.1:28888 -n <进程名>
```

默认监听仍是官方 **27042**。换端口用 `-l`。

## 下载

zip 文件名带 `17.17.1`，引擎仍是 Frida 17.17.0 + magic `kxmwp`：

| 资产 | 说明 |
|------|------|
| `frida-kxmwp-17.17.1-linux-x86_64-server.zip` | Linux x64（**本轮 GUI 路径 stealth 重编**） |
| `frida-kxmwp-17.17.1-windows-x86_64-server.zip` | Win64（先前 stealth 重编） |
| `frida-kxmwp-17.17.1-windows-x86-server.zip` | Win32（去符号，未全量重编） |
| `frida-kxmwp-17.17.1-linux-arm64-server.zip` | Linux arm64（去符号） |
| `frida-kxmwp-17.17.1-android-arm64-server.zip` | Android arm64（去符号 + 标记补丁） |
| `frida-kxmwp-17.17.1-android-arm-server.zip` | Android arm |
| `frida-kxmwp-17.17.1-android-x86_64-server.zip` | 模拟器 x86_64 |
| `frida-kxmwp-17.17.1-android-x86-server.zip` | 模拟器 x86 |
| `frida-kxmwp-17.17.1-host-wheels.zip` | PC 客户端 wheel，必须与 server 成对 |

没有 iOS / macOS server，原因见 [kxmwp-17.17.0.md §6](./kxmwp-17.17.0.md)。

## 自己再编

GUI **v4.0.8+** 源码重编译 → 版本可填 `17.17.1`（会 clone 官方 17.17.0）或 `17.17.0` → deep → stealth 默认开。需要 Docker。

## 明确做不到的

- 全面逻辑混淆 / 全树符号改名
- 保证免杀、保证过游戏/加固检测

phantom-frida 一类项目也写过：**去掉字样 ≠ 行为隐身**。

## 合规

仅供授权测试与自我环境定制。遵守当地法律和目标系统授权。
