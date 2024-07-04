/*
hexreplace is a tool to patch binary file with hex string.
Usage: hexreplace <file path> <frida new name>
Example: hexreplace /Users/xxx/Desktop/frida-ios-dump/FridaGadget.dylib frida
Author: suifei@gmail.com
Github: https://github.com/suifei/fridare/tree/master/hexreplace
Version: 2.1

changelog:
- 2.1:
  - Refactored the Replacements structure to support multiple sections:
  - Added a new 'SectionName' field to the Replacements struct
  - Changed 'Replacements' to contain an 'Items' slice of Replacement structs
  - Modified the buildReplacements function to return a slice of Replacements, allowing for multiple section definitions
  - Updated handleSignleArchitecture and patchArchitecture functions to process multiple sections:
  - Now iterates through all defined sections in the Replacements slice
  - Applies replacements to each specified section individually
  - Adjusted the replaceInSection function to accept a slice of Replacement structs
  - Improved error handling and logging:
  - Added more detailed error messages for each section processing step
  - Included section names in log messages for better traceability
  - Enhanced code flexibility and extensibility:
  - Made it easier to add new sections for replacement in the future
  - Improved code organization by grouping replacement rules by section

- 2.0: support multiple architectures, add more ARM and ARM64 subtypes, add more replacements, macho.File.Section() returns a pointer to macho.Section, add more error handling
- 1.0: initial version
*/
package main

import (
	"debug/macho"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type Replacement struct {
	Old []byte
	New []byte
}

type Replacements struct {
	SectionName string
	Items       []*Replacement
}

type CPUSubtype uint32

// ARM subtypes
const (
	CPUSubtypeArmAll    CPUSubtype = 0
	CPUSubtypeArmV4T    CPUSubtype = 5
	CPUSubtypeArmV6     CPUSubtype = 6
	CPUSubtypeArmV5Tej  CPUSubtype = 7
	CPUSubtypeArmXscale CPUSubtype = 8
	CPUSubtypeArmV7     CPUSubtype = 9
	CPUSubtypeArmV7F    CPUSubtype = 10
	CPUSubtypeArmV7S    CPUSubtype = 11
	CPUSubtypeArmV7K    CPUSubtype = 12
	CPUSubtypeArmV8     CPUSubtype = 13
	CPUSubtypeArmV6M    CPUSubtype = 14
	CPUSubtypeArmV7M    CPUSubtype = 15
	CPUSubtypeArmV7Em   CPUSubtype = 16
	CPUSubtypeArmV8M    CPUSubtype = 17
)

// ARM64 subtypes
const (
	CPUSubtypeArm64All      CPUSubtype = 0
	CPUSubtypeArm64V8       CPUSubtype = 1
	CPUSubtypeArm64E        CPUSubtype = 2
	CPUSubtypeArm64E_pauth0 CPUSubtype = 0x80000002
)

// ARM64_32 subtypes
const (
	CPUSubtypeArm6432All CPUSubtype = 0
	CPUSubtypeArm6432V8  CPUSubtype = 1
)

func describeArch(f *macho.File) string {
	cpu := cpuTypeToString(f.Cpu)
	subtype := cpuSubtypeToString(f.Cpu, CPUSubtype(f.SubCpu))

	byteOrder := "Little Endian"
	if f.ByteOrder == binary.BigEndian {
		byteOrder = "Big Endian"
	}

	return fmt.Sprintf("CPU: %s, Subtype: %s, Byte Order: %s, File Type: %s",
		cpu, subtype, byteOrder, f.Type.String())
}

func cpuTypeToString(cpu macho.Cpu) string {
	switch cpu {
	case macho.Cpu386:
		return "x86"
	case macho.CpuAmd64:
		return "x86_64"
	case macho.CpuArm:
		return "ARM"
	case macho.CpuArm64:
		return "ARM64"
	case macho.CpuPpc:
		return "PowerPC"
	case macho.CpuPpc64:
		return "PowerPC 64"
	default:
		return fmt.Sprintf("Unknown CPU type: %d", cpu)
	}
}

func cpuSubtypeToString(cpu macho.Cpu, subtype CPUSubtype) string {
	switch cpu {
	case macho.CpuArm:
		switch subtype {
		case CPUSubtypeArmAll:
			return "All"
		case CPUSubtypeArmV4T:
			return "V4T"
		case CPUSubtypeArmV6:
			return "V6"
		case CPUSubtypeArmV5Tej:
			return "V5TEJ"
		case CPUSubtypeArmXscale:
			return "Xscale"
		case CPUSubtypeArmV7:
			return "V7"
		case CPUSubtypeArmV7F:
			return "V7F"
		case CPUSubtypeArmV7S:
			return "V7S"
		case CPUSubtypeArmV7K:
			return "V7K"
		case CPUSubtypeArmV8:
			return "V8"
		case CPUSubtypeArmV6M:
			return "V6M"
		case CPUSubtypeArmV7M:
			return "V7M"
		case CPUSubtypeArmV7Em:
			return "V7EM"
		case CPUSubtypeArmV8M:
			return "V8M"
		default:
			return fmt.Sprintf("Unknown ARM subtype: %d", subtype)
		}
	case macho.CpuArm64:
		switch subtype {
		case CPUSubtypeArm64All:
			return "All"
		case CPUSubtypeArm64V8:
			return "V8"
		case CPUSubtypeArm64E:
			return "E"
		case CPUSubtypeArm64E_pauth0:
			return "E_pauth0"
		default:
			return fmt.Sprintf("Unknown ARM64 subtype: %d", subtype)
		}
	case macho.Cpu386, macho.CpuAmd64:
		return "All" // x86 and x86_64 typically don't have meaningful subtypes in this context
	default:
		return fmt.Sprintf("Unknown subtype: %d", subtype)
	}
}

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: program <input file path> <frida new name> <output file path>")
		os.Exit(1)
	}

	inputFilePath := os.Args[1]
	fridaNewName := os.Args[2]
	outputFilePath := os.Args[3]

	if len(fridaNewName) != 5 || !isStringAlpha(fridaNewName) {
		fmt.Println("Error: frida new name must be a 5-character alphabetic string")
		os.Exit(1)
	}

	// 首先复制输入文件到输出文件
	if err := copyFile(inputFilePath, outputFilePath); err != nil {
		fmt.Println("Error copying file:", err)
		os.Exit(1)
	}

	fatFile, err := macho.OpenFat(outputFilePath)
	if err != nil {
		fmt.Println("Open Fat error:", err)
		file, err := macho.Open(outputFilePath)
		if err == nil {
			handleSignleArchitecture(file, outputFilePath, fridaNewName)
		} else {
			fmt.Println("Error opening file:", err)
			os.Exit(1)
		}
	} else {
		handleMultipleArchitectures(fatFile, outputFilePath, fridaNewName)
	}

	fmt.Println("Patch success")
}

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

	os.Chmod(dst, 0755)
	return err
}

