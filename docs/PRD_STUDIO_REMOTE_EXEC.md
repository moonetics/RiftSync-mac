# PRD: RiftSync Studio Remote Exec

Dokumen ini mendefinisikan fitur untuk menjalankan command Luau multi-line dari terminal lokal ke Roblox Studio melalui RiftSync, lalu mengembalikan outputnya ke terminal lokal.

## Ringkasan

RiftSync saat ini sudah punya local Go server dan plugin Studio yang berkomunikasi lewat HTTP JSON. Fitur ini menambahkan jalur command queue agar AI, developer, atau terminal lokal bisa mengirim Luau command ke Studio tanpa mengetik manual di Command Bar.

Target pengalaman:

```powershell
@'
local part = workspace:WaitForChild("MyPart", 10)
if not part then
    error("MyPart tidak ketemu")
end

print("Found:", part:GetFullName())
part.Name = "RenamedFromLocal"

return part.Name, part.ClassName
'@ | .\riftsync-server.exe exec --stdin --timeout 15
```

Output terminal:

```text
[print] Found: Workspace.MyPart
[return] RenamedFromLocal, Part
[ok] 128ms
```

Fitur ini bukan kontrol UI Command Bar Roblox secara literal. Plugin menjalankan Luau di Studio context dengan behavior yang dibuat mirip Command Bar dan lebih cocok untuk automation lokal.

## Tujuan

- Mengizinkan command Luau multi-line dari terminal lokal dieksekusi di Roblox Studio.
- Mendukung input command dari argument, stdin, dan file `.luau`.
- Mengembalikan `print`, `warn`, return values, error, dan durasi eksekusi ke terminal lokal.
- Mendukung command yang yield, misalnya `WaitForChild`, `task.wait`, atau operasi async pendek.
- Menyediakan timeout agar terminal tidak menggantung tanpa batas.
- Menjaga fitur default tetap aman dengan explicit enable, token, dan host local-only.
- Mempertahankan kontrak sync RiftSync yang sudah ada tanpa mengganggu `/changes`, `/snapshot`, `/bootstrap`, dan `/ack`.

## Non-Tujuan

- Tidak mengontrol atau mengetik ke UI Command Bar Roblox secara langsung.
- Tidak menjalankan command ke server production Roblox.
- Tidak membuka akses jaringan dari perangkat lain.
- Tidak menggantikan sistem test runner Roblox.
- Tidak mendukung eksekusi runtime server saat Play mode pada MVP, kecuali diputuskan sebagai fase lanjutan.
- Tidak mencoba menangkap semua Output window Studio global pada MVP.

## Persona dan Use Case

Persona utama:

- Developer lokal yang ingin menjalankan inspeksi cepat di Studio dari terminal.
- AI coding agent yang punya akses terminal lokal dan perlu memeriksa/memodifikasi state Studio.
- Power user RiftSync yang ingin workflow automation tanpa bolak-balik ke Command Bar.

Use case utama:

- Inspect instance:

```luau
local folder = game.ReplicatedStorage:WaitForChild("Modules", 10)
print(folder:GetFullName(), #folder:GetChildren())
return folder.Name
```

- Membuat instance sementara:

```luau
local marker = Instance.new("Folder")
marker.Name = "GeneratedFromLocal"
marker.Parent = workspace
print("created", marker:GetFullName())
return marker:GetFullName()
```

- Debug path hasil sync:

```luau
local target = game.ServerScriptService:FindFirstChild("Foo")
if not target then
    error("Foo script missing")
end
print(target.ClassName)
return target.Source:sub(1, 120)
```

## Arsitektur

```text
Terminal / AI
    |
    | POST command, wait result
    v
riftsync local Go server
    |
    | queued command, result store
    v
Roblox Studio RiftSync plugin
    |
    | poll command, execute Luau, post result
    v
riftsync local Go server
    |
    | print result
    v
Terminal / AI
```

Alasan memakai polling:

- Roblox Studio plugin bisa melakukan HTTP request ke local server.
- Local terminal tidak bisa mem-push langsung ke Studio karena Studio tidak expose HTTP listener untuk plugin.
- Pola polling konsisten dengan arsitektur RiftSync saat ini.

## Komponen

### Go Server

Package utama:

