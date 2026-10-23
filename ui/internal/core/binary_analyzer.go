package core

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"fmt"
	"os"
	"strings"
	"unicode"

	"fridare-gui/internal/i18n"
)

// BinaryAnalyzer 二进制文件分析器
type BinaryAnalyzer struct {
	filePath string
	fileType string
	data     []byte
}

// SectionInfo 段信息
type SectionInfo struct {
	Index      int
	Name       string
	Offset     uint64 // Start address
	EndOffset  uint64 // End address
	Size       uint64
	Type       string
	ArchIndex  int    // 架构索引（Fat Mach-O使用）
	ArchOffset uint64 // 架构在文件中的偏移（Fat Mach-O使用）

	// 扩展属性（IDA风格）
	Readable   bool   // R - 可读
	Writable   bool   // W - 可写
	Executable bool   // X - 可执行
	Data       bool   // D - 数据段
	Loaded     bool   // L - 已加载
	Align      string // 对齐方式
	Base       string // 基址类型
	Class      string // 段类别 (CODE/DATA/BSS等)
	AddressDep string // AD - 地址依赖
	Bitness    string // T - 位数 (32/64)
	DataSize   string // DS - 数据大小

	// 动态数据类型识别
	DataType      string // 数据类型: "pointer", "string", "code", "data"
	PointerSize   int    // 指针大小（字节）
	ContentFormat string // 内容格式: "address", "char", "byte"
}

// StringInfo 字符串信息
type StringInfo struct {
	Index  int
	Offset uint64
	Data   string
	Length int
	String string
}

// FileInfo 文件信息
type FileInfo struct {
	FilePath        string
	FileType        string
	FileSize        int64
	Architecture    string
	DetailedInfo    string // 详细的架构信息，来自arch_desc.go
	IsFatMachO      bool   // 是否为 Fat Mach-O 文件
	Sections        []SectionInfo
	SelectedSection *SectionInfo // 当前选中的段
	SectionData     []byte       // 选中段的数据
}

// NewBinaryAnalyzer 创建二进制分析器
func NewBinaryAnalyzer(filePath string) *BinaryAnalyzer {
	return &BinaryAnalyzer{
		filePath: filePath,
	}
}

// AnalyzeFile 分析文件
func (ba *BinaryAnalyzer) AnalyzeFile() (*FileInfo, error) {
	// 获取文件信息
	file, err := os.Open(ba.filePath)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("打开文件失败: %v"), err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf(i18n.T("获取文件信息失败: %v"), err)
	}

	// 使用hexreplace.go中的检测和打开逻辑
	binFile, format, err := detectAndOpenFile(ba.filePath)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("检测文件格式失败: %v"), err)
	}

	// 创建文件信息
	fileInfo := &FileInfo{
		FilePath: ba.filePath,
		FileType: formatToString(format),
		FileSize: stat.Size(),
	}

	// 获取详细的架构信息（使用arch_desc.go）
	fileInfo.DetailedInfo = describeArch(binFile, format)

	// 根据文件类型解析段信息
	switch f := binFile.(type) {
	case *macho.File:
		fileInfo.Architecture = cpuTypeToString(f.Cpu)
		fileInfo.IsFatMachO = false
		bitness := ba.getMachOBitness(f)
		fileInfo.Sections = ba.parseMachOSections(f, fileInfo.FileType, bitness)
	case *macho.FatFile:
		fileInfo.Architecture = "Universal Binary (Multiple Architectures)"
		fileInfo.IsFatMachO = true
		fileInfo.Sections = ba.parseFatMachOSections(f)
	case *elf.File:
		fileInfo.Architecture = f.Machine.String()
		fileInfo.IsFatMachO = false
		bitness := ba.getELFBitness(f)
		fileInfo.Sections = ba.parseELFSections(f, fileInfo.FileType, bitness)
	case *pe.File:
		fileInfo.Architecture = fmt.Sprintf("Machine: %d", f.Machine)
		fileInfo.IsFatMachO = false
		bitness := ba.getPEBitness(f)
		fileInfo.Sections = ba.parsePESections(f, fileInfo.FileType, bitness)
	default:
		return nil, fmt.Errorf(i18n.T("不支持的文件格式"))
	}

	return fileInfo, nil
}