func handleSignleArchitecture(file *macho.File, outputFilePath, fridaNewName string) {
	replacementsList := buildReplacements(fridaNewName)
	for _, replacements := range replacementsList {
		section := file.Section(replacements.SectionName)
		if section == nil {
			fmt.Printf("Warning: %s section not found in file\n", replacements.SectionName)
			continue
		}
		data, err := section.Data()
		if err != nil {
			fmt.Printf("Error reading section data for %s: %v\n", replacements.SectionName, err)
			continue
		}
		modifiedData := replaceInSection(data, replacements.Items)
		if err := writeModifiedSection(outputFilePath, int64(section.Offset), modifiedData); err != nil {
			fmt.Printf("Error writing modified data for %s: %v\n", replacements.SectionName, err)
			continue
		}
		fmt.Printf("Successfully patched %s section in architecture: %s\n", replacements.SectionName, describeArch(file))
	}
}

func patchArchitecture(arch macho.FatArch, filePath, fridaNewName string) {
	replacementsList := buildReplacements(fridaNewName)
	for _, replacements := range replacementsList {
		section := arch.Section(replacements.SectionName)
		if section == nil {
			fmt.Printf("Warning: %s section not found in architecture %s\n", replacements.SectionName, describeArch(arch.File))
			continue
		}
		data, err := section.Data()
		if err != nil {
			fmt.Printf("Error reading section data for %s in architecture %s: %v\n", replacements.SectionName, describeArch(arch.File), err)
			continue
		}
		modifiedData := replaceInSection(data, replacements.Items)
		if err := writeModifiedSection(filePath, int64(arch.Offset+section.Offset), modifiedData); err != nil {
			fmt.Printf("Error writing modified data for %s in architecture %s: %v\n", replacements.SectionName, describeArch(arch.File), err)
			continue
		}
		fmt.Printf("Successfully patched %s section in architecture: %s\n", replacements.SectionName, describeArch(arch.File))
	}
}

func handleMultipleArchitectures(fatFile *macho.FatFile, filePath, fridaNewName string) {
	for _, arch := range fatFile.Arches {
		patchArchitecture(arch, filePath, fridaNewName)
	}
}

func isStringAlpha(s string) bool {
	for _, c := range s {
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}

func buildReplacements(fridaNewName string) []Replacements {
	return []Replacements{
		{
			SectionName: "__cstring",
			Items: []*Replacement{
				{Old: []byte("frida_server_"), New: []byte(fridaNewName + "_server_")},
				{Old: []byte("frida-server-main-loop"), New: []byte(fridaNewName + "-server-main-loop")},
				{Old: []byte("frida-main-loop"), New: []byte(fridaNewName + "-main-loop")},
				{Old: []byte("frida:rpc"), New: []byte(fridaNewName + ":rpc")},
				{Old: []byte("frida-agent.dylib"), New: []byte(fridaNewName + "-agent.dylib")},
				{Old: []byte("/usr/lib/frida/"), New: []byte("/usr/lib/" + fridaNewName + "/")},
			},
		},
		{
			SectionName: "__const",
			Items: []*Replacement{
				{Old: []byte("frida:rpc"), New: []byte(fridaNewName + ":rpc")},
			},
		},
	}
}

func replaceInSection(data []byte, replacements []*Replacement) []byte {
	modifiedData := make([]byte, len(data))
	copy(modifiedData, data)

	for _, replacement := range replacements {
		oldBytes := replacement.Old
		newBytes := replacement.New

		for i := 0; i <= len(modifiedData)-len(oldBytes); i++ {
			if bytesEqual(modifiedData[i:i+len(oldBytes)], oldBytes) {
				replacement := make([]byte, len(oldBytes))
				copy(replacement, newBytes)
				for j := len(newBytes); j < len(oldBytes); j++ {
					replacement[j] = 0
				}
				copy(modifiedData[i:i+len(oldBytes)], replacement)
			}
		}
	}

	return modifiedData
}

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

func writeModifiedSection(filePath string, offset int64, data []byte) error {
	f, err := os.OpenFile(filePath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteAt(data, offset)
	return err
}
