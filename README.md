# RiftSync macOS (One-Way + Properties Tree)

Fork macOS dari RiftSync dengan aplikasi native `RiftSync.app`, server Apple Silicon, dan profile Local plugin yang mengikuti project di aplikasi. Dukungan build Windows tetap dipertahankan di source yang sama.

Versi: `4.1.11`
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

Gunakan struktur ini saat menyusun plugin di Roblox Studio:

```text
RiftSyncPlugin (Script / PluginScript)
└── API (ModuleScript)
    ├── TypeList (ModuleScript)
    └── Identity (ModuleScript)
```

Sumber file:

- `plugin/RiftSyncPlugin.lua` -> isi `RiftSyncPlugin`
- `plugin/API.lua` -> isi `API` (child dari `RiftSyncPlugin`)
- `plugin/TypeList.lua` -> isi `TypeList` (child dari `API`)
- `plugin/Identity.lua` -> isi `Identity` (child dari `API`)

## Struktur Project

```text
plugin/              Source Lua produksi untuk Roblox Studio
cmd/                 Entrypoint Go server
internal/            Package Go server
docs/                Dokumen rencana migrasi dan PRD
sync_config.json     Konfigurasi server/sync root
sync_config.example.json  Template aman untuk setup baru
```

`sync_config.json` adalah konfigurasi lokal dan tidak boleh menyimpan token pribadi di
repository. Mulai dari [`sync_config.example.json`](sync_config.example.json), lalu
simpan token Remote Exec hanya di file lokal. Di macOS, path Windows absolute akan
memunculkan migration warning; sync root yang tidak ada atau tidak dapat diakses
menghentikan startup sampai folder diperbaiki melalui `Open Folder`.

## Naming Convention File Lokal

Untuk referensi lengkap yang bisa dipakai AI/developer saat menulis UI Roblox sebagai JSON, baca:

- `docs/AI_UI_SYNC_GUIDE.md`

Dokumen itu menjelaskan format `init.meta.json`, `*.model.json`, typed values seperti `UDim2`, `Color3`, `ColorSequence`, `NumberSequence`, contoh `UIGradient`, `ImageLabel`, `ImageButton`, responsive UI, dan aturan safety plugin.

- Canonical script source file di folder instance:
  - `<Name>.server.luau` -> `Script`
  - `<Name>.client.luau` -> `LocalScript`
  - `<Name>.module.luau` -> `ModuleScript`
- `properties.init.json` -> metadata + properti persistent yang aman untuk script, ValueBase, environment, UI, audio/effect, attachment/constraint, dan instance non-geometry lain.
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

Contoh script dengan properti:

```text
ServerScriptService/
└─ Bootstrap.Script/
   ├─ properties.init.json
   └─ Bootstrap.server.luau
```

```json
{
  "properties": {
    "Enabled": false,
    "RunContext": {
      "$type": "Enum",
      "enumType": "RunContext",
      "value": "Server"
    }
  }
}
```

`Enabled` adalah property canonical untuk `Script` dan `LocalScript`; metadata lama dengan `Disabled` tetap diterima saat import. Jika keduanya ada, `Enabled` menang. `Source` tetap ditulis di file `.server.luau`/`.client.luau`/`.module.luau`, bukan di `properties.init.json`.

### Initial Sync dan Delete

- Project yang benar-benar baru dan belum memiliki file sync otomatis menjalankan `Studio -> Folder` satu kali.
- Setelah project pernah diinisialisasi, folder lokal tetap menjadi sumber utama meskipun seluruh file sync sengaja dihapus. RiftSync tidak menarik ulang object lama dari Studio.
- Delete lokal disimpan sebagai tombstone di `.rblxsync/project-state.json` agar Studio yang baru tersambung tetap dapat membersihkan instance lama secara aman.
- RiftSync hanya menghapus instance dengan bukti ownership yang cocok. Service root, Terrain, BasePart, Model, ignored path, dan instance unmanaged tetap dilindungi.
- Saat Local -> Studio, satu object unmanaged pada destination yang path dan class-nya persis sama dapat diadopsi, termasuk metadata lokal dengan suffix `~rid_...`. StableId yang bentrok, beberapa kandidat yang sama-sama cocok, rename/delete, dan penggantian class tetap diblokir.
- Orphan `properties.init.json` milik script dan folder canonical `<Name>.<ClassName>` yang benar-benar kosong dipangkas otomatis. Folder nonempty tidak dihapus secara rekursif.

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

