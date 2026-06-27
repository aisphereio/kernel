# Aisphere Kernel tools installer for Windows.
#
# Recommended entrypoints:
#   .\scripts\tools.cmd
#   make tools
#
# This script installs local development tools into .\.bin without changing
# machine-wide PowerShell execution policy or global Go settings.

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

function Install-LocalCommand {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Path
    )

    if (Test-Path $Path) {
        Invoke-Step "install $Name" {
            Push-Location $Path
            try {
                go install .
            } finally {
                Pop-Location
            }
        }
    } else {
        Write-Host "skip ${Name}: $Path not found" -ForegroundColor Yellow
    }
}

function Install-GoTool {
    param(
        [Parameter(Mandatory = $true)][string]$Package
    )

    Invoke-Step "install $Package" {
        go install $Package
    }
}

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $RepoRoot

$LocalBin = Join-Path $RepoRoot ".bin"
New-Item -ItemType Directory -Force -Path $LocalBin | Out-Null

$oldGobIn = $env:GOBIN
$oldPath = $env:PATH

try {
    $env:GOBIN = $LocalBin
    $env:PATH = "$LocalBin;$env:PATH"

    Write-Host "installing local Kernel tools into $LocalBin" -ForegroundColor Green

    Install-LocalCommand -Name "kernel" -Path (Join-Path $RepoRoot "cmd/kernel")
    Install-LocalCommand -Name "protoc-gen-go-http" -Path (Join-Path $RepoRoot "cmd/protoc-gen-go-http")
    Install-LocalCommand -Name "protoc-gen-go-errors" -Path (Join-Path $RepoRoot "cmd/protoc-gen-go-errors")

    Install-GoTool "google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11"
    Install-GoTool "google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1"

    $bufExe = Join-Path $LocalBin "buf.exe"
    $bufOnPath = Get-Command buf -ErrorAction SilentlyContinue
    if ((Test-Path $bufExe) -or $bufOnPath) {
        Write-Host "skip buf: already available" -ForegroundColor Yellow
    } else {
        Install-GoTool "github.com/bufbuild/buf/cmd/buf@v1.50.0"
    }

    Write-Host "tools installation passed." -ForegroundColor Green
} finally {
    $env:GOBIN = $oldGobIn
    $env:PATH = $oldPath
}
