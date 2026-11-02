# Frida 17.24.3 深度魔改产物（tag `xcand-17.24.3`）

上游是官方 Frida **17.24.3**；本 tag 是本仓库产品标签（magic=`xcand`）。  
本批经 GUI **源码重编译 + deep**：Android arm64 / x86_64 server + 多平台 host wheels。**不是 GUI latest。**

Release：<https://github.com/suifei/fridare/releases/tag/xcand-17.24.3>  
源码基线：`main@03f0bb6`

**不是免杀包。** 端口用 `-l`。客户端必须用 **同一 magic** 的 host wheel（pip 上是 `frida==17.24.3` 再打协议补丁）。

## 相对此前 kxmwp 产物的关键修复

- **[issue #41](https://github.com/suifei/fridare/issues/41)**：Android `backend_class != null` — deep 阶段重写预置 `helper.dex` 的 `re/frida`→`re/xcand`；导出时对齐嵌入 dex（含 string_ids 重排与校验），并写 helper 对齐校验旁路文件。
- catalog 多架构导出隔离：同次勾选 arm64 与 x86_64 时不再互相覆盖（见 v4.0.9 / [#38](https://github.com/suifei/fridare/issues/38)）。
- Windows 子进程无黑窗：`CREATE_NO_WINDOW`（GUI/e2e 编译路径；Linux/macOS 不变）。

## 用法

```powershell
# 解压本 tag 的 host-wheels.zip（zip 根目录是 host/<plat>/）
pip install --force-reinstall --no-deps .\host\windows-amd64\frida-17.24.3-cp37-abi3-win_amd64.whl
pip install --force-reinstall --no-deps .\frida_tools-14.11.0+frida.17.24.3.fridare.xcand-py3-none-any.whl
```

tools 文件名必须带 PEP 440 的 `+`：`frida_tools-14.11.0+frida.17.24.3.fridare.xcand-py3-none-any.whl`。不要用官方未魔改的 `frida` 连本包 server。

### Android

```bash
adb push xcand-server /data/local/tmp/xcand-server
chmod 755 /data/local/tmp/xcand-server
# 若 tmp 不能执行：cp 到 /data/xcand-server
./xcand-server -l 0.0.0.0:27042
```

PC 端：

```powershell
adb forward tcp:27042 tcp:27042
frida -H 127.0.0.1:27042 -n <进程名>
```

若仍看到 `backend_class != null`，请确认下载的是本 tag 的 android zip（helper.dex 已对齐 `re.xcand`），不要混用旧 `kxmwp-*` server。

Frida **17** 的 Python `session.create_script` **不会**自动带上 `Java`。`frida` CLI / `frida-trace` 会加载 `frida-java-bridge`。自己写 Python 注入时要处理 `frida:load-bridge`，否则 `Java.perform` 报 `Java is not defined`。

默认监听仍是官方 **27042**。换端口用 `-l`。

## 下载

| 资产 | 说明 |
|------|------|
| `frida-xcand-17.24.3-android-arm64-server.zip` | 真机 arm64（deep + helper.dex 对齐） |
| `frida-xcand-17.24.3-android-x86_64-server.zip` | 模拟器 x86_64（已验证计算器 hook） |
| `frida-xcand-17.24.3-host-wheels.zip` | PC 客户端 wheel，必须与 server 成对 |

**本 tag 只有 Android 上述两架构 + host wheels。** 不含 iOS / macOS / Windows / Linux server。iOS 请走路线 A：GUI「iOS DEB 魔改」或 `fridare-patch` 改官方 deb。历史多平台包见 [kxmwp-17.17.1](./kxmwp-17.17.1.md)。

## 自己再编

GUI **v4.1.0+**（建议带 `03f0bb6` 及以后的 helper.dex / catalog 修复）→ 源码重编译 → 版本填 `17.24.3` → magic 五位小写（如 `xcand`）→ deep。需要 Docker。

## 明确做不到的

- 全面逻辑混淆 / 全树符号改名
- 保证免杀、保证过游戏/加固检测

phantom-frida 一类项目也写过：**去掉字样 ≠ 行为隐身**。

## 合规

仅供授权测试与自我环境定制。遵守当地法律和目标系统授权。