- `internal/state`: menyimpan queue command, status, result, dan waiter.
- `internal/httpapi`: endpoint command queue dan result.
- `cmd/riftsync-server`: subcommand CLI `exec`.
- Opsional fase lanjutan: `cmd/riftsync-app` menampilkan toggle/status remote exec di UI.

Tanggung jawab:

- Menerima command dari terminal lokal.
- Memberi command ID unik.
- Menyimpan command dalam queue sampai diambil plugin.
- Menunggu result dengan timeout.
- Mengembalikan output terstruktur ke CLI.
- Membersihkan command lama dari memory.

### Studio Plugin

File utama:

- `plugin/TypeList.lua`: endpoint constants dan default setting.
- `plugin/API.lua`: polling command, executor, result poster.
- `plugin/RiftSyncPlugin.lua`: UI toggle enable/disable remote exec.

Tanggung jawab:

- Hanya polling command jika sync berjalan dan remote exec di-enable.
- Menolak eksekusi saat Play mode untuk MVP.
- Menjalankan command di task terpisah.
- Capture `print`, `warn`, return values, dan error traceback.
- Kirim hasil ke server lokal.

## Endpoint HTTP

Semua endpoint hanya tersedia di local RiftSync server.

### `POST /exec/commands`

Dipakai terminal/CLI untuk submit command.

Request:

```json
{
  "source": "print(workspace.Name)\nreturn workspace.Name",
  "timeout_sec": 15,
  "client": "riftsync-cli",
  "mode": "edit"
}
```

Response:

```json
{
  "status": "ok",
  "command_id": "cmd_01H...",
  "queued_at": 1710000000.123
}
```

Validation:

- `source` wajib string dan tidak kosong.
- `source` maksimal default 256 KB pada MVP.
- `timeout_sec` default 10, minimum 1, maksimum 120.
- Server menolak request jika remote exec belum di-enable di config atau runtime state.

### `GET /exec/commands/next?client_id=...&timeout=25`

Dipakai plugin untuk long-poll command berikutnya.

Response saat ada command:

```json
{
  "status": "ok",
  "command": {
    "id": "cmd_01H...",
    "source": "print(workspace.Name)",
    "timeout_sec": 15,
    "created_at": 1710000000.123
  }
}
```

Response saat timeout tanpa command:

```json
{
  "status": "ok",
  "command": null
}
```

Behavior:

- Hanya satu plugin client boleh claim satu command.
- Command yang sudah di-claim masuk status `running`.
- Jika plugin mati sebelum result, command menjadi `expired` setelah timeout.

### `POST /exec/commands/result`

Dipakai plugin untuk mengirim hasil.

Request:

```json
{
  "command_id": "cmd_01H...",
  "client_id": "studio-plugin-...",
  "ok": true,
  "duration_ms": 128,
  "prints": [
    {
      "level": "print",
      "text": "Found: Workspace.MyPart",
      "at_ms": 10
    }
  ],
  "returns": [
    "RenamedFromLocal",
    "Part"
  ],
  "error": "",
  "traceback": ""
}
```

Response:

```json
{
  "status": "ok"
}
```

### `GET /exec/commands/{id}`

Dipakai CLI untuk polling result jika tidak memakai long wait internal.

Response:

```json
{
  "status": "ok",
  "command": {
    "id": "cmd_01H...",
    "state": "done",
    "ok": true,
    "duration_ms": 128,
    "prints": [],
    "returns": ["Workspace"],
    "error": "",
    "traceback": ""
  }
}
```

Status command:

- `queued`
- `running`
- `done`
- `error`
- `timeout`
- `expired`
- `cancelled`

## CLI

Subcommand MVP di `riftsync-server.exe`:

```powershell
.\riftsync-server.exe exec "print(workspace.Name)"
.\riftsync-server.exe exec --stdin --timeout 15
.\riftsync-server.exe exec --file .\studio-command.luau --timeout 30
```

Flag:

- `--config sync_config.json`
- `--host 127.0.0.1`
- `--port 8765`
- `--timeout 10`
- `--stdin`
- `--file path.luau`
- `--json`
- `--raw`

Output default:

```text
[print] hello
[warn] careful
[return] value1, value2
[ok] 128ms
```

Output error:

```text
[print] before failure
[error] MyPart tidak ketemu
[traceback] ...
```

Exit code:

