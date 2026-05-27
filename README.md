# RiftSync Plugin (One-Way + Properties Tree)

Versi: `3.0.0`
Protocol sync: `rbxsync/2.0.0`

One-way sync untuk Roblox Studio:

- Edit file di VS Code (folder lokal).
- Server Go lokal mendeteksi perubahan file dan mengirim delta ke Studio.
- Plugin Roblox Studio pull delta dan update Script + properties tree di Studio.
- Perubahan dari Studio tidak dikirim balik ke folder lokal.
- Saat `Start`, sekarang ada 2 mode:
  - `Folder -> Studio` (default, aman untuk file lokal yang sudah ada).
  - `Studio -> Folder` (override isi folder lokal dari Studio).

## Hierarki Plugin

Gunakan struktur ini saat menyusun plugin:

```text
RiftSyncPlugin (Script / PluginScript)
└─ API (ModuleScript)
   └─ TypeList (ModuleScript)
```

Sumber file:

- `plugin/RiftSyncPlugin.lua` -> isi `RiftSyncPlugin`
- `plugin/API.lua` -> isi `API` (child dari `RiftSyncPlugin`)
- `plugin/TypeList.lua` -> isi `TypeList` (child dari `API`)

## Struktur Project

```text
plugin/              Source Lua produksi untuk Roblox Studio
cmd/                 Entrypoint Go server
internal/            Package Go server
docs/                Dokumen rencana migrasi dan PRD
sync_config.json     Konfigurasi server/sync root
```

## Naming Convention File Lokal

Untuk referensi lengkap yang bisa dipakai AI/developer saat menulis UI Roblox sebagai JSON, baca:

- `docs/AI_UI_SYNC_GUIDE.md`

Dokumen itu menjelaskan format `init.meta.json`, `*.model.json`, typed values seperti `UDim2`, `Color3`, `ColorSequence`, `NumberSequence`, contoh `UIGradient`, `ImageLabel`, `ImageButton`, responsive UI, dan aturan safety plugin.

- Canonical script source file di folder instance:
  - `<Name>.server.luau` -> `Script`
  - `<Name>.client.luau` -> `LocalScript`
  - `<Name>.module.luau` -> `ModuleScript`
- `properties.init.json` -> metadata + properti instance (folder-per-instance)
- Legacy script format (`source.*.luau`, `*.server.lua`, `*.client.lua`, `*.module.lua`) masih dibaca untuk kompatibilitas migrasi.

Aturan penamaan:

- Setiap instance Roblox direpresentasikan sebagai satu folder canonical: `<Name>.<ClassName>`.
- Properties + script source hidup bersama di folder instance yang sama.
- Saat `Studio -> Folder`, nama instance yang tidak aman untuk path Windows (contoh: mengandung `/`, trailing spasi/titik, reserved name seperti `CON`) akan di-encode otomatis ke format `~x_<HEX>` agar sinkronisasi tidak gagal.

Contoh:

- `<sync_root>/StarterGui/MainScreen.ScreenGui/properties.init.json`
- `<sync_root>/StarterGui/MainScreen.ScreenGui/Loader.LocalScript/properties.init.json`
- `<sync_root>/StarterGui/MainScreen.ScreenGui/Loader.LocalScript/Loader.client.luau`
- `<sync_root>/StarterGui/MainScreen.ScreenGui/TopBar.Frame/properties.init.json`

### Format Folder UI

Setiap instance UI ditulis sebagai folder `<Name>.<ClassName>` dan file `properties.init.json`:

```text
StarterGui/
└─ MainScreen.ScreenGui/
   ├─ properties.init.json
   └─ TopBar.Frame/
      ├─ properties.init.json
      └─ UIGradient.UIGradient/
         └─ properties.init.json
```

`properties.init.json` menyimpan:

- `id` stable id untuk rename safety
- `className`
- `name`
- `properties` (typed JSON, contoh `UDim2`, `Color3`, `Enum`, dst)
- `attributes`
- `tags`

### Format UI Rojo-Like (`init.meta.json` + `.model.json`)

Selain `properties.init.json`, UI juga bisa ditulis manual dengan format yang lebih mirip Rojo:

```text
StarterGui/
└─ MainGui/
   ├─ init.meta.json
   ├─ TimerLabel.model.json
   └─ VotingFrame/
      ├─ init.meta.json
      ├─ UICorner.model.json
      ├─ UIStroke.model.json
      ├─ UIScale.model.json
      └─ TitleLabel.model.json
```

Contoh singkat `init.meta.json`:

```json
{
  "className": "ScreenGui",
  "properties": {
    "ResetOnSpawn": false,
    "IgnoreGuiInset": true
  }
}
```

Contoh singkat `TitleLabel.model.json`:

```json
{
  "$className": "TextLabel",
  "Name": "TitleLabel",
  "Text": "Vote Next Game",
  "Size": { "UDim2": [1, 0, 0, 50] },
  "BackgroundTransparency": 1,
  "TextScaled": true
}
```

Untuk daftar class UI, gradient, image, sequence/keypoints, responsive pattern, dan aturan authoring untuk AI, lihat `docs/AI_UI_SYNC_GUIDE.md`.

