param(
    [string]$Version = ""
)

# Verify that command tools are released from the root Go module.
$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$rootModule = "github.com/aisphereio/kernel"
$commandPackages = @(
    "cmd/kernel",
    "cmd/protoc-gen-go-authz",
    "cmd/protoc-gen-go-errors",
    "cmd/protoc-gen-go-http"
)

function Fail {
    param([Parameter(Mandatory = $true)][string]$Message)
    throw $Message
}

Push-Location $repoRoot
try {
    $rootGoMod = Join-Path $repoRoot "go.mod"
    $rootModuleLine = Get-Content $rootGoMod | Where-Object { $_ -match '^module\s+' } | Select-Object -First 1
    $gotRootModule = ($rootModuleLine -replace '^module\s+', '').Trim()
    if ($gotRootModule -ne $rootModule) {
        Fail "wrong root module path: want $rootModule, got $gotRootModule"
    }
    Write-Host "ok root module $rootModule" -ForegroundColor Green

    foreach ($dir in $commandPackages) {
        if (-not (Test-Path $dir)) {
            Fail "missing command package directory: $dir"
        }

        $goMod = Join-Path $dir "go.mod"
        if (Test-Path $goMod) {
            Fail "command package must stay in root module; remove nested go.mod: $goMod"
        }

        Write-Host "ok root-owned package $rootModule/$dir" -ForegroundColor Green
    }

    if ($Version -ne "") {
        git diff --quiet
        if ($LASTEXITCODE -ne 0) {
            Fail "worktree has unstaged changes; commit before checking release tag $Version"
        }
        git diff --cached --quiet
        if ($LASTEXITCODE -ne 0) {
            Fail "worktree has staged changes; commit before checking release tag $Version"
        }

        git rev-parse -q --verify "refs/tags/$Version" | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Fail "missing root module tag: $Version"
        }
        $tagCommit = git rev-list -n 1 $Version
        $headCommit = git rev-parse HEAD
        if ($tagCommit -ne $headCommit) {
            Fail "root tag $Version does not point to HEAD"
        }
        Write-Host "ok root tag $Version" -ForegroundColor Green
    }
} finally {
    Pop-Location
}
