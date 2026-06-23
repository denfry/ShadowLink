# Build the Go core as a c-shared DLL and stage the native DLLs for Flutter.
# Prereqs: Go 1.24+, mingw-w64 (x86_64) on PATH (CC=x86_64-w64-mingw32-gcc).
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$native = Join-Path $root "app\windows\native"
New-Item -ItemType Directory -Force $native | Out-Null

$env:CGO_ENABLED = "1"
$env:CC = "x86_64-w64-mingw32-gcc"
go build -tags "libshadowlink with_utls with_gvisor with_clash_api" `
  -buildmode=c-shared `
  -o (Join-Path $native "shadowlink_core.dll") `
  ./cmd/libshadowlink
Copy-Item (Join-Path $root "bin\wintun.dll") $native -Force
Write-Host "Built shadowlink_core.dll + staged wintun.dll into $native"
