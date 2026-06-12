param(
    [string]$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path,
    [string]$SourcePng = "",
    [switch]$SkipSyso
)

$ErrorActionPreference = "Stop"

$brandDir = Join-Path $Root "assets\brand"
$defaultSource = Join-Path $brandDir "riftsync-logo.png"
$appSyso = Join-Path $Root "cmd\riftsync-app\rsrc_windows_amd64.syso"
$serverSyso = Join-Path $Root "cmd\riftsync-server\rsrc_windows_amd64.syso"
$pngPath = Join-Path $brandDir "riftsync-icon-512.png"
$icoPath = Join-Path $brandDir "riftsync.ico"

if ([string]::IsNullOrWhiteSpace($SourcePng)) {
    $SourcePng = $defaultSource
}

if (-not [System.IO.Path]::IsPathRooted($SourcePng)) {
    $SourcePng = Join-Path $Root $SourcePng
}

if (-not (Test-Path -LiteralPath $SourcePng)) {
    throw "Icon source not found: $SourcePng"
}

New-Item -ItemType Directory -Force -Path $brandDir | Out-Null
Add-Type -AssemblyName System.Drawing

function New-IconBitmapFromSource {
    param(
        [System.Drawing.Image]$SourceImage,
        [int]$Size
    )

    $bitmap = [System.Drawing.Bitmap]::new($Size, $Size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
    $graphics.Clear([System.Drawing.Color]::Transparent)

    $sourceWidth = [double]$SourceImage.Width
    $sourceHeight = [double]$SourceImage.Height
    $scale = [Math]::Min($Size / $sourceWidth, $Size / $sourceHeight)
    $targetWidth = [int][Math]::Round($sourceWidth * $scale)
    $targetHeight = [int][Math]::Round($sourceHeight * $scale)
    $left = [int][Math]::Round(($Size - $targetWidth) / 2)
    $top = [int][Math]::Round(($Size - $targetHeight) / 2)

    $targetRect = [System.Drawing.Rectangle]::new($left, $top, $targetWidth, $targetHeight)
    $graphics.DrawImage($SourceImage, $targetRect)
    $graphics.Dispose()
    return $bitmap
}

$sourceStream = [System.IO.File]::OpenRead($SourcePng)
$sourceImage = $null
$bitmap512 = $null
try {
    $sourceImage = [System.Drawing.Image]::FromStream($sourceStream, $true, $true)

    $bitmap512 = New-IconBitmapFromSource $sourceImage 512
    $bitmap512.Save($pngPath, [System.Drawing.Imaging.ImageFormat]::Png)
}
finally {
    if ($bitmap512) {
        $bitmap512.Dispose()
    }
}

$iconSizes = @(16, 24, 32, 48, 64, 128, 256)
$memoryStreams = @()
$fileStream = [System.IO.File]::Create($icoPath)
try {
    $writer = [System.IO.BinaryWriter]::new($fileStream)
    $writer.Write([UInt16]0)
    $writer.Write([UInt16]1)
    $writer.Write([UInt16]$iconSizes.Count)

    $imageBytes = @()
    foreach ($iconSize in $iconSizes) {
        $bitmap = New-IconBitmapFromSource $sourceImage $iconSize
        $stream = [System.IO.MemoryStream]::new()
        $bitmap.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
        $bitmap.Dispose()
        $bytes = $stream.ToArray()
        $memoryStreams += $stream
        $imageBytes += ,$bytes
    }

    $offset = 6 + (16 * $iconSizes.Count)
    for ($i = 0; $i -lt $iconSizes.Count; $i++) {
        $sizeByte = if ($iconSizes[$i] -eq 256) { 0 } else { $iconSizes[$i] }
        $writer.Write([byte]$sizeByte)
        $writer.Write([byte]$sizeByte)
        $writer.Write([byte]0)
        $writer.Write([byte]0)
        $writer.Write([UInt16]1)
        $writer.Write([UInt16]32)
        $writer.Write([UInt32]$imageBytes[$i].Length)
        $writer.Write([UInt32]$offset)
        $offset += $imageBytes[$i].Length
    }

    foreach ($bytes in $imageBytes) {
        $writer.Write($bytes)
    }
    $writer.Flush()
}
finally {
    foreach ($stream in $memoryStreams) {
        $stream.Dispose()
    }
    $fileStream.Dispose()
    if ($sourceImage) {
        $sourceImage.Dispose()
    }
    $sourceStream.Dispose()
}

if (-not $SkipSyso) {
    go run github.com/akavel/rsrc@latest -arch amd64 -ico $icoPath -o $appSyso
    go run github.com/akavel/rsrc@latest -arch amd64 -ico $icoPath -o $serverSyso
}

Write-Host "Generated from $SourcePng"
Write-Host "  $pngPath"
Write-Host "  $icoPath"
if (-not $SkipSyso) {
    Write-Host "  $appSyso"
    Write-Host "  $serverSyso"
}