// parseMachOSections 解析Mach-O单架构文件的段信息
func (ba *BinaryAnalyzer) parseMachOSections(f *macho.File, fileType string, bitness int) []SectionInfo {
	var sections []SectionInfo
	index := 0

	// 遍历所有段
	for _, sect := range f.Sections {
		sectInfo := SectionInfo{
			Index:     index,
			Name:      sect.Name,
			Offset:    uint64(sect.Offset),
			EndOffset: uint64(sect.Offset) + sect.Size,
			Size:      sect.Size,
			Type:      fmt.Sprintf("Flags: 0x%X", sect.Flags),
		}

		// 设置段属性（基于Mach-O段标志）
		ba.setMachOSectionAttributes(&sectInfo, sect)

		// 动态识别数据类型
		ba.IdentifySectionDataType(&sectInfo, fileType, bitness)

		sections = append(sections, sectInfo)
		index++
	}

	return sections
}

// setMachOSectionAttributes 设置Mach-O段的详细属性
func (ba *BinaryAnalyzer) setMachOSectionAttributes(sectInfo *SectionInfo, sect *macho.Section) {
	// 设置基本权限（基于段标志）
	// flags := sect.Flags // 暂时不使用，保留供将来扩展

	// 基于段名称和类型设置属性
	sectInfo.Readable = true // Mach-O段默认可读
	sectInfo.Loaded = true   // 默认已加载
	sectInfo.Bitness = "64"  // 假设64位，实际应该从文件头获取
	sectInfo.AddressDep = "00"
	sectInfo.DataSize = "0C"

	// 根据段名称设置权限和类型
	switch {
	case strings.HasPrefix(sect.Name, "__text") || strings.HasPrefix(sect.Name, "__stubs") ||
		strings.HasPrefix(sect.Name, "__stub_helper") || strings.HasPrefix(sect.Name, "__objc_stubs"):
		sectInfo.Executable = true
		sectInfo.Writable = false
		sectInfo.Class = "CODE"
		sectInfo.Base = fmt.Sprintf("%02X", sectInfo.Index+1)
		sectInfo.Align = getAlignmentType(sect.Name)

	case strings.HasPrefix(sect.Name, "__const") || strings.HasPrefix(sect.Name, "__cstring") ||
		strings.HasPrefix(sect.Name, "__objc_") || strings.HasPrefix(sect.Name, "__cfstring"):
		sectInfo.Executable = false
		sectInfo.Writable = false
		sectInfo.Class = "DATA"
		sectInfo.Base = fmt.Sprintf("%02X", sectInfo.Index+1)
		sectInfo.Align = getAlignmentType(sect.Name)

	case strings.HasPrefix(sect.Name, "__data") || strings.HasPrefix(sect.Name, "__got") ||
		strings.HasPrefix(sect.Name, "__la_symbol_ptr") || strings.HasPrefix(sect.Name, "__mod_init_func"):
		sectInfo.Executable = false
		sectInfo.Writable = true
		sectInfo.Class = "DATA"
		sectInfo.Base = fmt.Sprintf("%02X", sectInfo.Index+1)
		sectInfo.Align = getAlignmentType(sect.Name)

	case strings.HasPrefix(sect.Name, "__bss") || strings.HasPrefix(sect.Name, "__common"):
		sectInfo.Executable = false
		sectInfo.Writable = true
		sectInfo.Class = "BSS"
		sectInfo.Base = fmt.Sprintf("%02X", sectInfo.Index+1)
		sectInfo.Align = getAlignmentType(sect.Name)

	default:
		sectInfo.Executable = false
		sectInfo.Writable = false
		sectInfo.Class = "DATA"
		sectInfo.Base = fmt.Sprintf("%02X", sectInfo.Index+1)
		sectInfo.Align = "byte"
	}

	sectInfo.Data = sectInfo.Class == "DATA" || sectInfo.Class == "BSS"
}

// getAlignmentType 根据段名获取对齐类型
func getAlignmentType(sectName string) string {
	switch {
	case strings.Contains(sectName, "__cstring") || strings.Contains(sectName, "__objc_classname"):
		return "byte"
	case strings.Contains(sectName, "__text") || strings.Contains(sectName, "__const"):
		return "para"
	case strings.Contains(sectName, "__stubs") || strings.Contains(sectName, "__stub_helper"):
		return "dword"
	case strings.Contains(sectName, "__objc_stubs"):
		return "align_32"
	case strings.Contains(sectName, "__got") || strings.Contains(sectName, "__la_symbol_ptr") ||
		strings.Contains(sectName, "__data") || strings.Contains(sectName, "__cfstring"):
		return "qword"
	case strings.Contains(sectName, "__ustring") || strings.Contains(sectName, "__objc_methname"):
		return "word"
	case strings.Contains(sectName, "__eh_frame"):
		return "qword"
	default:
		return "byte"
	}
}

