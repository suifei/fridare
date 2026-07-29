# Validate repo *.cmd / *.bat for Windows cmd.exe:
#   1) ASCII only (bytes 0-127)  -- UTF-8 Chinese breaks under GBK code page
#   2) CRLF line endings         -- LF-only can break batch parsing
#   3) No UTF-8 BOM
$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

$files = Get-ChildItem -Path $root -Recurse -Include *.cmd,*.bat -File |
    Where-Object { $_.FullName -notmatch '\\\.git\\' }

if (-not $files) {
    Write-Host "test_cmd_ascii: no .cmd/.bat found"
    exit 0
}

$failed = $false
foreach ($f in $files) {
    $bytes = [System.IO.File]::ReadAllBytes($f.FullName)
    $rel = $f.FullName.Substring($root.Length).TrimStart('\')
    $issues = @()

    if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
        $issues += "UTF-8 BOM present"
    }

    $nonAscii = 0
    $crlf = 0
    $lfOnly = 0
    for ($i = 0; $i -lt $bytes.Length; $i++) {
        if ($bytes[$i] -gt 127) { $nonAscii++ }
        if ($bytes[$i] -eq 13 -and ($i + 1) -lt $bytes.Length -and $bytes[$i + 1] -eq 10) {
            $crlf++
            $i++
        } elseif ($bytes[$i] -eq 10) {
            $lfOnly++
        }
    }

    if ($nonAscii -gt 0) {
        $issues += "non-ASCII bytes=$nonAscii (use English ASCII only)"
    }
    if ($lfOnly -gt 0) {
        $issues += "LF-only newlines=$lfOnly (need CRLF)"
    }
    if ($crlf -eq 0 -and $bytes.Length -gt 0) {
        $issues += "no CRLF line endings found"
    }

    if ($issues.Count -gt 0) {
        $failed = $true
        Write-Host "FAIL $rel : $($issues -join '; ')"
    } else {
        Write-Host "OK   $rel (CRLF=$crlf, ASCII)"
    }
}

if ($failed) {
    Write-Host "test_cmd_ascii: FAILED"
    Write-Host "Rule: .cmd/.bat must be English ASCII + CRLF (see win/README.md and README.md)."
    exit 1
}
Write-Host "test_cmd_ascii: OK ($($files.Count) files)"
exit 0
