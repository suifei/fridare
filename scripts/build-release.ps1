# Multi-platform release builder for Fridare.
# Formal output root: dist/release-v{Version}/
# Tools (create/patch/hexreplace): pure Go, CGO_ENABLED=0 (fully static).
# GUI (Fyne): requires CGO + OpenGL; built for host Windows only (windows-amd64).
#
# Usage:
#   powershell -File scripts\build-release.ps1 -Version 4.0.6
#   powershell -File scripts\build-release.ps1 -Version 4.0.6 -Only windows-amd64

param(
    [string]$Version = "4.0.6",
    # Optional: build only one platform dir name, e.g. windows-amd64
    [string]$Only = ""
)

$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$OutRoot = Join-Path $Root "dist\release-v$Version"
$UiDir = Join-Path $Root "ui"
$HexDir = Join-Path $Root "hexreplace"

Write-Host "=== Fridare release build v$Version ===" -ForegroundColor Cyan
Write-Host "Formal output: $OutRoot"
if ($Only) { Write-Host "Only platform: $Only" -ForegroundColor Yellow }

# When building a single platform, only wipe that subdir + zip (keep other arches)
if (-not $Only) {
    if (Test-Path $OutRoot) { Remove-Item -Recurse -Force $OutRoot }
}
New-Item -ItemType Directory -Force -Path $OutRoot | Out-Null

$ToolTargets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Ext = ".exe"; Dir = "windows-amd64" },
    @{ GOOS = "windows"; GOARCH = "arm64"; Ext = ".exe"; Dir = "windows-arm64" },
    @{ GOOS = "linux";   GOARCH = "amd64"; Ext = "";     Dir = "linux-amd64" },
    @{ GOOS = "linux";   GOARCH = "arm64"; Ext = "";     Dir = "linux-arm64" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; Ext = "";     Dir = "darwin-amd64" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; Ext = "";     Dir = "darwin-arm64" }
)
if ($Only) {
    $ToolTargets = @($ToolTargets | Where-Object { $_.Dir -eq $Only })
    if ($ToolTargets.Count -eq 0) {
        throw "Unknown -Only platform '$Only'. Use e.g. windows-amd64"
    }
}

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
        # Windows PE: embed icon + version (syso only linked for windows/amd64 typically)
        if ($GOOS -eq "windows" -and $GOARCH -eq "amd64") {
            & powershell -NoProfile -File (Join-Path $UiDir "scripts\gen-winres.ps1") 2>$null
        }
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
        # Embed PE icon + FileVersion/ProductName via goversioninfo .syso
        Write-Host "  [winres] generate icon/version resources"
        & powershell -NoProfile -File (Join-Path $UiDir "scripts\gen-winres.ps1")
        $gui = Join-Path $OutDir "fridare-gui.exe"
        Write-Host "  [cgo] fridare-gui windows/amd64"
        & go build -trimpath -ldflags "-s -w -H windowsgui" -o $gui ./cmd/gui
        if ($LASTEXITCODE -ne 0) { throw "gui failed" }
        # Also stamp create/patch with version+icon when building windows-amd64 tools next to GUI
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

Listen port:
  Default 27042 is fine. Start server with -l IP:PORT and connect the
  client to that same port. Example:
    kxmwp-server -l 0.0.0.0:28888
    frida -H 127.0.0.1:28888 -n <process>
  Changing the baked-in default port is optional.

Source rebuild (GUI tab: source rebuild / Docker):
  Docker image feature: toolchain-v4-ndk29-node20-go124-mingw-aarch64
  (Node 20 + Go 1.24 + NDK r29 + MinGW + aarch64-linux-gnu)
  First run builds the image; later jobs reuse it.

Docs in this zip (docs/):
  dual-track.md       two tracks (static hex + Docker rebuild)
  kxmwp-17.17.0.md    prebuilt Frida 17.17.0 deep products

Prebuilt servers (not in this zip):
  https://github.com/suifei/fridare/releases/tag/kxmwp-17.17.0

Windows batch helpers (ASCII + CRLF only):
  See repository win/patch-frida.cmd and win/patch-frida-tools.cmd

Project: https://github.com/suifei/fridare
Release: https://github.com/suifei/fridare/releases/tag/v$Version
"@
    # README can be UTF-8; tools binaries are static. Use ASCII for consistency on Windows.
    $ascii = [System.Text.Encoding]::ASCII.GetBytes(($text -replace "`n", "`r`n"))
    [System.IO.File]::WriteAllBytes($Path, $ascii)
}

# --- build selected tool platforms ---
foreach ($t in $ToolTargets) {
    $dir = Join-Path $OutRoot $t.Dir
    if (Test-Path $dir) { Remove-Item -Recurse -Force $dir }
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

    $docsDir = Join-Path $dir "docs"
    New-Item -ItemType Directory -Force -Path $docsDir | Out-Null
    Copy-Item (Join-Path $Root "docs\dual-track.md") $docsDir
    Copy-Item (Join-Path $Root "docs\kxmwp-17.17.0.md") $docsDir
    Copy-Item (Join-Path $Root "CHANGELOG") (Join-Path $dir "CHANGELOG.txt")

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