- `0`: command selesai dan `ok=true`.
- `1`: command dieksekusi tapi error.
- `2`: CLI argument/config invalid.
- `3`: server offline atau remote exec disabled.
- `4`: timeout menunggu Studio/plugin.

## Executor Studio

Eksekusi MVP:

- Wrap source dalam function agar `return` multi-value bisa ditangkap.
- Jalankan di task thread terpisah.
- Override `print` dan `warn` di environment command.
- Pakai `xpcall` untuk capture traceback.
- Convert return values ke string/JSON-safe values.

Pseudo flow:

```luau
local prints = {}
local env = makeCommandEnvironment(prints)
local fn, compileError = loadstring("return function()\n" .. source .. "\nend")
setfenv(fn, env)
local commandFn = fn()
local ok, resultOrError = xpcall(function()
    return pack(commandFn())
end, debug.traceback)
postResult(commandId, ok, prints, returnsOrError)
```

Catatan:

- Detail exact `loadstring`/environment mengikuti kemampuan Luau Studio saat implementasi.
- Jika `loadstring` tidak tersedia di plugin context pada versi Studio tertentu, fallback MVP harus ditentukan sebelum coding besar.
- Command bisa yield selama dijalankan di thread yang mendukung yielding.

Environment command minimal:

- Global umum Luau.
- `game`, `workspace`, `script`, `plugin`.
- `Instance`, `Enum`, `Color3`, `Vector3`, `UDim2`, dan tipe Roblox lain dari global Studio.
- `print` dan `warn` yang dicapture.

Pembatasan MVP:

- Tidak menjamin sandbox kuat. Ini trusted local execution.
- Tidak menjalankan saat Play mode.
- Tidak streaming real-time kecuali fase lanjutan.

## Security

Fitur ini adalah remote code execution lokal ke Roblox Studio. Karena itu harus aman secara default.

Requirement:

- Default disabled.
- Toggle eksplisit di plugin UI: `Allow Local Exec`.
- Config server: `remote_exec_enabled`.
- Host tetap dibatasi `127.0.0.1` atau `localhost`.
- Auth token random disimpan lokal dan dikirim oleh CLI/plugin.
- Token tidak ditulis ke log normal.
- Endpoint exec menolak request tanpa token valid.
- Plugin menampilkan status ketika remote exec aktif.
- Command result mencatat client dan timestamp di debug events.

Config tambahan:

```json
{
  "remote_exec_enabled": false,
  "remote_exec_token": "",
  "remote_exec_max_source_bytes": 262144,
  "remote_exec_default_timeout_sec": 10,
  "remote_exec_max_timeout_sec": 120
}
```

Token behavior:

- Jika `remote_exec_enabled=true` dan token kosong, server generate token dan simpan ke config atau metadata lokal.
- CLI membaca config yang sama.
- Plugin menerima token dari setting/manual field atau dari handshake response jika diputuskan aman.

Open decision:

- Apakah token dikirim via `/handshake` hanya setelah plugin toggle enabled, atau user paste token ke widget?

## UI Plugin

Tambahan minimal di widget:

- Toggle `Local Exec OFF/ON`.
- Status kecil: `Exec disabled`, `Exec ready`, `Exec running`, `Exec error`.
- Warning text pendek saat enable pertama kali.

Behavior:

- Saat OFF, plugin tidak poll `/exec/commands/next`.
- Saat ON, plugin poll command hanya setelah handshake sukses.
- Saat sync stopped, command tidak dieksekusi.
- Saat Play mode, command ditolak dengan error jelas.

## Debug dan Observability

`/debug/state` menambahkan:

- `remote_exec_enabled`
- `remote_exec_queue_count`
- `remote_exec_running_count`
- `remote_exec_completed_count`
- `remote_exec_error_count`
- `remote_exec_last_command_at`
- `remote_exec_last_result_status`

Debug event examples:

- `exec queued id=cmd_... bytes=120 timeout=15`
- `exec claimed id=cmd_... client=studio-plugin`
- `exec done id=cmd_... ok=true duration=128ms`
- `exec error id=cmd_... message=...`

## Streaming Output

MVP boleh non-streaming:

- Terminal baru menerima output setelah command selesai.
- Cukup untuk inspect, modify, dan debug pendek.

Fase lanjutan:

