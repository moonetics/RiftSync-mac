#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PROJECT_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
APP_BUNDLE="$PROJECT_ROOT/dist/RiftSync.app"
BUILD_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/riftsync-macos-build.XXXXXX")
ICONSET="$BUILD_TEMP/RiftSync.iconset"

if command -v go >/dev/null 2>&1; then
    GO_CMD=$(command -v go)
elif [ -x "${HOME}/.local/go/bin/go" ]; then
    GO_CMD="${HOME}/.local/go/bin/go"
else
    echo "Go was not found. Install Go or add it to PATH." >&2
    exit 1
fi

cleanup() {
    rm -rf "$BUILD_TEMP"
}
trap cleanup EXIT INT TERM

mkdir -p "$ICONSET" "$APP_BUNDLE/Contents/MacOS" "$APP_BUNDLE/Contents/Resources"

cd "$PROJECT_ROOT"
CGO_ENABLED=1 "$GO_CMD" build -trimpath -o riftsync-app-macos-arm64 ./cmd/riftsync-app
CGO_ENABLED=0 "$GO_CMD" build -trimpath -o riftsync-server-macos-arm64 ./cmd/riftsync-server
cp riftsync-app-macos-arm64 "$APP_BUNDLE/Contents/MacOS/RiftSync"
cp scripts/macos/Info.plist "$APP_BUNDLE/Contents/Info.plist"

sips -z 16 16 assets/brand/riftsync-icon-512.png --out "$ICONSET/icon_16x16.png" >/dev/null
sips -z 32 32 assets/brand/riftsync-icon-512.png --out "$ICONSET/icon_16x16@2x.png" >/dev/null
sips -z 32 32 assets/brand/riftsync-icon-512.png --out "$ICONSET/icon_32x32.png" >/dev/null
sips -z 64 64 assets/brand/riftsync-icon-512.png --out "$ICONSET/icon_32x32@2x.png" >/dev/null
sips -z 128 128 assets/brand/riftsync-icon-512.png --out "$ICONSET/icon_128x128.png" >/dev/null
sips -z 256 256 assets/brand/riftsync-icon-512.png --out "$ICONSET/icon_128x128@2x.png" >/dev/null
sips -z 256 256 assets/brand/riftsync-icon-512.png --out "$ICONSET/icon_256x256.png" >/dev/null
sips -z 512 512 assets/brand/riftsync-icon-512.png --out "$ICONSET/icon_256x256@2x.png" >/dev/null
cp assets/brand/riftsync-icon-512.png "$ICONSET/icon_512x512.png"
sips -z 1024 1024 assets/brand/riftsync-icon-512.png --out "$ICONSET/icon_512x512@2x.png" >/dev/null
iconutil -c icns "$ICONSET" -o "$APP_BUNDLE/Contents/Resources/RiftSync.icns"

plutil -lint "$APP_BUNDLE/Contents/Info.plist" >/dev/null
codesign --force --deep --sign - "$APP_BUNDLE"

echo "Built:"
echo "  $APP_BUNDLE"
echo "  $PROJECT_ROOT/riftsync-server-macos-arm64"
