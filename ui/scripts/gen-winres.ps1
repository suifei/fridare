# Generate Windows PE version info + icon (.syso) for GUI / create / patch.
# Requires: go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
# Usage: powershell -File ui/scripts/gen-winres.ps1

$ErrorActionPreference = "Stop"
$UiRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$LogoIco = Join-Path $UiRoot "..\logo\AppIcon.ico"
if (-not (Test-Path $LogoIco)) {
    throw "Icon not found: $LogoIco"
}

$gv = Get-Command goversioninfo -ErrorAction SilentlyContinue
if (-not $gv) {
    Write-Host "Installing goversioninfo..." -ForegroundColor Yellow
    go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
    $gv = Get-Command goversioninfo -ErrorAction SilentlyContinue
    if (-not $gv) {
        # GOPATH/bin may not be on PATH
        $gopath = (go env GOPATH).Trim()
        $cand = Join-Path $gopath "bin\goversioninfo.exe"
        if (Test-Path $cand) {
            $gvPath = $cand
        } else {
            throw "goversioninfo not found after install"
        }
    } else {
        $gvPath = $gv.Source
    }
} else {
    $gvPath = $gv.Source
}

function Invoke-WinRes {
    param($CmdDir)
    Push-Location $CmdDir
    try {
        Write-Host "  gen winres: $CmdDir" -ForegroundColor Cyan
        # -64 for amd64 PE; produces resource.syso linked automatically by go build
        & $gvPath -64 -o resource_windows_amd64.syso -icon (Resolve-Path $LogoIco).Path
        if ($LASTEXITCODE -ne 0) { throw "goversioninfo failed in $CmdDir" }
        if (-not (Test-Path "resource_windows_amd64.syso")) {
            # older goversioninfo may write resource.syso
            if (Test-Path "resource.syso") {
                Move-Item -Force resource.syso resource_windows_amd64.syso
            } else {
                Get-ChildItem *.syso | Format-Table Name
                throw "no .syso produced in $CmdDir"
            }
        }
    } finally {
        Pop-Location
    }
}

foreach ($d in @("gui", "create", "patch")) {
    $dir = Join-Path $UiRoot "cmd\$d"
    if (-not (Test-Path (Join-Path $dir "versioninfo.json"))) {
        throw "missing versioninfo.json in $dir"
    }
    Invoke-WinRes $dir
}

Write-Host "Done. Windows builds will embed icon + FileVersion from versioninfo.json" -ForegroundColor Green
