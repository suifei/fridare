@echo off
setlocal
cd /d "%~dp0"

echo === Fridare GUI 构建工具 (Windows) ===
echo.

if not exist build mkdir build

echo 清理旧的构建文件...
del /q build\fridare-gui.exe 2>nul
del /q build\fridare-create.exe 2>nul
del /q build\fridare-patch.exe 2>nul

echo 构建应用程序...
where fyne >nul 2>nul
if %ERRORLEVEL%==0 (
    echo 使用 fyne build 构建 GUI...
    fyne build --src cmd/gui -o ../../build/fridare-gui.exe
    if errorlevel 1 goto :error
) else (
    echo 未找到 fyne CLI，使用 go build...
    echo 提示: go install fyne.io/fyne/v2/cmd/fyne@latest
    go build -ldflags "-H windowsgui" -o build\fridare-gui.exe .\cmd\gui
    if errorlevel 1 goto :error
)

go build -o build\fridare-create.exe .\cmd\create
if errorlevel 1 goto :error
go build -o build\fridare-patch.exe .\cmd\patch
if errorlevel 1 goto :error

echo.
echo 构建完成!
echo.
echo 生成的文件:
dir /b build\fridare-*.exe
echo.
echo 运行: build\fridare-gui.exe
goto :eof

:error
echo 构建失败
exit /b 1
