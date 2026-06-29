param(
    [string]$Version = ""
)

# Verify that command tools are ready for multi-module Go releases.
$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$rootModule = "github.com/aisphereio/kernel"
$commandModules = @(
    "cmd/kernel",
    "cmd/protoc-gen-go-authz",
    "cmd/protoc-gen-go-errors",
    "cmd/protoc-gen-go-http"
)

function Fail {
    param([Parameter(Mandatory = $true)][string]$Message)
    throw $Message
}

function Read-ModulePath {
    param([Parameter(Mandatory = $true)][string]$GoMod)

    $line = Get-Content $GoMod | Where-Object { $_ -match '^module\s+' } | Select-Object -First 1
    if (-not $line) {
        Fail "missing module declaration: $GoMod"
    }
    return ($line -replace '^module\s+', '').Trim()
}

Push-Location $repoRoot
try {
    foreach ($dir in $commandModules) {
        $goMod = Join-Path $dir "go.mod"
        if (-not (Test-Path $goMod)) {
            Fail "missing command module go.mod: $goMod"
        }

        $want = "$rootModule/$dir"
        $got = Read-ModulePath $goMod
        if ($got -ne $want) {
            Fail "wrong module path in ${goMod}: want $want, got $got"
        }

        Write-Host "ok module $want" -ForegroundColor Green

        if ($Version -ne "") {
            $tag = "$dir/$Version"
            git rev-parse -q --verify "refs/tags/$tag" | Out-Null
            if ($LASTEXITCODE -ne 0) {
                Fail "missing command module tag: $tag"
            }
            Write-Host "ok tag $tag" -ForegroundColor Green
        }
    }

    if ($Version -ne "") {
        git rev-parse -q --verify "refs/tags/$Version" | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Fail "missing root module tag: $Version"
        }
        Write-Host "ok tag $Version" -ForegroundColor Green
    }
} finally {
    Pop-Location
}
