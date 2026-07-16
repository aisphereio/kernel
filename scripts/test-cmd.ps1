# Run command package tests on Windows.
$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $RepoRoot

$dirs = @(
    "cmd/kernel",
    "cmd/protoc-gen-go-http",
    "cmd/protoc-gen-go-errors",
    "cmd/protoc-gen-go-authz",
    "cmd/protoc-gen-go-gateway",
    "cmd/protoc-gen-go-deploy",
    "cmd/protoc-gen-go-kernel",
    "cmd/buf-check-aisphere",
    "cmd/openapi-contract"
)

foreach ($dir in $dirs) {
    if (-not (Test-Path $dir)) {
        Write-Host "skip ${dir}: not found" -ForegroundColor Yellow
        continue
    }
    Write-Host "==> test $dir" -ForegroundColor Cyan
    Push-Location $dir
    try {
        go test ./...
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
    } finally {
        Pop-Location
    }
}
