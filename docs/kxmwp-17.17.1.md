# Frida 17.17.0 深度魔改产物 r2（tag `kxmwp-17.17.1`）

在 [kxmwp-17.17.0](./kxmwp-17.17.0.md) 之上补了 **产物去符号、行为标记、注入 TU 花指令**，并修了几处会编坏树的替换。

Release：<https://github.com/suifei/fridare/releases/tag/kxmwp-17.17.1>

**这不是免杀包。** 去掉字样 ≠ 行为隐身。端口用 `-l` 自己指定。客户端必须用 **同一 magic** 的 host wheel。

上游仍是官方 Frida **17.17.0**；`17.17.1` 只是本仓库这批产物的 r2 标签。

## 这版相对 17.17.0 改了什么

| 项 | 实测（本机扫 catalog 二进制） |
|----|------------------------------|
| 协议面 | `kxmwp:rpc` / `re.kxmwp.` / `/re/kxmwp/` 都在；`frida:rpc` / `re.frida.` / `/re/frida/` = **0** |
| 产物去符号 | ELF dynsym `frida_*=0`；另把 Vala 残留 `_frida_*` / `on_frida_thread` 改成 `_kxmwp_*` |
| 行为标记 | 源码：`u:object_r:kxmwp_file`、`kxmwp_memfd`、`/kxmwp-zymbiote-`；meson `linjector.vala` **未**被误改。Android server 里能扫到 `kxmwp_file` / `u:object_r:kxmwp`。linux desktop server **不含** SELinux 代码（正常） |
| 嵌入 blob 路径 | 产物后处理把残留 `frida-zymbiote` 改成 `kxmwp-zymbiote`（同长度） |
| 注入 TU 花指令 | 源码含 `FRIDARE_JUNK`（constructor + **noinline** + rodata words）。linux server 种子常量各 **6** 处；win64 server 各 **13** 处、agent.dll 各 **3** 处（官方 strip 之后仍在） |
| Windows x64 | 源码重编 `kxmwp-server.exe`；本机 `-l 127.0.0.1:28888` 可 Listen |
| 其它平台 | **未全量重编**（小时级）；对 17.17.0 产物做了去符号 + 同长度标记补丁 |

## 用法（与上版相同）

```powershell
.\kxmwp-server.exe -l 0.0.0.0:28888
# 装 host-wheels 里同一 magic 的 frida + frida_tools
frida -H 127.0.0.1:28888 -n <进程名>
```

Android：`adb push` 到 `/data/local/tmp`，再 `adb forward tcp:28888 tcp:28888`。

默认监听仍是官方 **27042**（二进制里一般看不到 ASCII `27042`，是整型常量）。换端口用 `-l`，不必先改二进制。

随机 agent 落盘名前缀默认关（裸改 `frida-agent-` 会误伤 meson `.version`）。GUI 可选勾选，且只改 `*-64.so` 等 dump 名。

## 下载

zip 文件名带 `17.17.1`，内容仍是 Frida 17.17.0 + magic `kxmwp`：

| 资产 | 说明 |
|------|------|
| `frida-kxmwp-17.17.1-windows-x86_64-server.zip` | Win64 server / agent / gadget / helper / inject（stealth 重编） |
| `frida-kxmwp-17.17.1-windows-x86-server.zip` | Win32（去符号，未全量重编） |
| `frida-kxmwp-17.17.1-linux-x86_64-server.zip` | Linux x64（stealth 增量重编） |
| `frida-kxmwp-17.17.1-linux-arm64-server.zip` | Linux arm64（去符号） |
| `frida-kxmwp-17.17.1-android-arm64-server.zip` | Android arm64（去符号 + 标记补丁） |
| `frida-kxmwp-17.17.1-android-arm-server.zip` | Android arm |
| `frida-kxmwp-17.17.1-android-x86_64-server.zip` | 模拟器 x86_64 |
| `frida-kxmwp-17.17.1-android-x86-server.zip` | 模拟器 x86 |
| `frida-kxmwp-17.17.1-host-wheels.zip` | PC 客户端 wheel，必须与 server 成对 |

没有 iOS / macOS server，原因见 [kxmwp-17.17.0.md §6](./kxmwp-17.17.0.md)。

## 自己再编

Fridare GUI **源码重编译** → deep → 勾选 stealth（去符号 / 花指令 / 行为标记，默认开）→ 一键深度定制。需要 Docker。编译只在 Linux 容器里跑。静态「frida 魔改」只能勾导出后去符号，做不到花指令。

GUI 当前发行版仍是 [v4.0.6](https://github.com/suifei/fridare/releases/tag/v4.0.6)；本 tag 只发 **Frida 产物**，不替换 GUI latest。

## 明确做不到的

- 全面逻辑混淆 / 控制流平坦化（会把 Frida 编坏）
- 全树符号改名（meson / C ABI / valac 对不上）
- 保证免杀、保证过游戏/加固检测

phantom-frida 一类项目也写过：**去掉字样 ≠ 行为隐身**。

## 合规

仅供授权测试与自我环境定制。遵守当地法律和目标系统授权。
