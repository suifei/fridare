# Android frida-server 魔改教程

## 前提条件

- 安装 fridare.sh 脚本
- 确保系统中已安装 Python 3 和 Golang
- 确保已安装 Frida

## 步骤

### 1. 检查环境

首先，运行 `fridare.sh upgrade` 命令来检查环境并更新 FRIDA_MODULES：

```bash
$ ./fridare.sh upgrade
```

这将显示您的环境信息，包括 Python 版本、Frida 版本、Go 版本等。

### 2. 配置代理（可选）

如果需要使用代理，可以通过以下命令设置：

```bash
$ ./fridare.sh conf edit
```

选择选项 1 来编辑 HTTP 代理，输入代理地址（例如：socks5://localhost:1080）。

### 3. 查看可用的 Frida 模块

运行以下命令来查看所有可用的 Frida 模块：

```bash
$ ./fridare.sh lm
```

这将列出所有可用的 Frida 模块，包括它们支持的操作系统和架构。

### 4. 魔改 frida-server

使用以下命令来修补 frida-server：

```bash
$ ./fridare.sh patch -m frida-server -latest -os android -arch arm64 -o ./patched
```

参数说明：
- `-m frida-server`: 指定要修补的模块
- `-latest`: 使用最新版本的 Frida
- `-os android`: 指定操作系统为 Android
- `-arch arm64`: 指定架构为 arm64
- `-o ./patched`: 指定输出目录

### 5. 输入魔改名称

在执行过程中，脚本会提示您输入一个魔改名称。这个名称应该是 5 个字母（a-z 或 A-Z）。例如：

```
请输入本次所采用的 Frida 魔改名: axjdf
```

### 6. 等待修补完成

脚本会自动下载指定的 frida-server，解压，然后进行修补。整个过程包括：

- 下载 frida-server
- 解压文件
- 修补二进制文件
- 替换字符串

### 7. 查看输出

修补完成后，您会看到类似以下的输出：

```
[SUCC] 模块修补完成: ./patched/frida-server_axjdf
```

这表示魔改后的 frida-server 已经生成，文件名为 `frida-server_axjdf`。

## 使用魔改后的 frida-server

1. 将修补后的 `frida-server_axjdf` 文件传输到 Android 设备上。
2. 赋予文件执行权限：`chmod +x frida-server_axjdf`
3. 在 Android 设备上运行魔改后的 frida-server：`./frida-server_axjdf`

注意：使用魔改后的 frida-server 时，确保客户端代码中使用了相同的魔改名称（本例中为 "axjdf"）。

通过这个教程，您可以成功地魔改 Android 版本的 frida-server，使其更难被检测到。记得每次使用时都更换魔改名称，以提高隐蔽性。