- Plugin POST incremental logs ke `/exec/commands/log`.
- CLI menampilkan log real-time.
- Result akhir tetap dikirim lewat `/exec/commands/result`.

## Timeout dan Cancellation

MVP:

- CLI wait timeout terpisah dari Studio execution timeout.
- Server menandai command `timeout` jika result tidak masuk tepat waktu.
- Plugin tetap berusaha mengirim result; server boleh menerima sebagai late result dengan status `expired_late`.

Fase lanjutan:

- `POST /exec/commands/{id}/cancel`.
- Plugin mengecek cancellation cooperative di sela yield.

## Compatibility

Fitur tidak boleh mengubah behavior:

- `GET /changes`
- `GET /snapshot`
- `POST /bootstrap`
- `POST /ack`
- Pull Studio ke local
- History revision
- Git versioning

Remote exec harus tetap bekerja saat tidak ada perubahan file sync.

## Acceptance Criteria

MVP dianggap selesai jika:

- `go test ./...` lulus.
- `riftsync-server.exe exec "print(workspace.Name)"` mengirim command ke Studio dan menampilkan output di terminal.
- `--stdin` mendukung source multi-line dengan `return`.
- `--file` membaca file `.luau`.
- Command dengan `WaitForChild("Missing", 1)` selesai setelah timeout Luau dan mengembalikan error/return sesuai source.
- `print`, `warn`, return values, compile error, runtime error, dan traceback tampil di terminal.
- CLI exit code mengikuti status command.
- Remote exec default OFF.
- Endpoint exec menolak request tanpa token valid saat auth aktif.
- Plugin tidak menjalankan command saat sync belum start.
- Plugin tidak menjalankan command saat Play mode pada MVP.
- Existing sync workflow tetap berjalan.

## Test Plan

Go unit tests:

- Submit command valid.
- Reject command kosong.
- Reject source melebihi max bytes.
- Claim command satu kali.
- Result mengubah state ke `done` atau `error`.
- Wait result timeout.
- Expire command running tanpa result.
- Auth token required.

HTTP API tests:

- `POST /exec/commands`.
- `GET /exec/commands/next`.
- `POST /exec/commands/result`.
- `GET /exec/commands/{id}`.
- Debug state includes exec metrics.

CLI tests:

- Parse inline source.
- Parse `--stdin`.
- Parse `--file`.
- Format output default.
- Format output `--json`.
- Exit code mapping.

Manual Studio tests:

- Enable remote exec toggle.
- Run inline `print`.
- Run multi-line stdin.
- Run command that creates an Instance.
- Run command with runtime error.
- Run command while Play mode is active and verify rejection.
- Stop sync and verify command remains queued or returns clear unavailable status.

## Rollout Plan

Phase 1: PRD and API shape

- Finalize endpoint names, state model, and CLI UX.
- Decide token flow.

Phase 2: Go server queue and API

- Add command state types and queue.
- Add HTTP endpoints and tests.
- Add debug metrics.

Phase 3: CLI

- Add `exec` subcommand.
- Support inline, stdin, file, timeout, json output.

Phase 4: Plugin polling and executor

- Add endpoint constants.
- Add remote exec toggle.
- Add polling loop.
- Add command executor and result POST.

Phase 5: Hardening

- Auth token.
- Cleanup old commands.
- Better error formatting.
- Docs and README examples.

Phase 6: Optional streaming

- Incremental log endpoint.
- `--stream` output.

## Risiko

- `loadstring` availability di plugin context bisa berubah antar versi Studio.
- Command yang yield terlalu lama bisa membuat result terlambat.
- Remote code execution berbahaya jika token/host tidak dibatasi.
- Capturing output tidak sama persis dengan Output window Studio.
- Runtime Play mode membutuhkan desain berbeda dari MVP.

## Open Questions

- Token sebaiknya disimpan di `sync_config.json` atau metadata `.rblxsync`?
- Apakah plugin toggle cukup, atau server config juga harus wajib enabled?
- Apakah CLI `exec` masuk ke `riftsync-server.exe`, atau dibuat binary baru `riftsync-cli.exe`?
- Apakah return values perlu JSON-safe detail untuk Roblox types seperti `Instance`, `Vector3`, dan `Color3`, atau cukup string dulu?
- Apakah command yang sudah timeout boleh tetap apply perubahan di Studio, atau perlu cancellation cooperative sejak MVP?
