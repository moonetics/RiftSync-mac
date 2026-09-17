---
name: riftsync
description: >-
  Cheatsheet dan panduan operasional RiftSync untuk AI Agent: aturan struktur folder canonical
  (<Name>.<ClassName>), ekstensi script (.server.luau, .client.luau, .module.luau),
  pedoman penulisan script Luau & typechecking (--!nonstrict vs --!strict, pencegahan bug type narrowing, resolusi linter unused variable),
  format typed JSON properties (UDim2, Color3, Enum, sequences, tags, attributes),
  authoring UI Roblox responsif, validasi project, serta Remote Exec Luau langsung ke Roblox Studio dari root project.
---

# RiftSync AI Agent Cheatsheet & Operational Guide

Panduan lengkap bagi AI Agent saat membuat script, mendesain UI responsif, mengelola hierarki instance Roblox, dan melakukan inspeksi/eksekusi runtime di Roblox Studio menggunakan **RiftSync**.

Seluruh perintah CLI (`exec`, `validate`) dieksekusi langsung dari root workspace.

---

## 1. Arsitektur & Alur Kerja Sinkronisasi

1. **One-Way Sync (Lokal ➔ Roblox Studio)**:
   - File diedit di folder lokal yang dikonfigurasi di `sync_config.json` (`sync_root`).
   - Server Go mendeteksi perubahan via file watcher (`fsnotify` berbasis event kernel OS, misal `FSEvents` di macOS) secara real-time (~120ms debounce) dan mengirim delta ke plugin Roblox Studio.
   - Background safety reconciliation scan berjalan santai di latar belakang (default `safety_scan_interval_sec`: 30 detik) tanpa membebani CPU.
   - Plugin di Studio mengaplikasikan perubahan ke DataModel Studio secara real-time.
2. **Server & Plugin**:
   - Server Go berjalan lokal (default port `8765`).
   - Plugin Studio menghubungkan DataModel ke server via HTTP loopback.
   - Mode default adalah **Event-Driven Watcher** (CPU idle < 0.5%). Flag `--legacy-scan` hanya digunakan jika filesystem tidak mendukung event kernel OS (misal remote network mount).
3. **Remote Exec**:
   - Menjalankan kode Luau langsung di Roblox Studio yang sedang aktif via terminal root project menggunakan `./riftsync-server exec`.

---

## 2. Struktur Folder & Naming Convention Canonical

### A. Folder Instance: `<Name>.<ClassName>`
Setiap instance Roblox yang memiliki metadata properti atau children direpresentasikan sebagai folder dengan format:
```text
<Name>.<ClassName>/
```
*Contoh*:
- `MainGui.ScreenGui/`
- `HeaderFrame.Frame/`
- `CombatManager.Script/`
- `Network.Folder/`
- `Coins.IntValue/`
- `UIGradient.UIGradient/`

### B. Ekstensi File Script Canonical
| Tipe Instance Roblox | Ekstensi File Canonical | Konteks Eksekusi |
| :--- | :--- | :--- |
| **Script** | `<Name>.server.luau` | Server-side (Roblox Server) |
| **LocalScript** | `<Name>.client.luau` | Client-side (Player Device) |
| **ModuleScript** | `<Name>.module.luau` | Shared / Reusable Module |

*Aturan Script dengan Properties*:
Jika script membutuhkan pengaturan properti khusus (seperti `RunContext`, `Enabled: false`, tags, atau attributes), tempatkan file script di dalam folder instance bersama file `properties.init.json`:
```text
ServerScriptService/
└─ Bootstrap.Script/
   ├─ properties.init.json
   └─ Bootstrap.server.luau
```

### C. Pemetaan Root Services
Folder level teratas di dalam `sync_root` langsung memetakan ke Service Roblox:
- `Workspace`
- `ReplicatedStorage`
- `ServerScriptService`
- `ServerStorage`
- `StarterGui`
- `StarterPlayer` (`StarterPlayerScripts` & `StarterCharacterScripts`)
- `StarterPack`
- `Lighting`
- `SoundService`
- `TextChatService`

