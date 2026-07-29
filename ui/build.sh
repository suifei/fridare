#!/bin/bash
# Fridare GUI 构建脚本
# 本机构建: ./build.sh
# 交叉编译 Windows GUI (在 Linux/macOS 上): ./build.sh windows
# Windows 本机请使用: build.ps1 或 build.bat

set -e

echo "=== Fridare GUI 构建工具 ==="
echo ""

# 进入项目目录
cd "$(dirname "$0")"
mkdir -p build

TARGET="${1:-native}"

# 清理旧的构建文件
echo "清理旧的构建文件..."
rm -f build/fridare-gui build/fridare-gui.exe
rm -f build/fridare-create build/fridare-create.exe
rm -f build/fridare-patch build/fridare-patch.exe

build_native() {
    echo "构建本机应用程序..."
    if command -v fyne >/dev/null 2>&1; then
        # fyne build 输出路径相对 src
        fyne build --src cmd/gui -o ../../build/fridare-gui
    else
        echo "未找到 fyne CLI，使用 go build（建议: go install fyne.io/fyne/v2/cmd/fyne@latest）"
        go build -o build/fridare-gui ./cmd/gui
    fi
    go build -o build/fridare-create ./cmd/create
    go build -o build/fridare-patch ./cmd/patch
}

build_windows_cross() {
    echo "交叉编译 Windows amd64..."
    if ! command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
        echo "错误: 需要 MinGW 交叉编译器 x86_64-w64-mingw32-gcc"
        echo "  Ubuntu/Debian: sudo apt install gcc-mingw-w64"
        echo "  macOS: brew install mingw-w64"
        exit 1
    fi
    export CGO_ENABLED=1
    export GOOS=windows
    export GOARCH=amd64
    export CC=x86_64-w64-mingw32-gcc
    export CXX=x86_64-w64-mingw32-g++

    if command -v fyne >/dev/null 2>&1; then
        fyne build --src cmd/gui -o ../../build/fridare-gui.exe
    else
        go build -ldflags "-H windowsgui" -o build/fridare-gui.exe ./cmd/gui
    fi
    go build -o build/fridare-create.exe ./cmd/create
    go build -o build/fridare-patch.exe ./cmd/patch
}

case "$TARGET" in
    native|"")
        build_native
        ;;
    windows|win)
        build_windows_cross
        ;;
    *)
        echo "用法: $0 [native|windows]"
        exit 1
        ;;
esac

echo ""
echo "✅ 构建完成！"
echo ""
echo "生成的文件："
ls -la build/fridare-* 2>/dev/null || true

echo ""
echo "运行应用程序："
if [ -f build/fridare-gui.exe ]; then
    echo "  ./build/fridare-gui.exe"
elif [ -f build/fridare-gui ]; then
    echo "  ./build/fridare-gui"
fi