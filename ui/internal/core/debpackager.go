package core

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"fridare-gui/internal/i18n"

	"github.com/ulikunitz/xz"
)

// DebPackager DEB包构建器
type DebPackager struct {
	TempDir string
}

// PackageInfo 包信息
type PackageInfo struct {
	Name         string
	Version      string
	Architecture string
	Maintainer   string
	Description  string
	Depends      string
	Section      string
	Priority     string
	Homepage     string
	Port         int
	MagicName    string
	IsRootless   bool // 是否为rootless结构
}

// PathMapper 路径映射器，用于处理不同架构的路径转换
type PathMapper struct {
	isRootless    bool
	originalPaths map[string]string // 原始路径 -> 新路径映射
}

// NewPathMapper 创建路径映射器
func NewPathMapper(extractDir string) *PathMapper {
	pm := &PathMapper{
		originalPaths: make(map[string]string),
	}

	// 检测是否为rootless结构
	rootlessPath := filepath.Join(extractDir, "var", "jb")
	if _, err := os.Stat(rootlessPath); err == nil {
		pm.isRootless = true
		log.Printf(i18n.T("DEBUG: 检测到rootless越狱结构，将使用 /var/re 路径"))
	} else {
		pm.isRootless = false
		log.Printf(i18n.T("DEBUG: 检测到传统越狱结构"))
	}

	return pm
}

// MapPath 映射路径，将敏感路径转换为安全路径
func (pm *PathMapper) MapPath(originalPath string) string {
	if !pm.isRootless {
		return originalPath
	}

	// 将 /var/jb 替换为 /var/re
	mapped := strings.ReplaceAll(originalPath, "var/jb", "var/re")
	mapped = strings.ReplaceAll(mapped, "/var/jb", "/var/re")

	// 记录映射关系
	if mapped != originalPath {
		pm.originalPaths[originalPath] = mapped
		log.Printf(i18n.T("DEBUG: 路径映射: %s -> %s"), originalPath, mapped)
	}

	return mapped
}

// DebModifier DEB包修改器结构
type DebModifier struct {
	InputPath  string
	OutputPath string
	MagicName  string
	Port       int
	TempDir    string
	ExtractDir string
	PathMapper *PathMapper // 路径映射器
}

// NewDebPackager 创建新的DEB包构建器
func NewDebPackager() *DebPackager {
	return &DebPackager{}
}

// CreateDebPackage 创建DEB包
func (dp *DebPackager) CreateDebPackage(fridaFile, outputPath string, info *PackageInfo, progressCallback func(float64, string)) error {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "fridare_deb_*")
	if err != nil {
		return fmt.Errorf(i18n.T("创建临时目录失败: %v"), err)
	}
	dp.TempDir = tempDir
	defer os.RemoveAll(tempDir)

	progressCallback(0.1, i18n.T("创建临时目录..."))

	// 检测Frida文件架构
	arch, err := dp.detectArchitecture(fridaFile)
	if err != nil {
		return fmt.Errorf(i18n.T("检测文件架构失败: %v"), err)
	}
	info.Architecture = arch

	progressCallback(0.2, i18n.T("检测文件架构: ")+arch)

	// 创建包目录结构
	err = dp.createPackageStructure(tempDir, arch, info)
	if err != nil {
		return fmt.Errorf(i18n.T("创建包结构失败: %v"), err)
	}

	progressCallback(0.3, i18n.T("创建包目录结构..."))

	// 复制Frida文件到正确位置
	err = dp.copyFridaFile(fridaFile, tempDir, arch, info)
	if err != nil {
		return fmt.Errorf(i18n.T("复制Frida文件失败: %v"), err)
	}

	progressCallback(0.5, i18n.T("复制Frida文件..."))

	// 创建控制文件
	err = dp.createControlFile(tempDir, info)
	if err != nil {
		return fmt.Errorf(i18n.T("创建控制文件失败: %v"), err)
	}

	progressCallback(0.6, i18n.T("创建控制文件..."))

	// 创建启动脚本
	err = dp.createLaunchDaemon(tempDir, arch, info)
	if err != nil {
		return fmt.Errorf(i18n.T("创建启动脚本失败: %v"), err)
	}

	progressCallback(0.7, i18n.T("创建启动脚本..."))

	// 创建安装后脚本
	err = dp.createPostInstScript(tempDir, info)
	if err != nil {
		return fmt.Errorf(i18n.T("创建安装脚本失败: %v"), err)
	}

	progressCallback(0.8, i18n.T("创建安装脚本..."))

	// 构建DEB包
	err = dp.buildDebPackage(tempDir, outputPath, info)
	if err != nil {
		return fmt.Errorf(i18n.T("构建DEB包失败: %v"), err)
	}

	progressCallback(1.0, i18n.T("DEB包构建完成"))

	return nil
}

// detectArchitecture 检测文件架构
func (dp *DebPackager) detectArchitecture(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 读取文件头部信息
	header := make([]byte, 16)
	_, err = file.Read(header)
	if err != nil {
		return "", err
	}

	// 检测Mach-O格式 (iOS)
	if len(header) >= 4 {
		magic := string(header[:4])
		switch magic {
		case "\xca\xfe\xba\xbe": // FAT binary
			return "iphoneos-arm", nil
		case "\xcf\xfa\xed\xfe": // 64-bit little endian
			return "iphoneos-arm64", nil
		case "\xce\xfa\xed\xfe": // 32-bit little endian
			return "iphoneos-arm", nil
		case "\xfe\xed\xfa\xce": // 32-bit big endian
			return "iphoneos-arm", nil
		case "\xfe\xed\xfa\xcf": // 64-bit big endian
			return "iphoneos-arm64", nil
		}
	}

	// 默认返回arm64
	return "iphoneos-arm64", nil
}

// createPackageStructure 创建包目录结构
func (dp *DebPackager) createPackageStructure(tempDir, arch string, info *PackageInfo) error {
	dirs := []string{
		"DEBIAN",
		"usr/bin",
		"Library/LaunchDaemons",
	}

	// 根据架构创建不同的目录结构
	if strings.Contains(arch, "arm64") {
		dirs = append(dirs, "var/re/usr/bin", "var/re/Library/LaunchDaemons")
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(tempDir, dir)
		err := os.MkdirAll(fullPath, 0755)
		if err != nil {
			return err
		}
	}

	return nil
}

// copyFridaFile 复制Frida文件到正确位置
func (dp *DebPackager) copyFridaFile(srcFile, tempDir, arch string, info *PackageInfo) error {
	var destPaths []string

	// 根据架构确定目标路径
	if strings.Contains(arch, "arm64") {
		destPaths = []string{
			filepath.Join(tempDir, "var/re/usr/bin", info.MagicName),
			filepath.Join(tempDir, "usr/bin", info.MagicName),
		}
	} else {
		destPaths = []string{
			filepath.Join(tempDir, "usr/bin", info.MagicName),
		}
	}

	// 复制文件到所有目标路径
	for _, destPath := range destPaths {
		err := dp.copyFile(srcFile, destPath)
		if err != nil {
			return err
		}

		// 设置可执行权限
		err = os.Chmod(destPath, 0755)
		if err != nil {
			return err
		}
	}

	return nil
}

// copyFile 复制文件
func (dp *DebPackager) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 确保目录存在
	err = os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// createControlFile 创建控制文件
func (dp *DebPackager) createControlFile(tempDir string, info *PackageInfo) error {
	controlPath := filepath.Join(tempDir, "DEBIAN", "control")

	// 计算安装大小
	installedSize, err := dp.calculateInstalledSize(tempDir)
	if err != nil {
		installedSize = 100 // 默认值
	}

	content := fmt.Sprintf(`Package: %s
Version: %s
Architecture: %s
Maintainer: %s
Installed-Size: %d
Depends: %s
Section: %s
Priority: %s
Homepage: %s
Description: %s
 Magic name: %s
 Default port: %d
`,
		info.Name,
		info.Version,
		info.Architecture,
		info.Maintainer,
		installedSize,
		info.Depends,
		info.Section,
		info.Priority,
		info.Homepage,
		info.Description,
		info.MagicName,
		info.Port,
	)

	return os.WriteFile(controlPath, []byte(content), 0644)
}