### Broad Properties Metadata (4.1)

Saat menjalankan `Studio -> Folder`, RiftSync 4.1 menelusuri seluruh descendant dari `managed_roots` dan membuat metadata untuk class non-geometry yang aman. Cakupannya meliputi script; seluruh concrete `ValueBase`; Workspace/Terrain/Lighting dan effect; UI; Folder/Configuration/remotes/bindables; Sound/SoundEffect/audio node; Animation; particle/beam/trail/highlight/light/decal/texture/mesh modifier; prompt/detector/tool; attachment; serta concrete Constraint.

`BasePart` dan `Model` tetap dipakai sebagai segmen path, tetapi tidak memperoleh `properties.init.json` dan tidak menjadi milik RiftSync. Contoh berikut tetap mengekspor `Health.IntValue`:

```text
Workspace/Map.Model/Trigger.Part/Health.IntValue/properties.init.json
```

Jika ancestor `Map` atau `Trigger` tidak ada saat `Folder -> Studio`, RiftSync melaporkan parent-missing dan tidak membuat geometry pengganti. Root service dan `Terrain` juga hanya dapat di-update, tidak dibuat, di-rename, atau dihapus. Voxel Terrain, Camera/runtime character, Player, joint/Motor6D, `Source`, serta state runtime/read-only tidak ikut metadata.

Reference antar-instance, misalnya `ObjectValue.Value`, disimpan sebagai typed value:

```json
{
  "$type": "InstanceRef",
  "stableId": "stable-guid",
  "path": "game.Workspace.Map.Target"
}
```

Nil reference ditulis sebagai `{ "$type": "InstanceRef", "null": true }`. Saat apply, RiftSync mencari stable ID terlebih dahulu lalu path; reference non-null yang tidak ditemukan menjadi sync error. Plugin dan server 4.1 sebaiknya digunakan bersama untuk project yang memakai `InstanceRef`.

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

Saat server/app start, RiftSync otomatis memastikan folder service dasar tersedia untuk experience baru yang masih kosong:

- `ServerScriptService`
- `ServerStorage`
- `ReplicatedStorage`
- `StarterGui`
- `StarterPack`
- `StarterPlayer`

RiftSync juga membuat `.guidebook/README.md` di dalam `sync_root` jika belum ada. File ini adalah playbook lokal untuk manusia/AI: struktur folder, workflow sync, Remote Exec, metadata yang tidak boleh disync, dan cara cek Git. RiftSync juga menulis `.guidebook/riftsync-exec.ps1` sebagai launcher lokal auto-generated untuk menjalankan Remote Exec dari folder game, serta `.guidebook/status.json` sebagai status mesin yang bisa dibaca tool/AI. Folder metadata `.git`, `.rblxsync`, dan `.guidebook` tidak dikirim ke Studio.

Contoh minimal:

