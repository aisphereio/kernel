# Aisphere Kernel errorx verification script.
#
# Direct execution may be blocked by Windows PowerShell execution policy when
# this file is downloaded from the internet or extracted from a zip archive.
# Recommended Windows entrypoint:
#   .\scripts\verify-errorx.cmd
#
# Alternative one-off execution:
#   powershell -NoLogo -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify-errorx.ps1
#
# This script itself does not change machine/user execution policy.

$ErrorActionPreference = "Stop"

function Invoke-Step {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][scriptblock]$Command
    )

    Write-Host "==> $Name" -ForegroundColor Cyan
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "Step failed: $Name, exit code: $LASTEXITCODE"
    }
}

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $RepoRoot

$legacyImports = Get-ChildItem -Recurse -Filter *.go | Select-String -SimpleMatch "github.com/aisphereio/kernel/errors"
if ($legacyImports) {
    $legacyImports | ForEach-Object { Write-Host $_.Path":"$_.LineNumber":"$_.Line -ForegroundColor Red }
    throw "legacy kernel/errors import found"
}

$legacyRootErrors = Join-Path $RepoRoot "errors"
if (Test-Path $legacyRootErrors) {
    throw "legacy root errors package still exists: errors"
}

$protoGenerator = Join-Path $RepoRoot "cmd/protoc-gen-go-errors"
if (-not (Test-Path $protoGenerator)) {
    throw "proto error generator is missing; it should be retained and generate errorx helpers"
}

$protoExtension = Join-Path $RepoRoot "third_party/errors/errors.proto"
if (-not (Test-Path $protoExtension)) {
    throw "third_party/errors/errors.proto is missing; proto enum option compatibility is required"
}

$legacyGeneratorRefs = foreach ($root in @("cmd/protoc-gen-go-errors", "third_party/errors")) {
    if (Test-Path $root) {
        Get-ChildItem $root -Recurse -Include *.go,*.proto -File | Select-String -SimpleMatch "kernel/errors"
    }
}
if ($legacyGeneratorRefs) {
    $legacyGeneratorRefs | ForEach-Object { Write-Host $_.Path":"$_.LineNumber":"$_.Line -ForegroundColor Red }
    throw "proto errorx generator still references legacy kernel/errors"
}

$errorxGeneratorRefs = Get-ChildItem "cmd/protoc-gen-go-errors" -Recurse -File | Select-String -SimpleMatch "github.com/aisphereio/kernel/errorx"
if (-not $errorxGeneratorRefs) {
    throw "protoc-gen-go-errors must generate errorx helpers"
}

Invoke-Step "go test ./errorx -v" { go test ./errorx -v }
Invoke-Step "go test ./errorx -race" {
    if (Get-Command gcc -ErrorAction SilentlyContinue) {
        go test ./errorx -race
    } else {
        Write-Host "skip -race: gcc not found (race detector requires CGO + GCC)" -ForegroundColor Yellow
    }
}
Invoke-Step "go test ./errorx -cover" { go test ./errorx -cover }
Invoke-Step "go test cmd/protoc-gen-go-errors" { Push-Location cmd/protoc-gen-go-errors; try { go test ./... } finally { Pop-Location } }
Invoke-Step "go test ./..." { go test ./... }
Invoke-Step "go vet ./..." { go vet ./... }
Invoke-Step "go run ./examples/errorx-basic" { go run ./examples/errorx-basic }
Invoke-Step "go test ./errorx -bench=." { go test ./errorx -bench=. }

Write-Host "errorx verification passed." -ForegroundColor Green
