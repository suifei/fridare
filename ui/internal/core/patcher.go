package core

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"fridare-gui/internal/i18n"
)

// PatchInfo 修补信息
type PatchInfo struct {
	OriginalHex    string
	ReplacementHex string
	Description    string
}

// BinaryPatcher 二进制文件修补器
type BinaryPatcher struct {
	workDir string
}

// NewBinaryPatcher 创建新的二进制修补器
func NewBinaryPatcher(workDir string) *BinaryPatcher {
	return &BinaryPatcher{
		workDir: workDir,
	}
}

// GetPredefinedPatches 获取预定义的修补模式
func (bp *BinaryPatcher) GetPredefinedPatches() map[string][]PatchInfo {
	return map[string][]PatchInfo{
		"frida-server": {
			{
				OriginalHex:    "667269646124736572766572",
				ReplacementHex: "", // 将在运行时设置
				Description:    i18n.T("修改服务器名称"),
			},
			{
				OriginalHex:    "000000000000FFFF0000FFFF",
				ReplacementHex: "", // 将在运行时设置
				Description:    i18n.T("修改默认端口"),
			},
		},
		"frida-gadget": {
			{
				OriginalHex:    "667269646124676164676574",
				ReplacementHex: "", // 将在运行时设置
				Description:    i18n.T("修改gadget标识"),
			},
		},
		"frida.dll": {
			{
				OriginalHex:    "6672696461",
				ReplacementHex: "", // 将在运行时设置
				Description:    i18n.T("修改DLL标识"),
			},
		},
	}
}

// DetectFileType 检测文件类型
func (bp *BinaryPatcher) DetectFileType(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf(i18n.T("读取文件失败: %w"), err)
	}

	// 检查文件头和内容特征
	fileName := strings.ToLower(filepath.Base(filePath))

	if strings.Contains(fileName, "frida-server") {
		return "frida-server", nil
	}
	if strings.Contains(fileName, "frida-gadget") || strings.Contains(fileName, "gadget") {
		return "frida-gadget", nil
	}
	if strings.HasSuffix(fileName, ".dll") && bytes.Contains(data, []byte("frida")) {
		return "frida.dll", nil
	}
	if strings.HasSuffix(fileName, ".so") && bytes.Contains(data, []byte("frida")) {
		return "libfrida.so", nil
	}
	if strings.HasSuffix(fileName, ".pyd") && bytes.Contains(data, []byte("frida")) {
		return "_frida.pyd", nil
	}

	// 通过内容检测
	if bytes.Contains(data, []byte("frida$server")) {
		return "frida-server", nil
	}
	if bytes.Contains(data, []byte("frida$gadget")) {
		return "frida-gadget", nil
	}

	return "unknown", nil
}

// PatchFile 修补文件
func (bp *BinaryPatcher) PatchFile(inputPath, outputPath string, patches []PatchInfo) error {
	// 读取原文件
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf(i18n.T("读取输入文件失败: %w"), err)
	}

	// 创建备份
	backupPath := inputPath + ".bak"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf(i18n.T("创建备份文件失败: %w"), err)
	}

	// 应用修补
	patchedData := make([]byte, len(data))
	copy(patchedData, data)

	for _, patch := range patches {
		if patch.OriginalHex == "" || patch.ReplacementHex == "" {
			continue
		}

		originalBytes, err := hex.DecodeString(patch.OriginalHex)
		if err != nil {
			return fmt.Errorf(i18n.T("解码原始十六进制失败: %w"), err)
		}

		replacementBytes, err := hex.DecodeString(patch.ReplacementHex)
		if err != nil {
			return fmt.Errorf(i18n.T("解码替换十六进制失败: %w"), err)
		}

		// 查找并替换
		patchedData = bytes.ReplaceAll(patchedData, originalBytes, replacementBytes)
	}

	// 保存修补后的文件
	if err := os.WriteFile(outputPath, patchedData, 0755); err != nil {
		return fmt.Errorf(i18n.T("写入输出文件失败: %w"), err)
	}

	return nil
}

// GenerateNamePatch 生成名称修补信息
func (bp *BinaryPatcher) GenerateNamePatch(newName string) (string, error) {
	if len(newName) != 5 {
		return "", fmt.Errorf(i18n.T("名称必须是5个字符"))
	}

	// 将新名称转换为十六进制，并添加$server后缀
	nameWithSuffix := newName + "$server"
	return hex.EncodeToString([]byte(nameWithSuffix)), nil
}

// GeneratePortPatch 生成端口修补信息
func (bp *BinaryPatcher) GeneratePortPatch(port int) (string, string, error) {
	if port < 1024 || port > 65535 {
		return "", "", fmt.Errorf(i18n.T("端口号必须在1024-65535范围内"))
	}

	// 这里需要根据具体的二进制格式来生成端口修补
	// 这是一个简化的实现，实际可能需要更复杂的逻辑
	originalPort := 27042 // 默认端口

	// 生成原始端口的十六进制表示
	originalHex := fmt.Sprintf("%04X", originalPort)
	// 生成新端口的十六进制表示
	newHex := fmt.Sprintf("%04X", port)

	return originalHex, newHex, nil
}

// ValidateFile 验证文件
func (bp *BinaryPatcher) ValidateFile(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf(i18n.T("文件不存在: %w"), err)
	}

	if info.IsDir() {
		return fmt.Errorf(i18n.T("路径是目录，不是文件"))
	}

	if info.Size() == 0 {
		return fmt.Errorf(i18n.T("文件为空"))
	}

	return nil
}

// CalculateFileMD5 计算文件MD5
func (bp *BinaryPatcher) CalculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GetFileInfo 获取文件信息
func (bp *BinaryPatcher) GetFileInfo(filePath string) (map[string]interface{}, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	md5Sum, err := bp.CalculateFileMD5(filePath)
	if err != nil {
		return nil, err
	}

	fileType, err := bp.DetectFileType(filePath)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"name":     info.Name(),
		"size":     info.Size(),
		"mod_time": info.ModTime(),
		"md5":      md5Sum,
		"type":     fileType,
	}, nil
}

// SearchPattern 在文件中搜索模式
func (bp *BinaryPatcher) SearchPattern(filePath, hexPattern string) ([]int64, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	pattern, err := hex.DecodeString(hexPattern)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("无效的十六进制模式: %w"), err)
	}

	var positions []int64
	for i := 0; i <= len(data)-len(pattern); i++ {
		if bytes.Equal(data[i:i+len(pattern)], pattern) {
			positions = append(positions, int64(i))
		}
	}

	return positions, nil
}
