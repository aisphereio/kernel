param(
    [string]$Output = "kernel.zip"
)

$exclude = @(".git", ".bin", ".gitignore", ".gitattributes", $Output)

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$items = Get-ChildItem -Path $root -Exclude $exclude

Compress-Archive -Path $items.FullName -DestinationPath $Output -Force

Write-Host "Done: $Output"