// parseFatMachOSections 解析Fat Mach-O文件的段信息
func (ba *BinaryAnalyzer) parseFatMachOSections(f *macho.FatFile) []SectionInfo {
	var sections []SectionInfo
	index := 0

	// 遍历所有架构
	for archIndex, arch := range f.Arches {
		// 添加架构信息作为一个节点
		sections = append(sections, SectionInfo{
			Index:      index,
			Name:       fmt.Sprintf("Architecture %d: %s", archIndex, arch.Cpu.String()),
			Offset:     uint64(arch.Offset),
			Size:       uint64(arch.Size),
			Type:       "Architecture",
			ArchIndex:  archIndex,
			ArchOffset: uint64(arch.Offset),
		})
		index++

		// 添加该架构的段信息
		for _, sect := range arch.Sections {
			sectInfo := SectionInfo{
				Index:      index,
				Name:       sect.Name,
				Offset:     uint64(arch.Offset + sect.Offset),
				EndOffset:  uint64(arch.Offset+sect.Offset) + sect.Size,
				Size:       sect.Size,
				Type:       fmt.Sprintf("Flags: 0x%X", sect.Flags),
				ArchIndex:  archIndex,
				ArchOffset: uint64(arch.Offset),
			}

			// 设置段属性
			ba.setMachOSectionAttributes(&sectInfo, sect)

			sections = append(sections, sectInfo)
			index++
		}
	}

	return sections
}

// parseELFSections 解析ELF文件的段信息
func (ba *BinaryAnalyzer) parseELFSections(f *elf.File, fileType string, bitness int) []SectionInfo {
	var sections []SectionInfo

	// 遍历所有段
	for i, sect := range f.Sections {
		sectInfo := SectionInfo{
			Index:  i,
			Name:   sect.Name,
			Offset: sect.Offset,
			Size:   sect.Size,
			Type:   sect.Type.String(),
		}

		// 设置段权限
		sectInfo.Readable = sect.Flags&elf.SHF_ALLOC != 0
		sectInfo.Writable = sect.Flags&elf.SHF_WRITE != 0
		sectInfo.Executable = sect.Flags&elf.SHF_EXECINSTR != 0

		// 动态识别数据类型
		ba.IdentifySectionDataType(&sectInfo, fileType, bitness)

		sections = append(sections, sectInfo)
	}

	return sections
}

// parsePESections 解析PE文件的段信息
func (ba *BinaryAnalyzer) parsePESections(f *pe.File, fileType string, bitness int) []SectionInfo {
	var sections []SectionInfo

	// 遍历所有段
	for i, sect := range f.Sections {
		sectInfo := SectionInfo{
			Index:  i,
			Name:   sect.Name,
			Offset: uint64(sect.Offset),
			Size:   uint64(sect.Size),
			Type:   fmt.Sprintf("VAddr: 0x%X", sect.VirtualAddress),
		}

		// 设置段权限（基于PE特性）
		sectInfo.Readable = sect.Characteristics&pe.IMAGE_SCN_MEM_READ != 0
		sectInfo.Writable = sect.Characteristics&pe.IMAGE_SCN_MEM_WRITE != 0
		sectInfo.Executable = sect.Characteristics&pe.IMAGE_SCN_MEM_EXECUTE != 0

		// 动态识别数据类型
		ba.IdentifySectionDataType(&sectInfo, fileType, bitness)

		sections = append(sections, sectInfo)
	}

	return sections
}

