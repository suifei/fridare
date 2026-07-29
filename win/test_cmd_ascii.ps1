# Fail if any .cmd/.bat under the repo contains non-ASCII bytes.
# cmd.exe is not UTF-8-safe; scripts must stay English/ASCII-only.
$ErrorActionPreference = "Stop"
$root = Split-Path (Split-Path $PSScriptRoot -Parent) -ErrorAction SilentlyContinue
if (-not $root) { $root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path }
# Script lives in win/; repo root is parent
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

$files = Get-ChildItem -Path $root -Recurse -Include *.cmd,*.bat -File |
    Where-Object { $_.FullName -notmatch '\\\.git\\' }

$failed = $false
foreach ($f in $files) {
    $bytes = [System.IO.File]::ReadAllBytes($f.FullName)
    $bad = @()
    for ($i = 0; $i -lt $bytes.Length; $i++) {
        if ($bytes[$i] -gt 127) {
            $bad += $i
            if ($bad.Count -ge 5) { break }
        }
    }
    if ($bad.Count -gt 0) {
        $failed = $true
        $rel = $f.FullName.Substring($root.Length).TrimStart('\')
        Write-Host "FAIL non-ASCII in $rel (first offsets: $($bad -join ', '))"
    } else {
        $rel = $f.FullName.Substring($root.Length).TrimStart('\')
        Write-Host "OK  $rel"
    }
}

if ($failed) {
    Write-Host "test_cmd_ascii: FAILED — keep .cmd/.bat English/ASCII only"
    exit 1
}
Write-Host "test_cmd_ascii: OK ($($files.Count) files)"
exit 0
