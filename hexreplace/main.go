/*
hexreplace is a tool to patch binary file with hex string.
Usage: hexreplace <file path> <frida new name>
Example: hexreplace /Users/xxx/Desktop/frida-ios-dump/FridaGadget.dylib frida
Author: suifei@gmail.com
Github: https://github.com/suifei/fridare/tree/master/hexreplace
Version: 1.0
*/
package main

import (
	"fmt"
	"os"
	"path"
)

// isAlpha 判断一个字符是否为英文字母（小写）
// 参数 c 表示待判断的字符
// 返回值 bool，表示字符是否为英文字母（小写），是则返回 true，否则返回 false
func isAlpha(c rune) bool {
	// 检查字符是否在 'a'-'z' 范围内
	return (c >= 'a' && c <= 'z')
}

// isStringAlpha 判断一个字符串是否全部由字母组成
// 参数 s：待判断的字符串
// 返回值：若字符串全部由字母组成，则返回true，否则返回false
func isStringAlpha(s string) bool {
	for _, c := range s {
		if !isAlpha(c) {
			return false
		}
	}
	return true
}

// main 函数是程序的入口点
func main() {
	fmt.Println(os.Args)
	if !parseAndValidateArgs() {
		os.Exit(1)
	}

	filePath := os.Args[1]
	fridaNewName := os.Args[2]

	buf, err := readFile(filePath)
	if err != nil {
		fmt.Println("read file failed: ", err)
		os.Exit(1)
	}

	resourcePatch := buildResourcePatch(fridaNewName)

	if !patchBinary(buf, resourcePatch) {
		os.Exit(1)
	}

	if err := writeFile(filePath, buf); err != nil {
		fmt.Println("write file failed: ", err)
		os.Exit(1)
	}

	fmt.Println("patch success")
	os.Exit(0)
}

// parseAndValidateArgs 函数用于解析和验证命令行参数
// 如果参数解析和验证成功，则返回true；否则返回false
func parseAndValidateArgs() bool {
	if len(os.Args) != 3 {
		fmt.Println("args error: usage: program <file path> <frida new name>")
		return false
	}

	filePath := os.Args[1]
	filePath = path.Join(".", filePath)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Println("args error: file not exist")
		return false
	}

	fridaNewName := os.Args[2]
	if len(fridaNewName) != 5 || !isStringAlpha(fridaNewName) {
		fmt.Println("args error: frida new name must be a 5-character string")
		return false
	}

	return true
}

// readFile 函数从给定的文件路径中读取文件内容，并返回文件的字节切片和可能发生的错误。
// 如果文件读取成功，则返回的字节切片包含文件的内容；如果发生错误，则返回非零的错误码。
//
// 参数：
// filePath string - 要读取的文件路径。
//
// 返回值：
// []byte - 文件的字节切片。
// error - 如果在读取文件时发生错误，则返回非零的错误码；否则为nil。
func readFile(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

// buildResourcePatch 根据给定的 fridaNewName 构建资源补丁映射
//
// 参数：
//
//	fridaNewName string - Frida 新名称
//
// 返回值：
//
//	map[string]string - 包含 Frida 相关资源名称映射的字典
func buildResourcePatch(fridaNewName string) map[string]string {
	if len(fridaNewName) != 5 {
		fmt.Println("(buildResourcePatch) frida new name must be a 5-character string")
		os.Exit(1)
	}
	return map[string]string{
		"frida_server_":          fridaNewName + "_server_",
		"frida-server-main-loop": fridaNewName + "-server-main-loop",
		"frida-main-loop":        fridaNewName + "-main-loop",
	}
}

// patchBinary 函数用于在二进制缓冲区中根据资源补丁（resourcePatch）进行替换操作
// 参数：
//
//	buf []byte - 需要进行替换操作的二进制缓冲区
//	resourcePatch map[string]string - 资源补丁映射，其中key表示原始资源关键字，value表示补丁后的关键字
//
// 返回值：
//
//	bool - 替换操作是否成功，成功返回true，失败返回false
func patchBinary(buf []byte, resourcePatch map[string]string) bool {
	for resourceKey, patchKey := range resourcePatch {
		indexes := QueryBinaryIndexByKeyword(buf, []byte(resourceKey))
		resourceKeyCount := len(indexes)
		fmt.Println("resource_key count: ", resourceKeyCount)
		buf = ReplaceBinary(buf, indexes, []byte(patchKey))
		indexes = QueryBinaryIndexByKeyword(buf, []byte(patchKey))
		patchKeyCount := len(indexes)
		fmt.Println("patch_key count: ", patchKeyCount)
		if resourceKeyCount != patchKeyCount {
			fmt.Println("patch failed: ", resourceKeyCount, "->", patchKeyCount)
			return false
		} else {
			fmt.Println("patch ", resourceKeyCount, "->", patchKeyCount)
		}
	}
	return true
}

// writeFile 函数将给定的字节切片buf写入到指定的文件路径filePath中，并返回可能产生的错误。
// filePath：要写入的文件路径，字符串类型。
// buf：要写入文件的字节切片。
// 返回值：
// error：如果写入过程中出现错误，则返回非nil的错误对象；否则返回nil。
func writeFile(filePath string, buf []byte) error {
	return os.WriteFile(filePath, buf, 0777)
}

// QueryBinaryIndexByKeyword 从给定的字节切片 buf 中查找与关键字 keyword 相匹配的起始索引，并返回这些索引的切片
//
// buf: 待搜索的字节切片
// keyword: 待查找的关键字字节切片
//
// 返回值:
//
//	[]int: 包含所有匹配关键字的起始索引的切片
func QueryBinaryIndexByKeyword(buf []byte, keyword []byte) []int {
	indexs := []int{}
	for i := 0; i <= len(buf)-len(keyword); i++ {
		match := true
		for j := 0; j < len(keyword); j++ {
			if buf[i+j] != keyword[j] {
				match = false
				break
			}
		}
		if match {
			indexs = append(indexs, i)
		}
	}
	return indexs
}

// ReplaceBinary 替换二进制数据中的指定位置为给定的补丁内容
//
// buf: 需要替换的原始二进制数据切片
// indexs: 需要替换的索引位置列表
// patchKey: 替换内容的补丁字节切片
//
// 返回值: 替换后的二进制数据切片
func ReplaceBinary(buf []byte, indexs []int, patchKey []byte) []byte {
	newBuf := make([]byte, len(buf))
	copy(newBuf, buf) // 复制原始buf到新切片newBuf

	for _, index := range indexs {
		if index+len(patchKey) <= len(buf) { // 检查是否越界
			copy(newBuf[index:index+len(patchKey)], patchKey)
		}
	}

	return newBuf
}
