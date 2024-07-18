# Frida-tools 魔改指南

fridare.sh 脚本提供了 `patch-tools` 命令，允许您修改 frida-tools 以适配魔改版本的 Frida。本指南将帮助您使用这个功能。

## 1. 修改 frida-tools

### 使用配置的魔改名

如果您已经在配置中设置了 `FRIDA_NAME`，可以直接使用：

```bash
./fridare.sh patch-tools name
```

这将使用配置中的魔改名来修改 frida-tools。

### 指定新的魔改名

如果您想使用一个不同的魔改名，可以直接在命令中指定：

```bash
./fridare.sh patch-tools name abcde
```

这里的 `abcde` 就是您指定的新魔改名。请确保使用恰好 5 个字母（a-z 或 A-Z）。

### 使用随机生成的魔改名

如果您既没有在配置中设置 `FRIDA_NAME`，也没有在命令中指定，脚本会随机生成一个魔改名：

```bash
./fridare.sh patch-tools name
```

## 2. 确认操作

执行命令后，脚本会显示找到的 frida-tools 路径，并询问您是否确认使用此路径。输入 'y' 确认，或 'n' 取消操作。

## 3. 查看结果

脚本会显示修改过程的详细信息，包括：

- 创建备份文件
- 修改 Python 库文件
- 更新 core.py 文件中的相关字符串

## 4. 恢复原版

如果您需要恢复 frida-tools 到原版，可以使用以下命令：

```bash
./fridare.sh patch-tools restore
```

这将使用之前创建的备份文件来恢复原始的 frida-tools 文件。

## 注意事项

1. 请确保在修改 frida-tools 之前已经成功构建了魔改版本的 Frida。
2. 修改操作会创建备份文件，以便于日后恢复。
3. 如果遇到权限问题，可能需要使用 sudo 运行脚本。
4. 修改后，建议测试 frida-tools 的功能，确保一切正常工作。

通过使用 `patch-tools` 命令，您可以轻松地将 frida-tools 与您的魔改版 Frida 保持同步，确保整个工具链的兼容性。