// GetSectionData 获取指定段的数据
func (ba *BinaryAnalyzer) GetSectionData(filePath string, sectionIndex int, sections []SectionInfo) ([]byte, error) {
	if sectionIndex < 0 || sectionIndex >= len(sections) {
		return nil, fmt.Errorf(i18n.T("无效的段索引"))
	}

	// 使用hexreplace.go中的检测和打开逻辑
	binFile, format, err := detectAndOpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("打开文件失败: %v"), err)
	}

	// 根据文件类型获取段数据
	switch f := binFile.(type) {
	case *macho.File:
		if sectionIndex < len(f.Sections) {
			return f.Sections[sectionIndex].Data()
		}
	case *macho.FatFile:
		// Fat文件需要特殊处理，根据段索引找到对应的架构和段
		if sectionIndex < len(sections) {
			section := sections[sectionIndex]

			// 如果是架构节点，返回空数据
			if section.Type == "Architecture" {
				return []byte{}, nil
			}

			// 找到对应的架构
			if section.ArchIndex < len(f.Arches) {
				arch := f.Arches[section.ArchIndex]

				// 在该架构中找到对应的段
				for _, sect := range arch.Sections {
					if sect.Name == section.Name {
						return sect.Data()
					}
				}
			}
		}
		return nil, fmt.Errorf(i18n.T("未找到 Fat Mach-O 段数据"))
	case *elf.File:
		if sectionIndex < len(f.Sections) {
			return f.Sections[sectionIndex].Data()
		}
	case *pe.File:
		if sectionIndex < len(f.Sections) {
			return f.Sections[sectionIndex].Data()
		}
	default:
		return nil, fmt.Errorf(i18n.T("不支持的文件格式: %v"), format)
	}

	return nil, fmt.Errorf(i18n.T("未找到段数据"))
}

// isPrintable 检查字符是否可打印
func isPrintable(b byte) bool {
	return unicode.IsPrint(rune(b)) && b < 127
}

// ExtractStringsFromData 从数据中提取字符串
func (ba *BinaryAnalyzer) ExtractStringsFromData(data []byte, baseOffset uint64) []StringInfo {
	var strings []StringInfo
	var currentString []byte
	var stringStart uint64
	index := 0

	for i, b := range data {
		if isPrintable(b) {
			if len(currentString) == 0 {
				stringStart = baseOffset + uint64(i)
			}
			currentString = append(currentString, b)
		} else {
			if len(currentString) >= 4 { // 只保留长度>=4的字符串
				str := string(currentString)
				hexData := fmt.Sprintf("%X", currentString)

				strings = append(strings, StringInfo{
					Index:  index,
					Offset: stringStart,
					Data:   hexData,
					Length: len(currentString),
					String: str,
				})
				index++
			}
			currentString = nil
		}
	}

	// 处理末尾的字符串
	if len(currentString) >= 4 {
		str := string(currentString)
		hexData := fmt.Sprintf("%X", currentString)

		strings = append(strings, StringInfo{
			Index:  index,
			Offset: stringStart,
			Data:   hexData,
			Length: len(currentString),
			String: str,
		})
	}

	return strings
}

// getMachOBitness 获取Mach-O文件的位数
func (ba *BinaryAnalyzer) getMachOBitness(f *macho.File) int {
	switch f.Cpu {
	case macho.Cpu386, macho.CpuArm:
		return 32
	case macho.CpuAmd64, macho.CpuArm64:
		return 64
	default:
		return 64 // 默认64位
	}
}

// getELFBitness 获取ELF文件的位数
func (ba *BinaryAnalyzer) getELFBitness(f *elf.File) int {
	switch f.Class {
	case elf.ELFCLASS32:
		return 32
	case elf.ELFCLASS64:
		return 64
	default:
		return 64 // 默认64位
	}
}

// getPEBitness 获取PE文件的位数
func (ba *BinaryAnalyzer) getPEBitness(f *pe.File) int {
	switch f.Machine {
	case pe.IMAGE_FILE_MACHINE_I386:
		return 32
	case pe.IMAGE_FILE_MACHINE_AMD64:
		return 64
	default:
		return 32 // PE默认32位
	}
}

// IdentifySectionDataType 动态识别段的数据类型
func (ba *BinaryAnalyzer) IdentifySectionDataType(section *SectionInfo, fileType string, bitness int) {
	// 设置指针大小
	section.PointerSize = bitness / 8

	// 根据段名称动态识别数据类型
	sectionName := strings.ToLower(section.Name)

	// 识别指针段
	if ba.isPointerSection(sectionName, fileType) {
		section.DataType = "pointer"
		section.ContentFormat = "address"
		return
	}

	// 识别字符串段
	if ba.isStringSection(sectionName, fileType) {
		section.DataType = "string"
		section.ContentFormat = "char"
		return
	}

	// 识别代码段
	if ba.isCodeSection(sectionName, fileType) || section.Executable {
		section.DataType = "code"
		section.ContentFormat = "byte"
		return
	}

	// 默认为数据段
	section.DataType = "data"
	section.ContentFormat = "byte"
}

