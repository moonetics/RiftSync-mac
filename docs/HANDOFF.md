# RiftSync Handoff

Tanggal handoff: 2026-06-12

## Ringkasan Proyek

RiftSync adalah tool sync Roblox Studio <-> local folder berbasis Go server + Studio plugin. User sedang menambahkan workflow supaya AI yang bekerja di folder game lokal bisa:

- Membaca `.guidebook/README.md` untuk memahami capability RiftSync.
- Membuat/edit file Roblox di `sync_root`.
- Menjalankan Studio command bar dari terminal lokal melalui Remote Exec.
- Melihat Git status dari repo `sync_root`, bukan selalu dari repo tool RiftSyncPlugin.

Repo kerja saat ini:

```text
C:\Users\BIMO YUDISTIRA ARIEL\Documents\RiftSyncPlugin
```

Jangan edit `sync_config.json` kecuali user eksplisit minta. File itu sudah tampak modified di working tree sebelum beberapa perubahan terakhir.

## Fitur Yang Sudah Diimplementasi

### Phase 1: Remote Exec Backend

Backend Go sudah punya command queue in-memory dan endpoint:

- `POST /exec/commands`
- `GET /exec/commands/next?client_id=...&timeout=...`
- `POST /exec/commands/result`
- `GET /exec/commands/{id}`

Config Remote Exec:

- `remote_exec_enabled`
- `remote_exec_token`
- `remote_exec_max_source_bytes`
- `remote_exec_default_timeout_sec`
- `remote_exec_max_timeout_sec`

Remote Exec default OFF. Endpoint exec reject kalau disabled. Saat enabled, endpoint butuh `Authorization: Bearer <remote_exec_token>`.

### Phase 2: CLI Exec

`cmd/riftsync-server` sudah punya subcommand:

```powershell
.\riftsync-server.exe exec "print(workspace.Name)"
.\riftsync-server.exe exec --stdin --timeout 15
.\riftsync-server.exe exec --file .\studio-command.luau --timeout 30
```

Output mode:

- default human-readable
- `--json`
- `--raw`

Exit code:

- `0` success
- `1` Luau error
- `2` CLI/config/source invalid
- `3` server/auth/disabled/offline error
- `4` timeout/expired/cancelled

### Phase 3: Studio Plugin Remote Exec

Plugin files updated:

- `plugin/TypeList.lua`
- `plugin/API.lua`
- `plugin/RiftSyncPlugin.lua`

Plugin UI now has:

- `Exec Token` textbox
- `Exec OFF/ON` toggle
- status label

Plugin behavior:

- Remote Exec OFF by default each plugin session.
- Token persists via plugin settings.
- Polling starts only after sync handshake/snapshot readiness.
- Uses `/exec/commands/next` and `/exec/commands/result`.
- Captures `print`, `warn`, returns, runtime error, traceback, `duration_ms`.
- Rejects command during Play mode with error result.
- If `loadstring` unavailable, posts clear error result.

Manual Roblox Studio verification has not been performed in this environment.

## Empty Experience Bootstrap + Guidebook

`internal/config/config.go` now contains scaffold logic:

- `MetadataDir = ".rblxsync"`
- `GuidebookDir = ".guidebook"`
- `GuidebookExecLauncher = "riftsync-exec.ps1"`
- `BootstrapServiceDirs`
- `EnsureSyncRootScaffold`

When RiftSync starts, it ensures these folders exist in `sync_root`:

- `ServerScriptService`
- `ServerStorage`
- `ReplicatedStorage`
- `StarterGui`
- `StarterPack`
- `StarterPlayer`

It also creates `.guidebook/README.md` if missing. Existing user-edited `.guidebook/README.md` is not overwritten.

Scanner/watcher/bootstrap preserve and ignore:

- `.git`
- `.rblxsync`
- `.guidebook`

`/bootstrap` replace preserves `.guidebook` and re-ensures service folders.

## Guidebook Content

The generated `.guidebook/README.md` is a detailed AI-readable playbook. It explains:

- RiftSync capabilities.
- Folder roots and their purpose.
- File naming and supported metadata:
  - `.server.luau`
  - `.client.luau`
  - `.luau`
  - `properties.init.json`
  - `init.meta.json`
  - `.model.json`
- Normal sync workflow.
- Remote Exec command bar from local terminal.
- Git/history usage.
- Config fields.
- Metadata folders.
- Troubleshooting.

