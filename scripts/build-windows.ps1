param(
    [string]$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
)

$ErrorActionPreference = "Stop"

& (Join-Path $PSScriptRoot "generate-brand-assets.ps1") -Root $Root

Push-Location $Root
try {
    go build -ldflags="-H=windowsgui" -o riftsync.exe ./cmd/riftsync-app
    go build -o riftsync-server.exe ./cmd/riftsync-server
}
finally {
    Pop-Location
}

Write-Host "Built:"
Write-Host "  $(Join-Path $Root 'riftsync.exe')"
Write-Host "  $(Join-Path $Root 'riftsync-server.exe')"
