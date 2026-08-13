# Frida 16.7.19 深度魔改产物（tag `kxmwp-16.7.19`）

官方 Frida **16.7.19**（`git clone --depth=1 --branch 16.7.19`），magic=`kxmwp`，经 **GUI 源码重编译同一条 JobConfig**（deep + 去符号/花指令/行为标记）在 Docker 内编译。

Release：<https://github.com/suifei/fridare/releases/tag/kxmwp-16.7.19>

**不是免杀包。** 端口用 `-l`。客户端必须同一 magic 的 host wheel。

与 [17.17.1 产物](./kxmwp-17.17.1.md) 对照：上游版本不同（16.7.19 vs 官方 17.17.0，产品标签 17.17.1）；协议面与用法相同。

## 用法

```powershell
.\kxmwp-server.exe -l 0.0.0.0:28888
# 装本 tag host-wheels 里同一 magic 的 frida + frida_tools
frida -H 127.0.0.1:28888 -n <进程名>
```

## 构建

GUI v4.0.8+：源码重编译 → 版本填 `16.7.19` → deep → stealth 默认开 → 一键深度定制。编译只在 Docker/Linux。
