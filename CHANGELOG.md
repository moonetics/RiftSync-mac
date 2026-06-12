# Changelog

Catatan perubahan penting RiftSync, diurutkan dari rilis terbaru ke rilis lama.

## 3.9.0 - 2026-06-12

- Increased generated Remote Exec tokens from 18 to 32 random alphanumeric characters.
- Regenerated the active config Remote Exec token.

## 3.8.0 - 2026-06-12

- Added Remote Exec token auto-generation for configs that do not have a token yet.
- Enabled Remote Exec for the active config and saved the generated token to `sync_config.json`.
- Added readonly Exec Token display and copy action in the desktop app Exec tab.

## 3.7.0 - 2026-06-12

Rilis development terkini. Entry ini menggabungkan iterasi internal `3.1.0` sampai `3.7.0` agar changelog tetap enak dibaca walaupun beberapa perubahan dibuat di hari yang sama.

- Added Remote Exec file shortcut untuk `.lua`/`.luau`, termasuk direct CLI shortcut `riftsync-server.exe exec .\studio-command.lua --timeout 30`.
- Added script `properties.init.json` support untuk folder-style scripts, termasuk properti `Disabled` untuk `Script` dan `LocalScript`.
- Fixed `Pull Studio` replace semantics agar file lokal yang sudah dihapus di Studio ikut terhapus secara lokal.
- Added compact RiftSync brand icon di header desktop app.
- Added `riftsync-server validate [--json]` untuk validasi config/project non-destruktif.
- Added generated `.guidebook/status.json` untuk status mesin yang bisa dibaca manusia/AI.
- Added safe backup `.rblxsync/backups/<timestamp>/` sebelum Pull Studio replace menghapus konten lokal.
- Added Remote Exec command history di `.rblxsync/exec-history.json`.
- Added `riftsync-server exec --last` untuk rerun command Remote Exec terakhir.
- Added desktop app `Exec` tab untuk paste/run Luau, melihat output, dan rerun recent commands.
- Added non-mutating Pull Studio preview dengan add/update/delete/unchanged counts.
- Added inline Confirm/Cancel flow di Studio plugin sebelum Pull Studio replace berjalan.
- Improved Studio plugin health checklist untuk HTTP/server reachability, connection, token, sync, exec, dan edit mode.

## 3.0.0 - 2026-05

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
