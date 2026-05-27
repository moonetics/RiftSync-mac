# PRD: RiftSync Go Local Server

Dokumen ini mencatat status final local server RiftSync berbasis Go.

## Tujuan

- Menyediakan local HTTP server ringan untuk Roblox Studio.
- Menyediakan embedded WebView app UI ringan untuk menjalankan dan memonitor server.
- Mendukung sinkronisasi `Folder -> Studio` dan `Studio -> Folder`.
- Menjaga kontrak HTTP JSON yang dipakai plugin Lua.
- Mendukung executable Windows `riftsync.exe` untuk app launcher dan `riftsync-server.exe` untuk console/headless.

## Arsitektur

```text
Roblox Studio Plugin
        |
        | HTTP JSON
        v
riftsync.exe
        |
        +-- embedded WebView app UI
        +-- config loader
        +-- scanner/cache
        +-- fsnotify watcher
        +-- HTTP API
        +-- revision state
        +-- async Git versioning
```

Package utama:

- `cmd/riftsync-app`: app launcher tanpa console untuk embedded WebView UI.
- `cmd/riftsync-server`: console/headless entrypoint, CLI flags, startup/shutdown.
- `internal/uiapp`: embedded WebView UI, local control server, dan app lifecycle endpoint.
- `internal/serverapp`: reusable server runner untuk UI dan headless mode.
- `internal/config`: load dan validasi `sync_config.json`.
- `internal/httpapi`: endpoint health, handshake, snapshot, changes, history, bootstrap, ack, debug.
- `internal/records`: parser record, path mapping, diff, payload change.
- `internal/scanner`: full scan, subtree scan, file parse, cache `size + mtime`.
- `internal/watcher`: event-based incremental sync dengan `fsnotify`.
- `internal/state`: revision, change log, snapshot cache, metrics.
- `internal/gitversion`: commit Git async setelah revision.

## Endpoint

- `GET /health`
- `POST /handshake`
- `GET /snapshot`
- `GET /changes?since_rev=x&timeout=y`
- `GET /history?limit=n`
- `GET /history?rev=x`
- `GET /history/<revision>`
- `POST /bootstrap`
- `POST /ack`
- `GET /debug/state`

Endpoint app UI lokal:

- `GET /app`
- `GET /app/status`
- `POST /app/start`
- `POST /app/stop`
- `POST /app/restart`
- `POST /app/open-folder`

## CLI

```powershell
go run ./cmd/riftsync-app
go build -ldflags="-H=windowsgui" -o riftsync.exe ./cmd/riftsync-app
go build -o riftsync-server.exe ./cmd/riftsync-server
.\riftsync.exe
```

Default app membuka window `RiftSync` embedded WebView dalam keadaan stopped. Untuk mode console:

```powershell
.\riftsync-server.exe --headless
```

Flag:

- `--config sync_config.json`
- `--port 8765`
- `--debug`
- `--legacy-scan`
- `--headless`

`--legacy-scan` adalah fallback Go periodic scan ketika event filesystem tidak bisa diandalkan.

## Behavior Utama

- Startup melakukan initial scan dari `sync_root`.
- Watcher memproses perubahan file/folder secara incremental.
- Scanner cache menghindari parse ulang file yang tidak berubah.
- `/snapshot` memakai payload cache dari state.
- `/changes` memakai long polling dan change log revision.
- Pull dari Studio memakai `/bootstrap`, menulis file ke `sync_root`, lalu publish revision bila ada perubahan.
- Git commit berjalan async dan tidak menahan response sync ke Studio.

## Performance dan Debug

`/debug/state` menampilkan:

- indexed counts;
- sessions;
- scan warnings;
- Git status;
- request metrics;
- last event batch size;
- debounce duration;
- parse duration;
- publish duration;
- cache hit/miss.

Target UX:

- server idle tidak melakukan kerja berat;
- save file kecil menghasilkan revision sekitar debounce + parse;
- masalah port, watcher, Git, dan invalid JSON terlihat dari debug state.

## Acceptance

- `go test ./...` stabil.
- `go build -ldflags="-H=windowsgui" -o riftsync.exe ./cmd/riftsync-app` berhasil.
- `go build -o riftsync-server.exe ./cmd/riftsync-server` berhasil.
- `.\riftsync.exe` bisa dipakai sebagai workflow utama tanpa console window saat double-click.
- UI memakai tabs `Overview`, `History`, dan `Config` tanpa page-level scroll.
- UI menampilkan status server, sync root, host/port, counts, Git state, error terakhir, History revision, Change Path, dan Save & Restart.
- Window app memakai custom header frameless jika Win32 style update tersedia, dengan fallback title bar normal.
- Plugin Lua produksi tetap memakai kontrak HTTP yang sama.
- Folder/file local project tetap berada di dalam `sync_root`.
