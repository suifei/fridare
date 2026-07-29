# Fridare Windows 使用指南

## 重要：`.cmd` / `.bat` 编码约定（Windows）

**cmd.exe 默认使用系统代码页（中文 Windows 多为 GBK/CP936），不是 UTF-8。**  
若在 `.cmd` / `.bat` 中写入中文（即使文件是 UTF-8），会导致：

- 提示乱码、条件判断异常、`goto` 标签失效
- 脚本中途退出或“执行不正常”

**约定（必须遵守）：**

| 文件类型 | 语言 |
|----------|------|
| `*.cmd` / `*.bat` | **仅 ASCII 英文**（注释、echo、提示全部英文） |
| `*.ps1` / GUI / Markdown | 可用中文（PowerShell / GUI 支持 Unicode） |

仓库内脚本：`win/patch-frida.cmd`、`win/patch-frida-tools.cmd`、`ui/build.bat` 均为全英文。  
贡献代码时请勿向这些文件添加中文；可用 `test_cmd_ascii.ps1` 做检查。

## 最新更新

### v4.0.1

- 完善 `patch-frida-tools.cmd`：自动查找 `_frida*.pyd`、同步修改 `core.py` 中的 `frida:rpc`
- 新增 GUI 本机构建脚本：`ui/build.ps1`、`ui/build.bat`（无需 bash）
- 修复 GUI 过度替换 Python 源码导致 `import _xxxxx` 失败的问题
- **Windows `.cmd`/`.bat` 全英文**：避免 cmd 代码页与 UTF-8 中文冲突

### v3.1.5 - Windows 支持

- 新增 `patch-frida.cmd` 脚本,用于在 Windows 环境下修改 frida-server
- 新增 `patch-frida-tools.cmd` 脚本,用于在 Windows 环境下修改 frida-tools
- 增加对 Windows 平台的全面支持
- 更新了使用说明,增加了 Windows 平台的详细教程

## 构建 GUI（Windows）

```powershell
# 安装 Go 后
go install fyne.io/fyne/v2/cmd/fyne@latest   # 可选
cd ui
.\build.ps1
.\build\fridare-gui.exe
```

## Windows 下的使用教程

### 准备工作

1. 确保你的系统已经安装了 Python 和 pip
2. 下载 Fridare 项目到本地

### 修改 frida-server

1. 打开命令提示符,进入 Fridare 项目目录
2. 运行以下命令:

```
patch-frida.cmd <frida-server路径> <5字符魔改名>
```

例如:

```
patch-frida.cmd frida-server-16.4.7-android-arm64 abcde
```

3. 脚本将会生成一个修改后的 frida-server 文件,文件名为 `frida-server-16.4.7-android-arm64_abcde`

### 修改 frida-tools

1. 在命令提示符中运行:

```
patch-frida-tools.cmd
```

2. 脚本会自动定位 frida 的安装路径
3. 根据提示输入 5 个字符的魔改名(必须是小写字母 a-z)
4. 脚本会自动修改 `core.py` 和 `_frida.pyd` 文件

### 注意事项

- 在修改 frida-tools 之前,脚本会自动备份原文件
- 确保你有足够的权限修改 Python 安装目录下的文件
- 修改后,建议重新启动你的 Python 环境以确保更改生效

## 故障排除

如果遇到 "Error: hexreplace tool not found" 错误,请确保 `hexreplace_windows_amd64.exe` 文件位于与脚本相同的目录中。

如果修改过程中遇到权限问题,尝试以管理员身份运行命令提示符。

## 恢复原始文件

如果需要恢复原始的 frida-tools 文件:

1. 找到 frida 的安装目录 (通常在运行 `patch-frida-tools.cmd` 时会显示)
2. 将 `core.py.fridare` 重命名为 `core.py`
3. 将 `_frida.pyd.fridare` 重命名为 `_frida.pyd`

## 贡献

欢迎提交问题和拉取请求。对于重大更改,请先开 issue 讨论您想要更改的内容。

## 许可证

[MIT LICENSE](LICENSE)