### D. Batasan Geometri (Safety Boundary)
- **BasePart & Model**: Bersifat *unmanaged*. Digunakan hanya sebagai segmen path hierarki (contoh: `Workspace/Map.Model/Spawner.Part/Config.Configuration`).
- **DILARANG** membuat `properties.init.json` untuk `BasePart` atau `Model` secara manual. RiftSync melindungi data geometri level agar tidak rusak atau tertimpa.
- **Tombstone**: Penghapusan file lokal dicatat di `.rblxsync/` agar instance lama di Studio dapat dihapus dengan aman dan sinkron.

---

## 3. Konvensi Penulisan Script Luau & Typechecking (Best Practices)

Untuk memastikan script Luau tidak memicu false-positive error atau warning yang mengganggu di VS Code / Antigravity IDE (via `luau-lsp`), ikuti pedoman berikut:

### A. Mode Typechecking di Header Script
Selalu cantumkan mode typechecking di baris pertama script:
1. **`--!nonstrict` (Rekomendasi Standar & Default)**:
   - Gunakan untuk hampir semua script gameplay, Service (`.server.luau`), Client Controller (`.client.luau`), dan UI logic.
   - Variabel tanpa tipe diperlakukan fleksibel (`any`) sehingga tidak memicu error palsu saat berinteraksi dinamis dengan hierarchy Roblox.
2. **`--!strict` (Hanya untuk Pure Type/Data Module)**:
   - Gunakan hanya untuk modul data, utility matematika, atau config schema yang murni komputasi dan tidak banyak mutasi state dinamis.
3. **`--!nocheck` (Nonaktifkan Typecheck Sepenuhnya)**:
   - Gunakan jika script memiliki banyak pemanggilan dinamis / refleksi dan Anda ingin mematikan type checker total (hanya memeriksa sintaks).
4. **HINDARI `--!nostrict`**:
   - Kata kunci `--!nostrict` **tidak valid** dalam spesifikasi Luau. Luau akan mengabaikannya dan kembali ke mode default. Selalu gunakan `--!nonstrict`.

### B. Mencegah Bug "Type Narrowing" pada Variabel State
Luau memiliki mekanisme *Control Flow Analysis* yang mempersempit tipe. Jika sebuah variabel diperiksa dengan guard:
```luau
if CurrentState ~= "IDLE" then return end
```
Luau mengira tipe `CurrentState` terkunci menjadi literal `"IDLE"`. Ketika diubah nilainya (`CurrentState = "COUNTDOWN"`) dan diperiksa lagi di dalam closure/loop:
```luau
-- ❌ SALAH: Jika CurrentState bertipe union literal ("IDLE" | "COUNTDOWN" | ...),
-- Luau memicu: TypeError: Type "IDLE" cannot be compared with "COUNTDOWN" Luau(1018)
while CountdownRemaining > 0 and CurrentState == "COUNTDOWN" do
```

**Solusi Standar RiftSync**:
- Deklarasikan variabel state dengan tipe primitif umum:
  ```luau
  -- ✅ BENAR: Gunakan string biasa atau any
  local CurrentState: string = "IDLE"
  ```
- Atau lakukan explicit type cast saat perbandingan di dalam closure:
  ```luau
  while CountdownRemaining > 0 and (CurrentState :: string) == "COUNTDOWN" do
  ```

### C. Menghindari Linter Warning "LocalUnused" (Garis Kuning)
Luau linter memberi garis bawah kuning (`LocalUnused Luau(7)`) pada variabel yang dideklarasikan atau diisi nilai tapi tidak pernah dibaca.
- Jika variabel sengaja disiapkan untuk implementasi lanjutan atau parameter opsional, beri awalan underscore `_`:
  ```luau
  local _CurrentDancer1Id: number? = nil
  local _IsAdminUser = false
  ```
- Untuk mematikan warning ini di seluruh workspace, pastikan `.vscode/settings.json` memiliki `"luau-lsp.lint.localUnused": false`.

---

## 4. Cheatsheet Format Typed Properties JSON (`properties.init.json`)

Setiap folder instance dapat memiliki file `properties.init.json` untuk mendefinisikan properti, attributes, dan tags.

### Struktur Dasar JSON
```json
{
  "id": "optional-stable-uuid",
  "className": "Frame",
  "name": "ShopFrame",
  "properties": {
    "Visible": true,
    "BackgroundTransparency": 0.1
  },
  "attributes": {
    "ShopType": "Weapons"
  },
  "tags": ["UIContainer", "Interactable"]
}
```

