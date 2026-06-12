# Changelog

Semua perubahan fitur, fix, penghapusan fitur, atau perubahan perilaku harus menaikkan versi dan dicatat di file ini.

## 3.3.0 - 2026-06-12

- Fixed `Pull Studio` to use replace semantics so local files deleted in Studio are removed locally.
- Kept `.git`, `.rblxsync`, and `.guidebook` preserved during Pull Studio replacement.
- Pull Studio status/debug summaries now include deleted file counts.
- Bumped RiftSync server and Studio plugin version from `3.2.0` to `3.3.0`.

## 3.2.0 - 2026-06-12

- Added script `properties.init.json` support for folder-style scripts.
- Script source records now carry optional `payload.properties`, so changing `Disabled` on `Script`/`LocalScript` produces a sync delta.
- Studio apply now writes script properties/attributes/tags after updating script source.
- Updated README and generated guidebook template with script properties examples.
- Bumped RiftSync server and Studio plugin version from `3.1.0` to `3.2.0`.

## 3.1.0 - 2026-06-12

- Added Remote Exec file shortcut: `.guidebook/riftsync-exec.ps1 .\studio-command.lua --timeout 30` sekarang otomatis membaca `.lua`/`.luau` sebagai source file.
- Added direct CLI shortcut: `riftsync-server.exe exec .\studio-command.lua --timeout 30` setara dengan `--file`.
- Updated generated `.guidebook/README.md` with file-based Remote Exec examples.
- Bumped RiftSync server and Studio plugin version from `3.0.0` to `3.1.0`.

## 3.0.0 - 2026-06-12

- Major refactor and migration from the older Python-based local server to a Go-based RiftSync app/server.
- Added `riftsync.exe` native app launcher and `riftsync-server.exe` console/headless server workflow.
- Added hybrid file watching, local revision state, history endpoints, and Git versioning support in the Go server.
- Kept the Studio plugin workflow while moving the backend/runtime foundation to Go for easier distribution and maintenance.

## 2.x - Historical

- Upgraded the original sync workflow with broader Roblox instance/property support.
- Added folder-per-instance metadata patterns such as `properties.init.json`.
- Added support for UI trees, remote/container instances, Studio-to-folder bootstrap/pull flows, and stronger sync safety checks.
- Improved Studio plugin UX, debug/status reporting, and compatibility handling before the Go migration.

## 1.x - Historical

- Initial RiftSync implementation using a Python local server.
- Established the basic Roblox Studio plugin plus local folder sync loop.
- Focused on early one-way script sync from local files into Roblox Studio.
