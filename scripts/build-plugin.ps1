param(
    [string]$Output = (Join-Path (Split-Path -Parent $PSScriptRoot) "RiftSyncPlugin.rbxm"),
    [string]$RojoPath = "rojo"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$stageRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("riftsync-plugin-" + [guid]::NewGuid().ToString("N"))
$sourceStage = Join-Path $stageRoot "src"
$pluginStage = Join-Path $sourceStage "RiftSyncPlugin"
$apiStage = Join-Path $pluginStage "API"

try {
    New-Item -ItemType Directory -Force -Path $apiStage | Out-Null
    Copy-Item -LiteralPath (Join-Path $Root "plugin\RiftSyncPlugin.lua") -Destination (Join-Path $pluginStage "init.server.lua")
    Copy-Item -LiteralPath (Join-Path $Root "plugin\API.lua") -Destination (Join-Path $apiStage "init.lua")
    Copy-Item -LiteralPath (Join-Path $Root "plugin\TypeList.lua") -Destination (Join-Path $apiStage "TypeList.lua")

    $projectPath = Join-Path $stageRoot "plugin.project.json"
    @{
        name = "RiftSyncPlugin"
        tree = @{
            '$path' = "src/RiftSyncPlugin"
        }
    } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $projectPath -Encoding utf8

    $outputDirectory = Split-Path -Parent ([System.IO.Path]::GetFullPath($Output))
    New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
    & $RojoPath build $projectPath -o $Output
    if ($LASTEXITCODE -ne 0) {
        throw "Rojo build failed with exit code $LASTEXITCODE"
    }
    Write-Host "Built Studio plugin: $Output"
}
finally {
    $resolvedTemp = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
    $resolvedStage = [System.IO.Path]::GetFullPath($stageRoot)
    if ($resolvedStage.StartsWith($resolvedTemp, [System.StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedStage)) {
        Remove-Item -LiteralPath $resolvedStage -Recurse -Force
    }
}
