# Fridare GUI Windows 构建脚本 (PowerShell)
# 用法: 在 PowerShell 中执行: .\build.ps1
# 依赖: Go 1.21+, CGO/gcc (Fyne), 可选 fyne CLI
# 图标与版本信息: scripts/gen-winres.ps1 → resource_windows_amd64.syso

$ErrorActionPreference = "Stop"

Write-Host "=== Fridare GUI 构建工具 (Windows) ===" -ForegroundColor Cyan
Write-Host ""

Set-Location $PSScriptRoot

# 创建输出目录
New-Item -ItemType Directory -Force -Path "build" | Out-Null

Write-Host "生成 Windows 图标 / 版本资源 (.syso)..." -ForegroundColor Yellow
& powershell -NoProfile -File (Join-Path $PSScriptRoot "scripts\gen-winres.ps1")
if ($LASTEXITCODE -ne 0) {
    Write-Host "警告: winres 生成失败，exe 可能无图标/版本信息" -ForegroundColor DarkYellow
}

Write-Host "清理旧的构建文件..." -ForegroundColor Yellow
Remove-Item -Force -ErrorAction SilentlyContinue build/fridare-gui.exe, build/fridare-create.exe, build/fridare-patch.exe

Write-Host "构建应用程序..." -ForegroundColor Yellow

# Windows GUI 必须用 -H windowsgui，否则双击会弹出黑色控制台。
# 注意：fyne build 不一定注入该子系统标志；正式构建一律 go build + syso。
# 需要 CGO（Fyne/OpenGL）；请确保 MinGW 在 PATH。
$env:CGO_ENABLED = "1"
Write-Host "go build GUI（-H windowsgui，无控制台）..." -ForegroundColor Green
& go build -trimpath -ldflags "-s -w -H windowsgui" -o build/fridare-gui.exe ./cmd/gui
if ($LASTEXITCODE -ne 0) { throw "go build gui 失败（检查 CGO/gcc 与 PATH）" }

& go build -ldflags "-s -w" -o build/fridare-create.exe ./cmd/create
if ($LASTEXITCODE -ne 0) { throw "go build create 失败" }

& go build -ldflags "-s -w" -o build/fridare-patch.exe ./cmd/patch
if ($LASTEXITCODE -ne 0) { throw "go build patch 失败" }

Write-Host ""
Write-Host "构建完成!" -ForegroundColor Green
Write-Host ""
Write-Host "生成的文件:" -ForegroundColor Cyan
Get-ChildItem build/fridare-*.exe | Format-Table Name, Length, LastWriteTime

Write-Host "运行应用程序:" -ForegroundColor Cyan
Write-Host "  .\build\fridare-gui.exe"