### Cheatsheet Tipe Data Typed Roblox

#### 1. `UDim2` & `UDim`
Format Shorthand Array:
```json
{
  "Size": { "UDim2": [0.8, 0, 0.6, 0] },
  "Position": { "UDim2": [0.5, 0, 0.5, 0] },
  "CornerRadius": { "UDim": [0, 12] }
}
```
Format Tagged Object:
```json
{
  "Size": {
    "$type": "UDim2",
    "xScale": 0.8,
    "xOffset": 0,
    "yScale": 0.6,
    "yOffset": 0
  },
  "CornerRadius": {
    "$type": "UDim",
    "scale": 0,
    "offset": 12
  }
}
```

#### 2. `Color3`
Format Shorthand Array `[R, G, B]` (0 - 255):
```json
{
  "BackgroundColor3": [30, 32, 36],
  "TextColor3": [255, 255, 255]
}
```
Format Tagged Object:
```json
{
  "BackgroundColor3": {
    "$type": "Color3",
    "r": 30,
    "g": 32,
    "b": 36
  }
}
```

#### 3. `Vector2` & `Vector3`
```json
{
  "AnchorPoint": { "Vector2": [0.5, 0.5] }
}
```
Atau format tagged:
```json
{
  "AnchorPoint": { "$type": "Vector2", "x": 0.5, "y": 0.5 },
  "Orientation": { "$type": "Vector3", "x": 0, "y": 90, "z": 0 }
}
```

#### 4. `Enum`
```json
{
  "RunContext": {
    "$type": "Enum",
    "enumType": "RunContext",
    "value": "Server"
  },
  "ZIndexBehavior": {
    "$type": "Enum",
    "enumType": "ZIndexBehavior",
    "value": "Sibling"
  },
  "TextXAlignment": {
    "$type": "Enum",
    "enumType": "TextXAlignment",
    "value": "Center"
  }
}
```

#### 5. `ColorSequence` (Untuk `UIGradient`, `Beam`, `ParticleEmitter`)
```json
{
  "Color": {
    "$type": "ColorSequence",
    "keypoints": [
      { "time": 0.0, "value": [255, 120, 50] },
      { "time": 1.0, "value": [160, 40, 220] }
    ]
  }
}
```

#### 6. `NumberSequence` & `NumberRange`
```json
{
  "Transparency": {
    "$type": "NumberSequence",
    "keypoints": [
      { "time": 0.0, "value": 0.0, "envelope": 0.0 },
      { "time": 1.0, "value": 1.0, "envelope": 0.0 }
    ]
  },
  "Lifetime": {
    "$type": "NumberRange",
    "min": 1.0,
    "max": 2.5
  }
}
```

#### 7. `Font`
```json
{
  "FontFace": {
    "$type": "Font",
    "family": "rbxasset://fonts/families/GothamSSm.json",
    "weight": "Bold",
    "style": "Normal"
  }
}
```

#### 8. `InstanceRef` (Referensi Objek Antar Instance)
```json
{
  "Value": {
    "$type": "InstanceRef",
    "path": "game.ReplicatedStorage.Network.Remotes"
  }
}
```
*Nil reference*: `{ "$type": "InstanceRef", "null": true }`.

#### 9. `Rect`
```json
{
  "$type": "Rect",
  "minX": 0,
  "minY": 0,
  "maxX": 100,
  "maxY": 100
}
```

---

## 5. Pola Authoring UI Responsif Roblox

Selalu buat antarmuka pengguna (UI) yang adaptif di berbagai resolusi layar (Mobile, Tablet, Desktop) dengan pola **AnchorPoint Center**:

### Struktur Contoh UI Toko (Shop):
```text
StarterGui/
└─ ShopScreen.ScreenGui/
   ├─ properties.init.json
   └─ MainContainer.Frame/
      ├─ properties.init.json
      ├─ UICorner.UICorner/
      │  └─ properties.init.json
      ├─ UIStroke.UIStroke/
      │  └─ properties.init.json
      ├─ Title.TextLabel/
      │  └─ properties.init.json
      └─ CloseButton.ImageButton/
         └─ properties.init.json
```

