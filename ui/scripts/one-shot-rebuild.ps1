# One-shot: Docker bootstrap → clone → mod → compile → catalog StageDone
# Drives shipped e2e-rebuild / orchestrator only (no parallel pipeline).
#
# Usage (from repo root or ui/):
#   .\ui\scripts\one-shot-rebuild.ps1
#   .\ui\scripts\one-shot-rebuild.ps1 -Work D:\works\fridare-rebuild-e2e -Version 17.16.4 -Magic abcde
#   .\ui\scripts\one-shot-rebuild.ps1 -DryRun   # stage wiring only
#   .\ui\scripts\one-shot-rebuild.ps1 -Agent    # use GrokAgent if grok on PATH + keys
#
# Success: process exit 0 and log contains "E2E OK" / stage=done
# Failure: non-zero exit with stage + error

param(
    [string]$Version = "17.16.4",
    [string]$Target = "android-arm64",
    [string]$Magic = "abcde",
    [int]$Port = 27142,
    [string]$Work = "",
    [string]$Proxy = $(if ($env:FRIDARE_PROXY) { $env:FRIDARE_PROXY } elseif ($env:HTTPS_PROXY) { $env:HTTPS_PROXY } elseif ($env:HTTP_PROXY) { $env:HTTP_PROXY } else { "http://localhost:8080" }),
    [string]$Mirror = $(if ($env:FRIDARE_DOCKER_MIRROR) { $env:FRIDARE_DOCKER_MIRROR } else { "docker.1ms.run" }),
    [string]$Image = "fridare/frida-builder:latest",
    [string]$Timeout = "6h",
    [string]$Profile = "safe",  # safe | explore (explore only affects Agent goals / directions)
    [switch]$DryRun,
    [switch]$Agent,             # UseExistingGrok when set
    [switch]$Help
)

$ErrorActionPreference = "Stop"
if ($Help) {
    Write-Host @"
Fridare one-shot rebuild (orchestrator → StageDone)

  -Version   Frida git tag (default 17.16.4)
  -Target    build target (default android-arm64)
  -Magic     5-letter magic (default abcde)
  -Work      work dir (default ~/.fridare/rebuild-e2e)
  -Proxy     container git proxy (localhost rewritten to host.docker.internal)
  -Mirror    docker hub mirror (default docker.1ms.run)
  -Profile   strip directions: safe | explore
  -DryRun    no real docker compile
  -Agent     enable GrokAgent refine when grok on PATH

Requires: Docker Engine/Desktop, disk space, optional HTTP proxy.
"@
    exit 0
}

# ui/scripts/one-shot-rebuild.ps1 → ui module root
$UiRoot = Split-Path $PSScriptRoot -Parent
Set-Location $UiRoot
if (-not (Test-Path "go.mod")) {
    Write-Error "go.mod not found in $UiRoot (expected ui module)"
}

$env:CGO_ENABLED = "0"
Write-Host "=== Fridare one-shot rebuild ===" -ForegroundColor Cyan
Write-Host "ui=$UiRoot version=$Version target=$Target magic=$Magic profile=$Profile"

$args = @(
    "run", "./cmd/e2e-rebuild",
    "-version", $Version,
    "-target", $Target,
    "-magic", $Magic,
    "-port", "$Port",
    "-proxy", $Proxy,
    "-mirror", $Mirror,
    "-image", $Image,
    "-timeout", $Timeout,
    "-profile", $Profile
)
if ($Work) { $args += @("-work", $Work) }
if ($DryRun) { $args += "-dry-run" }
if ($Agent) { $args += "-agent" }

Write-Host "go $($args -join ' ')" -ForegroundColor DarkGray
& go @args
exit $LASTEXITCODE
