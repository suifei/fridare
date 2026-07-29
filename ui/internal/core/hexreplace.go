package core

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"fmt"
	"io"
	"os"
)

// ExecutableFormat represents the type of executable file
type ExecutableFormat int

const (
	PE ExecutableFormat = iota
	MachO
	ELF
)

// Replacement represents a hex replacement operation
type Replacement struct {
	Old []byte
	New []byte
}

// Replacements represents a collection of replacements for a specific section
type Replacements struct {
	ExecutableFormat ExecutableFormat
	SectionName      string
	Items            []*Replacement
}

// HexReplacer handles binary file patching operations
type HexReplacer struct{}

// NewHexReplacer creates a new hex replacer instance
func NewHexReplacer() *HexReplacer {
	return &HexReplacer{}
}

// PatchFile patches a binary file with the given frida new name
func (hr *HexReplacer) PatchFile(inputFilePath, fridaNewName, outputFilePath string, progressCallback func(float64, string)) error {
	// Validate frida new name (exactly 5 lowercase a-z — same as hexreplace CLI / tools tab)
	if err := ValidateMagicName(fridaNewName); err != nil {
		return err
	}

	if progressCallback != nil {
		progressCallback(0.1, "正在复制文件...")
	}

	// Copy input file to output file
	if err := copyFile(inputFilePath, outputFilePath); err != nil {
		return fmt.Errorf("error copying file: %v", err)
	}

	if progressCallback != nil {
		progressCallback(0.3, "正在检测文件格式...")
	}

	// Detect and open file
	file, format, err := detectAndOpenFile(outputFilePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}

	if progressCallback != nil {
		progressCallback(0.5, fmt.Sprintf("正在处理 %s 格式文件...", formatToString(format)))
	}

	// Process based on file format
	switch f := file.(type) {
	case *macho.File:
		if progressCallback != nil {
			progressCallback(0.7, "正在修改 MachO 单架构文件...")
		}
		return hr.handleSingleArchitecture(f, outputFilePath, fridaNewName, format)
	case *macho.FatFile:
		if progressCallback != nil {
			progressCallback(0.7, "正在修改 MachO 多架构文件...")
		}
		return hr.handleMultipleArchitectures(f, outputFilePath, fridaNewName, format)
	case *elf.File:
		if progressCallback != nil {
			progressCallback(0.7, "正在修改 ELF 文件...")
		}
		return hr.handleELFFile(f, outputFilePath, fridaNewName, format)
	case *pe.File:
		if progressCallback != nil {
			progressCallback(0.7, "正在修改 PE 文件...")
		}
		return hr.handlePEFile(f, outputFilePath, fridaNewName, format)
	default:
		return fmt.Errorf("unsupported file type")
	}
}

// DescribeFile returns a description of the file format and architecture
func (hr *HexReplacer) DescribeFile(filePath string) (string, error) {
	file, format, err := detectAndOpenFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %v", err)
	}

	description := fmt.Sprintf("Detected file format: %s\n", formatToString(format))
	description += describeArch(file, format)

	return description, nil
}

// detectAndOpenFile detects file format and opens it
func detectAndOpenFile(filePath string) (interface{}, ExecutableFormat, error) {
	if machoFile, err := macho.Open(filePath); err == nil {
		return machoFile, MachO, nil
	}
	if fatFile, err := macho.OpenFat(filePath); err == nil {
		return fatFile, MachO, nil
	}
	if elfFile, err := elf.Open(filePath); err == nil {
		return elfFile, ELF, nil
	}
	if peFile, err := pe.Open(filePath); err == nil {
		return peFile, PE, nil
	}
	return nil, 0, fmt.Errorf("unsupported file format")
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Set executable permissions
	return os.Chmod(dst, 0755)
}

// handlePEFile handles PE format files
func (hr *HexReplacer) handlePEFile(file *pe.File, outputFilePath, fridaNewName string, format ExecutableFormat) error {
	replacementsList := buildReplacements(fridaNewName, format)
	for _, replacements := range replacementsList {
		section := file.Section(replacements.SectionName)
		if section == nil {
			continue // Skip missing sections
		}

		data, err := section.Data()
		if err != nil {
			return fmt.Errorf("error reading section data for %s: %v", replacements.SectionName, err)
		}

		modifiedData := replaceInSection(data, replacements.Items)
		if err := writeModifiedSection(outputFilePath, int64(section.Offset), modifiedData); err != nil {
			return fmt.Errorf("error writing modified data for %s: %v", replacements.SectionName, err)
		}
	}
	return nil
}

