# Fridare GUI Windows 构建脚本 (PowerShell)
# 用法: 在 PowerShell 中执行: .\build.ps1
# 依赖: Go 1.21+, fyne CLI (go install fyne.io/fyne/v2/cmd/fyne@latest)

$ErrorActionPreference = "Stop"

Write-Host "=== Fridare GUI 构建工具 (Windows) ===" -ForegroundColor Cyan
Write-Host ""

Set-Location $PSScriptRoot

# 创建输出目录
New-Item -ItemType Directory -Force -Path "build" | Out-Null

Write-Host "清理旧的构建文件..." -ForegroundColor Yellow
Remove-Item -Force -ErrorAction SilentlyContinue build/fridare-gui.exe, build/fridare-create.exe, build/fridare-patch.exe

Write-Host "构建应用程序..." -ForegroundColor Yellow

# 优先使用 fyne build（更好的图标/资源打包）；不可用则回退 go build
$fyne = Get-Command fyne -ErrorAction SilentlyContinue
if ($fyne) {
    Write-Host "使用 fyne build 构建 GUI..." -ForegroundColor Green
    & fyne build --src cmd/gui -o ../../build/fridare-gui.exe
    if ($LASTEXITCODE -ne 0) { throw "fyne build 失败" }
} else {
    Write-Host "未找到 fyne CLI，使用 go build 构建 GUI..." -ForegroundColor Yellow
    Write-Host "提示: go install fyne.io/fyne/v2/cmd/fyne@latest" -ForegroundColor Gray
    & go build -ldflags "-H windowsgui" -o build/fridare-gui.exe ./cmd/gui
    if ($LASTEXITCODE -ne 0) { throw "go build gui 失败" }
}

& go build -o build/fridare-create.exe ./cmd/create
if ($LASTEXITCODE -ne 0) { throw "go build create 失败" }

& go build -o build/fridare-patch.exe ./cmd/patch
if ($LASTEXITCODE -ne 0) { throw "go build patch 失败" }

Write-Host ""
Write-Host "构建完成!" -ForegroundColor Green
Write-Host ""
Write-Host "生成的文件:" -ForegroundColor Cyan
Get-ChildItem build/fridare-*.exe | Format-Table Name, Length, LastWriteTime

Write-Host "运行应用程序:" -ForegroundColor Cyan
Write-Host "  .\build\fridare-gui.exe"
