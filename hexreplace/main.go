package main

import (
    "bytes"
    "encoding/hex"
    "fmt"
    "io/ioutil"
    "os"
    "syscall"
)

func main() {
    if len(os.Args) != 4 {
        fmt.Println("使用方法: program <文件路径> <查找的hex串> <替换的hex串>")
        os.Exit(1)
    }

    filePath := os.Args[1]
    searchHex := os.Args[2]
    replaceHex := os.Args[3]

    // 获取文件信息
    fileInfo, err := os.Stat(filePath)
    if err != nil {
        fmt.Printf("获取文件信息失败: %v\n", err)
        os.Exit(1)
    }

    // 读取文件
    data, err := os.ReadFile(filePath)
    if err != nil {
        fmt.Printf("读取文件失败: %v\n", err)
        os.Exit(1)
    }

    // 解码搜索和替换的十六进制字符串
    search, err := hex.DecodeString(searchHex)
    if err != nil {
        fmt.Printf("解码搜索hex串失败: %v\n", err)
        os.Exit(1)
    }

    replace, err := hex.DecodeString(replaceHex)
    if err != nil {
        fmt.Printf("解码替换hex串失败: %v\n", err)
        os.Exit(1)
    }

    // 查找并替换
    count := 0
    for {
        index := bytes.Index(data, search)
        if index == -1 {
            break
        }
        count++
        data = append(data[:index], append(replace, data[index+len(search):]...)...)
    }

    // 创建临时文件
    tempFile, err := ioutil.TempFile("", "temp_")
    if err != nil {
        fmt.Printf("创建临时文件失败: %v\n", err)
        os.Exit(1)
    }
    tempFilePath := tempFile.Name()
    defer os.Remove(tempFilePath) // 确保临时文件被删除

    // 写入临时文件
    if err := os.WriteFile(tempFilePath, data, fileInfo.Mode()); err != nil {
        fmt.Printf("写入临时文件失败: %v\n", err)
        os.Exit(1)
    }

    // 获取原文件的所有者和组信息
    stat := fileInfo.Sys().(*syscall.Stat_t)
    uid := int(stat.Uid)
    gid := int(stat.Gid)

    // 设置临时文件的所有者和组
    if err := os.Chown(tempFilePath, uid, gid); err != nil {
        fmt.Printf("设置文件所有者失败: %v\n", err)
        os.Exit(1)
    }

    // 重命名临时文件，替换原文件
    if err := os.Rename(tempFilePath, filePath); err != nil {
        fmt.Printf("替换原文件失败: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("替换完成，共替换 %d 处\n", count)
}