### Properti Penting:
1. **ScreenGui (`ShopScreen.ScreenGui/properties.init.json`)**:
   ```json
   {
     "className": "ScreenGui",
     "properties": {
       "ResetOnSpawn": false,
       "IgnoreGuiInset": true,
       "ZIndexBehavior": {
         "$type": "Enum",
         "enumType": "ZIndexBehavior",
         "value": "Sibling"
       }
     }
   }
   ```
2. **Centered Frame (`MainContainer.Frame/properties.init.json`)**:
   ```json
   {
     "className": "Frame",
     "properties": {
       "AnchorPoint": { "Vector2": [0.5, 0.5] },
       "Position": { "UDim2": [0.5, 0, 0.5, 0] },
       "Size": { "UDim2": [0.65, 0, 0.7, 0] },
       "BackgroundColor3": [24, 25, 28],
       "BorderSizePixel": 0
     }
   }
   ```
3. **Corner Smoothness (`UICorner.UICorner/properties.init.json`)**:
   ```json
   {
     "className": "UICorner",
     "properties": {
       "CornerRadius": { "UDim": [0, 14] }
     }
   }
   ```

---

## 6. Remote Exec: Eksekusi Luau di Roblox Studio dari Terminal

RiftSync memiliki CLI Remote Exec yang terhubung langsung ke sesi Roblox Studio aktif. AI Agent dapat menjalankan script, memeriksa hierarki DataModel, atau menjalankan unit test tanpa intervensi manual.

Semua perintah dijalankan langsung dari root project RiftSync:

### A. Perintah Inline Cepat
```bash
./riftsync-server exec "print(workspace.Name)"
```

### B. Eksekusi Script dari File
```bash
./riftsync-server exec --file ./scripts/test_runner.luau --timeout 15 --json
```

### C. Eksekusi Multi-line via Stdin
```bash
cat << 'EOF' | ./riftsync-server exec --stdin --timeout 15
local replicated = game:GetService("ReplicatedStorage")
local network = replicated:FindFirstChild("Network")
print("Network ready:", network ~= nil)
return network ~= nil
EOF
```

### D. Output JSON Terstruktur
Gunakan opsi `--json` untuk menangkap data terstruktur bagi AI:
```bash
./riftsync-server exec "return { name = workspace.Name, partCount = #workspace:GetChildren() }" --json
```
Format return JSON:
- `ok`: boolean (apakah script selesai tanpa error)
- `returns`: array data yang di-`return` oleh script
- `logs`: array output `print` dan `warn`
- `error`: pesan error runtime (jika gagal)
- `duration_ms`: waktu eksekusi dalam milidetik

### E. Exit Codes
- `0`: Berhasil (Success)
- `1`: Runtime error pada kode Luau di Studio
- `2`: Parameter CLI tidak valid / file tidak ditemukan
- `3`: Server offline / Remote Exec disabled / token tidak cocok
- `4`: Eksekusi timed out

### Resep Praktis Remote Exec untuk AI Agent:
1. **Verifikasi Apakah Instance Sudah Tersinkron**:
   ```bash
   ./riftsync-server exec "return game:GetService('ReplicatedStorage'):FindFirstChild('Network') ~= nil" --json
   ```
2. **Inspeksi Runtime Object / Attribute**:
   ```bash
   ./riftsync-server exec "local obj = workspace:FindFirstChild('Map'); return obj and obj:GetAttributes()" --json
   ```
3. **Menjalankan Unit Test Modul**:
   ```bash
   ./riftsync-server exec "local mod = require(game.ReplicatedStorage.Modules.Calculator); return mod.add(2, 3) == 5" --json
   ```

---

## 7. Validasi Project

Sebelum melakukan sinkronisasi atau saat memeriksa keutuhan metadata project, jalankan `validate` dari root project:

```bash
./riftsync-server validate
```
Atau dalam format JSON:
```bash
./riftsync-server validate --json
```

Validasi ini memeriksa:
- Ketersediaan folder `sync_root`.
- Kesalahan sintaks pada seluruh file JSON properti.
- Konflik duplikasi Stable ID antar instance.
- Path traversal atau format path tidak valid.
