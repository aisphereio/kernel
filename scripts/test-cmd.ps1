# Run command submodule tests on Windows.
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

foreach ($dir in @("cmd/kernel", "cmd/protoc-gen-go-http", "cmd/protoc-gen-go-errors", "cmd/protoc-gen-go-authz")) {
    if (Test-Path $dir) {
        Invoke-Step "go test ./$dir" {
            Push-Location $dir
            try { go test ./... } finally { Pop-Location }
        }
    } else {
        Write-Host "skip ${dir}: not found" -ForegroundColor Yellow
    }
}