```text
<sync_root>/
├─ .git/
├─ .rblxsync/
├─ .guidebook/
│  ├─ README.md
│  ├─ status.json
│  └─ riftsync-exec.ps1
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

- `riftsync.exe` / `RiftSync.app`: app launcher untuk Windows atau macOS. Ini membuka window app embedded WebView dan tidak menampilkan console.
- `riftsync-server.exe`: console server untuk terminal. Secara default menampilkan startup, event sync/Studio/Remote Exec/Git, heartbeat status, error, dan shutdown; tambahkan `--debug` untuk metrics scan/cache/payload yang lebih detail.

Build keduanya dari root project:

```powershell
go build -ldflags="-H=windowsgui" -o riftsync.exe ./cmd/riftsync-app
go build -o riftsync-server.exe ./cmd/riftsync-server
```

Build dengan icon Windows:

```powershell
.\scripts\build-windows.ps1
```

Build aplikasi native macOS (Apple Silicon), server, bundle, dan icon:

```bash
./scripts/build-macos.sh
open ./dist/RiftSync.app
```

Bundle hasilnya berada di `dist/RiftSync.app`. UI macOS memakai WebKit bawaan sistem; folder picker dan tombol Open Folder memakai dialog native macOS.

Build paket plugin Studio dengan Rojo:

```powershell
.\scripts\build-plugin.ps1 -Output .\RiftSyncPlugin.rbxm
```

Di macOS gunakan `./scripts/build-plugin.sh`. Script build macOS menjalankan
build plugin lebih dulu dan sengaja gagal jika Rojo tidak tersedia agar asset
`.rbxm` lama tidak dipakai diam-diam. `sync_config.example.json` adalah template
portable; jangan commit `sync_config.json` lokal atau token pribadi.

Unit test suite Go dapat dijalankan dengan `go test -count=1 ./...`.
Smoke test integration plugin dilakukan langsung di Roblox Studio.

Script ini membuat asset dari `assets/brand/riftsync-logo.png`, menghasilkan:

- `assets/brand/riftsync-icon-512.png`
- `assets/brand/riftsync.ico`
- `cmd/riftsync-app/rsrc_windows_amd64.syso`
- `cmd/riftsync-server/rsrc_windows_amd64.syso`

File `.syso` otomatis dipakai Go linker saat `go build`, jadi `riftsync.exe` dan `riftsync-server.exe` punya icon Windows.

Untuk pemakaian normal, jalankan:

```powershell
.\riftsync.exe
```

Window app `RiftSync` akan terbuka pada ukuran default **1280×720** memakai embedded WebView2, bukan tab browser eksternal. Semua project dimuat dalam kondisi stopped sampai kamu klik **Start** pada project terkait atau **Start All**.

### Multi-Instance

- Daftar project disimpan di `%AppData%\RiftSync\instances.json`; file ini hanya menyimpan ID, nama tampilan, config path, project aktif, dan preferensi sidebar.
- `sync_config.json` tetap terpisah per project sehingga CLI, headless server, guidebook, dan endpoint Studio lama tetap kompatibel.
- Config project baru dibuat di `<sync_root>/.rblxsync/sync_config.json`, dengan port lokal kosong mulai `8765` dan token Remote Exec tersendiri.
- Config yang diberikan lewat `--config` di-import sebagai project pertama tanpa ditulis ulang saat migrasi.
- Project dapat berjalan bersamaan. Setiap project memiliki runner, watcher, port, status, history, dan antrean Remote Exec sendiri.
- Menghapus project dari sidebar hanya menghapus entry registry; folder, config, history, dan file game tidak dihapus.
- Sidebar normalnya berupa rail ikon dan melebar saat hover/focus. Tombol pin mempertahankan sidebar dalam keadaan terbuka.
- Plugin Studio mengambil profile **Local** dari daftar project aplikasi secara otomatis melalui discovery loopback read-only di `127.0.0.1:8749`; nama, host, port, dan token tidak perlu disalin manual. Profile **Custom** tetap tersedia untuk koneksi remote/manual.

Untuk menjalankan server langsung dari terminal dengan live log:

```powershell
.\riftsync-server.exe
```

Untuk validasi project tanpa menjalankan server:

```powershell
.\riftsync-server.exe validate
.\riftsync-server.exe validate --json
```

`validate` mengecek config, akses `sync_root`, metadata JSON invalid, duplicate stable ID, path script/UI yang tidak didukung, folder metadata yang di-ignore, dan proteksi path traversal. Warning tidak membuat exit code gagal; error validasi mengembalikan exit code `1`, sedangkan error argumen/config load mengembalikan exit code `2`.

Untuk mengirim command Luau ke Studio melalui Remote Exec CLI dari `sync_root`, jalankan server/app terlebih dahulu lalu gunakan launcher lokal:

```powershell
.\.guidebook\riftsync-exec.ps1 "print(workspace.Name)"
.\.guidebook\riftsync-exec.ps1 .\studio-command.lua --timeout 30
.\.guidebook\riftsync-exec.ps1 --last
@'
local part = workspace:WaitForChild("MyPart", 10)
print(part:GetFullName())
return part.Name
'@ | .\.guidebook\riftsync-exec.ps1 --stdin --timeout 15
.\.guidebook\riftsync-exec.ps1 --file .\studio-command.luau --timeout 30
```

Remote Exec membutuhkan `"remote_exec_enabled": true` dan `"remote_exec_token": "..."` di config lokal. App akan generate token 32 karakter jika token belum ada, lalu menampilkannya sebagai readonly field di tab `Exec` supaya bisa dicopy ke field `Exec Token` di Studio plugin. CLI membaca token itu dan mengirim `Authorization: Bearer <token>` ke server. Launcher `.guidebook/riftsync-exec.ps1` auto-generated per mesin dan meneruskan argumen ke `riftsync-server.exe --config <config> exec`. Jika argumen pertama berakhiran `.lua` atau `.luau`, launcher/CLI membacanya sebagai file source sehingga script command bar bisa disimpan dan dirawat seperti file biasa. Subcommand `exec` hanya menghubungi server yang sudah berjalan; ia tidak menyalakan server baru. Gunakan `--json` untuk output terstruktur atau `--raw` untuk output plain tanpa label.

Setiap command Remote Exec dari CLI dan tab Exec app dicatat ke `.rblxsync/exec-history.json` milik project terkait. History ini local-only, menyimpan source penuh dan detail hasil untuk View/rerun, dan otomatis dibatasi ke 100 entry terbaru. Jalankan ulang command terakhir dengan:

```powershell
.\riftsync-server.exe --config .\sync_config.json exec --last
.\.guidebook\riftsync-exec.ps1 --last
```

App `riftsync.exe` juga memiliki tab `Exec` untuk paste Luau, set timeout, run command, melihat output, dan rerun command terbaru/recent. Tab ini tetap membutuhkan server sudah Start, Studio plugin sudah sync, token cocok, dan `Exec ON` aktif.

`.guidebook/status.json` berisi `sync_root`, path config absolut, host/port, versi RiftSync, status Remote Exec, format file yang didukung, revision terakhir, dan timestamp generasi. File ini boleh dioverwrite otomatis oleh RiftSync; `.guidebook/README.md` tetap tidak dioverwrite jika sudah ada.

Fallback langsung jika tidak berada di `sync_root`:

```powershell
.\riftsync-server.exe --config .\sync_config.json exec "print(workspace.Name)"
.\riftsync-server.exe --config .\sync_config.json exec .\studio-command.lua --timeout 30
.\riftsync-server.exe --config .\sync_config.json exec --last
```

Manual test Studio Remote Exec:

1. Jalankan `.\riftsync.exe`, pilih project, lalu Start; token akan muncul di tab `Exec`.
2. Copy token dari app.
3. Di widget RiftSync Studio, pilih profile **Local** yang otomatis mengikuti project aplikasi. Gunakan **Custom** hanya untuk koneksi remote/manual. Profile aktif diingat per Roblox Place.
4. Klik `Start Sync`, lalu toggle `Exec ON`. Fresh session selalu mulai dari `Exec OFF`.
5. Jalankan inline command:

```powershell
.\.guidebook\riftsync-exec.ps1 "print(workspace.Name)"
```

6. Jalankan multi-line command:

```powershell
@'
local part = workspace:WaitForChild("MyPart", 10)
warn("checking", part)
return workspace.Name, part and part.Name
'@ | .\.guidebook\riftsync-exec.ps1 --stdin --timeout 15
```

7. Jalankan command dari file:

```powershell
.\.guidebook\riftsync-exec.ps1 .\studio-command.lua --timeout 30
```

8. Coba error path:

```powershell
.\.guidebook\riftsync-exec.ps1 "error('boom from Studio')"
```

9. Masuk Play mode lalu jalankan command lagi; Studio harus mengirim error `Remote Exec is disabled during Play mode`.

Jika Studio/plugin context tidak menyediakan `loadstring`, CLI akan menerima error eksplisit bahwa Remote Exec loadstring tidak tersedia. Dalam kondisi itu arbitrary Luau dari terminal belum bisa dieksekusi oleh plugin sampai environment Studio mengizinkannya.

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
- `--debug`: tambahkan heartbeat cepat serta detail scan, cache, payload, Git/watcher, dan error ke live log `riftsync-server.exe`.
- `--legacy-scan`: fallback periodic scan Go jika watcher event filesystem bermasalah.
- `--headless`: alias kompatibilitas pada `riftsync-server.exe`; console/headless sekarang sudah menjadi mode default.

App UI lokal memakai dark subtle glassmorphism untuk shell dan panel utama, dipadukan dengan kontrol flat berkontras tinggi:

- Sidebar project: pindah project, melihat status ring, Add Project, Start All, hover-expand, dan pin.
- `Overview`: status, sync root, tiga metric utama, aktivitas terakhir, dan Diagnostics yang dapat dibuka saat diperlukan.
- `History`: revision list + detail viewer dengan scroll internal.
- `Exec`: paste/run Remote Exec Luau, output, recent rerun, dan View read-only untuk source/result penuh.
- `Config`: Project dan Connection sebagai pengaturan utama; config path dan debug berada di Advanced, sedangkan remove registry tetap non-destruktif.

Semua panel membatasi kontennya sendiri. Path, error, atau command panjang di-wrap/ellipsis dan list memiliki internal scroll sehingga tidak memperlebar window. Tabs, sidebar, disclosures, dan modal dapat digunakan dengan keyboard; status selalu memiliki teks selain warna, dan animasi mengikuti preferensi reduced motion sistem.

Header app custom menggantikan title bar Windows bila frameless mode berhasil. Jika Win32 frameless gagal di mesin tertentu, app tetap jalan dengan title bar normal.

### Git Versioning + History Rev

- Watcher otomatis membuat repo Git di `sync_root` (`src/game`) saat server boot jika `git_versioning_enabled` aktif.
- Setiap revision yang terdeteksi watcher dibuat sebagai commit `RiftSync rev <rev>: <count> changes`.
- Rollback dilakukan lewat Git di folder `src/game`, misalnya checkout/revert commit yang sesuai kebutuhan.
- Endpoint `GET /history?limit=50` menampilkan daftar revision terbaru, dan `GET /history?rev=<rev>` menampilkan detail perubahan khusus revision itu.
- Viewer history utama ada di `riftsync.exe`. Widget Studio dibuat ringan dan hanya menampilkan hint bahwa history tersedia di app.

Untuk mengecek perubahan file game, jalankan Git dari folder `sync_root`, bukan dari repo tool RiftSyncPlugin kecuali memang sedang mengecek source tool ini:

```powershell
cd <sync_root>
git status
git diff
git diff --check
```

`git status` menampilkan file modified/untracked tanpa perlu `git add`. `git add` hanya memasukkan perubahan ke staging area. `git diff` menampilkan isi perubahan, sedangkan `git diff --check` hanya mengecek whitespace/patch issue dan bukan pengganti `git status`.

## Gunakan di Roblox Studio

1. Enable HTTP requests di Studio (Game Settings -> Security -> Allow HTTP Requests).
2. Install plugin dengan struktur hierarki di atas.
3. Buka toolbar `RiftSync`, isi host/port (default `127.0.0.1:8765`).
   - Profile Local dimuat ulang otomatis saat widget dibuka. Setelah menambah project di aplikasi RiftSync, klik tombol `↻` di samping pemilih profile jika widget masih terbuka.
4. Klik `Start Sync`. RiftSync selalu meminta sumber awal dan tidak menampilkan compare:
	- Opsional, aktifkan `Force ID repair` hanya untuk memulihkan error duplicate/corrupt StableId. Opsi ini berlaku satu kali untuk Start tersebut dan kembali `OFF` setelah dipakai.
	- `Use Studio`: Studio menjadi sumber awal dan mengganti isi folder lokal. Sebelum replace, backup lokal dibuat di `.rblxsync/backups/<timestamp>/`.
	- `Use Local`: folder lokal menjadi sumber awal dan diterapkan ke Studio. Perubahan awal dicatat sebagai `RiftSync: Start from Local` agar bisa di-Undo dari Studio.
	- `Cancel`: tidak mengubah Studio maupun folder lokal.
5. Setelah start awal selesai, arah live sync selalu `Local -> Studio`, apa pun sumber awal yang dipilih.
6. Save file di VS Code, perubahan akan otomatis muncul di Studio.
7. Jika ada error, status bar tampil ringkas dan detail lengkap muncul di panel `Output` Studio.
8. Klik `Pull Studio` untuk mirror Studio ke folder lokal. Widget akan menampilkan preview add/update/delete/unchanged, lalu kamu harus pilih `Confirm` atau `Cancel`.
9. Aktifkan `Debug` di widget untuk checklist live:
   - HTTP/server reachable
   - connected to server
   - token valid
   - sync active
   - exec active
   - edit mode
   plus tail event log dan ringkasan status server (`/debug/state`).

### Icon Plugin Studio

Toolbar button Studio memakai icon Roblox asset dari `plugin/TypeList.lua`:

```lua
TypeList.PLUGIN_ICON = "rbxassetid://72034413544662"
```

Jika ingin mengganti logo lagi, upload PNG baru ke Roblox lalu ganti asset id tersebut. Jika kosong, plugin tetap berjalan normal tapi toolbar button memakai icon default/no icon.

### Better Error Guidance

Widget menampilkan hint pendek untuk error umum:

- `Invalid JSON`: cek syntax JSON, koma, bracket, dan typed value.
- `Unsupported property`: cek nama/type property atau tambahkan ke `extra_allowed_properties`.
- `Unsupported class`: cek `className` dan `managed_roots`.
- `Unmanaged conflict`: upsert otomatis mengadopsi object existing bila path persis sama, class cocok, dan StableId tidak bertentangan. Class berbeda, sibling ambigu, rename/delete, atau ID berbeda tetap dihentikan agar object tidak tertukar.
- `Duplicate stable id`: ganti/hapus duplicate `id` di `properties.init.json`.
- Duplicate sibling dengan `Name` dan `ClassName` yang sama didukung memakai suffix stable ID internal (`~rid_...`) pada folder lokal. Suffix tersebut bukan bagian dari `Name` object di Roblox Studio.
- `Server offline / HTTP disabled`: jalankan `.\riftsync.exe` lalu klik **Start**, atau gunakan `.\riftsync-server.exe --headless` untuk debug terminal. Cek host/port dan aktifkan HTTP Requests di Studio.

### Troubleshooting Server

- Progress berhenti di fase **Uploading/indexing**: request Roblox masih berjalan sebagai satu HTTP request; desktop menampilkan progres penulisan server dan Studio memakai indikator indeterminate sampai respons kembali.
- Pilihan sumber selalu muncul setiap `Start Sync` dan tidak diingat otomatis. Pilih **Use Studio** bila perubahan terakhir ada di Studio; pilih **Use Local** hanya bila folder lokal memang harus menjadi sumber utama.
- Untuk error StableId duplikat, nyalakan **Force ID repair** lalu pilih sumber yang benar: **Use Studio** membuat ID baru dari object Studio lalu mengganti local dengan backup; **Use Local** mempertahankan ID dari file local dan memasangnya kembali ke object Studio. Folder ignored dan subtree rollback tidak disentuh. Bila bootstrap gagal, RiftSync mencoba mengembalikan ID Studio lama dan berhenti tanpa masuk live sync.
- Snapshot recovery milik tool lain diabaikan otomatis bila berupa child langsung `ServerStorage` dengan nama tersembunyi berpola `__*Rollback*` (tidak peka huruf besar/kecil). Salinan rollback tetap berada di Studio, tetapi atribut StableId di dalam backup tidak ikut dianggap sebagai identity aktif RiftSync.
- Metadata di bawah Model/Part yang belum ada di Studio dilewati sebagai warning teragregasi; RiftSync tidak membuat geometry pengganti.
- Port sudah dipakai: jalankan dengan `--port 8766`, lalu samakan port di widget Studio.
- HTTP Requests belum aktif: buka `Game Settings -> Security -> Allow HTTP Requests`.
- Firewall/security prompt Windows: izinkan local server untuk private network jika diminta.
- Host/port plugin tidak cocok: gunakan host `127.0.0.1` dan port yang sama dengan output server.
- Git tidak tersedia: install Git atau set `"git_versioning_enabled": false` di `sync_config.json`.
- Watcher tidak mendeteksi perubahan di folder network/cloud sync: jalankan `.\riftsync.exe --legacy-scan` atau `.\riftsync-server.exe --headless --legacy-scan`.

## Menjadikan RBXM

1. Di Studio, buat `Model` bernama `RiftSyncPluginModel`.
2. Pindahkan `RiftSyncPlugin` beserta child module bertingkat (`API` -> `TypeList` dan `Identity`) ke dalam model itu.
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
- Pull Studio memakai replace semantics: file lokal yang sudah tidak ada di Studio ikut dihapus dari folder lokal.
- Pull Studio selalu menampilkan preview add/update/delete/unchanged dan hanya melakukan replace setelah `Confirm`.
- `Cancel` pada preview tidak mengubah file lokal.
- Setelah `Confirm`, sebelum Pull Studio replace menghapus konten lokal, RiftSync membuat backup di `.rblxsync/backups/<timestamp>/` untuk konten yang akan dihapus/diganti. `.git`, `.rblxsync`, dan `.guidebook` tetap dipreservasi dan tidak dicopy ke backup.
- Root service (`StarterGui`, `Workspace`, `Lighting`, `ReplicatedStorage`, `ReplicatedFirst`) dan `Terrain` tetap diproteksi agar tidak ter-destroy salah.
- Untuk scope property sync (`StarterGui`, `Workspace`, `Lighting`), delete instance non-managed juga diproses (filesystem authoritative) supaya kasus folder terhapus tapi instance tetap nongol di Studio tidak terjadi lagi.