// handleELFFile handles ELF format files
func (hr *HexReplacer) handleELFFile(file *elf.File, outputFilePath, fridaNewName string, format ExecutableFormat) error {
	replacementsList := buildReplacements(fridaNewName, format)
	for _, replacements := range replacementsList {
		section := file.Section(replacements.SectionName)
		if section == nil {
			continue // Skip missing sections
		}

		data, err := section.Data()
		if err != nil {
			return fmt.Errorf("error reading section data for %s: %v", replacements.SectionName, err)
		}

		modifiedData := replaceInSection(data, replacements.Items)
		if err := writeModifiedSection(outputFilePath, int64(section.Offset), modifiedData); err != nil {
			return fmt.Errorf("error writing modified data for %s: %v", replacements.SectionName, err)
		}
	}
	return nil
}

// handleSingleArchitecture handles single architecture MachO files
func (hr *HexReplacer) handleSingleArchitecture(file *macho.File, outputFilePath, fridaNewName string, format ExecutableFormat) error {
	replacementsList := buildReplacements(fridaNewName, format)
	for _, replacements := range replacementsList {
		section := file.Section(replacements.SectionName)
		if section == nil {
			continue // Skip missing sections
		}

		data, err := section.Data()
		if err != nil {
			return fmt.Errorf("error reading section data for %s: %v", replacements.SectionName, err)
		}

		modifiedData := replaceInSection(data, replacements.Items)
		if err := writeModifiedSection(outputFilePath, int64(section.Offset), modifiedData); err != nil {
			return fmt.Errorf("error writing modified data for %s: %v", replacements.SectionName, err)
		}
	}
	return nil
}

// handleMultipleArchitectures handles fat MachO files with multiple architectures
func (hr *HexReplacer) handleMultipleArchitectures(fatFile *macho.FatFile, filePath, fridaNewName string, format ExecutableFormat) error {
	for _, arch := range fatFile.Arches {
		if err := hr.patchArchitecture(arch, filePath, fridaNewName, format); err != nil {
			return err
		}
	}
	return nil
}

// patchArchitecture patches a specific architecture in a fat binary
func (hr *HexReplacer) patchArchitecture(arch macho.FatArch, filePath, fridaNewName string, format ExecutableFormat) error {
	replacementsList := buildReplacements(fridaNewName, format)
	for _, replacements := range replacementsList {
		section := arch.Section(replacements.SectionName)
		if section == nil {
			continue // Skip missing sections
		}

		data, err := section.Data()
		if err != nil {
			return fmt.Errorf("error reading section data for %s in architecture %s: %v", replacements.SectionName, arch.Cpu.String(), err)
		}

		modifiedData := replaceInSection(data, replacements.Items)
		if err := writeModifiedSection(filePath, int64(arch.Offset+section.Offset), modifiedData); err != nil {
			return fmt.Errorf("error writing modified data for %s in architecture %s: %v", replacements.SectionName, arch.Cpu.String(), err)
		}
	}
	return nil
}