Important: `README.md` is only generated when missing, so updates to the template only affect new sync roots or roots where `.guidebook/README.md` does not exist.

## Auto-Generated Remote Exec Launcher

New file generated in each `sync_root`:

```text
.guidebook/riftsync-exec.ps1
```

Purpose: AI can be inside `D:\MyGame\src\game` and run Remote Exec without knowing where `riftsync-server.exe` or `sync_config.json` live.

Example from `sync_root`:

```powershell
.\.guidebook\riftsync-exec.ps1 "print(workspace.Name)"
```

Multi-line:

```powershell
@'
print(workspace.Name)
return game.PlaceId
'@ | .\.guidebook\riftsync-exec.ps1 --stdin --timeout 15
```

Implementation details:

- Launcher is auto-generated and can be overwritten every RiftSync start.
- It uses absolute path to `riftsync-server.exe`.
- It uses absolute path to the active config.
- Paths are escaped with PowerShell single-quote escaping.
- If running process is `riftsync-server.exe`, resolver uses `os.Executable()`.
- If running process is `riftsync.exe`, resolver uses sibling `riftsync-server.exe`.
- If sibling is missing, launcher is still generated with a warning comment.

Key functions:

- `EnsureSyncRootScaffold`
- `writeGuidebookExecLauncher`
- `resolveRiftSyncServerPath`
- `buildGuidebookExecLauncher`
- `powershellSingleQuoted`

## Tests Added/Updated

Config tests:

- Scaffold creates service folders and guidebook.
- Scaffold does not overwrite README.
- Launcher is created and overwritten.
- PowerShell path escaping works.
- Server path resolver works for server/app/missing sibling.

Integration/regression:

- Runner start creates service folders, guidebook, and launcher.
- Bootstrap replace preserves `.guidebook`, README, and launcher.
- Scanner ignores `.guidebook`.
- Watcher ignores `.guidebook`.

Last verification run:

```powershell
go test ./...
git diff --check
```

Both passed.

## Current Working Tree Notes

`git status --short` shows many modified/untracked files because this conversation included Phase 1/2/3 Remote Exec, branding/icon work, guidebook scaffolding, and launcher generation support.

Known modified/untracked areas include:

- `README.md`
- `cmd/riftsync-server/*`
- `internal/config/*`
- `internal/httpapi/*`
- `internal/state/*`
- `internal/scanner/*`
- `internal/serverapp/*`
- `internal/watcher/*`
- `plugin/*`
- `docs/PRD_STUDIO_REMOTE_EXEC.md`
- `assets/brand/*`
- `scripts/*`
- `.syso` Windows resource files

`sync_config.json` is also modified in the working tree. Treat it as user-owned and do not change/revert it without explicit instruction.

## Manual Verification Still Needed

Roblox Studio cannot be verified from this environment. User or next AI should manually test:

1. Start RiftSync app/server.
2. Open Studio plugin.
3. Paste matching Exec Token.
4. Start sync.
5. Toggle `Exec ON`.
6. From `sync_root`, run:

```powershell
.\.guidebook\riftsync-exec.ps1 "print(workspace.Name)"
```

7. Test multi-line stdin.
8. Test runtime error and traceback.
9. Test Play mode rejection.
10. Confirm `.guidebook` files do not sync to Studio.

## Useful Commands

Run all tests:

```powershell
go test ./...
```

Check patch whitespace:

```powershell
git diff --check
```

Check game file changes, from `sync_root`:

```powershell
git status
git diff
git diff --check
```

Remote Exec from `sync_root`:

```powershell
.\.guidebook\riftsync-exec.ps1 "print(workspace.Name)"
```

Fallback direct Remote Exec:

```powershell
.\riftsync-server.exe --config .\sync_config.json exec "print(workspace.Name)"
```

## User Preferences And Decisions

- User wants private/local workflow, not public Roblox plugin publishing.
- Remote Exec should be trusted local automation, default OFF.
- `.guidebook` should explain all RiftSync capabilities in detail for AI.
- `.guidebook/README.md` should not be overwritten once user customizes it.
- `.guidebook/riftsync-exec.ps1` may be overwritten because it is machine-local generated launcher.
- Do not copy `riftsync-server.exe` into game project; generate wrapper only.
- No hardcoded path like Bimo's username; paths must be detected per machine.

