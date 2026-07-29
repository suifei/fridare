# Multi-platform release builder for Fridare.
# Tools (create/patch/hexreplace): pure Go, CGO_ENABLED=0 (fully static).
# GUI (Fyne): requires CGO + OpenGL; built for host Windows only.
#
# Usage: powershell -File scripts\build-release.ps1 [-Version 4.0.3]

param(
    [string]$Version = "4.0.3"
)

$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$OutRoot = Join-Path $Root "dist\release-v$Version"
$UiDir = Join-Path $Root "ui"
$HexDir = Join-Path $Root "hexreplace"

Write-Host "=== Fridare release build v$Version ===" -ForegroundColor Cyan
Write-Host "Output: $OutRoot"

if (Test-Path $OutRoot) { Remove-Item -Recurse -Force $OutRoot }
New-Item -ItemType Directory -Force -Path $OutRoot | Out-Null

$ToolTargets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Ext = ".exe"; Dir = "windows-amd64" },
    @{ GOOS = "windows"; GOARCH = "arm64"; Ext = ".exe"; Dir = "windows-arm64" },
    @{ GOOS = "linux";   GOARCH = "amd64"; Ext = "";     Dir = "linux-amd64" },
    @{ GOOS = "linux";   GOARCH = "arm64"; Ext = "";     Dir = "linux-arm64" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; Ext = "";     Dir = "darwin-amd64" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; Ext = "";     Dir = "darwin-arm64" }
)

$Ldflags = "-s -w -trimpath"
# Note: -trimpath is a go build flag, not ldflag
$BuildArgs = @("-trimpath", "-ldflags", "-s -w")

function Build-StaticTool {
    param($GOOS, $GOARCH, $OutDir, $Ext)

    $env:CGO_ENABLED = "0"
    $env:GOOS = $GOOS
    $env:GOARCH = $GOARCH

    New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

    Push-Location $UiDir
    try {
        $create = Join-Path $OutDir "fridare-create$Ext"
        $patch  = Join-Path $OutDir "fridare-patch$Ext"
        Write-Host "  [static] fridare-create $GOOS/$GOARCH"
        & go build @BuildArgs -o $create ./cmd/create
        if ($LASTEXITCODE -ne 0) { throw "create failed $GOOS/$GOARCH" }
        Write-Host "  [static] fridare-patch $GOOS/$GOARCH"
        & go build @BuildArgs -o $patch ./cmd/patch
        if ($LASTEXITCODE -ne 0) { throw "patch failed $GOOS/$GOARCH" }
    } finally {
        Pop-Location
    }

    Push-Location $HexDir
    try {
        $hex = Join-Path $OutDir "hexreplace$Ext"
        Write-Host "  [static] hexreplace $GOOS/$GOARCH"
        & go build @BuildArgs -o $hex .
        if ($LASTEXITCODE -ne 0) { throw "hexreplace failed $GOOS/$GOARCH" }
    } finally {
        Pop-Location
    }
}

function Build-WindowsGui {
    param($OutDir)

    # Fyne GUI: host Windows only, CGO required (not pure-static OpenGL)
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    $env:CGO_ENABLED = "1"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"

    New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
    Push-Location $UiDir
    try {
        $gui = Join-Path $OutDir "fridare-gui.exe"
        Write-Host "  [cgo] fridare-gui windows/amd64"
        & go build -trimpath -ldflags "-s -w -H windowsgui" -o $gui ./cmd/gui
        if ($LASTEXITCODE -ne 0) { throw "gui failed" }
    } finally {
        Pop-Location
    }
}

function Write-Readme {
    param($Path, $PlatformLabel, $HasGui)

    $guiLine = if ($HasGui) {
        "- fridare-gui.exe  GUI (requires Windows graphics stack; built with CGO)`r`n"
    } else {
        "- GUI: not included for this platform (Fyne needs native CGO/OpenGL). Build on target OS: cd ui && ./build.sh`r`n"
    }

    $text = @"
Fridare v$Version - $PlatformLabel
================================

Contents (tools are pure-Go static binaries, CGO_ENABLED=0):
- fridare-create   Create modified iOS deb packages
- fridare-patch    Patch existing frida deb packages
- hexreplace       Binary string replacer for frida-server / frida-tools
$guiLine
Magic name must be exactly 5 lowercase letters a-z.
Device frida-server and PC frida-tools must use the same magic name.

Windows batch helpers (ASCII + CRLF only):
  See repository win/patch-frida.cmd and win/patch-frida-tools.cmd

Project: https://github.com/suifei/fridare
Release: https://github.com/suifei/fridare/releases/tag/v$Version
"@
    # README can be UTF-8; tools binaries are static. Use ASCII for consistency on Windows.
    $ascii = [System.Text.Encoding]::ASCII.GetBytes(($text -replace "`n", "`r`n"))
    [System.IO.File]::WriteAllBytes($Path, $ascii)
}

# --- build all tool platforms ---
foreach ($t in $ToolTargets) {
    $dir = Join-Path $OutRoot $t.Dir
    Write-Host "Building tools $($t.Dir)..." -ForegroundColor Yellow
    Build-StaticTool -GOOS $t.GOOS -GOARCH $t.GOARCH -OutDir $dir -Ext $t.Ext

    $hasGui = $false
    if ($t.GOOS -eq "windows" -and $t.GOARCH -eq "amd64") {
        Build-WindowsGui -OutDir $dir
        $hasGui = $true
        # ASCII batch helpers for Windows packages
        Copy-Item (Join-Path $Root "win\patch-frida.cmd") $dir
        Copy-Item (Join-Path $Root "win\patch-frida-tools.cmd") $dir
        Copy-Item (Join-Path $dir "hexreplace.exe") (Join-Path $dir "hexreplace_windows_amd64.exe")
    }

    Write-Readme -Path (Join-Path $dir "README.txt") -PlatformLabel $t.Dir -HasGui $hasGui

    $zip = Join-Path $OutRoot "fridare-v$Version-$($t.Dir).zip"
    if (Test-Path $zip) { Remove-Item $zip -Force }
    Compress-Archive -Path (Join-Path $dir "*") -DestinationPath $zip -Force
    Write-Host "  -> $zip" -ForegroundColor Green
}

# Restore env
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
$env:CGO_ENABLED = "1"

Write-Host ""
Write-Host "=== Artifacts ===" -ForegroundColor Cyan
Get-ChildItem $OutRoot -File | Format-Table Name, Length
Write-Host "Done."
