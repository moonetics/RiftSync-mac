# Changelog

Catatan perubahan penting RiftSync, diurutkan dari rilis terbaru ke rilis lama.

## 4.2.05 - 2026-09-19

- Added interactive "Auto-Rename Sibling Duplicates" button in Studio plugin widget to resolve conflicting sibling names with one click.
- Uses standard copy naming convention (`Name (Copy)`, `Name (Copy 2)`, etc.) with full Studio Undo (`Ctrl+Z`) support via `ChangeHistoryService`.
- Time-sliced renaming with 20ms frame yields to prevent Studio freezing on large DataModels.
- Bumped app and plugin version to `4.2.05`.

## 4.2.04 - 2026-09-19

- Standardized minor/patch release numbering schema to 2-digit format (`x.x.xx`).
- Fixed `ambiguous_target` error when applying local snapshot where unmanaged Studio instances had identical names in different branches.
- Rebuilt native macOS app (`RiftSync.app`), CLI server, and Roblox Studio plugin.

## 4.2.3 - 2026-09-19

- Added interactive "Clean Duplicate IDs" feature in plugin to detect and clear duplicate/conflicting StableIds with one click without affecting other instances.
- Optimized `clearStudioStableIds` with frame yielding (time-slicing every 20ms) and memory reduction to prevent Roblox Studio freezes on large places (50,000+ instances).
- Bumped app and plugin version to `4.2.3`.

## 4.2.2 - 2026-09-18

- Excluded `SoundService` by default alongside `Workspace` in the Explorer service filter dropdown for a cleaner, focused default tree view.
- Removed redundant batch enablement buttons and stray separator characters from the VM Protection panel.
- Bumped app and plugin version to `4.2.2`.

## 4.2.1 - 2026-09-18

- Added Service Filter multi-select dropdown beside search bar with bulk toggle (`All`/`None`), excluding massive `Workspace` by default for instant tree rendering.
- Fixed dark fallback icons for Roblox root services and post-processing effects with authentic, vibrant vector SVG icons (`SoundService`, `ServerScriptService`, `ReplicatedStorage`, `ServerStorage`, `StarterGui`, `StarterPack`, `Bloom`, `ColorCorrection`, etc.).
- Fixed `SoundService` tree row alignment and removed confusing `Off` badge on non-script UI instances.
- Streamlined Security & Batch action panel layout with compact button heights and spacing to eliminate vertical clipping.
- Added visible custom WebKit scrollbars on Explorer tree and Inspector sub-panels.

## 4.2.0 - 2026-09-18

- Added Sorevium VM Obfuscator integration inside RiftSync GUI Webview with dedicated **Protection** tab.
- Integrated full Roblox Studio Explorer replica tree with official Roblox service and script icons, collapse/expand carets, and real-time search filter.
- In-place script protection using Local Vault (`.rblxsync/vault/`): active game files are substituted with zero-lag Sorevium VM bytecode without breaking `require()` paths or game hierarchies.
- 1-click single-script and batch-folder protection and restoration.
- Single standalone binary compilation with embedded Luau AST CGO bridge and VM generator.

## 4.1.11 - 2026-09-17

- Optimized idle CPU and I/O performance by separating event-driven `fsnotify` watcher from the legacy periodic scan loop.
- Introduced `safety_scan_interval_sec` (default 30 seconds) for background reconciliation, replacing the relentless 400ms project-wide file traversal loop.
- Properly respected `--legacy-scan` flag so periodic scanning only runs when explicitly requested, eliminating duplicate scanning during watcher mode.
- Synchronous file changes remain instantaneous (< 0.2s) via OS kernel `FSEvents` while idle CPU drops to near 0%.

## 4.1.10 - 2026-09-11

- Completed generic unmanaged exact-target adoption for StableId-suffixed local paths. UI and script apply now reuse one uniquely matching same-class unmanaged object, while ambiguous candidates, conflicting IDs, destructive operations, and unsafe class replacement remain blocked.

## 4.1.9 - 2026-09-11

- Fixed snapshot preflight rejecting an existing unmanaged Studio object even though the apply path could safely reuse it. Exact-path, same-class upserts with compatible StableIds are now adopted generically; destructive operations, class mismatches, ambiguous siblings, and conflicting IDs remain blocked.

## 4.1.8 - 2026-09-11

- Added a one-shot `Force ID repair` option to the Start source chooser. It clears only active RiftSync identities before the selected Studio or Local bootstrap, excludes ignored and rollback subtrees, records Studio mutations for Undo, and restores the previous IDs if bootstrap fails.

## 4.1.7 - 2026-09-11

- Replaced vendor-specific rollback exclusions with a general, case-insensitive `ServerStorage.__*Rollback*` subtree rule, covering rollback snapshots from any Studio tool while avoiding ordinary project folders.

## 4.1.6 - 2026-09-11

- Fixed Studio bootstrap rejecting duplicate StableIds copied into `ServerStorage.__WeAreDevsRollback_*`; both the current WeAreDevs name and the legacy WrexDev rollback name are excluded from traversal and identity indexes without deleting their backup contents.

## 4.1.5 - 2026-09-10

- Added the current application version beside the RiftSync title in the desktop header.
- Added an explicit no-compare source chooser on every Start: Studio replaces local with backup, or local applies to Studio with an Undo recording; live sync remains Local -> Studio.
- Fixed overlapping managed roots being able to index the same Studio instance more than once, and condensed duplicate StableId reports to include the conflicting Studio paths once.
- Excluded `ServerStorage.__WrexDevRollback_*` recovery snapshots from Studio traversal, identity indexes, references, and sync exports so copied StableIds inside third-party rollback data do not block syncing.
- Added StableId-first identity validation, duplicate sibling suffix support, and structured validation conflicts.
- Added read-only Studio preflight, rollback journaling for unexpected apply failures, and full conflict details in activity/debug payloads.
- Added portable config template, macOS plugin build guard, pure Luau identity tests, and consistent product versioning.

## 4.1.4 - 2026-09-04

- Added lossless sync for same-name, same-class Roblox siblings through stable-ID local path suffixes while preserving their Studio names.
- Added stable-ID-first duplicate apply behavior and Decal/Texture `Face` synchronization.
- Added automatic Local profile discovery on widget open plus a compatible Studio refresh icon.

## 4.1.3 - 2026-08-04

- Fixed transient Git indexing failures during automatic revision commits by retrying `git add -A` when project state files are still settling.

## 4.1.2 - 2026-08-02

- Fixed Studio deletes being silently skipped for instances exported by Auto Pull Studio.
- Added persistent initialized-project state and deletion tombstones so intentionally empty local projects remain authoritative.
- Added exact legacy delete repair through stable ID or RiftSync-owned script path/class proof.
- Added safe cleanup for orphan script metadata and empty canonical `Name.ClassName` folders.
- Bootstrap errors now prevent ownership adoption and revision acknowledgement instead of reporting partial writes as successful.

## 4.1.1 - 2026-08-01

- Added structured, accessible sync progress bars to the desktop dashboard and Studio widget.
- Fixed Studio snapshot exports mistaking same-named children such as `UICorner` for properties.
- Fixed dotted Roblox names such as `Cube.004` by resolving apply targets from typed local-path segments.
- Removed the redundant Studio -> Folder fetch/apply-back pass and automatically seed an empty Folder -> Studio project from Studio once.
- Added cooperative snapshot yields, per-class property caching, tolerant orphan metadata handling, and capped error samples.
- Added an accessible `Syncing` desktop state, neutral stopped indicators, healthy `No issues` feedback, and semantic success/warning/error/info toasts.

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