### Format Folder Workspace + Lighting

```text
Workspace/
├─ properties.init.json
└─ Terrain.Terrain/
   └─ properties.init.json

Lighting/
├─ properties.init.json
├─ Atmosphere.Atmosphere/
│  └─ properties.init.json
├─ Sky.Sky/
│  └─ properties.init.json
└─ Bloom.BloomEffect/
   └─ properties.init.json
```

Catatan:

- Yang disinkronkan untuk `Terrain` adalah **properties** aman (bukan voxel terrain data).
- Root service (`Workspace`, `Lighting`) di-resolve dari service yang sudah ada, bukan dibuat/dihapus ulang.

### Format Folder Remotes

Remote sekarang ikut disinkronkan lewat folder instance yang sama:

```text
ReplicatedStorage/
└─ Network.Folder/
   ├─ properties.init.json
   ├─ Ping.RemoteEvent/
   │  └─ properties.init.json
   ├─ Rpc.RemoteFunction/
   │  └─ properties.init.json
   └─ Bootstrap.BindableEvent/
      └─ properties.init.json
```

Class remote yang didukung:

- `RemoteEvent`
- `RemoteFunction`
- `UnreliableRemoteEvent`
- `BindableEvent`
- `BindableFunction`

Scope default remote:

- `ReplicatedStorage`
- `ReplicatedFirst`
- `Workspace`
- `StarterGui`
- `StarterPlayer`
- `StarterPack`
- `ServerScriptService`
- `ServerStorage`

Catatan:

- Create/update/delete/rename/move remote dari filesystem sekarang ikut diaplikasikan ke Studio.
- Parent path remote yang belum ada bisa dibuat otomatis sebagai `Folder` saat apply, agar nested remote tidak gagal import.

## Setup Folder

Watcher default membaca dari `sync_root` di `sync_config.json`. Pada repo ini default-nya masih `src/game`, tetapi folder `src/` bukan bagian plugin dan boleh tidak ada sampai kamu menaruh project lokal sendiri.

Contoh minimal:

```text
<sync_root>/
├─ ServerScriptService/
│  └─ Bootstrap.Script/
│     └─ Bootstrap.server.luau
├─ ReplicatedStorage/
│  └─ Shared.Folder/
│     └─ Util.ModuleScript/
│        └─ Util.module.luau
└─ StarterPlayer/
   └─ StarterPlayerScripts/
      └─ Main.LocalScript/
         └─ Main.client.luau
```

## Jalankan App Go

Go app adalah workflow recommended. Ada dua executable:

- `riftsync.exe`: app launcher untuk double-click. Ini membuka window app embedded WebView2 dan tidak menampilkan console.
- `riftsync-server.exe`: console/headless/debug mode untuk terminal.

Build keduanya dari root project:

```powershell
go build -ldflags="-H=windowsgui" -o riftsync.exe ./cmd/riftsync-app
go build -o riftsync-server.exe ./cmd/riftsync-server
```

Untuk pemakaian normal, jalankan:

```powershell
.\riftsync.exe
```

Window app `RiftSync` akan terbuka memakai embedded WebView2, bukan tab browser eksternal. Server berada dalam kondisi stopped sampai kamu klik **Start**. Tombol **Change Path** membuka input manual untuk mengganti `sync_root`; tidak ada folder picker supaya tetap ringan di laptop low-end.

Untuk debug/automation dari terminal:

```powershell
.\riftsync-server.exe --headless
```

Flag opsional:

```powershell
.\riftsync.exe --config sync_config.json
.\riftsync.exe --host 127.0.0.1
.\riftsync.exe --port 8766 --debug
.\riftsync.exe --sync-root "D:\MyGame"
.\riftsync.exe --legacy-scan
.\riftsync-server.exe --headless --port 8766
```

- `--config`: path config, default `sync_config.json`.
- `--host`: bind host lokal, hanya `127.0.0.1` atau `localhost`.
- `--port`: override port dari config.
- `--sync-root`: override folder sinkronisasi dari config.
- `--debug`: print detail startup, scan warning, Git/watcher status, dan shutdown.
- `--legacy-scan`: fallback periodic scan Go jika watcher event filesystem bermasalah.
- `--headless`: hanya tersedia di `riftsync-server.exe`, menjalankan server console tanpa UI untuk automation.

App UI lokal memakai layout tab tanpa scroll halaman utama:

- `Overview`: status, sync root, indexed counts, health, Git, dan quick actions.
- `History`: revision list + detail viewer dengan scroll internal.
- `Config`: config path, sync root, host, port, Git/debug/legacy scan, dan Save & Restart.

Header app custom menggantikan title bar Windows bila frameless mode berhasil. Jika Win32 frameless gagal di mesin tertentu, app tetap jalan dengan title bar normal.

### Git Versioning + History Rev

