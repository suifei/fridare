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
# 解压本 tag 已替换的 host-wheels.zip（zip 根目录是 host/<plat>/，没有 python/ 前缀）
pip install --force-reinstall --no-deps .\host\windows-amd64\frida-17.17.0-cp37-abi3-win_amd64.whl
pip install --force-reinstall --no-deps .\frida_tools-14.10.4+frida.17.17.0.fridare.kxmwp-py3-none-any.whl
frida -H 127.0.0.1:28888 -n <进程名>
```

tools 文件名必须带 PEP 440 的 `+`：`frida_tools-14.10.4+frida.17.17.0.fridare.kxmwp-py3-none-any.whl`。请重新下载 `frida-kxmwp-17.17.1-host-wheels.zip`（INSTALL.txt 已改成相对路径 `host/<plat>/`）。旧 zip 若仍是 `14.10.4.frida.`，pip 会报 `Invalid wheel filename`。

### Android：`backend_class != null` abort

旧 android-arm64 zip 里 helper dex 仍是 `Lre/frida/HelperBackend`，native 已改 `re.kxmwp`，启动立刻：

`kxmwp_android_helper_service_do_start: assertion failed: (backend_class != null)`

请重新下载本 tag 的 `frida-kxmwp-17.17.1-android-arm64-server.zip`（已替换）。导出流水线会跑 `PatchAndroidHelperInBinary`（dex 描述符 + SHA-1/Adler32 + `kxmwp_agent_main` GNU/sysv hash）。

```bash
adb push kxmwp-server /data/local/tmp/kxmwp-server
chmod 755 /data/local/tmp/kxmwp-server
# 若 tmp 不能执行：cp 到 /data/kxmwp-server
./kxmwp-server -l 0.0.0.0:27042
```

MIUI `theme_compatibility.xml` ENOENT 可忽略。PC 端：

```powershell
adb forward tcp:27042 tcp:27042
pip install --force-reinstall --no-deps frida-17.17.0-*.whl
pip install --force-reinstall --no-deps frida_tools-14.10.4+frida.17.17.0.fridare.kxmwp-py3-none-any.whl
frida -H 127.0.0.1:27042 -n Starbucks -l hook.js
```

Frida **17** 的 Python `session.create_script` **不会**自动带上 `Java`。`frida` CLI / `frida-trace` 会加载 `frida-java-bridge`。自己写 Python 注入时要处理 `frida:load-bridge`（与 REPL 相同），否则 `Java.perform` 报 `Java is not defined`。

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
