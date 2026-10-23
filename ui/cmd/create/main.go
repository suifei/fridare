package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"fridare-gui/internal/core"
	"fridare-gui/internal/i18n"
)

func main() {
	i18n.InitFromUserConfig()
	var (
		fridaServerPath  = flag.String("server", "", i18n.T("frida-server文件路径 (必需)"))
		fridaAgentPath   = flag.String("agent", "", i18n.T("frida-agent.dylib文件路径 (必需)"))
		outputPath       = flag.String("output", "", i18n.T("输出DEB文件路径 (必需)"))
		magicName        = flag.String("magic", "", i18n.T("魔改名称 (5个字符, 必需)"))
		port             = flag.Int("port", 27042, i18n.T("服务端口 (默认: 27042)"))
		isRootless       = flag.Bool("rootless", false, i18n.T("是否为rootless结构 (默认: false, 即root结构)"))
		packageName      = flag.String("name", "", i18n.T("包名 (可选, 自动生成)"))
		version          = flag.String("version", "17.2.17", i18n.T("版本号 (默认: 17.2.17)"))
		architecture     = flag.String("arch", "iphoneos-arm64", i18n.T("架构 (默认: iphoneos-arm64)"))
		maintainer       = flag.String("maintainer", "Fridare Team <support@fridare.com>", i18n.T("维护者"))
		description      = flag.String("desc", "", i18n.T("包描述 (可选, 自动生成)"))
		depends          = flag.String("depends", "firmware (>= 12.0)", i18n.T("依赖 (默认: firmware (>= 12.0))"))
		section          = flag.String("section", "Development", i18n.T("分类 (默认: Development)"))
		priority         = flag.String("priority", "optional", i18n.T("优先级 (默认: optional)"))
		homepage         = flag.String("homepage", "https://frida.re/", i18n.T("主页 (默认: https://frida.re/)"))
		extractDebPath   = flag.String("extract-deb", "", i18n.T("从现有DEB包中提取frida-agent.dylib (可选)"))
		extractAgentOnly = flag.Bool("extract-agent-only", false, i18n.T("仅提取agent文件到当前目录，不创建新DEB包"))
		help             = flag.Bool("help", false, i18n.T("显示帮助信息"))
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, i18n.T("Fridare DEB包创建工具\n\n"))
		fmt.Fprintf(os.Stderr, i18n.T("用法: %s [选项]\n\n"), os.Args[0])
		fmt.Fprintf(os.Stderr, i18n.T("选项:\n"))
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, i18n.T("\n示例:\n"))
		fmt.Fprintf(os.Stderr, i18n.T("  # 创建Root结构的DEB包\n"))
		fmt.Fprintf(os.Stderr, "  %s -server frida-server -agent frida-agent.dylib -magic agent -output agent.deb\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, i18n.T("  # 创建Rootless结构的DEB包\n"))
		fmt.Fprintf(os.Stderr, "  %s -server frida-server -agent frida-agent.dylib -magic agent -rootless -port 27043 -output agent-rootless.deb\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, i18n.T("  # 从现有DEB包中提取agent并创建新DEB包\n"))
		fmt.Fprintf(os.Stderr, "  %s -server frida-server -extract-deb frida_17.2.17_iphoneos-arm64.deb -magic agent -output agent.deb\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, i18n.T("  # 仅从DEB包中提取agent文件\n"))
		fmt.Fprintf(os.Stderr, "  %s -extract-deb frida_17.2.17_iphoneos-arm64.deb -extract-agent-only\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, i18n.T("注意:\n"))
		fmt.Fprintf(os.Stderr, i18n.T("  - magic名称必须是5个字符，且符合命名规则 (字母开头，包含字母数字)\n"))
		fmt.Fprintf(os.Stderr, i18n.T("  - rootless结构用于现代越狱环境 (如checkra1n, unc0ver等)\n"))
		fmt.Fprintf(os.Stderr, i18n.T("  - root结构用于传统越狱环境\n"))
		fmt.Fprintf(os.Stderr, i18n.T("  - frida-agent.dylib文件是必需的，确保完整功能\n"))
	}

	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	// 处理仅提取agent文件的情况
	if *extractAgentOnly {
		if *extractDebPath == "" {
			fmt.Fprintf(os.Stderr, i18n.T("错误: 使用 -extract-agent-only 时必须指定 -extract-deb 参数\n\n"))
			flag.Usage()
			os.Exit(1)
		}

		fmt.Printf(i18n.T("INFO: 该功能将在后续版本中实现\n"))
		fmt.Printf(i18n.T("INFO: 当前可以手动解压DEB包获取agent文件:\n"))
		fmt.Printf("  ar -x %s\n", *extractDebPath)
		fmt.Printf("  tar -xf data.tar.xz\n")
		fmt.Printf("  find . -name '*agent*.dylib'\n")
		return
	}

	// 如果指定了extract-deb参数，尝试从中获取agent文件路径
	if *extractDebPath != "" && *fridaAgentPath == "" {
		fmt.Printf(i18n.T("INFO: 从DEB包中自动提取agent文件功能将在后续版本中实现\n"))
		fmt.Printf(i18n.T("INFO: 请手动提取agent文件并使用 -agent 参数指定\n"))
	}

	// 验证必需参数
	if *fridaServerPath == "" {
		fmt.Fprintf(os.Stderr, i18n.T("错误: 必须指定frida-server文件路径 (-server)\n\n"))
		flag.Usage()
		os.Exit(1)
	}

	if *fridaAgentPath == "" && *extractDebPath == "" {
		fmt.Fprintf(os.Stderr, i18n.T("错误: 必须指定frida-agent.dylib文件路径 (-agent) 或使用 -extract-deb 从现有DEB包提取\n\n"))
		flag.Usage()
		os.Exit(1)
	}

	if *outputPath == "" {
		fmt.Fprintf(os.Stderr, i18n.T("错误: 必须指定输出DEB文件路径 (-output)\n\n"))
		flag.Usage()
		os.Exit(1)
	}

	if *magicName == "" {
		fmt.Fprintf(os.Stderr, i18n.T("错误: 必须指定魔改名称 (-magic)\n\n"))
		flag.Usage()
		os.Exit(1)
	}

	// 验证magic名称
	if len(*magicName) != 5 {
		fmt.Fprintf(os.Stderr, i18n.T("错误: 魔改名称必须是5个字符，当前: %d个字符\n"), len(*magicName))
		os.Exit(1)
	}

	if !isValidMagicName(*magicName) {
		fmt.Fprintf(os.Stderr, i18n.T("错误: 魔改名称格式无效，必须以字母开头，包含字母和数字\n"))
		os.Exit(1)
	}

	// 验证文件存在
	if _, err := os.Stat(*fridaServerPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, i18n.T("错误: frida-server文件不存在: %s\n"), *fridaServerPath)
		os.Exit(1)
	}

	// 检查frida-server文件大小
	if stat, err := os.Stat(*fridaServerPath); err == nil {
		if stat.Size() < 1024*1024 { // 小于1MB可能有问题
			fmt.Fprintf(os.Stderr, i18n.T("警告: frida-server文件大小异常: %.2f MB\n"), float64(stat.Size())/(1024*1024))
		} else {
			fmt.Printf(i18n.T("INFO: frida-server文件大小: %.2f MB\n"), float64(stat.Size())/(1024*1024))
		}
	}

	if *fridaAgentPath != "" {
		if _, err := os.Stat(*fridaAgentPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, i18n.T("错误: frida-agent文件不存在: %s\n"), *fridaAgentPath)
			os.Exit(1)
		}
		// 检查frida-agent文件大小
		if stat, err := os.Stat(*fridaAgentPath); err == nil {
			fmt.Printf(i18n.T("INFO: frida-agent文件大小: %.2f MB\n"), float64(stat.Size())/(1024*1024))
		}
	}

	// 验证端口范围
	if *port < 1 || *port > 65535 {
		fmt.Fprintf(os.Stderr, i18n.T("错误: 端口必须在1-65535范围内\n"))
		os.Exit(1)
	}

	// 自动生成包名（如果未指定）
	if *packageName == "" {
		// 将"frida"替换为魔改名称
		*packageName = fmt.Sprintf("re.%s.server", *magicName)
	}

	// 自动生成描述（如果未指定）
	if *description == "" {
		*description = fmt.Sprintf("Dynamic instrumentation toolkit for developers, security researchers, and reverse engineers (Modified: %s)", *magicName)
	}

	// 显示配置信息
	fmt.Printf(i18n.T("=== Fridare DEB包创建工具 ===\n"))
	fmt.Printf(i18n.T("输入文件:\n"))
	fmt.Printf("  frida-server: %s\n", *fridaServerPath)
	if *fridaAgentPath != "" {
		fmt.Printf("  frida-agent:  %s\n", *fridaAgentPath)
	} else {
		fmt.Printf(i18n.T("  frida-agent:  (未指定)\n"))
	}
	fmt.Printf(i18n.T("输出文件: %s\n"), *outputPath)
	fmt.Printf(i18n.T("包配置:\n"))
	fmt.Printf(i18n.T("  包名:     %s\n"), *packageName)
	fmt.Printf(i18n.T("  版本:     %s\n"), *version)
	fmt.Printf(i18n.T("  架构:     %s\n"), *architecture)
	fmt.Printf(i18n.T("  魔改名:   %s\n"), *magicName)
	fmt.Printf(i18n.T("  端口:     %d\n"), *port)
	fmt.Printf(i18n.T("  结构:     %s\n"), map[bool]string{true: "Rootless", false: "Root"}[*isRootless])
	fmt.Printf(i18n.T("  维护者:   %s\n"), *maintainer)
	fmt.Printf(i18n.T("  描述:     %s\n"), *description)
	fmt.Printf("=============================\n\n")

	// 创建包信息
	packageInfo := &core.PackageInfo{
		Name:         *packageName,
		Version:      *version,
		Architecture: *architecture,
		Maintainer:   *maintainer,
		Description:  *description,
		Depends:      *depends,
		Section:      *section,
		Priority:     *priority,
		Homepage:     *homepage,
		Port:         *port,
		MagicName:    *magicName,
		IsRootless:   *isRootless,
	}

	// 创建DEB构建器
	creator := core.NewCreateFridaDeb(*fridaServerPath, *outputPath, packageInfo)
	if *fridaAgentPath != "" {
		creator.FridaAgentPath = *fridaAgentPath
	}

	// 执行构建
	err := creator.CreateDebPackage()
	if err != nil {
		fmt.Fprintf(os.Stderr, i18n.T("错误: DEB包创建失败: %v\n"), err)
		os.Exit(1)
	}

	// 显示成功信息
	fmt.Printf(i18n.T("\n✅ DEB包创建成功!\n"))
	fmt.Printf(i18n.T("输出文件: %s\n"), *outputPath)

	// 显示文件大小
	if stat, err := os.Stat(*outputPath); err == nil {
		fmt.Printf(i18n.T("文件大小: %.2f MB\n"), float64(stat.Size())/(1024*1024))
	}

	fmt.Printf(i18n.T("\n📦 安装命令:\n"))
	fmt.Printf("  dpkg -i %s\n", filepath.Base(*outputPath))
	fmt.Printf(i18n.T("\n🔧 服务控制:\n"))
	if *isRootless {
		fmt.Printf(i18n.T("  启动: launchctl load /var/re/Library/LaunchDaemons/re.%s.server.plist\n"), *magicName)
		fmt.Printf(i18n.T("  停止: launchctl unload /var/re/Library/LaunchDaemons/re.%s.server.plist\n"), *magicName)
	} else {
		fmt.Printf(i18n.T("  启动: launchctl load /Library/LaunchDaemons/re.%s.server.plist\n"), *magicName)
		fmt.Printf(i18n.T("  停止: launchctl unload /Library/LaunchDaemons/re.%s.server.plist\n"), *magicName)
	}
	fmt.Printf(i18n.T("\n🌐 连接信息:\n"))
	fmt.Printf(i18n.T("  端口: %d\n"), *port)
	fmt.Printf(i18n.T("  frida命令: frida -H <设备IP>:%d <进程名>\n"), *port)
}

// isValidMagicName 验证魔改名称格式
func isValidMagicName(s string) bool {
	// 必须以字母开头
	if len(s) == 0 {
		return false
	}

	first := s[0]
	if !((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')) {
		return false
	}

	// 检查其余字符必须是字母或数字
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}

	return true
}