- Watcher otomatis membuat repo Git di `sync_root` (`src/game`) saat server boot jika `git_versioning_enabled` aktif.
- Setiap revision yang terdeteksi watcher dibuat sebagai commit `RiftSync rev <rev>: <count> changes`.
- Rollback dilakukan lewat Git di folder `src/game`, misalnya checkout/revert commit yang sesuai kebutuhan.
- Endpoint `GET /history?limit=50` menampilkan daftar revision terbaru, dan `GET /history?rev=<rev>` menampilkan detail perubahan khusus revision itu.
- Viewer history utama ada di `riftsync.exe`. Widget Studio dibuat ringan dan hanya menampilkan hint bahwa history tersedia di app.

## Gunakan di Roblox Studio

1. Enable HTTP requests di Studio (Game Settings -> Security -> Allow HTTP Requests).
2. Install plugin dengan struktur hierarki di atas.
3. Buka toolbar `RiftSync`, isi host/port (default `127.0.0.1:8765`).
4. Pilih mode `Start` di widget:
   - `Folder -> Studio` jika source of truth ada di folder lokal.
   - `Studio -> Folder` jika mau override folder lokal dari Studio.
   - Jika pilih `Folder -> Studio` tapi folder masih kosong, plugin akan auto seed dari Studio sekali dulu.
5. Klik `Start`.
6. Save file di VS Code, perubahan akan otomatis muncul di Studio.
7. Jika ada error, status bar tampil ringkas dan detail lengkap muncul di panel `Output` Studio.
8. Aktifkan `Debug` di widget untuk checklist live:
   - Handshake
   - Snapshot
   - Poll
   - Delta
   - ACK Sent
   - ACK OK
   plus tail event log dan ringkasan status server (`/debug/state`).

### Better Error Guidance

Widget menampilkan hint pendek untuk error umum:

- `Invalid JSON`: cek syntax JSON, koma, bracket, dan typed value.
- `Unsupported property`: cek nama/type property atau tambahkan ke `extra_allowed_properties`.
- `Unsupported class`: cek `className` dan `managed_roots`.
- `Unmanaged conflict`: rename/remove instance di Studio atau jadikan target managed.
- `Duplicate stable id`: ganti/hapus duplicate `id` di `properties.init.json`.
- `Server offline / HTTP disabled`: jalankan `.\riftsync.exe` lalu klik **Start**, atau gunakan `.\riftsync-server.exe --headless` untuk debug terminal. Cek host/port dan aktifkan HTTP Requests di Studio.

### Troubleshooting Server

- Port sudah dipakai: jalankan dengan `--port 8766`, lalu samakan port di widget Studio.
- HTTP Requests belum aktif: buka `Game Settings -> Security -> Allow HTTP Requests`.
- Firewall/security prompt Windows: izinkan local server untuk private network jika diminta.
- Host/port plugin tidak cocok: gunakan host `127.0.0.1` dan port yang sama dengan output server.
- Git tidak tersedia: install Git atau set `"git_versioning_enabled": false` di `sync_config.json`.
- Watcher tidak mendeteksi perubahan di folder network/cloud sync: jalankan `.\riftsync.exe --legacy-scan` atau `.\riftsync-server.exe --headless --legacy-scan`.

## Menjadikan RBXM

1. Di Studio, buat `Model` bernama `RiftSyncPluginModel`.
2. Pindahkan `RiftSyncPlugin` beserta child module bertingkat (`API` -> `TypeList`) ke dalam model itu.
3. Pilih model tersebut.
4. `File -> Export Selection...` lalu simpan sebagai `.rbxm`.

Saat import `.rbxm` di project lain, gunakan model itu untuk dibuat jadi local plugin sesuai workflow tim kamu.

## Konfigurasi Server

Saat pertama kali dijalankan, watcher membuat `sync_config.json` otomatis jika belum ada.

Field penting:

- `port`
- `sync_root`
- `managed_roots`
- `ignored_rbx_paths` (path Roblox yang tidak ikut disync, default include `game.ServerScriptService.RiftSyncPlugin` dan nama lama untuk kompatibilitas)
- `scan_interval_sec`
- `poll_timeout_sec`
- `change_retention`
- `debug_event_retention` (jumlah event debug yang disimpan server untuk endpoint `/debug/state`)
- `git_versioning_enabled` (`true` default): buat commit Git otomatis per server revision di `sync_root`.
- `strict_property_whitelist`:
  - `false` (default): mode soft whitelist. Property non-whitelist tetap dicoba apply jika valid (pcall-safe), lalu dilog sebagai notice.
  - `true`: mode strict. Property non-whitelist di-skip.
- `extra_allowed_properties`: override property per class tanpa patch code, contoh:
  - `"extra_allowed_properties": { "Lighting": ["FuturePropertyName"], "Sky": ["AnotherNewProp"] }`

## Catatan Deletion

- Delete folder/file di VS Code dipropagasikan end-to-end menjadi opcode `delete` dan `Destroy()` di Studio.
- Root service (`StarterGui`, `Workspace`, `Lighting`, `ReplicatedStorage`, `ReplicatedFirst`) dan `Terrain` tetap diproteksi agar tidak ter-destroy salah.
- Untuk scope property sync (`StarterGui`, `Workspace`, `Lighting`), delete instance non-managed juga diproses (filesystem authoritative) supaya kasus folder terhapus tapi instance tetap nongol di Studio tidak terjadi lagi.