// buildReplacements builds replacement rules based on file format
func buildReplacements(fridaNewName string, format ExecutableFormat) []Replacements {
	switch format {
	case MachO:
		return []Replacements{
			{
				ExecutableFormat: format,
				SectionName:      "__cstring",
				Items: []*Replacement{
					{Old: []byte("frida_server_"), New: []byte(fridaNewName + "_server_")},
					{Old: []byte("frida-server-main-loop"), New: []byte(fridaNewName + "-server-main-loop")},
					{Old: []byte("frida-main-loop"), New: []byte(fridaNewName + "-main-loop")},
					{Old: []byte("frida:rpc"), New: []byte(fridaNewName + ":rpc")},
					{Old: []byte("frida-agent.dylib"), New: []byte(fridaNewName + "-agent.dylib")},
					{Old: []byte("/usr/lib/frida/"), New: []byte("/usr/lib/" + fridaNewName + "/")},
					{Old: []byte("gum-"), New: []byte(fridaNewName[:3] + "-")},
				},
			},
			{
				ExecutableFormat: format,
				SectionName:      "__const",
				Items: []*Replacement{
					{Old: []byte("frida:rpc"), New: []byte(fridaNewName + ":rpc")},
				},
			},
		}
	case ELF:
		return []Replacements{
			{
				ExecutableFormat: format,
				SectionName:      ".rodata",
				Items: []*Replacement{
					{Old: []byte("frida_server_"), New: []byte(fridaNewName + "_server_")},
					{Old: []byte("frida-main-loop"), New: []byte(fridaNewName + "-main-loop")},
					{Old: []byte("frida:rpc"), New: []byte(fridaNewName + ":rpc")},
					{Old: []byte("frida-agent-<arch>.so"), New: []byte(fridaNewName + "-agent-<arch>.so")},
					{Old: []byte("frida-agent-arm.so"), New: []byte(fridaNewName + "-agent-arm.so")},
					{Old: []byte("frida-agent-arm64.so"), New: []byte(fridaNewName + "-agent-arm64.so")},
					{Old: []byte("frida-agent-32.so"), New: []byte(fridaNewName + "-agent-32.so")},
					{Old: []byte("frida-agent-64.so"), New: []byte(fridaNewName + "-agent-64.so")},
					{Old: []byte("gum-"), New: []byte(fridaNewName[:3] + "-")},
				},
			},
			{
				ExecutableFormat: format,
				SectionName:      ".text",
				Items: []*Replacement{
					{Old: []byte("frida:rpc"), New: []byte(fridaNewName + ":rpc")},
					{Old: []byte("gum-"), New: []byte(fridaNewName[:3] + "-")},
				},
			},
		}
	case PE:
		return []Replacements{
			{
				ExecutableFormat: format,
				SectionName:      ".rdata",
				Items: []*Replacement{
					{Old: []byte("frida-"), New: []byte(fridaNewName + "-")},
					{Old: []byte("frida_"), New: []byte(fridaNewName + "_")},
					{Old: []byte("frida_server_"), New: []byte(fridaNewName + "_server_")},
					{Old: []byte("frida-main-loop"), New: []byte(fridaNewName + "-main-loop")},
					{Old: []byte("gum-"), New: []byte(fridaNewName[:3] + "-")},
					{Old: []byte("frida-thread"), New: []byte(fridaNewName + "-thread")},
					{Old: []byte("frida:rpc"), New: []byte(fridaNewName + ":rpc")},
					{Old: []byte("frida-agent"), New: []byte(fridaNewName + "-agent")},
				},
			},
		}
	}
	return nil
}

// replaceInSection performs replacements in a section's data
func replaceInSection(data []byte, replacements []*Replacement) []byte {
	modifiedData := make([]byte, len(data))
	copy(modifiedData, data)

	for _, replacement := range replacements {
		oldBytes := replacement.Old
		newBytes := replacement.New

		for i := 0; i <= len(modifiedData)-len(oldBytes); i++ {
			if bytesEqual(modifiedData[i:i+len(oldBytes)], oldBytes) {
				replacementBytes := make([]byte, len(oldBytes))
				copy(replacementBytes, newBytes)
				// Pad with zeros if new bytes are shorter
				for j := len(newBytes); j < len(oldBytes); j++ {
					replacementBytes[j] = 0
				}
				copy(modifiedData[i:i+len(oldBytes)], replacementBytes)
			}
		}
	}

	return modifiedData
}

// bytesEqual compares two byte slices for equality
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// writeModifiedSection writes modified data to a specific offset in the file
func writeModifiedSection(filePath string, offset int64, data []byte) error {
	f, err := os.OpenFile(filePath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteAt(data, offset)
	return err
}

// formatToString converts ExecutableFormat to string
func formatToString(format ExecutableFormat) string {
	switch format {
	case PE:
		return "PE"
	case MachO:
		return "MachO"
	case ELF:
		return "ELF"
	default:
		return "Unknown"
	}
}
