#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PROJECT_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
OUTPUT=${1:-"$PROJECT_ROOT/RiftSyncPlugin.rbxm"}

if ! command -v rojo >/dev/null 2>&1; then
    echo "Rojo was not found; refusing to keep or reuse a stale RiftSyncPlugin.rbxm." >&2
    echo "Install Rojo and retry: https://rojo.space/docs/installation/" >&2
    exit 1
fi

BUILD_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/riftsync-plugin.XXXXXX")
cleanup() { rm -rf "$BUILD_TEMP"; }
trap cleanup EXIT INT TERM

mkdir -p "$BUILD_TEMP/src/RiftSyncPlugin/API"
cp "$PROJECT_ROOT/plugin/RiftSyncPlugin.lua" "$BUILD_TEMP/src/RiftSyncPlugin/init.server.lua"
cp "$PROJECT_ROOT/plugin/API.lua" "$BUILD_TEMP/src/RiftSyncPlugin/API/init.lua"
cp "$PROJECT_ROOT/plugin/TypeList.lua" "$BUILD_TEMP/src/RiftSyncPlugin/API/TypeList.lua"
cp "$PROJECT_ROOT/plugin/Identity.lua" "$BUILD_TEMP/src/RiftSyncPlugin/API/Identity.lua"
cat > "$BUILD_TEMP/plugin.project.json" <<'EOF'
{
  "name": "RiftSyncPlugin",
  "tree": { "$path": "src/RiftSyncPlugin" }
}
EOF

mkdir -p "$(dirname "$OUTPUT")"
rojo build "$BUILD_TEMP/plugin.project.json" -o "$PROJECT_ROOT/RiftSyncPlugin.rbxm"
rojo build "$BUILD_TEMP/plugin.project.json" -o "$PROJECT_ROOT/RiftSyncPlugin.rbxmx"
echo "Built Studio plugin: $PROJECT_ROOT/RiftSyncPlugin.rbxm"
echo "Built Studio plugin: $PROJECT_ROOT/RiftSyncPlugin.rbxmx"

ROBLOX_PLUGINS_DIR="${HOME}/Documents/Roblox/Plugins"
if [ -d "$ROBLOX_PLUGINS_DIR" ]; then
    if [ -f "$ROBLOX_PLUGINS_DIR/RiftSyncPlugin.rbxmx" ]; then
        cp "$ROBLOX_PLUGINS_DIR/RiftSyncPlugin.rbxmx" "$ROBLOX_PLUGINS_DIR/RiftSyncPlugin.rbxmx.previous"
    fi
    cp "$PROJECT_ROOT/RiftSyncPlugin.rbxmx" "$ROBLOX_PLUGINS_DIR/RiftSyncPlugin.rbxmx"
    echo "Installed Studio plugin to: $ROBLOX_PLUGINS_DIR/RiftSyncPlugin.rbxmx"
fi

