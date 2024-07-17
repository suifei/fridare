# iOS frida-server 魔改教程

## 前提条件

- 安装 fridare.sh 脚本
- 确保系统中已安装 Python 3 和 Golang
- 确保已安装 Frida 和相关依赖

## 步骤

### 1. 运行魔改脚本

使用以下命令开始魔改过程：

```bash
$ ./fridare.sh build -latest
```

这个命令会自动下载最新版本的 Frida，并开始魔改过程。

### 2. 确认免责声明

脚本会显示一个免责声明。仔细阅读并确认是否同意：

```
您是否同意以上免责声明并允许使用sudo权限？(y/N) y
```

输入 `y` 并按回车以继续。

### 3. 等待下载和修改过程

脚本会自动完成以下步骤：

- 下载最新版本的 Frida deb 包（适用于 arm 和 arm64 架构）
- 修改 plist 文件
- 修改 DEBIAN 文件夹中的文件
- 修改二进制文件
- 重新打包 deb 文件

### 4. 查看输出信息

魔改完成后，脚本会显示重要信息：

```
[INFO] 新版本名：duquj
[INFO] 请使用新版本名：duquj 进行调试
[INFO] 请使用端口：8899 进行调试
[INFO] 新版本 deb 文件：../dist/frida_16.4.4_iphoneos-arm64_duquj_tcp.deb
```

记下新的版本名（本例中为 "duquj"）和端口号，这些在之后的使用中会用到。

### 5. 安装到 iOS 设备

按照脚本提供的说明，将魔改后的 deb 文件安装到 iOS 设备上：

```
[INFO] iPhone 安装：
[INFO] scp dist/frida_16.4.4_iphoneos-arm64_duquj_tcp.deb root@<iPhone-IP>:/var/root
[INFO] ssh root@<iPhone-IP>
[INFO] dpkg -i /var/root/frida_16.4.4_iphoneos-arm64_duquj_tcp.deb
```

替换 `<iPhone-IP>` 为你的 iOS 设备实际 IP 地址。

### 6. 连接和使用

使用以下命令连接到魔改后的 frida-server：

```
[INFO] PC 连接：
[INFO] frida -U -f com.xxx.xxx -l
[INFO] frida -H <iPhone-IP>:8899 -f com.xxx.xxx --no-pause
```

将 `com.xxx.xxx` 替换为目标应用的包名，`<iPhone-IP>` 替换为 iOS 设备的 IP 地址。

### 7. 修改 frida-tools（可选）

脚本会询问是否要修改本地的 frida-tools 以适配魔改版本：

```
本脚本将自动修改本地 frida-tools，以适配魔改版本的 Frida。（跳过 frida-tools 魔改。某些功能可能无法使用，建议修改）
您是否同意？(y/N)
```

如果需要完整功能，建议输入 `y` 同意修改。

## 注意事项

1. 每次运行脚本都会生成新的魔改名称，确保使用最新生成的名称和端口号。
2. 魔改后的 frida-server 可能会绕过一些基本的检测，但不保证能绕过所有检测机制。
3. 使用魔改版本可能会影响某些 Frida 功能，特别是如果没有修改 frida-tools。
4. 确保在合法和授权的情况下使用此工具。

