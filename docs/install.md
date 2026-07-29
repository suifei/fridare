# Fridare 安装说明

Fridare 是一个用于魔改 Frida 的工具，可以帮助你绕过一些基本的检测机制。以下是安装和配置 Fridare 的步骤。

## 快速安装

使用以下命令可以快速安装 Fridare：

```bash
curl -s https://raw.githubusercontent.com/suifei/fridare/main/fridare.sh | bash -s install
```

这个命令会在当前目录下创建一个 `fridare` 文件夹，其中包含了安装好的 Fridare 工具。

## 配置环境变量

安装完成后，你需要将 Fridare 添加到系统的环境变量中，以便可以在任何位置使用 `fridare` 命令。

### 对于 Bash 用户

1. 编辑你的 `.bashrc` 文件：

   ```bash
   nano ~/.bashrc
   ```

2. 在文件末尾添加以下行：

   ```bash
   export PATH=$PATH:/path/to/fridare
   ```

   请将 `/path/to/fridare` 替换为实际的 Fridare 安装路径。

3. 保存并关闭文件，然后运行：

   ```bash
   source ~/.bashrc
   ```

### 对于 Zsh 用户

1. 编辑你的 `.zshrc` 文件：

   ```bash
   nano ~/.zshrc
   ```

2. 在文件末尾添加以下行：

   ```bash
   export PATH=$PATH:/path/to/fridare
   ```

   请将 `/path/to/fridare` 替换为实际的 Fridare 安装路径。

3. 保存并关闭文件，然后运行：

   ```bash
   source ~/.zshrc
   ```

## 验证安装

安装完成后，你可以通过运行以下命令来验证 Fridare 是否已正确安装：

```bash
fridare.sh help
```

如果安装成功，你应该能看到 Fridare 的版本信息。

## 使用 Fridare

现在你可以在终端的任何位置使用 `fridare.sh` 命令来运行 Fridare 工具。例如：

```bash
fridare.sh build -latest
```

## 构建 GUI 版本

### Windows 本机

需要安装 [Go](https://go.dev/dl/)（1.21+）。可选安装 fyne CLI：

```powershell
go install fyne.io/fyne/v2/cmd/fyne@latest
```

然后在 `ui` 目录执行：

```powershell
cd ui
.\build.ps1
# 或
.\build.bat
```

生成：`ui\build\fridare-gui.exe`、`fridare-create.exe`、`fridare-patch.exe`。

### Linux / macOS 本机构建

```bash
cd ui
./build.sh
```

### 在 Linux/macOS 上交叉编译 Windows GUI

```bash
# Ubuntu: sudo apt install gcc-mingw-w64
# macOS: brew install mingw-w64
cd ui
./build.sh windows
```

说明：Windows 专用 API（如隐藏控制台窗口）已使用 Go build tags 隔离，交叉编译时不会再出现 `HideWindow` / `CreationFlags` 编译错误。

这个命令会下载并魔改最新版本的 Frida。

## 注意事项

- 确保你有足够的权限来修改系统环境变量。
- 如果你使用的是其他 shell，请相应地修改相关的配置文件。
- 定期检查 Fridare 的更新，以获得最新的功能和安全修复。

现在你已经成功安装了 Fridare，可以开始使用它来魔改 Frida 了。请确保在合法和授权的情况下使用此工具。