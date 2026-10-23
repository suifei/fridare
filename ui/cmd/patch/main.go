package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fridare-gui/internal/core"
	"fridare-gui/internal/i18n"
)

func main() {
	i18n.InitFromUserConfig()
	if len(os.Args) < 4 {
		fmt.Println(i18n.T("用法: fridare-patch.exe <输入DEB文件> <输出DEB文件> <魔改名称> [端口]"))
		fmt.Println(i18n.T("示例: fridare-patch.exe frida_17.2.17_iphoneos-arm64.deb frida_modified.deb test-frida 27042"))
		fmt.Println("")
		fmt.Println(i18n.T("说明:"))
		fmt.Println(i18n.T("  - 输入DEB文件: 原始的frida DEB包文件路径"))
		fmt.Println(i18n.T("  - 输出DEB文件: 修改后的DEB包输出路径"))
		fmt.Println(i18n.T("  - 魔改名称: 用于替换frida字符串的5字符名称"))
		fmt.Println(i18n.T("  - 端口: 可选，服务端口号，默认27042"))
		os.Exit(1)
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]
	magicName := os.Args[3]
	port := 27042
	if len(os.Args) > 4 {
		fmt.Sscanf(os.Args[4], "%d", &port)
	}

	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println(i18n.T("=== Fridare DEB包修改工具 ==="))
	fmt.Printf(i18n.T("输入文件: %s\n"), inputPath)
	fmt.Printf(i18n.T("输出文件: %s\n"), outputPath)
	fmt.Printf(i18n.T("魔改名称: %s\n"), magicName)
	fmt.Printf(i18n.T("端口: %d\n"), port)
	fmt.Println("=============================")
	fmt.Println()

	// 检查输入文件是否存在
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		log.Fatalf(i18n.T("错误: 输入文件不存在: %s"), inputPath)
	}

	// 创建输出目录
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf(i18n.T("错误: 创建输出目录失败: %v"), err)
	}

	// 创建DEB修改器
	modifier := core.NewDebModifier(inputPath, outputPath, magicName, port)

	// 进度回调函数
	progressCallback := func(progress float64, message string) {
		fmt.Printf("[%.0f%%] %s\n", progress*100, message)
	}

	// 执行修改
	err := modifier.ModifyDebPackage(progressCallback)
	if err != nil {
		log.Fatalf(i18n.T("错误: DEB包修改失败: %v"), err)
	}

	fmt.Println()
	fmt.Println(i18n.T("✅ DEB包修改成功完成!"))

	// 显示输出文件信息
	if stat, err := os.Stat(outputPath); err == nil {
		fmt.Printf(i18n.T("输出文件: %s\n"), outputPath)
		fmt.Printf(i18n.T("文件大小: %.2f MB\n"), float64(stat.Size())/1024/1024)
	}

	fmt.Println()
	fmt.Println(i18n.T("📦 安装命令:"))
	fmt.Printf("  dpkg -i %s\n", filepath.Base(outputPath))
	fmt.Println()
	fmt.Println(i18n.T("🔧 服务控制:"))
	fmt.Printf(i18n.T("  启动: launchctl load /Library/LaunchDaemons/re.%s.server.plist\n"), magicName)
	fmt.Printf(i18n.T("  停止: launchctl unload /Library/LaunchDaemons/re.%s.server.plist\n"), magicName)
	fmt.Println()
	fmt.Println(i18n.T("🌐 连接信息:"))
	fmt.Printf(i18n.T("  端口: %d\n"), port)
	fmt.Printf(i18n.T("  frida命令: frida -H <设备IP>:%d <进程名>\n"), port)
}