// isPointerSection 判断是否为指针段（根据文件类型和段名）
func (ba *BinaryAnalyzer) isPointerSection(sectionName, fileType string) bool {
	switch fileType {
	case "Mach-O":
		pointerSections := []string{
			"__got",            // Global Offset Table
			"__la_symbol_ptr",  // Lazy symbol pointers
			"__nl_symbol_ptr",  // Non-lazy symbol pointers
			"__mod_init_func",  // Module initialization functions
			"__mod_term_func",  // Module termination functions
			"__auth_ptr",       // Authenticated pointers
			"__objc_classlist", // Objective-C class list
			"__objc_protolist", // Objective-C protocol list
			"__objc_imageinfo", // Objective-C image info
			"__objc_const",     // Objective-C constants (often pointers)
		}
		for _, pattern := range pointerSections {
			if strings.Contains(sectionName, pattern) {
				return true
			}
		}

	case "ELF":
		pointerSections := []string{
			".got",        // Global Offset Table
			".got.plt",    // GOT for PLT
			".plt",        // Procedure Linkage Table
			".init_array", // Initialization function pointers
			".fini_array", // Finalization function pointers
			".ctors",      // Constructor pointers
			".dtors",      // Destructor pointers
			".dynamic",    // Dynamic linking information
		}
		for _, pattern := range pointerSections {
			if strings.Contains(sectionName, pattern) {
				return true
			}
		}

	case "PE":
		pointerSections := []string{
			".idata", // Import data
			".edata", // Export data
			".reloc", // Relocation data
			".tls",   // Thread Local Storage
		}
		for _, pattern := range pointerSections {
			if strings.Contains(sectionName, pattern) {
				return true
			}
		}
	}

	return false
}

// isStringSection 判断是否为字符串段（根据文件类型和段名）
func (ba *BinaryAnalyzer) isStringSection(sectionName, fileType string) bool {
	switch fileType {
	case "Mach-O":
		stringSections := []string{
			"__cstring",        // C strings
			"__cfstring",       // Core Foundation strings
			"__string",         // General strings
			"__const",          // Constants (often strings)
			"__objc_methname",  // Objective-C method names
			"__objc_classname", // Objective-C class names
			"__objc_methtype",  // Objective-C method types
		}
		for _, pattern := range stringSections {
			if strings.Contains(sectionName, pattern) {
				return true
			}
		}

	case "ELF":
		stringSections := []string{
			".rodata",   // Read-only data (often strings)
			".strtab",   // String table
			".shstrtab", // Section header string table
			".dynstr",   // Dynamic string table
			".comment",  // Comments
		}
		for _, pattern := range stringSections {
			if strings.Contains(sectionName, pattern) {
				return true
			}
		}

	case "PE":
		stringSections := []string{
			".rdata", // Read-only data
			".rsrc",  // Resources (may contain strings)
		}
		for _, pattern := range stringSections {
			if strings.Contains(sectionName, pattern) {
				return true
			}
		}
	}

	return false
}

// isCodeSection 判断是否为代码段（根据文件类型和段名）
func (ba *BinaryAnalyzer) isCodeSection(sectionName, fileType string) bool {
	switch fileType {
	case "Mach-O":
		codeSections := []string{
			"__text",        // Code section
			"__stubs",       // Symbol stubs
			"__stub_helper", // Stub helper
		}
		for _, pattern := range codeSections {
			if strings.Contains(sectionName, pattern) {
				return true
			}
		}

	case "ELF":
		codeSections := []string{
			".text", // Code section
			".init", // Initialization code
			".fini", // Finalization code
			".plt",  // Procedure linkage table (code)
		}
		for _, pattern := range codeSections {
			if strings.Contains(sectionName, pattern) {
				return true
			}
		}

	case "PE":
		codeSections := []string{
			".text", // Code section
		}
		for _, pattern := range codeSections {
			if strings.Contains(sectionName, pattern) {
				return true
			}
		}
	}

	return false
}
