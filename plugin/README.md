# Plugin Source

Folder ini berisi source Lua produksi yang dipasang ke Roblox Studio.

Struktur Studio:

```text
RiftSyncPlugin (Script / PluginScript)
└── API (ModuleScript)
    ├── TypeList (ModuleScript)
    └── Identity (ModuleScript)
```

Mapping file:

- `RiftSyncPlugin.lua` -> `RiftSyncPlugin`
- `API.lua` -> `API` (child dari `RiftSyncPlugin`)
- `TypeList.lua` -> `TypeList` (child dari `API`)
- `Identity.lua` -> `Identity` (child dari `API`, sejajar dengan `TypeList`)

Catatan migrasi Go:

- File Lua ini tetap menjadi produksi.
- Migrasi awal Go server harus menjaga kontrak HTTP agar file ini tidak perlu diubah.
- Jika perlu eksperimen Lua, clone dari folder ini ke folder kerja terpisah.

## Connection Profiles

Widget RiftSync membedakan dua jenis connection profile. Profile yang dipilih tetap disimpan per Roblox Place.

- **Local** diambil otomatis dari daftar project aplikasi RiftSync melalui endpoint discovery loopback `127.0.0.1:8749`. Nama, host, port, status, dan Remote Exec token mengikuti aplikasi sehingga tidak diedit ulang di Studio.
- **Custom** disimpan oleh plugin untuk server remote atau alamat manual dan tetap dapat ditambah, diedit, serta dihapus ketika sync berhenti.
- Setting host/port/token versi lama otomatis dimigrasikan menjadi profile Custom `Default`.
- Daftar Local diperbarui saat widget dibuka atau tombol **Refresh Local** ditekan. Endpoint discovery hanya menerima request dari loopback dan bersifat read-only.
- Editor profile dan Diagnostics tertutup secara default agar aksi sync utama tetap menjadi fokus.
- Widget menggunakan frosted surface ringan, kontrol flat berlabel, `ScrollingFrame`, `AutomaticCanvasSize`, dan layout otomatis agar panel tetap berada di dalam dock sempit.

## Broad Properties Metadata

`Studio -> Folder` mengekspor `properties.init.json` untuk class non-geometry yang tercantum dalam schema `TypeList.lua`, termasuk script, ValueBase, environment/effect, UI, audio, animation, visual effect, prompt, tool, attachment, dan constraint. `BasePart`/`Model` hanya menjadi anchor path; descendant yang didukung di bawahnya tetap diekspor.

- Script menulis `Enabled` dan, untuk `Script`, `RunContext`; `Disabled` lama hanya dipakai sebagai input kompatibilitas.
- `ObjectValue` dan property reference lain memakai `$type: "InstanceRef"`; `RayValue` memakai `$type: "Ray"`.
- Attributes dan tags ikut snapshot. Source script tetap dimiliki file `.server.luau`, `.client.luau`, atau `.module.luau`.
- Gunakan plugin dan server versi 4.1 bersama ketika project memakai `InstanceRef`.
