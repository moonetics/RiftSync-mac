# Changelog

Catatan perubahan penting RiftSync, diurutkan dari rilis terbaru ke rilis lama.

## 4.1.0 - 2026-08-01

- Expanded `Studio -> Folder` metadata snapshots to scripts, ValueBase, Workspace/Terrain/Lighting effects, UI, audio, animation, visual effects, prompts, tools, attachments, constraints, and other curated non-geometry classes.
- Added full managed-root descendant traversal while keeping `BasePart` and `Model` as path-only anchors that never receive metadata or automatic ownership.
- Added canonical `Enabled` script metadata with backward-compatible `Disabled` import.
- Added typed `InstanceRef` and `Ray` serialization, stable-ID-first/path-fallback resolution, explicit nil references, and two-phase property apply for forward and cyclic references.
- Added deterministic JSON formatting for pulled metadata to prevent random key-order Git diffs.
- Protected root services, Terrain, missing geometry ancestors, script source ownership, and unmanaged instances during metadata create/delete operations.

## 4.0.0 - 2026-07-31

- Added concurrent multi-instance desktop management with an AppData registry, isolated configs, ports, runtime state, Remote Exec queues, and histories.
- Added automatic editable port allocation, Start All, safe first-run import, and non-destructive project removal.
- Rebuilt the desktop app with subtle dark glass surfaces, consistent flat controls, a hover-expandable and pinnable project sidebar, progressive disclosure, and keyboard-accessible tabs/modals.
- Fixed long Remote Exec history rows so they remain bounded, and added a read-only View action with full source and stored result details.
- Rebuilt the Roblox Studio widget with flat frosted surfaces, automatic scrolling/layout, collapsible profile/diagnostic controls, and named connection profiles remembered per Place.
- Changed server startup to validate the listening port synchronously before reporting a running state.

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
- Added script `properties.init.json` support untuk folder-style scripts, termasuk properti legacy `Disabled` untuk `Script` dan `LocalScript`.
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