// createLaunchDaemon 创建启动守护程序配置
func (dp *DebPackager) createLaunchDaemon(tempDir, arch string, info *PackageInfo) error {
	var plistPaths []string

	// 根据架构确定plist文件路径
	if strings.Contains(arch, "arm64") {
		plistPaths = []string{
			filepath.Join(tempDir, "var/re/Library/LaunchDaemons", fmt.Sprintf("re.%s.server.plist", info.MagicName)),
			filepath.Join(tempDir, "Library/LaunchDaemons", fmt.Sprintf("re.%s.server.plist", info.MagicName)),
		}
	} else {
		plistPaths = []string{
			filepath.Join(tempDir, "Library/LaunchDaemons", fmt.Sprintf("re.%s.server.plist", info.MagicName)),
		}
	}

	// plist内容
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>re.%s.server</string>
	<key>ProgramArguments</key>
	<array>
		<string>/usr/bin/%s</string>
		<string>-l</string>
		<string>0.0.0.0:%d</string>
	</array>
	<key>UserName</key>
	<string>root</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
`, info.MagicName, info.MagicName, info.Port)

	// 写入所有plist文件
	for _, plistPath := range plistPaths {
		err := os.WriteFile(plistPath, []byte(plistContent), 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

// createPostInstScript 创建安装后脚本 (修复版本)
func (dp *DebPackager) createPostInstScript(tempDir string, info *PackageInfo) error {
	postinstPath := filepath.Join(tempDir, "DEBIAN", "postinst")

	var content string
	if info.IsRootless {
		// Rootless结构
		content = fmt.Sprintf(`#!/bin/bash
set -e

# 设置可执行权限 (Rootless)
if [ -f /var/re/usr/sbin/%s ]; then
    chmod 755 /var/re/usr/sbin/%s
    chown root:wheel /var/re/usr/sbin/%s
fi

# 设置dylib权限
if [ -d /var/re/usr/lib/%s ]; then
    chmod 755 /var/re/usr/lib/%s/*
    chown root:wheel /var/re/usr/lib/%s/*
fi

# 加载守护程序
if [ -f /var/re/Library/LaunchDaemons/re.%s.server.plist ]; then
    launchctl load /var/re/Library/LaunchDaemons/re.%s.server.plist
fi

echo "Fridare %s 安装完成 (Rootless)"
echo "服务已启动在端口 %d"
`,
			info.MagicName, info.MagicName, info.MagicName,
			info.MagicName, info.MagicName, info.MagicName,
			info.MagicName, info.MagicName,
			info.MagicName, info.Port,
		)
	} else {
		// Root结构
		content = fmt.Sprintf(`#!/bin/bash
set -e

# 设置可执行权限 (Root)
if [ -f /usr/sbin/%s ]; then
    chmod 755 /usr/sbin/%s
    chown root:wheel /usr/sbin/%s
fi

# 设置dylib权限
if [ -d /usr/lib/%s ]; then
    chmod 755 /usr/lib/%s/*
    chown root:wheel /usr/lib/%s/*
fi

# 加载守护程序
if [ -f /Library/LaunchDaemons/re.%s.server.plist ]; then
    launchctl load /Library/LaunchDaemons/re.%s.server.plist
fi

echo "Fridare %s 安装完成 (Root)"
echo "服务已启动在端口 %d"
`,
			info.MagicName, info.MagicName, info.MagicName,
			info.MagicName, info.MagicName, info.MagicName,
			info.MagicName, info.MagicName,
			info.MagicName, info.Port,
		)
	}

	err := os.WriteFile(postinstPath, []byte(content), 0755)
	if err != nil {
		return err
	}

	// 创建卸载前脚本
	prermPath := filepath.Join(tempDir, "DEBIAN", "prerm")
	var prermContent string

	if info.IsRootless {
		prermContent = fmt.Sprintf(`#!/bin/bash
set -e

# 停止守护程序 (Rootless)
if [ -f /var/re/Library/LaunchDaemons/re.%s.server.plist ]; then
    launchctl unload /var/re/Library/LaunchDaemons/re.%s.server.plist 2>/dev/null || true
fi

echo "Fridare %s 服务已停止 (Rootless)"
`,
			info.MagicName, info.MagicName, info.MagicName,
		)
	} else {
		prermContent = fmt.Sprintf(`#!/bin/bash
set -e

# 停止守护程序 (Root)
if [ -f /Library/LaunchDaemons/re.%s.server.plist ]; then
    launchctl unload /Library/LaunchDaemons/re.%s.server.plist 2>/dev/null || true
fi

echo "Fridare %s 服务已停止 (Root)"
`,
			info.MagicName, info.MagicName, info.MagicName,
		)
	}

	return os.WriteFile(prermPath, []byte(prermContent), 0755)
}

// calculateInstalledSize 计算安装大小（KB）
func (dp *DebPackager) calculateInstalledSize(dir string) (int, error) {
	var totalSize int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.Contains(path, "DEBIAN") {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	// 转换为KB并向上取整
	sizeKB := int((totalSize + 1023) / 1024)
	return sizeKB, nil
}

// buildDebPackage 构建DEB包
func (dp *DebPackager) buildDebPackage(tempDir, outputPath string, info *PackageInfo) error {
	// 检查是否有dpkg-deb命令
	if _, err := exec.LookPath("dpkg-deb"); err == nil {
		return dp.buildWithDpkgDeb(tempDir, outputPath)
	}

	// 如果没有dpkg-deb，使用内置方法
	return dp.buildWithTar(tempDir, outputPath, info)
}

// buildWithDpkgDeb 使用dpkg-deb构建
func (dp *DebPackager) buildWithDpkgDeb(tempDir, outputPath string) error {
	cmd := exec.Command("dpkg-deb", "--build", tempDir, outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(i18n.T("dpkg-deb 执行失败: %v, 输出: %s"), err, string(output))
	}
	return nil
}

// buildWithTar 使用tar构建（内置方法）
func (dp *DebPackager) buildWithTar(tempDir, outputPath string, info *PackageInfo) error {
	// 创建data.tar.gz
	dataPath := filepath.Join(tempDir, "data.tar.gz")
	err := dp.createDataTar(tempDir, dataPath)
	if err != nil {
		return err
	}

	// 创建control.tar.gz
	controlPath := filepath.Join(tempDir, "control.tar.gz")
	err = dp.createControlTar(tempDir, controlPath)
	if err != nil {
		return err
	}

	// 创建debian-binary文件
	debianBinaryPath := filepath.Join(tempDir, "debian-binary")
	err = os.WriteFile(debianBinaryPath, []byte("2.0\n"), 0644)
	if err != nil {
		return err
	}

	// 创建最终的.deb文件
	return dp.createDebFile(tempDir, outputPath)
}

// createDataTar 创建data.tar.gz
func (dp *DebPackager) createDataTar(tempDir, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// 遍历并添加非DEBIAN目录的所有文件
	return filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(tempDir, path)
		if err != nil {
			return err
		}

		// 跳过DEBIAN目录和临时文件
		if strings.HasPrefix(relPath, "DEBIAN") || strings.Contains(relPath, ".tar.gz") || relPath == "debian-binary" {
			return nil
		}

		if relPath == "." {
			return nil
		}

		// 创建tar头部
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		header.Name = "./" + relPath

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return err
		}

		// 如果是文件，写入内容
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// createControlTar 创建control.tar.gz
func (dp *DebPackager) createControlTar(tempDir, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	debianDir := filepath.Join(tempDir, "DEBIAN")

	// 遍历DEBIAN目录
	return filepath.Walk(debianDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(debianDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		// 创建tar头部
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		header.Name = "./" + relPath

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return err
		}

		// 如果是文件，写入内容
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// createDebFile 创建最终的.deb文件
func (dp *DebPackager) createDebFile(tempDir, outputPath string) error {
	debFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer debFile.Close()

	// 写入debian-binary
	debianBinaryPath := filepath.Join(tempDir, "debian-binary")
	err = dp.appendFile(debFile, debianBinaryPath)
	if err != nil {
		return err
	}

	// 写入control.tar.gz
	controlPath := filepath.Join(tempDir, "control.tar.gz")
	err = dp.appendFile(debFile, controlPath)
	if err != nil {
		return err
	}

	// 写入data.tar.gz
	dataPath := filepath.Join(tempDir, "data.tar.gz")
	return dp.appendFile(debFile, dataPath)
}

// appendFile 将文件内容追加到另一个文件
func (dp *DebPackager) appendFile(dst *os.File, srcPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	_, err = io.Copy(dst, src)
	return err
}

// ValidatePackageInfo 验证包信息
func (dp *DebPackager) ValidatePackageInfo(info *PackageInfo) error {
	if info.Name == "" {
		return fmt.Errorf(i18n.T("包名不能为空"))
	}
	if info.Version == "" {
		return fmt.Errorf(i18n.T("版本不能为空"))
	}
	if info.Maintainer == "" {
		return fmt.Errorf(i18n.T("维护者不能为空"))
	}
	if info.MagicName == "" {
		return fmt.Errorf(i18n.T("魔改名称不能为空"))
	}
	if len(info.MagicName) != 5 {
		return fmt.Errorf(i18n.T("魔改名称必须为5个字符"))
	}
	if info.Port <= 0 || info.Port > 65535 {
		return fmt.Errorf(i18n.T("端口号必须在1-65535之间"))
	}
	return nil
}

// GetDefaultPackageInfo 获取默认包信息
func (dp *DebPackager) GetDefaultPackageInfo() *PackageInfo {
	return &PackageInfo{
		Name:         "com.fridare.server",
		Version:      "1.0.0",
		Architecture: "iphoneos-arm64",
		Maintainer:   "Fridare Team <suifei@gmail.com>",
		Description:  "Modified Frida Server for iOS",
		Depends:      "",
		Section:      "Development",
		Priority:     "optional",
		Homepage:     "https://github.com/suifei/fridare",
		Port:         27042,
		MagicName:    "frida",
	}
}

// NewDebModifier 创建新的DEB包修改器
func NewDebModifier(inputPath, outputPath, magicName string, port int) *DebModifier {
	return &DebModifier{
		InputPath:  inputPath,
		OutputPath: outputPath,
		MagicName:  magicName,
		Port:       port,
	}
}

// ModifyDebPackage 修改现有DEB包 - 主要功能函数
func (dm *DebModifier) ModifyDebPackage(progressCallback func(float64, string)) error {
	log.Printf(i18n.T("INFO: 开始修改DEB包 - 输入: %s, 输出: %s, 魔改名: %s, 端口: %d"),
		dm.InputPath, dm.OutputPath, dm.MagicName, dm.Port)

	// 获取输入文件大小
	if stat, err := os.Stat(dm.InputPath); err == nil {
		log.Printf(i18n.T("INFO: 输入DEB文件大小: %d 字节 (%.2f KB)"), stat.Size(), float64(stat.Size())/1024)
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "fridare_modify_*")
	if err != nil {
		log.Printf(i18n.T("ERROR: 创建临时目录失败: %v"), err)
		return fmt.Errorf(i18n.T("创建临时目录失败: %v"), err)
	}
	dm.TempDir = tempDir
	log.Printf(i18n.T("DEBUG: 创建临时目录: %s"), tempDir)
	defer func() {
		log.Printf(i18n.T("DEBUG: 清理临时目录: %s"), tempDir)
		os.RemoveAll(tempDir)
	}()

	dm.ExtractDir = filepath.Join(tempDir, "extracted")
	err = os.MkdirAll(dm.ExtractDir, 0755)
	if err != nil {
		log.Printf(i18n.T("ERROR: 创建解压目录失败: %v"), err)
		return fmt.Errorf(i18n.T("创建解压目录失败: %v"), err)
	}
	log.Printf(i18n.T("DEBUG: 创建解压目录: %s"), dm.ExtractDir)

	progressCallback(0.1, i18n.T("解压DEB包..."))

	// 1. 解压现有DEB包
	log.Printf(i18n.T("INFO: 步骤1 - 解压DEB包"))
	err = dm.extractDebPackage()
	if err != nil {
		log.Printf(i18n.T("ERROR: 解压DEB包失败: %v"), err)
		return fmt.Errorf(i18n.T("解压DEB包失败: %v"), err)
	}

	// 初始化路径映射器
	dm.PathMapper = NewPathMapper(dm.ExtractDir)

	progressCallback(0.3, i18n.T("读取包信息..."))

	// 2. 读取包信息
	log.Printf(i18n.T("INFO: 步骤2 - 读取包信息"))
	packageInfo, err := dm.readPackageInfo()
	if err != nil {
		log.Printf(i18n.T("ERROR: 读取包信息失败: %v"), err)
		return fmt.Errorf(i18n.T("读取包信息失败: %v"), err)
	}
	log.Printf(i18n.T("DEBUG: 包信息 - 名称: %s, 版本: %s, 架构: %s"), packageInfo.Name, packageInfo.Version, packageInfo.Architecture)

	progressCallback(0.4, i18n.T("修改包元数据..."))

	// 3. 修改包元数据
	log.Printf(i18n.T("INFO: 步骤3 - 修改包元数据"))
	err = dm.modifyPackageMetadata(packageInfo)
	if err != nil {
		log.Printf(i18n.T("ERROR: 修改包元数据失败: %v"), err)
		return fmt.Errorf(i18n.T("修改包元数据失败: %v"), err)
	}

	progressCallback(0.5, i18n.T("修改二进制文件..."))

	// 4. 修改二进制文件名
	log.Printf(i18n.T("INFO: 步骤4 - 修改二进制文件"))
	err = dm.modifyBinaryFiles()
	if err != nil {
		log.Printf(i18n.T("ERROR: 修改二进制文件失败: %v"), err)
		return fmt.Errorf(i18n.T("修改二进制文件失败: %v"), err)
	}

	progressCallback(0.7, i18n.T("修改启动守护进程..."))

	// 5. 修改启动守护进程配置
	log.Printf(i18n.T("INFO: 步骤5 - 修改启动守护进程配置"))
	err = dm.modifyLaunchDaemon()
	if err != nil {
		log.Printf(i18n.T("ERROR: 修改启动守护进程失败: %v"), err)
		return fmt.Errorf(i18n.T("修改启动守护进程失败: %v"), err)
	}

	progressCallback(0.8, i18n.T("修改DEBIAN脚本..."))

	// 6. 修改DEBIAN目录中的脚本
	log.Printf(i18n.T("INFO: 步骤6 - 修改DEBIAN脚本"))
	err = dm.modifyDebianScripts()
	if err != nil {
		log.Printf(i18n.T("ERROR: 修改DEBIAN脚本失败: %v"), err)
		return fmt.Errorf(i18n.T("修改DEBIAN脚本失败: %v"), err)
	}

	progressCallback(0.9, i18n.T("重新打包DEB..."))

	// 7. 重新打包
	log.Printf(i18n.T("INFO: 步骤7 - 重新打包DEB文件"))
	err = dm.repackageDebFile()
	if err != nil {
		log.Printf(i18n.T("ERROR: 重新打包失败: %v"), err)
		return fmt.Errorf(i18n.T("重新打包失败: %v"), err)
	}

	// 获取输出文件大小
	if stat, err := os.Stat(dm.OutputPath); err == nil {
		log.Printf(i18n.T("INFO: 输出DEB文件大小: %d 字节 (%.2f KB)"), stat.Size(), float64(stat.Size())/1024)
	}

	progressCallback(1.0, i18n.T("DEB包修改完成!"))
	log.Printf(i18n.T("SUCCESS: DEB包修改完成: %s"), dm.OutputPath)
	return nil
}

// extractDebPackage 解压DEB包（纯Go实现）
func (dm *DebModifier) extractDebPackage() error {
	// DEB包是AR格式的档案，包含debian-binary、control.tar.xz、data.tar.xz
	return dm.extractDebWithGoAr()
}

// extractDebWithGoAr 使用纯Go方式解析AR格式解压DEB文件
func (dm *DebModifier) extractDebWithGoAr() error {
	log.Printf(i18n.T("INFO: 开始解压DEB文件: %s -> %s"), dm.InputPath, dm.ExtractDir)

	file, err := os.Open(dm.InputPath)
	if err != nil {
		log.Printf(i18n.T("ERROR: 打开DEB文件失败: %v"), err)
		return fmt.Errorf(i18n.T("打开DEB文件失败: %v"), err)
	}
	defer file.Close()

	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		log.Printf(i18n.T("WARNING: 获取DEB文件大小失败: %v"), err)
	} else {
		log.Printf(i18n.T("INFO: DEB文件大小: %d 字节"), stat.Size())
	}

	// 读取AR文件头部
	header := make([]byte, 8)
	_, err = file.Read(header)
	if err != nil {
		log.Printf(i18n.T("ERROR: 读取AR头部失败: %v"), err)
		return fmt.Errorf(i18n.T("读取AR头部失败: %v"), err)
	}

	if string(header) != "!<arch>\n" {
		log.Printf(i18n.T("ERROR: 不是有效的AR文件，头部: %q"), string(header))
		return fmt.Errorf(i18n.T("不是有效的AR文件"))
	}
	log.Printf(i18n.T("DEBUG: AR文件头部验证通过"))

	var controlData, dataArchive []byte
	entryCount := 0

	// 解析AR文件条目
	for {
		entry, err := dm.readArEntry(file)
		if err == io.EOF {
			log.Printf(i18n.T("DEBUG: AR文件解析完成，共处理 %d 个条目"), entryCount)
			break
		}
		if err != nil {
			log.Printf(i18n.T("ERROR: 读取AR条目失败: %v"), err)
			return fmt.Errorf(i18n.T("读取AR条目失败: %v"), err)
		}

		entryCount++
		log.Printf(i18n.T("DEBUG: 处理AR条目 %d: 名称=%s, 大小=%d 字节"), entryCount, entry.Name, entry.Size)

		// 读取文件内容
		content := make([]byte, entry.Size)
		_, err = io.ReadFull(file, content)
		if err != nil {
			log.Printf(i18n.T("ERROR: 读取AR文件内容失败: %s, 错误: %v"), entry.Name, err)
			return fmt.Errorf(i18n.T("读取AR文件内容失败: %v"), err)
		}

		switch entry.Name {
		case "control.tar.xz", "control.tar.gz":
			controlData = content
			log.Printf(i18n.T("INFO: 保存control档案: %s, 大小: %d 字节"), entry.Name, len(content))
		case "data.tar.xz", "data.tar.gz":
			dataArchive = content
			log.Printf(i18n.T("INFO: 保存data档案: %s, 大小: %d 字节"), entry.Name, len(content))
		case "debian-binary":
			log.Printf(i18n.T("INFO: debian-binary内容: %q"), string(content))
			continue
		default:
			log.Printf(i18n.T("INFO: 跳过未知条目: %s"), entry.Name)
		}

		// AR文件要求偶数字节对齐
		if entry.Size%2 == 1 {
			file.Seek(1, 1)
			log.Printf(i18n.T("DEBUG: 跳过对齐填充字节: %s"), entry.Name)
		}
	}

	// 解压control.tar.xz到DEBIAN目录
	controlDir := filepath.Join(dm.ExtractDir, "DEBIAN")
	err = os.MkdirAll(controlDir, 0755)
	if err != nil {
		log.Printf(i18n.T("ERROR: 创建DEBIAN目录失败: %v"), err)
		return err
	}

	if len(controlData) > 0 {
		log.Printf(i18n.T("INFO: 开始解压control档案到: %s"), controlDir)
		err = dm.extractTarArchive(controlData, controlDir)
		if err != nil {
			log.Printf(i18n.T("ERROR: 解压control.tar失败: %v"), err)
			return fmt.Errorf(i18n.T("解压control.tar失败: %v"), err)
		}
	} else {
		log.Printf(i18n.T("WARNING: 未找到control档案数据"))
	}

	// 解压data.tar.xz到根目录
	if len(dataArchive) > 0 {
		log.Printf(i18n.T("INFO: 开始解压data档案到: %s"), dm.ExtractDir)
		err = dm.extractTarArchive(dataArchive, dm.ExtractDir)
		if err != nil {
			log.Printf(i18n.T("ERROR: 解压data.tar失败: %v"), err)
			return fmt.Errorf(i18n.T("解压data.tar失败: %v"), err)
		}
	} else {
		log.Printf(i18n.T("WARNING: 未找到data档案数据"))
	}

	log.Printf(i18n.T("SUCCESS: DEB文件解压完成: %s"), dm.ExtractDir)
	return nil
}

// arEntry AR文件条目
type arEntry struct {
	Name string
	Size int64
}

// readArEntry 读取AR文件条目头部
func (dm *DebModifier) readArEntry(file *os.File) (*arEntry, error) {
	header := make([]byte, 60)
	n, err := file.Read(header)
	if err != nil {
		return nil, err
	}
	if n != 60 {
		return nil, io.EOF
	}

	// 解析文件名（16字节）
	nameBytes := header[0:16]
	name := strings.TrimSpace(string(nameBytes))

	// 解析文件大小（10字节，位置48-58）
	sizeBytes := header[48:58]
	sizeStr := strings.TrimSpace(string(sizeBytes))
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("解析文件大小失败: %v"), err)
	}

	return &arEntry{
		Name: name,
		Size: size,
	}, nil
}

// extractTarArchive 解压tar档案（支持gzip和xz压缩）
func (dm *DebModifier) extractTarArchive(data []byte, targetDir string) error {
	log.Printf(i18n.T("DEBUG: 开始解压tar档案，数据大小: %d 字节，目标目录: %s"), len(data), targetDir)

	var reader io.Reader = bytes.NewReader(data)
	var compressionType string = i18n.T("未压缩")

	// 检查压缩格式
	if len(data) >= 2 {
		// 检查是否是gzip格式 (0x1f, 0x8b)
		if data[0] == 0x1f && data[1] == 0x8b {
			compressionType = "gzip"
			log.Printf(i18n.T("DEBUG: 检测到gzip压缩格式"))
			gzReader, err := gzip.NewReader(reader)
			if err != nil {
				log.Printf(i18n.T("ERROR: 创建gzip读取器失败: %v"), err)
				return fmt.Errorf(i18n.T("创建gzip读取器失败: %v"), err)
			}
			defer gzReader.Close()
			reader = gzReader
		} else if len(data) >= 6 {
			// 检查是否是xz格式 (0xFD, '7', 'z', 'X', 'Z', 0x00)
			xzHeader := []byte{0xFD, '7', 'z', 'X', 'Z', 0x00}
			if bytes.Equal(data[:6], xzHeader) {
				compressionType = "xz"
				log.Printf(i18n.T("DEBUG: 检测到xz压缩格式"))
				xzReader, err := xz.NewReader(reader)
				if err != nil {
					log.Printf(i18n.T("ERROR: 创建xz读取器失败: %v"), err)
					return fmt.Errorf(i18n.T("创建xz读取器失败: %v"), err)
				}
				reader = xzReader
			} else {
				log.Printf(i18n.T("DEBUG: 检测到未知压缩格式，头部字节: %x"), data[:6])
			}
		}
	}

	log.Printf(i18n.T("INFO: tar档案压缩类型: %s"), compressionType)

	// 解压tar档案
	tarReader := tar.NewReader(reader)

	var fileCount, dirCount int
	var totalSize int64

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf(i18n.T("ERROR: 读取tar头失败: %v"), err)
			return fmt.Errorf(i18n.T("读取tar头失败: %v"), err)
		}

		// 构造目标路径，应用路径映射
		originalName := header.Name
		mappedName := originalName

		// 如果是rootless结构，将 var/jb 替换为 var/re
		if strings.Contains(originalName, "var/jb") {
			mappedName = strings.ReplaceAll(originalName, "var/jb", "var/re")
			log.Printf(i18n.T("DEBUG: 路径映射: %s -> %s"), originalName, mappedName)
		}

		targetPath := filepath.Join(targetDir, mappedName)
		log.Printf(i18n.T("DEBUG: 处理tar条目: %s -> %s, 类型: %d, 大小: %d, 权限: %o"),
			mappedName, targetPath, header.Typeflag, header.Size, header.Mode)

		// 确保目标目录存在
		if header.Typeflag == tar.TypeDir {
			dirCount++
			err = os.MkdirAll(targetPath, os.FileMode(header.Mode))
			if err != nil {
				log.Printf(i18n.T("ERROR: 创建目录失败: %s, 错误: %v"), targetPath, err)
				return fmt.Errorf(i18n.T("创建目录失败: %v"), err)
			}
			log.Printf(i18n.T("DEBUG: 目录创建成功: %s"), targetPath)
			continue
		}

		// 确保父目录存在
		parentDir := filepath.Dir(targetPath)
		err = os.MkdirAll(parentDir, 0755)
		if err != nil {
			log.Printf(i18n.T("ERROR: 创建父目录失败: %s, 错误: %v"), parentDir, err)
			return fmt.Errorf(i18n.T("创建父目录失败: %v"), err)
		}

		// 创建文件
		switch header.Typeflag {
		case tar.TypeReg:
			fileCount++
			totalSize += header.Size

			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				log.Printf(i18n.T("ERROR: 创建文件失败: %s, 错误: %v"), targetPath, err)
				return fmt.Errorf(i18n.T("创建文件失败: %v"), err)
			}

			written, err := io.Copy(file, tarReader)
			file.Close()
			if err != nil {
				log.Printf(i18n.T("ERROR: 写入文件内容失败: %s, 错误: %v"), targetPath, err)
				return fmt.Errorf(i18n.T("写入文件内容失败: %v"), err)
			}

			if written != header.Size {
				log.Printf(i18n.T("WARNING: 文件大小不匹配: %s, 期望: %d, 实际: %d"), targetPath, header.Size, written)
			}

			log.Printf(i18n.T("DEBUG: 文件创建成功: %s, 写入: %d 字节"), targetPath, written)

		case tar.TypeSymlink:
			log.Printf(i18n.T("DEBUG: 创建符号链接: %s -> %s"), targetPath, header.Linkname)
			err = os.Symlink(header.Linkname, targetPath)
			if err != nil {
				log.Printf(i18n.T("WARNING: 创建符号链接失败: %s -> %s, 错误: %v"), targetPath, header.Linkname, err)
				// Windows下可能不支持符号链接，忽略错误
				continue
			}
		default:
			log.Printf(i18n.T("WARNING: 跳过不支持的tar条目类型: %s, 类型: %d"), header.Name, header.Typeflag)
		}
	}

	log.Printf(i18n.T("INFO: tar档案解压完成 - 目录数: %d, 文件数: %d, 总大小: %d 字节"), dirCount, fileCount, totalSize)
	return nil
}

// readPackageInfo 读取包信息
func (dm *DebModifier) readPackageInfo() (*PackageInfo, error) {
	controlFile := filepath.Join(dm.ExtractDir, "DEBIAN", "control")

	file, err := os.Open(controlFile)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("打开control文件失败: %v"), err)
	}
	defer file.Close()

	info := &PackageInfo{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "Package":
					info.Name = value
				case "Version":
					info.Version = value
				case "Architecture":
					info.Architecture = value
				case "Maintainer":
					info.Maintainer = value
				case "Description":
					info.Description = value
				case "Depends":
					info.Depends = value
				case "Section":
					info.Section = value
				case "Priority":
					info.Priority = value
				case "Homepage":
					info.Homepage = value
				}
			}
		}
	}

	return info, scanner.Err()
}

// modifyPackageMetadata 修改包元数据
func (dm *DebModifier) modifyPackageMetadata(info *PackageInfo) error {
	// 修改包名，添加魔改名称
	if !strings.Contains(info.Name, dm.MagicName) {
		info.Name = strings.Replace(info.Name, "frida", dm.MagicName, -1)
	}

	// 更新control文件
	controlFile := filepath.Join(dm.ExtractDir, "DEBIAN", "control")
	return dm.updateControlFile(controlFile, info)
}

// updateControlFile 更新control文件
func (dm *DebModifier) updateControlFile(controlFile string, info *PackageInfo) error {
	file, err := os.Open(controlFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])

				switch key {
				case "Package":
					line = fmt.Sprintf("Package: %s", info.Name)
				case "Version":
					// 保持原版本或更新
					line = fmt.Sprintf("Version: %s", info.Version)
				case "Description":
					line = fmt.Sprintf("Description: %s (Modified with %s)", strings.TrimSpace(parts[1]), dm.MagicName)
				}
			}
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// 写回文件
	return dm.writeLinesToFile(controlFile, lines)
}

// modifyBinaryFiles 修改二进制文件名和内容
func (dm *DebModifier) modifyBinaryFiles() error {
	log.Printf(i18n.T("INFO: 开始修改二进制文件名和内容"))

	// 验证魔改名称 (必须为5个字符，且符合命名规则)
	if len(dm.MagicName) != 5 {
		return fmt.Errorf(i18n.T("魔改名称必须是5个字符，当前: %s (%d字符)"), dm.MagicName, len(dm.MagicName))
	}

	// 验证字符规则：必须以字母开头，包含字母和数字
	if !dm.isValidMagicName(dm.MagicName) {
		return fmt.Errorf(i18n.T("魔改名称必须以字母开头，只能包含字母和数字: %s"), dm.MagicName)
	}

	// 创建HexReplacer实例
	hexReplacer := NewHexReplacer()

	// 查找frida-server文件
	var fridaServerPaths []string

	// 传统路径
	traditionalPath := filepath.Join(dm.ExtractDir, "usr", "sbin", "frida-server")
	if _, err := os.Stat(traditionalPath); err == nil {
		fridaServerPaths = append(fridaServerPaths, traditionalPath)
		log.Printf(i18n.T("DEBUG: 找到传统frida-server路径: %s"), traditionalPath)
	}

	// Rootless路径 (使用 /var/re 避免敏感词汇)
	rootlessPath := filepath.Join(dm.ExtractDir, "var", "re", "usr", "sbin", "frida-server")
	if _, err := os.Stat(rootlessPath); err == nil {
		fridaServerPaths = append(fridaServerPaths, rootlessPath)
		log.Printf(i18n.T("DEBUG: 找到rootless frida-server路径: %s"), rootlessPath)
	}

	// 对找到的frida-server文件执行hex替换和重命名
	for _, oldPath := range fridaServerPaths {
		// 1. 首先进行二进制内容修改
		log.Printf(i18n.T("INFO: 开始修改二进制文件内容: %s"), oldPath)

		// 获取原文件权限
		stat, err := os.Stat(oldPath)
		if err != nil {
			return fmt.Errorf(i18n.T("获取文件权限失败: %v"), err)
		}
		originalMode := stat.Mode()

		// 创建最终目标文件路径
		dir := filepath.Dir(oldPath)
		newPath := filepath.Join(dir, dm.MagicName)

		// 进度回调函数
		progressCallback := func(progress float64, message string) {
			log.Printf(i18n.T("DEBUG: HEX替换进度 %.1f%% - %s"), progress*100, message)
		}

		// 执行hex替换 (直接输出到最终文件名)
		err = hexReplacer.PatchFile(oldPath, dm.MagicName, newPath, progressCallback)
		if err != nil {
			log.Printf(i18n.T("ERROR: 二进制内容修改失败: %s, 错误: %v"), oldPath, err)
			return fmt.Errorf(i18n.T("修改二进制文件内容失败 %s: %v"), oldPath, err)
		}
		log.Printf(i18n.T("INFO: 成功修改二进制文件内容: %s"), oldPath)

		// 2. 删除原文件
		err = os.Remove(oldPath)
		if err != nil {
			log.Printf(i18n.T("WARNING: 删除原文件失败: %s, 错误: %v"), oldPath, err)
			// 不要因为删除失败而终止，继续执行
		}

		// 3. 设置新文件权限
		err = os.Chmod(newPath, originalMode)
		if err != nil {
			log.Printf(i18n.T("WARNING: 设置文件权限失败: %s, 权限: %o, 错误: %v"), newPath, originalMode, err)
		}

		log.Printf(i18n.T("INFO: 成功修改和重命名frida-server: %s -> %s"), oldPath, newPath)
	}

	// 查找并重命名frida-agent.dylib文件
	return dm.renameAgentLibraries()
}

// renameWithPermissions 保持权限的重命名函数
func (dm *DebModifier) renameWithPermissions(oldPath, newPath string) error {
	// 获取原文件权限
	stat, err := os.Stat(oldPath)
	if err != nil {
		return fmt.Errorf(i18n.T("获取文件信息失败: %v"), err)
	}

	originalMode := stat.Mode()
	log.Printf(i18n.T("DEBUG: 保持文件权限重命名: %s -> %s, 权限: %o"), oldPath, newPath, originalMode.Perm())

	// 执行重命名
	err = os.Rename(oldPath, newPath)
	if err != nil {
		return err
	}

	// 重新设置权限
	err = os.Chmod(newPath, originalMode)
	if err != nil {
		log.Printf(i18n.T("WARNING: 设置文件权限失败: %s, 权限: %o, 错误: %v"), newPath, originalMode, err)
		// 权限设置失败不算致命错误，继续执行
	}

	return nil
}

// renameAgentLibraries 重命名agent库文件
func (dm *DebModifier) renameAgentLibraries() error {
	log.Printf(i18n.T("INFO: 开始重命名agent库文件"))

	// 传统路径
	traditionalDir := filepath.Join(dm.ExtractDir, "usr", "lib", "frida")
	if _, err := os.Stat(traditionalDir); err == nil {
		log.Printf(i18n.T("DEBUG: 找到传统库目录: %s"), traditionalDir)
		newDir := filepath.Join(dm.ExtractDir, "usr", "lib", dm.MagicName)

		err = dm.renameWithPermissions(traditionalDir, newDir)
		if err != nil {
			log.Printf(i18n.T("ERROR: 重命名库目录失败: %s -> %s, 错误: %v"), traditionalDir, newDir, err)
			return fmt.Errorf(i18n.T("重命名库目录失败: %v"), err)
		}
		log.Printf(i18n.T("INFO: 成功重命名库目录: %s -> %s"), traditionalDir, newDir)

		// 重命名dylib文件
		err = dm.renameLibraryFiles(newDir)
		if err != nil {
			return err
		}
	}

	// Rootless路径 (使用 /var/re 避免敏感词汇)
	rootlessDir := filepath.Join(dm.ExtractDir, "var", "re", "usr", "lib", "frida")
	if _, err := os.Stat(rootlessDir); err == nil {
		log.Printf(i18n.T("DEBUG: 找到rootless库目录: %s"), rootlessDir)
		newDir := filepath.Join(dm.ExtractDir, "var", "re", "usr", "lib", dm.MagicName)

		err = dm.renameWithPermissions(rootlessDir, newDir)
		if err != nil {
			log.Printf(i18n.T("ERROR: 重命名rootless库目录失败: %s -> %s, 错误: %v"), rootlessDir, newDir, err)
			return fmt.Errorf(i18n.T("重命名rootless库目录失败: %v"), err)
		}
		log.Printf(i18n.T("INFO: 成功重命名rootless库目录: %s -> %s"), rootlessDir, newDir)

		// 重命名dylib文件
		err = dm.renameLibraryFiles(newDir)
		if err != nil {
			return err
		}
	}

	return nil
}

// renameLibraryFiles 重命名并修改库文件内容
func (dm *DebModifier) renameLibraryFiles(libDir string) error {
	log.Printf(i18n.T("DEBUG: 开始重命名和修改库目录中的文件: %s"), libDir)

	// 创建HexReplacer实例
	hexReplacer := NewHexReplacer()

	entries, err := os.ReadDir(libDir)
	if err != nil {
		log.Printf(i18n.T("ERROR: 读取库目录失败: %s, 错误: %v"), libDir, err)
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		oldName := entry.Name()
		if strings.Contains(oldName, "frida-agent") {
			newName := strings.Replace(oldName, "frida-agent", dm.MagicName+"-agent", -1)
			oldPath := filepath.Join(libDir, oldName)
			newPath := filepath.Join(libDir, newName)

			// 1. 首先进行二进制内容修改
			log.Printf(i18n.T("INFO: 开始修改agent库文件内容: %s"), oldPath)

			// 获取原文件权限
			stat, err := os.Stat(oldPath)
			if err != nil {
				return fmt.Errorf(i18n.T("获取agent文件权限失败: %v"), err)
			}
			originalMode := stat.Mode()

			// 进度回调函数
			progressCallback := func(progress float64, message string) {
				log.Printf(i18n.T("DEBUG: Agent HEX替换进度 %.1f%% - %s"), progress*100, message)
			}

			// 执行hex替换 (直接输出到最终文件名)
			err = hexReplacer.PatchFile(oldPath, dm.MagicName, newPath, progressCallback)
			if err != nil {
				log.Printf(i18n.T("ERROR: agent文件内容修改失败: %s, 错误: %v"), oldPath, err)
				return fmt.Errorf(i18n.T("修改agent文件内容失败 %s: %v"), oldPath, err)
			}
			log.Printf(i18n.T("INFO: 成功修改agent文件内容: %s"), oldPath)

			// 2. 删除原文件
			err = os.Remove(oldPath)
			if err != nil {
				log.Printf(i18n.T("WARNING: 删除原agent文件失败: %s, 错误: %v"), oldPath, err)
				// 不要因为删除失败而终止，继续执行
			}

			// 3. 设置新文件权限
			err = os.Chmod(newPath, originalMode)
			if err != nil {
				log.Printf(i18n.T("WARNING: 设置agent文件权限失败: %s, 权限: %o, 错误: %v"), newPath, originalMode, err)
			}

			log.Printf(i18n.T("INFO: 成功修改和重命名agent文件: %s -> %s"), oldName, newName)
		}
	}

	return nil
}

// modifyLaunchDaemon 修改启动守护进程配置
func (dm *DebModifier) modifyLaunchDaemon() error {
	log.Printf(i18n.T("DEBUG: 开始修改启动守护进程配置"))

	// 查找LaunchDaemons目录 (使用 /var/re 避免敏感词汇)
	launchDirs := []string{
		filepath.Join(dm.ExtractDir, "Library", "LaunchDaemons"),
		filepath.Join(dm.ExtractDir, "var", "re", "Library", "LaunchDaemons"),
	}

	for _, launchDir := range launchDirs {
		if _, err := os.Stat(launchDir); err != nil {
			log.Printf(i18n.T("DEBUG: LaunchDaemons目录不存在: %s"), launchDir)
			continue
		}

		log.Printf(i18n.T("DEBUG: 处理LaunchDaemons目录: %s"), launchDir)
		entries, err := os.ReadDir(launchDir)
		if err != nil {
			log.Printf(i18n.T("WARNING: 读取LaunchDaemons目录失败: %s, 错误: %v"), launchDir, err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			filename := entry.Name()
			if strings.Contains(filename, "frida.server") || strings.Contains(filename, "re.frida.server") {
				oldPath := filepath.Join(launchDir, filename)

				// 生成新文件名
				newFilename := strings.Replace(filename, "frida", dm.MagicName, -1)
				newPath := filepath.Join(launchDir, newFilename)

				log.Printf(i18n.T("DEBUG: 修改plist文件: %s -> %s"), filename, newFilename)

				// 修改plist内容
				err = dm.modifyPlistContent(oldPath, newPath)
				if err != nil {
					log.Printf(i18n.T("ERROR: 修改plist文件失败: %s, 错误: %v"), oldPath, err)
					return fmt.Errorf(i18n.T("修改plist文件失败: %v"), err)
				}
				log.Printf(i18n.T("INFO: 成功修改plist文件: %s"), newFilename)
			}
		}
	}

	return nil
}

// modifyPlistContent 修改plist文件内容
func (dm *DebModifier) modifyPlistContent(oldPath, newPath string) error {
	log.Printf(i18n.T("DEBUG: 修改plist文件内容: %s -> %s"), oldPath, newPath)

	// 获取原文件权限
	oldInfo, err := os.Stat(oldPath)
	if err != nil {
		log.Printf(i18n.T("ERROR: 获取plist文件信息失败: %s, 错误: %v"), oldPath, err)
		return err
	}

	log.Printf(i18n.T("DEBUG: 原plist文件权限: %s (%04o)"), oldPath, oldInfo.Mode().Perm())

	// 读取原文件
	content, err := os.ReadFile(oldPath)
	if err != nil {
		log.Printf(i18n.T("ERROR: 读取plist文件失败: %s, 错误: %v"), oldPath, err)
		return err
	}

	// 修改内容
	modifiedContent := string(content)

	// 替换二进制路径 (使用 /var/re 避免敏感词汇)
	modifiedContent = strings.ReplaceAll(modifiedContent, "/usr/sbin/frida-server", "/usr/sbin/"+dm.MagicName)
	modifiedContent = strings.ReplaceAll(modifiedContent, "/var/jb/usr/sbin/frida-server", "/var/re/usr/sbin/"+dm.MagicName)
	modifiedContent = strings.ReplaceAll(modifiedContent, "<string>/var/jb/", "<string>/var/re/")
	// 替换标签
	modifiedContent = strings.ReplaceAll(modifiedContent, "re.frida.server", "re."+dm.MagicName+".server")

	// 替换端口（如果存在）
	portRegex := regexp.MustCompile(`<string>27042</string>`)
	modifiedContent = portRegex.ReplaceAllString(modifiedContent, fmt.Sprintf("<string>%d</string>", dm.Port))

	// 添加端口启动参数 -l 0.0.0.0:端口 到 ProgramArguments
	// 只有当端口不是默认端口27042时才添加
	if dm.Port != 27042 {
		log.Printf(i18n.T("DEBUG: 添加端口启动参数: -l 0.0.0.0:%d"), dm.Port)

		// 查找 </array> 标签，在其前面添加 -l 和端口参数（确保无多余空行）
		arrayCloseRegex := regexp.MustCompile(`(\s*)</array>`)
		modifiedContent = arrayCloseRegex.ReplaceAllString(modifiedContent,
			fmt.Sprintf("$1\t<string>-l</string>\n$1\t<string>0.0.0.0:%d</string>\n$1</array>", dm.Port))

		log.Printf(i18n.T("DEBUG: 端口启动参数添加完成"))
	} else {
		log.Printf(i18n.T("DEBUG: 使用默认端口27042，无需添加启动参数"))
	}

	// 写入新文件（保持原权限）
	err = os.WriteFile(newPath, []byte(modifiedContent), oldInfo.Mode().Perm())
	if err != nil {
		log.Printf(i18n.T("ERROR: 写入新plist文件失败: %s, 错误: %v"), newPath, err)
		return err
	}

	// 验证新文件权限
	newInfo, err := os.Stat(newPath)
	if err == nil {
		log.Printf(i18n.T("DEBUG: 新plist文件权限: %s (%04o)"), newPath, newInfo.Mode().Perm())
	}

	// 删除旧文件
	if oldPath != newPath {
		err = os.Remove(oldPath)
		if err != nil {
			log.Printf(i18n.T("WARNING: 删除旧plist文件失败: %s, 错误: %v"), oldPath, err)
		} else {
			log.Printf(i18n.T("DEBUG: 删除旧plist文件: %s"), oldPath)
		}
	}

	return nil
}

// modifyDebianScripts 修改DEBIAN目录中的脚本
func (dm *DebModifier) modifyDebianScripts() error {
	debianDir := filepath.Join(dm.ExtractDir, "DEBIAN")

	// 修改extrainst_文件
	extrainstFile := filepath.Join(debianDir, "extrainst_")
	if _, err := os.Stat(extrainstFile); err == nil {
		err = dm.modifyScriptFile(extrainstFile)
		if err != nil {
			return fmt.Errorf(i18n.T("修改extrainst_失败: %v"), err)
		}
	}

	// 修改prerm文件
	prermFile := filepath.Join(debianDir, "prerm")
	if _, err := os.Stat(prermFile); err == nil {
		err = dm.modifyScriptFile(prermFile)
		if err != nil {
			return fmt.Errorf(i18n.T("修改prerm失败: %v"), err)
		}
	}

	return nil
}

// modifyScriptFile 修改脚本文件
func (dm *DebModifier) modifyScriptFile(scriptFile string) error {
	content, err := os.ReadFile(scriptFile)
	if err != nil {
		return err
	}

	modifiedContent := string(content)

	// 替换plist路径
	modifiedContent = strings.ReplaceAll(modifiedContent,
		"re.frida.server.plist",
		"re."+dm.MagicName+".server.plist")

	// 替换launchctl命令中的plist路径 (使用 /var/re 避免敏感词汇)
	modifiedContent = strings.ReplaceAll(modifiedContent,
		"/Library/LaunchDaemons/re.frida.server.plist",
		"/Library/LaunchDaemons/re."+dm.MagicName+".server.plist")

	modifiedContent = strings.ReplaceAll(modifiedContent,
		"/var/jb/Library/LaunchDaemons/re.frida.server.plist",
		"/var/re/Library/LaunchDaemons/re."+dm.MagicName+".server.plist")

	return os.WriteFile(scriptFile, []byte(modifiedContent), 0755)
}

// repackageDebFile 重新打包DEB文件（纯Go实现）
func (dm *DebModifier) repackageDebFile() error {
	return dm.repackageWithGoAr()
}

// repackageWithGoAr 使用纯Go方式重新打包DEB文件
func (dm *DebModifier) repackageWithGoAr() error {
	log.Printf(i18n.T("INFO: 开始重新打包DEB文件: %s -> %s"), dm.InputPath, dm.OutputPath)

	// 创建输出文件
	outputFile, err := os.Create(dm.OutputPath)
	if err != nil {
		log.Printf(i18n.T("ERROR: 创建输出文件失败: %v"), err)
		return fmt.Errorf(i18n.T("创建输出文件失败: %v"), err)
	}
	defer outputFile.Close()

	// 写入AR文件头部 "!<arch>\n"
	arHeaderWritten, err := outputFile.Write([]byte("!<arch>\n"))
	if err != nil {
		log.Printf(i18n.T("ERROR: 写入AR头部失败: %v"), err)
		return fmt.Errorf(i18n.T("写入AR头部失败: %v"), err)
	}
	log.Printf(i18n.T("DEBUG: AR文件头部写入完成: %d 字节"), arHeaderWritten)

	// 创建AR写入器
	arWriter := &arWriter{w: outputFile}

	// 1. 写入debian-binary文件
	debianBinary := []byte("2.0\n")
	log.Printf(i18n.T("DEBUG: 准备写入debian-binary: %d 字节"), len(debianBinary))
	err = arWriter.writeFile("debian-binary", debianBinary)
	if err != nil {
		log.Printf(i18n.T("ERROR: 写入debian-binary失败: %v"), err)
		return fmt.Errorf(i18n.T("写入debian-binary失败: %v"), err)
	}

	// 2. 创建并写入control.tar.xz
	log.Printf(i18n.T("INFO: 开始创建control.tar.xz"))
	controlData, err := dm.createControlTarData()
	if err != nil {
		log.Printf(i18n.T("ERROR: 创建control.tar数据失败: %v"), err)
		return fmt.Errorf(i18n.T("创建control.tar数据失败: %v"), err)
	}

	compressedControl, err := dm.compressWithXz(controlData)
	if err != nil {
		log.Printf(i18n.T("ERROR: 压缩control.tar失败: %v"), err)
		return fmt.Errorf(i18n.T("压缩control.tar失败: %v"), err)
	}

	err = arWriter.writeFile("control.tar.xz", compressedControl)
	if err != nil {
		log.Printf(i18n.T("ERROR: 写入control.tar.xz失败: %v"), err)
		return fmt.Errorf(i18n.T("写入control.tar.xz失败: %v"), err)
	}

	// 3. 创建并写入data.tar.xz
	log.Printf(i18n.T("INFO: 开始创建data.tar.xz"))
	dataArchive, err := dm.createDataTarData()
	if err != nil {
		log.Printf(i18n.T("ERROR: 创建data.tar数据失败: %v"), err)
		return fmt.Errorf(i18n.T("创建data.tar数据失败: %v"), err)
	}

	compressedData, err := dm.compressWithXz(dataArchive)
	if err != nil {
		log.Printf(i18n.T("ERROR: 压缩data.tar失败: %v"), err)
		return fmt.Errorf(i18n.T("压缩data.tar失败: %v"), err)
	}

	err = arWriter.writeFile("data.tar.xz", compressedData)
	if err != nil {
		log.Printf(i18n.T("ERROR: 写入data.tar.xz失败: %v"), err)
		return fmt.Errorf(i18n.T("写入data.tar.xz失败: %v"), err)
	}

	// 获取输出文件大小
	stat, err := outputFile.Stat()
	if err != nil {
		log.Printf(i18n.T("WARNING: 获取输出文件大小失败: %v"), err)
	} else {
		log.Printf(i18n.T("INFO: DEB文件重新打包完成，总大小: %d 字节"), stat.Size())
	}

	log.Printf(i18n.T("SUCCESS: DEB文件重新打包成功: %s"), dm.OutputPath)

	// 可选：验证生成的DEB文件
	err = dm.validateGeneratedDeb()
	if err != nil {
		log.Printf(i18n.T("WARNING: DEB文件验证失败: %v"), err)
	}

	return nil
}

// validateGeneratedDeb 验证生成的DEB文件
func (dm *DebModifier) validateGeneratedDeb() error {
	log.Printf(i18n.T("INFO: 开始验证生成的DEB文件: %s"), dm.OutputPath)

	// 尝试解析生成的DEB文件
	file, err := os.Open(dm.OutputPath)
	if err != nil {
		return fmt.Errorf(i18n.T("打开DEB文件失败: %v"), err)
	}
	defer file.Close()

	// 验证AR文件头部
	header := make([]byte, 8)
	_, err = file.Read(header)
	if err != nil {
		return fmt.Errorf(i18n.T("读取AR头部失败: %v"), err)
	}

	if string(header) != "!<arch>\n" {
		return fmt.Errorf(i18n.T("AR文件头部无效: %q"), string(header))
	}
	log.Printf(i18n.T("DEBUG: AR文件头部验证通过"))

	entryCount := 0
	var dataSize int64

	// 验证AR条目
	for {
		entry, err := dm.readArEntry(file)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf(i18n.T("读取AR条目失败: %v"), err)
		}

		entryCount++
		log.Printf(i18n.T("DEBUG: 验证AR条目 %d: %s (%d 字节)"), entryCount, entry.Name, entry.Size)

		if entry.Name == "data.tar.xz" {
			dataSize = entry.Size
			log.Printf(i18n.T("INFO: 找到data.tar.xz，大小: %d 字节"), dataSize)

			// 读取data.tar.xz内容进行验证
			dataContent := make([]byte, entry.Size)
			_, err = io.ReadFull(file, dataContent)
			if err != nil {
				return fmt.Errorf(i18n.T("读取data.tar.xz内容失败: %v"), err)
			}

			// 验证XZ压缩格式
			if len(dataContent) >= 6 {
				xzHeader := []byte{0xFD, '7', 'z', 'X', 'Z', 0x00}
				if bytes.Equal(dataContent[:6], xzHeader) {
					log.Printf(i18n.T("DEBUG: data.tar.xz XZ格式验证通过"))
				} else {
					log.Printf(i18n.T("WARNING: data.tar.xz XZ头部不匹配: %x"), dataContent[:6])
				}
			}

			// 尝试解压验证
			err = dm.validateTarXzContent(dataContent)
			if err != nil {
				return fmt.Errorf(i18n.T("data.tar.xz内容验证失败: %v"), err)
			}
		} else {
			// 跳过其他条目
			_, err = file.Seek(entry.Size, 1)
			if err != nil {
				return fmt.Errorf(i18n.T("跳过条目失败: %v"), err)
			}
		}

		// AR对齐
		if entry.Size%2 == 1 {
			file.Seek(1, 1)
		}
	}

	log.Printf(i18n.T("INFO: DEB文件验证完成 - 条目数: %d, data.tar.xz大小: %d 字节"), entryCount, dataSize)
	return nil
}

// validateTarXzContent 验证tar.xz内容
func (dm *DebModifier) validateTarXzContent(data []byte) error {
	log.Printf(i18n.T("DEBUG: 开始验证tar.xz内容，大小: %d 字节"), len(data))

	reader := bytes.NewReader(data)

	// 解压XZ
	xzReader, err := xz.NewReader(reader)
	if err != nil {
		return fmt.Errorf(i18n.T("创建XZ读取器失败: %v"), err)
	}

	// 读取TAR内容
	tarReader := tar.NewReader(xzReader)

	var fileCount, dirCount int
	var totalSize int64

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf(i18n.T("读取tar头失败: %v"), err)
		}

		if header.Typeflag == tar.TypeDir {
			dirCount++
		} else if header.Typeflag == tar.TypeReg {
			fileCount++
			totalSize += header.Size
		}

		log.Printf(i18n.T("DEBUG: TAR条目: %s, 类型: %d, 大小: %d"), header.Name, header.Typeflag, header.Size)
	}

	log.Printf(i18n.T("INFO: TAR内容验证完成 - 目录数: %d, 文件数: %d, 总大小: %d 字节"), dirCount, fileCount, totalSize)
	return nil
} // arWriter AR格式写入器
type arWriter struct {
	w io.Writer
}

// writeFile 写入AR文件条目
func (aw *arWriter) writeFile(name string, data []byte) error {
	log.Printf(i18n.T("DEBUG: 写入AR文件条目: %s, 大小: %d 字节"), name, len(data))

	// AR文件头格式: 名称(16字节) + 修改时间(12字节) + 用户ID(6字节) + 组ID(6字节) + 文件模式(8字节) + 文件大小(10字节) + 结束标记(2字节)
	header := make([]byte, 60)

	// 文件名（最多16字符，不足的用空格填充）
	copy(header[0:16], []byte(fmt.Sprintf("%-16s", name)))

	// 修改时间（Unix时间戳）
	copy(header[16:28], []byte(fmt.Sprintf("%-12d", 0)))

	// 用户ID
	copy(header[28:34], []byte("0     "))

	// 组ID
	copy(header[34:40], []byte("0     "))

	// 文件模式
	copy(header[40:48], []byte("100644  "))

	// 文件大小
	copy(header[48:58], []byte(fmt.Sprintf("%-10d", len(data))))

	// 结束标记
	copy(header[58:60], []byte("`\n"))

	// 写入头部
	headerWritten, err := aw.w.Write(header)
	if err != nil {
		log.Printf(i18n.T("ERROR: 写入AR头部失败: %s, 错误: %v"), name, err)
		return err
	}

	if headerWritten != 60 {
		log.Printf(i18n.T("WARNING: AR头部写入字节数不匹配: %s, 期望: 60, 实际: %d"), name, headerWritten)
	}

	// 写入数据
	dataWritten, err := aw.w.Write(data)
	if err != nil {
		log.Printf(i18n.T("ERROR: 写入AR数据失败: %s, 错误: %v"), name, err)
		return err
	}

	if dataWritten != len(data) {
		log.Printf(i18n.T("WARNING: AR数据写入字节数不匹配: %s, 期望: %d, 实际: %d"), name, len(data), dataWritten)
	}

	// AR文件要求每个条目都是偶数字节对齐
	if len(data)%2 == 1 {
		padWritten, err := aw.w.Write([]byte{'\n'})
		if err != nil {
			log.Printf(i18n.T("ERROR: 写入AR对齐填充失败: %s, 错误: %v"), name, err)
			return err
		}
		log.Printf(i18n.T("DEBUG: AR文件 %s 添加对齐填充: %d 字节"), name, padWritten)
	}

	log.Printf(i18n.T("INFO: AR条目写入完成: %s, 头部: %d 字节, 数据: %d 字节"), name, headerWritten, dataWritten)
	return nil
}

// createControlTarData 创建control.tar数据
func (dm *DebModifier) createControlTarData() ([]byte, error) {
	debianDir := filepath.Join(dm.ExtractDir, "DEBIAN")
	log.Printf(i18n.T("DEBUG: 开始创建control.tar数据，DEBIAN目录: %s"), debianDir)

	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)
	defer tarWriter.Close()

	var fileCount, dirCount int
	var totalSize int64

	err := filepath.Walk(debianDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf(i18n.T("ERROR: 遍历DEBIAN目录失败: %s, 错误: %v"), path, err)
			return err
		}

		// 跳过目录本身
		if path == debianDir {
			log.Printf(i18n.T("DEBUG: 跳过DEBIAN根目录: %s"), path)
			return nil
		}

		// 获取相对路径
		relPath, err := filepath.Rel(debianDir, path)
		if err != nil {
			log.Printf(i18n.T("ERROR: 计算DEBIAN相对路径失败: %s, 错误: %v"), path, err)
			return err
		}

		// 使用Unix路径分隔符
		originalRelPath := relPath
		relPath = strings.ReplaceAll(relPath, "\\", "/")
		if originalRelPath != relPath {
			log.Printf(i18n.T("DEBUG: DEBIAN路径分隔符转换: %s -> %s"), originalRelPath, relPath)
		}

		if info.IsDir() {
			dirCount++
			log.Printf(i18n.T("DEBUG: 添加DEBIAN目录: %s/, 权限: %o"), relPath, info.Mode().Perm())
			header := &tar.Header{
				Name:     relPath + "/",
				Mode:     int64(info.Mode().Perm()),
				Typeflag: tar.TypeDir,
				ModTime:  info.ModTime(),
				Format:   tar.FormatGNU, // 使用GNU格式匹配原始文件
			}
			err := tarWriter.WriteHeader(header)
			if err != nil {
				log.Printf(i18n.T("ERROR: 写入DEBIAN目录头部失败: %s, 错误: %v"), relPath, err)
			}
			return err
		} else {
			fileCount++
			totalSize += info.Size()

			// 根据文件类型设置正确的权限（符合DEB包规范）
			var perm os.FileMode
			switch relPath {
			case "control":
				perm = 0644 // control文件标准权限
			case "postinst", "prerm", "postrm", "preinst":
				perm = 0755 // 脚本文件需要可执行权限
			case "extrainst_":
				perm = 0755 // extrainst_文件需要可执行权限
			default:
				perm = 0644 // 其他文件默认权限
			}

			log.Printf(i18n.T("DEBUG: 添加DEBIAN文件: %s, 大小: %d 字节, 权限: %o"), relPath, info.Size(), perm)

			file, err := os.Open(path)
			if err != nil {
				log.Printf(i18n.T("ERROR: 打开DEBIAN文件失败: %s, 错误: %v"), path, err)
				return err
			}
			defer file.Close()

			header := &tar.Header{
				Name:     relPath,
				Mode:     int64(perm),
				Size:     info.Size(),
				Typeflag: tar.TypeReg,
				ModTime:  info.ModTime(),
				Uid:      0, // root
				Gid:      0, // root
				Uname:    "root",
				Gname:    "root",
				Format:   tar.FormatGNU, // 使用GNU格式匹配原始文件
			}

			err = tarWriter.WriteHeader(header)
			if err != nil {
				log.Printf(i18n.T("ERROR: 写入DEBIAN文件头部失败: %s, 错误: %v"), relPath, err)
				return err
			}

			written, err := io.Copy(tarWriter, file)
			if err != nil {
				log.Printf(i18n.T("ERROR: 复制DEBIAN文件内容失败: %s, 错误: %v"), relPath, err)
				return err
			}

			if written != info.Size() {
				log.Printf(i18n.T("WARNING: DEBIAN文件大小不匹配: %s, 期望: %d, 实际写入: %d"), relPath, info.Size(), written)
			}

			return nil
		}
	})

	if err != nil {
		log.Printf(i18n.T("ERROR: 遍历DEBIAN目录失败: %v"), err)
		return nil, err
	}

	err = tarWriter.Close()
	if err != nil {
		log.Printf(i18n.T("ERROR: 关闭control.tar写入器失败: %v"), err)
		return nil, err
	}

	tarData := buf.Bytes()
	log.Printf(i18n.T("INFO: control.tar创建完成 - 目录数: %d, 文件数: %d, 总文件大小: %d 字节, tar数据大小: %d 字节"),
		dirCount, fileCount, totalSize, len(tarData))

	return tarData, nil
}

// createDataTarData 创建data.tar数据
func (dm *DebModifier) createDataTarData() ([]byte, error) {
	log.Printf(i18n.T("DEBUG: 开始创建data.tar数据，提取目录: %s"), dm.ExtractDir)

	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)
	defer tarWriter.Close()

	var fileCount, dirCount int
	var totalSize int64

	// 检测是否为rootless结构
	isRootless := false
	if _, err := os.Stat(filepath.Join(dm.ExtractDir, "var", "re")); err == nil {
		isRootless = true
		log.Printf(i18n.T("DEBUG: 检测到rootless结构"))
	}

	// 所有结构都需要添加根目录 "." 条目，rootless也是从./var开始
	log.Printf(i18n.T("DEBUG: 添加TAR根目录条目 './'"))
	rootHeader := &tar.Header{
		Name:     "./",
		Mode:     int64(0755), // drwxr-xr-x
		Typeflag: tar.TypeDir,
		ModTime:  time.Now(),
		Uid:      0, // root
		Gid:      0, // root
		Uname:    "root",
		Gname:    "root",
		Format:   tar.FormatGNU, // 使用GNU格式匹配原始文件
	}
	err := tarWriter.WriteHeader(rootHeader)
	if err != nil {
		log.Printf(i18n.T("ERROR: 写入根目录头部失败: %v"), err)
		return nil, err
	}
	dirCount++

	err = filepath.Walk(dm.ExtractDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			log.Printf(i18n.T("ERROR: 遍历路径 %s 时出错: %v"), path, walkErr)
			return walkErr
		}

		// 跳过根目录和DEBIAN目录
		if path == dm.ExtractDir {
			log.Printf(i18n.T("DEBUG: 跳过根目录: %s"), path)
			return nil
		}

		relPath, err := filepath.Rel(dm.ExtractDir, path)
		if err != nil {
			log.Printf(i18n.T("ERROR: 计算相对路径失败，路径: %s, 错误: %v"), path, err)
			return err
		}

		// 跳过DEBIAN目录
		if strings.HasPrefix(relPath, "DEBIAN") {
			log.Printf(i18n.T("DEBUG: 跳过DEBIAN目录: %s"), relPath)
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 使用Unix路径分隔符
		originalRelPath := relPath
		relPath = strings.ReplaceAll(relPath, "\\", "/")
		if originalRelPath != relPath {
			log.Printf(i18n.T("DEBUG: 路径分隔符转换: %s -> %s"), originalRelPath, relPath)
		}

		// 根据路径类型决定TAR路径格式
		var tarPath string
		if isRootless && strings.HasPrefix(relPath, "var") {
			// rootless结构中的var路径使用"./"前缀: ./var/re/...
			tarPath = "./" + relPath
			log.Printf(i18n.T("DEBUG: TAR路径处理(rootless): %s -> %s"), relPath, tarPath)
		} else if !isRootless {
			// 传统结构添加"./"前缀
			tarPath = "./" + relPath
			log.Printf(i18n.T("DEBUG: TAR路径处理(传统): %s -> %s"), relPath, tarPath)
		} else {
			// rootless结构中的其他路径（如果有的话）
			tarPath = "./" + relPath
			log.Printf(i18n.T("DEBUG: TAR路径处理(其他): %s -> %s"), relPath, tarPath)
		}

		if info.IsDir() {
			dirCount++
			// 目录应该使用755权限 (drwxr-xr-x)
			dirMode := os.FileMode(0755)
			log.Printf(i18n.T("DEBUG: 添加目录: %s/, 权限: %04o (修正为755)"), tarPath, dirMode)
			header := &tar.Header{
				Name:     tarPath + "/",
				Mode:     int64(dirMode),
				Typeflag: tar.TypeDir,
				ModTime:  info.ModTime(),
				Uid:      0, // root
				Gid:      0, // root
				Uname:    "root",
				Gname:    "root",
				Format:   tar.FormatGNU, // 使用GNU格式匹配原始文件
			}
			err := tarWriter.WriteHeader(header)
			if err != nil {
				log.Printf(i18n.T("ERROR: 写入目录头部失败: %s, 错误: %v"), relPath, err)
			}
			return err
		} else {
			fileCount++
			totalSize += info.Size()

			// 根据文件类型设置正确的权限
			var perm os.FileMode
			switch {
			case strings.HasSuffix(tarPath, "/frida-server") || strings.HasSuffix(tarPath, "/"+dm.MagicName):
				// frida-server 和重命名后的服务器需要可执行权限
				perm = 0755
				log.Printf(i18n.T("DEBUG: 设置服务器可执行权限: %s -> 755"), tarPath)
			case strings.Contains(tarPath, "frida-agent") || strings.Contains(tarPath, dm.MagicName+"-agent"):
				// agent 库文件需要可执行权限
				perm = 0755
				log.Printf(i18n.T("DEBUG: 设置agent可执行权限: %s -> 755"), tarPath)
			case strings.HasSuffix(tarPath, ".plist"):
				// plist 文件使用标准权限
				perm = 0644
				log.Printf(i18n.T("DEBUG: 设置plist权限: %s -> 644"), tarPath)
			default:
				// 其他文件保持当前权限
				perm = info.Mode().Perm()
				log.Printf(i18n.T("DEBUG: 保持原权限: %s -> %04o"), tarPath, perm)
			}

			log.Printf(i18n.T("DEBUG: 添加文件: %s, 大小: %d 字节, 权限: %04o"), tarPath, info.Size(), perm)

			file, err := os.Open(path)
			if err != nil {
				log.Printf(i18n.T("ERROR: 打开文件失败: %s, 错误: %v"), path, err)
				return err
			}
			defer file.Close()

			header := &tar.Header{
				Name:     tarPath,
				Mode:     int64(perm),
				Size:     info.Size(),
				Typeflag: tar.TypeReg,
				ModTime:  info.ModTime(),
				Uid:      0, // root
				Gid:      0, // root
				Uname:    "root",
				Gname:    "root",
				Format:   tar.FormatGNU, // 使用GNU格式匹配原始文件
			}

			err = tarWriter.WriteHeader(header)
			if err != nil {
				log.Printf(i18n.T("ERROR: 写入文件头部失败: %s, 错误: %v"), relPath, err)
				return err
			}

			written, err := io.Copy(tarWriter, file)
			if err != nil {
				log.Printf(i18n.T("ERROR: 复制文件内容失败: %s, 错误: %v"), relPath, err)
				return err
			}

			if written != info.Size() {
				log.Printf(i18n.T("WARNING: 文件大小不匹配: %s, 期望: %d, 实际写入: %d"), relPath, info.Size(), written)
			}

			return nil
		}
	})

	if err != nil {
		log.Printf(i18n.T("ERROR: 遍历目录失败: %v"), err)
		return nil, err
	}

	err = tarWriter.Close()
	if err != nil {
		log.Printf(i18n.T("ERROR: 关闭tar写入器失败: %v"), err)
		return nil, err
	}

	tarData := buf.Bytes()
	log.Printf(i18n.T("INFO: data.tar创建完成 - 目录数: %d, 文件数: %d, 总文件大小: %d 字节, tar数据大小: %d 字节"),
		dirCount, fileCount, totalSize, len(tarData))

	return tarData, nil
}

// compressWithXz 使用XZ压缩数据
func (dm *DebModifier) compressWithXz(data []byte) ([]byte, error) {
	log.Printf(i18n.T("DEBUG: 开始XZ压缩，原始数据大小: %d 字节"), len(data))

	var buf bytes.Buffer

	// 创建与原始DEB文件匹配的XZ配置
	// 算法: LZMA2:23 CRC64, 数据流1, 字块3, 簇大小25165824
	config := xz.WriterConfig{
		DictCap:   16 << 20, // 16MB字典，匹配LZMA2:23级别
		BlockSize: 25165824, // 25MB块大小，匹配原始簇大小
		CheckSum:  xz.CRC64, // 使用CRC64匹配原始算法
	}

	writer, err := config.NewWriter(&buf)
	if err != nil {
		log.Printf(i18n.T("ERROR: 创建XZ写入器失败: %v"), err)
		return nil, err
	}

	written, err := writer.Write(data)
	if err != nil {
		writer.Close()
		log.Printf(i18n.T("ERROR: XZ压缩写入失败: %v"), err)
		return nil, err
	}

	if written != len(data) {
		log.Printf(i18n.T("WARNING: XZ压缩写入字节数不匹配，期望: %d, 实际: %d"), len(data), written)
	}

	err = writer.Close()
	if err != nil {
		log.Printf(i18n.T("ERROR: 关闭XZ写入器失败: %v"), err)
		return nil, err
	}

	compressed := buf.Bytes()
	compressionRatio := float64(len(compressed)) / float64(len(data)) * 100
	log.Printf(i18n.T("INFO: XZ压缩完成 - 原始: %d 字节, 压缩后: %d 字节, 压缩率: %.2f%%"),
		len(data), len(compressed), compressionRatio)

	return compressed, nil
} // writeLinesToFile 写入行到文件
func (dm *DebModifier) writeLinesToFile(filename string, lines []string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}

	return writer.Flush()
}

// isValidMagicName 验证魔改名称格式
func (dm *DebModifier) isValidMagicName(s string) bool {
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

// CreateFridaDeb 创建新的Frida DEB包
type CreateFridaDeb struct {
	FridaServerPath string       // frida-server 文件路径
	FridaAgentPath  string       // frida-agent.dylib 文件路径 (可选)
	OutputPath      string       // 输出DEB文件路径
	PackageInfo     *PackageInfo // 包信息
	TempDir         string       // 临时目录
}

// NewCreateFridaDeb 创建新的Frida DEB构建器
func NewCreateFridaDeb(fridaServerPath, outputPath string, info *PackageInfo) *CreateFridaDeb {
	return &CreateFridaDeb{
		FridaServerPath: fridaServerPath,
		OutputPath:      outputPath,
		PackageInfo:     info,
	}
}

// CreateDebPackage 创建新的DEB包
func (cfd *CreateFridaDeb) CreateDebPackage() error {
	log.Printf(i18n.T("INFO: 开始创建新的Frida DEB包"))
	log.Printf(i18n.T("INFO: 输入文件: %s"), cfd.FridaServerPath)
	log.Printf(i18n.T("INFO: 输出文件: %s"), cfd.OutputPath)
	log.Printf(i18n.T("INFO: 包名: %s, 版本: %s, 架构: %s"),
		cfd.PackageInfo.Name, cfd.PackageInfo.Version, cfd.PackageInfo.Architecture)
	log.Printf(i18n.T("INFO: 魔改名称: %s, 端口: %d"),
		cfd.PackageInfo.MagicName, cfd.PackageInfo.Port)
	log.Printf(i18n.T("INFO: 结构类型: %s"),
		map[bool]string{true: "Rootless", false: "Root"}[cfd.PackageInfo.IsRootless])

	// 1. 创建临时目录
	tempDir, err := os.MkdirTemp("", "fridare-create-*")
	if err != nil {
		return fmt.Errorf(i18n.T("创建临时目录失败: %v"), err)
	}
	cfd.TempDir = tempDir
	defer os.RemoveAll(tempDir)

	log.Printf(i18n.T("DEBUG: 临时目录: %s"), tempDir)

	// 2. 创建包目录结构
	err = cfd.createPackageStructure()
	if err != nil {
		return fmt.Errorf(i18n.T("创建包结构失败: %v"), err)
	}

	// 3. 复制并修改frida-server文件
	err = cfd.copyAndPatchFridaServer()
	if err != nil {
		return fmt.Errorf(i18n.T("复制和修改frida-server失败: %v"), err)
	}

	// 4. 复制并修改frida-agent文件（如果提供）
	if cfd.FridaAgentPath != "" {
		err = cfd.copyAndPatchFridaAgent()
		if err != nil {
			return fmt.Errorf(i18n.T("复制和修改frida-agent失败: %v"), err)
		}
	} else {
		// 检查是否为完整的frida包创建，如果是则提示需要agent文件
		log.Printf(i18n.T("WARNING: 未提供frida-agent.dylib文件，创建的DEB包将只包含frida-server"))
		log.Printf(i18n.T("INFO: 如需完整功能，请使用 -agent 参数指定frida-agent.dylib文件"))
	}

	// 5. 创建LaunchDaemon配置
	err = cfd.createLaunchDaemonConfig()
	if err != nil {
		return fmt.Errorf(i18n.T("创建启动守护程序配置失败: %v"), err)
	}

	// 6. 创建控制文件
	err = cfd.createControlFiles()
	if err != nil {
		return fmt.Errorf(i18n.T("创建控制文件失败: %v"), err)
	}

	// 7. 构建DEB包
	err = cfd.buildDebPackage()
	if err != nil {
		return fmt.Errorf(i18n.T("构建DEB包失败: %v"), err)
	}

	log.Printf(i18n.T("SUCCESS: Frida DEB包创建成功: %s"), cfd.OutputPath)
	return nil
}

// createPackageStructure 创建包目录结构
func (cfd *CreateFridaDeb) createPackageStructure() error {
	log.Printf(i18n.T("INFO: 创建包目录结构"))

	var dirs []string

	if cfd.PackageInfo.IsRootless {
		// Rootless结构
		dirs = []string{
			"DEBIAN",
			"var/re/usr/sbin",
			"var/re/usr/lib/" + cfd.PackageInfo.MagicName,
			"var/re/Library/LaunchDaemons",
		}
	} else {
		// Root结构
		dirs = []string{
			"DEBIAN",
			"usr/sbin",
			"usr/lib/" + cfd.PackageInfo.MagicName,
			"Library/LaunchDaemons",
		}
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(cfd.TempDir, dir)
		err := os.MkdirAll(fullPath, 0755)
		if err != nil {
			return fmt.Errorf(i18n.T("创建目录失败 %s: %v"), dir, err)
		}
		log.Printf(i18n.T("DEBUG: 创建目录: %s"), dir)
	}

	return nil
}

// copyAndPatchFridaServer 复制并修改frida-server文件
func (cfd *CreateFridaDeb) copyAndPatchFridaServer() error {
	log.Printf(i18n.T("INFO: 开始复制和修改frida-server文件"))

	// 目标路径
	var targetPath string
	if cfd.PackageInfo.IsRootless {
		targetPath = filepath.Join(cfd.TempDir, "var/re/usr/sbin", cfd.PackageInfo.MagicName)
	} else {
		targetPath = filepath.Join(cfd.TempDir, "usr/sbin", cfd.PackageInfo.MagicName)
	}

	// 如果需要patch，使用HexReplacer
	if cfd.PackageInfo.MagicName != "frida-server" {
		log.Printf(i18n.T("INFO: 开始对frida-server进行HEX替换"))

		// 创建HexReplacer实例
		hexReplacer := NewHexReplacer()

		// 进度回调
		progressFunc := func(progress float64, message string) {
			log.Printf(i18n.T("INFO: Server HEX替换进度 %.1f%% - %s"), progress, message)
		}

		// 执行hex替换
		err := hexReplacer.PatchFile(cfd.FridaServerPath, cfd.PackageInfo.MagicName, targetPath, progressFunc)
		if err != nil {
			return fmt.Errorf(i18n.T("HEX替换frida-server失败: %v"), err)
		}

		// 设置可执行权限
		err = os.Chmod(targetPath, 0755)
		if err != nil {
			log.Printf(i18n.T("WARNING: 设置frida-server权限失败: %v"), err)
		}

		log.Printf(i18n.T("SUCCESS: frida-server HEX替换完成"))
	} else {
		// 直接复制文件
		err := cfd.copyFileWithPermissions(cfd.FridaServerPath, targetPath, 0755)
		if err != nil {
			return fmt.Errorf(i18n.T("复制frida-server失败: %v"), err)
		}
	}

	log.Printf(i18n.T("INFO: frida-server文件处理完成: %s"), targetPath)
	return nil
}

// copyAndPatchFridaAgent 复制并修改frida-agent文件
func (cfd *CreateFridaDeb) copyAndPatchFridaAgent() error {
	log.Printf(i18n.T("INFO: 开始复制和修改frida-agent文件"))

	// 目标路径
	var targetDir string
	if cfd.PackageInfo.IsRootless {
		targetDir = filepath.Join(cfd.TempDir, "var/re/usr/lib", cfd.PackageInfo.MagicName)
	} else {
		targetDir = filepath.Join(cfd.TempDir, "usr/lib", cfd.PackageInfo.MagicName)
	}

	// 获取原文件名
	originalName := filepath.Base(cfd.FridaAgentPath)
	newName := strings.ReplaceAll(originalName, "frida-agent", cfd.PackageInfo.MagicName+"-agent")
	targetPath := filepath.Join(targetDir, newName)

	// 如果需要patch，使用HexReplacer
	if cfd.PackageInfo.MagicName != "frida" {
		log.Printf(i18n.T("INFO: 开始对frida-agent进行HEX替换"))

		// 创建HexReplacer实例
		hexReplacer := NewHexReplacer()

		// 进度回调
		progressFunc := func(progress float64, message string) {
			log.Printf(i18n.T("INFO: Agent HEX替换进度 %.1f%% - %s"), progress, message)
		}

		// 执行hex替换
		err := hexReplacer.PatchFile(cfd.FridaAgentPath, cfd.PackageInfo.MagicName, targetPath, progressFunc)
		if err != nil {
			return fmt.Errorf(i18n.T("HEX替换frida-agent失败: %v"), err)
		}

		// 设置可执行权限
		err = os.Chmod(targetPath, 0755)
		if err != nil {
			log.Printf(i18n.T("WARNING: 设置frida-agent权限失败: %v"), err)
		}

		log.Printf(i18n.T("SUCCESS: frida-agent HEX替换完成"))
	} else {
		// 直接复制文件
		err := cfd.copyFileWithPermissions(cfd.FridaAgentPath, targetPath, 0755)
		if err != nil {
			return fmt.Errorf(i18n.T("复制frida-agent失败: %v"), err)
		}
	}

	log.Printf(i18n.T("INFO: frida-agent文件处理完成: %s"), targetPath)
	return nil
}

// createLaunchDaemonConfig 创建启动守护程序配置
func (cfd *CreateFridaDeb) createLaunchDaemonConfig() error {
	log.Printf(i18n.T("INFO: 创建启动守护程序配置"))

	var plistPath string
	var programPath string

	if cfd.PackageInfo.IsRootless {
		plistPath = filepath.Join(cfd.TempDir, "var/re/Library/LaunchDaemons",
			fmt.Sprintf("re.%s.server.plist", cfd.PackageInfo.MagicName))
		// 注意：rootless环境下实际运行路径是/var/jb，但我们内部使用/var/re避免检测
		programPath = fmt.Sprintf("/var/re/usr/sbin/%s", cfd.PackageInfo.MagicName)
	} else {
		plistPath = filepath.Join(cfd.TempDir, "Library/LaunchDaemons",
			fmt.Sprintf("re.%s.server.plist", cfd.PackageInfo.MagicName))
		programPath = fmt.Sprintf("/usr/sbin/%s", cfd.PackageInfo.MagicName)
	}

	// 创建plist内容
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>re.%s.server</string>
	<key>Program</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>`,
		cfd.PackageInfo.MagicName, programPath, programPath)

	// 如果端口不是默认端口，添加端口参数
	if cfd.PackageInfo.Port != 27042 {
		plistContent += fmt.Sprintf(`
		<string>-l</string>
		<string>0.0.0.0:%d</string>`, cfd.PackageInfo.Port)
	}

	// 根据结构类型添加不同的配置
	if cfd.PackageInfo.IsRootless {
		// Rootless结构 - 简化配置
		plistContent += `
	</array>
	<key>UserName</key>
	<string>root</string>
	<key>POSIXSpawnType</key>
	<string>Interactive</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ThrottleInterval</key>
	<integer>5</integer>
	<key>ExecuteAllowed</key>
	<true/>
</dict>
</plist>
`
	} else {
		// Root结构 - 包含环境变量和系统级限制
		plistContent += `
	</array>
	<key>EnvironmentVariables</key>
	<dict>
		<key>_MSSafeMode</key>
		<string>1</string>
	</dict>
	<key>UserName</key>
	<string>root</string>
	<key>POSIXSpawnType</key>
	<string>Interactive</string>
	<key>RunAtLoad</key>
	<true/>
	<key>LimitLoadToSessionType</key>
	<string>System</string>
	<key>KeepAlive</key>
	<true/>
	<key>ThrottleInterval</key>
	<integer>5</integer>
	<key>ExecuteAllowed</key>
	<true/>
</dict>
</plist>
`
	}

	err := os.WriteFile(plistPath, []byte(plistContent), 0644)
	if err != nil {
		return fmt.Errorf(i18n.T("写入plist文件失败: %v"), err)
	}

	log.Printf(i18n.T("INFO: 启动守护程序配置创建完成: %s"), plistPath)
	return nil
}

// createControlFiles 创建控制文件
func (cfd *CreateFridaDeb) createControlFiles() error {
	log.Printf(i18n.T("INFO: 创建控制文件"))

	// 创建control文件 - 与原版格式保持一致，但替换frida字符串
	controlPath := filepath.Join(cfd.TempDir, "DEBIAN", "control")

	// 计算安装大小
	installedSize, err := cfd.calculateInstalledSize()
	if err != nil {
		log.Printf(i18n.T("WARNING: 计算安装大小失败: %v"), err)
		installedSize = 52864 // 使用原版默认值
	}

	// 计算包大小（近似值）
	packageSize := installedSize * 3 / 10 // 大约30%的压缩比

	// 生成与原版一致的control内容，但替换frida相关字符串
	controlContent := fmt.Sprintf(`Package: re.%s.server
Name: %s
Version: %s
Priority: %s
Size: %d
Installed-Size: %d
Architecture: %s
Description: Observe and reprogram running programs.
Homepage: %s
Maintainer: %s Developers <%s@nowsecure.com>
Author: %s Developers <%s@nowsecure.com>
Section: %s
Conflicts: re.%s.server64
`,
		cfd.PackageInfo.MagicName,                // re.{magic}.server
		strings.Title(cfd.PackageInfo.MagicName), // Name: {Magic}
		cfd.PackageInfo.Version,                  // Version
		cfd.PackageInfo.Priority,                 // Priority
		packageSize,                              // Size
		installedSize,                            // Installed-Size
		cfd.PackageInfo.Architecture,             // Architecture
		cfd.PackageInfo.Homepage,                 // Homepage
		strings.Title(cfd.PackageInfo.MagicName), // Maintainer: {Magic} Developers
		cfd.PackageInfo.MagicName,                // email prefix
		strings.Title(cfd.PackageInfo.MagicName), // Author: {Magic} Developers
		cfd.PackageInfo.MagicName,                // email prefix
		cfd.PackageInfo.Section,                  // Section
		cfd.PackageInfo.MagicName,                // Conflicts: re.{magic}.server64
	)

	err = os.WriteFile(controlPath, []byte(controlContent), 0644)
	if err != nil {
		return fmt.Errorf(i18n.T("写入control文件失败: %v"), err)
	}

	// 创建extrainst_文件（空文件，保持与原版一致）
	extrainstPath := filepath.Join(cfd.TempDir, "DEBIAN", "extrainst_")
	err = os.WriteFile(extrainstPath, []byte(""), 0755)
	if err != nil {
		return fmt.Errorf(i18n.T("创建extrainst_文件失败: %v"), err)
	}

	// 创建postinst脚本
	err = cfd.createPostInstScript()
	if err != nil {
		return fmt.Errorf(i18n.T("创建postinst脚本失败: %v"), err)
	}

	// 创建prerm脚本
	err = cfd.createPreRmScript()
	if err != nil {
		return fmt.Errorf(i18n.T("创建prerm脚本失败: %v"), err)
	}

	log.Printf(i18n.T("INFO: 控制文件创建完成"))
	return nil
}

// createPostInstScript 创建安装后脚本
func (cfd *CreateFridaDeb) createPostInstScript() error {
	postinstPath := filepath.Join(cfd.TempDir, "DEBIAN", "postinst")

	var content string
	if cfd.PackageInfo.IsRootless {
		content = fmt.Sprintf(`#!/bin/bash
set -e

# 设置可执行权限 (Rootless)
chmod 755 /var/re/usr/sbin/%s
chown root:wheel /var/re/usr/sbin/%s

# 设置 agent dylib 权限 (Rootless)
if [ -d "/var/re/usr/lib/%s" ]; then
    chmod 755 /var/re/usr/lib/%s/*
    chown root:wheel /var/re/usr/lib/%s/*
fi

# 加载守护程序
if [ -f /var/re/Library/LaunchDaemons/re.%s.server.plist ]; then
    launchctl load /var/re/Library/LaunchDaemons/re.%s.server.plist
fi

echo "Fridare %s 安装完成 (Rootless)"
echo "服务已启动在端口 %d"
`,
			cfd.PackageInfo.MagicName, cfd.PackageInfo.MagicName,
			cfd.PackageInfo.MagicName, cfd.PackageInfo.MagicName, cfd.PackageInfo.MagicName,
			cfd.PackageInfo.MagicName, cfd.PackageInfo.MagicName,
			cfd.PackageInfo.MagicName, cfd.PackageInfo.Port,
		)
	} else {
		content = fmt.Sprintf(`#!/bin/bash
set -e

# 设置可执行权限 (Root)
chmod 755 /usr/sbin/%s
chown root:wheel /usr/sbin/%s

# 设置 agent dylib 权限 (Root)
if [ -d "/usr/lib/%s" ]; then
    chmod 755 /usr/lib/%s/*
    chown root:wheel /usr/lib/%s/*
fi

# 加载守护程序
if [ -f /Library/LaunchDaemons/re.%s.server.plist ]; then
    launchctl load /Library/LaunchDaemons/re.%s.server.plist
fi

echo "Fridare %s 安装完成 (Root)"
echo "服务已启动在端口 %d"
`,
			cfd.PackageInfo.MagicName, cfd.PackageInfo.MagicName,
			cfd.PackageInfo.MagicName, cfd.PackageInfo.MagicName, cfd.PackageInfo.MagicName,
			cfd.PackageInfo.MagicName, cfd.PackageInfo.MagicName,
			cfd.PackageInfo.MagicName, cfd.PackageInfo.Port,
		)
	}

	return os.WriteFile(postinstPath, []byte(content), 0755)
}

// createPreRmScript 创建卸载前脚本
func (cfd *CreateFridaDeb) createPreRmScript() error {
	prermPath := filepath.Join(cfd.TempDir, "DEBIAN", "prerm")

	var content string
	if cfd.PackageInfo.IsRootless {
		content = fmt.Sprintf(`#!/bin/bash
set -e

# 停止守护程序 (Rootless)
if [ -f /var/re/Library/LaunchDaemons/re.%s.server.plist ]; then
    launchctl unload /var/re/Library/LaunchDaemons/re.%s.server.plist
fi

echo "Fridare %s 服务已停止 (Rootless)"
`,
			cfd.PackageInfo.MagicName, cfd.PackageInfo.MagicName,
			cfd.PackageInfo.MagicName,
		)
	} else {
		content = fmt.Sprintf(`#!/bin/bash
set -e

# 停止守护程序 (Root)
if [ -f /Library/LaunchDaemons/re.%s.server.plist ]; then
    launchctl unload /Library/LaunchDaemons/re.%s.server.plist
fi

echo "Fridare %s 服务已停止 (Root)"
`,
			cfd.PackageInfo.MagicName, cfd.PackageInfo.MagicName,
			cfd.PackageInfo.MagicName,
		)
	}

	return os.WriteFile(prermPath, []byte(content), 0755)
}

// calculateInstalledSize 计算安装大小（KB）
func (cfd *CreateFridaDeb) calculateInstalledSize() (int, error) {
	var totalSize int64

	err := filepath.Walk(cfd.TempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过DEBIAN目录
		if strings.Contains(path, "DEBIAN") {
			return nil
		}

		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	// 转换为KB并向上取整
	sizeKB := int((totalSize + 1023) / 1024)
	return sizeKB, nil
}

// buildDebPackage 构建DEB包
func (cfd *CreateFridaDeb) buildDebPackage() error {
	log.Printf(i18n.T("INFO: 开始构建DEB包"))

	// 使用内置的纯Go方式构建
	modifier := &DebModifier{
		OutputPath: cfd.OutputPath,
		ExtractDir: cfd.TempDir,
		MagicName:  cfd.PackageInfo.MagicName, // 传递魔改名称
	}

	return modifier.repackageWithGoAr()
}

// copyFileWithPermissions 复制文件并设置权限
func (cfd *CreateFridaDeb) copyFileWithPermissions(src, dst string, perm os.FileMode) error {
	// 确保目标目录存在
	dstDir := filepath.Dir(dst)
	err := os.MkdirAll(dstDir, 0755)
	if err != nil {
		return fmt.Errorf(i18n.T("创建目标目录失败: %v"), err)
	}

	// 复制文件
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf(i18n.T("打开源文件失败: %v"), err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf(i18n.T("创建目标文件失败: %v"), err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf(i18n.T("复制文件内容失败: %v"), err)
	}

	// 设置权限
	err = os.Chmod(dst, perm)
	if err != nil {
		return fmt.Errorf(i18n.T("设置文件权限失败: %v"), err)
	}

	return nil
}

// GetDefaultCreatePackageInfo 获取默认的创建包信息
func GetDefaultCreatePackageInfo(magicName string, port int, isRootless bool) *PackageInfo {
	packageName := fmt.Sprintf("re.frida.server.%s", magicName)
	if isRootless {
		packageName += ".rootless"
	}

	return &PackageInfo{
		Name:         packageName,
		Version:      "17.2.17",
		Architecture: "iphoneos-arm64",
		Maintainer:   "Fridare Team <support@fridare.com>",
		Description:  fmt.Sprintf("Dynamic instrumentation toolkit for developers, security researchers, and reverse engineers (Modified: %s)", magicName),
		Depends:      "firmware (>= 12.0)",
		Section:      "Development",
		Priority:     "optional",
		Homepage:     "https://frida.re/",
		Port:         port,
		MagicName:    magicName,
		IsRootless:   isRootless,
	}
}
