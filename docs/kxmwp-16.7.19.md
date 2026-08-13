# Frida 16.7.19 深度魔改产物（tag `kxmwp-16.7.19`）

官方 Frida **16.7.19**（`git clone --depth=1 --branch 16.7.19`），magic=`kxmwp`，经 **GUI 源码重编译同一条 JobConfig**（`e2e-rebuild` = 源码页 `startDevelop`：deep + 去符号/花指令/行为标记）在 Docker 内编译。

Release：<https://github.com/suifei/fridare/releases/tag/kxmwp-16.7.19>

**不是免杀包。** 行为隐身覆盖运行时可见向量，**不能保证**杀软/游戏加固效果。端口用 `-l`。客户端必须同一 magic 的 host wheel。

与 [17.17.1 产物](./kxmwp-17.17.1.md) 对照：上游 16.7.19 vs 官方 17.17.0（产品标签 17.17.1）；协议面与用法相同。

## 本批实测（linux-x86_64 `kxmwp-server`）

| 串 | 次数 |
|----|------|
| `kxmwp:rpc` | 12 |
| `re.kxmwp.` | 48 |
| `/re/kxmwp/` | 21 |
| `frida:rpc` / `re.frida.` / `/re/frida/` | **0** |

`PROTOCOL-SYNC.json`：serverOK + clientOK。

## 用法

```powershell
# Linux 产物是 ELF kxmwp-server（本 tag 本轮只发 linux-x86_64）
./kxmwp-server -l 0.0.0.0:28888
# 装本 tag host-wheels 里同一 magic 的 frida + frida_tools
frida -H 127.0.0.1:28888 -n <进程名>
```

默认监听仍是官方 **27042**。换端口用 `-l`。

## 下载

| 资产 | 说明 |
|------|------|
| `frida-kxmwp-16.7.19-linux-x86_64-server.zip` | Linux x64 server / agent / gadget（GUI 路径 stealth 重编） |
| `frida-kxmwp-16.7.19-host-wheels.zip` | PC 客户端 wheel，必须与 server 成对 |

## 自己再编

GUI **v4.0.8+** 源码重编译 → 版本填 `16.7.19` → deep → stealth 默认开 → 一键深度定制。需要 Docker。编译只在 Linux 容器里跑。

16.x 官方 SDK 的 `v8-mksnapshot` 在本 builder 上会 SIGTRAP，流水线自动 `-Dfrida-core:compiler_snapshot=disabled`。另 `--disable-frida-tools`（host wheel 在本机打）。

## 明确做不到的

- 全面逻辑混淆 / 全树符号改名
- 保证免杀、保证过游戏/加固检测

## 合规

仅供授权测试与自我环境定制。遵守当地法律和目标系统授权。
