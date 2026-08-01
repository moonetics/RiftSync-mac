# Plugin Source

Folder ini berisi source Lua produksi yang dipasang ke Roblox Studio.

Struktur Studio:

```text
RiftSyncPlugin (Script / PluginScript)
└─ API (ModuleScript)
   └─ TypeList (ModuleScript)
```

Mapping file:

- `RiftSyncPlugin.lua` -> `RiftSyncPlugin`
- `API.lua` -> `API`
- `TypeList.lua` -> `TypeList`

Catatan migrasi Go:

- File Lua ini tetap menjadi produksi.
- Migrasi awal Go server harus menjaga kontrak HTTP agar file ini tidak perlu diubah.
- Jika perlu eksperimen Lua, clone dari folder ini ke folder kerja terpisah.

## Connection Profiles

Widget RiftSync 4.1 menyimpan beberapa connection profile bernama. Setiap profile berisi `host`, `port`, dan Remote Exec `token` untuk satu project desktop. Profile yang dipilih disimpan per Roblox Place.

- Setting host/port/token versi lama otomatis dimigrasikan menjadi profile `Default`.
- Profile hanya dapat ditambah, diedit, dipilih, atau dihapus ketika sync berhenti.
- Minimal satu profile selalu dipertahankan.
- Editor profile dan Diagnostics tertutup secara default agar aksi sync utama tetap menjadi fokus.
- Widget menggunakan frosted surface ringan, kontrol flat berlabel, `ScrollingFrame`, `AutomaticCanvasSize`, dan layout otomatis agar panel tetap berada di dalam dock sempit.

## Broad Properties Metadata

`Studio -> Folder` mengekspor `properties.init.json` untuk class non-geometry yang tercantum dalam schema `TypeList.lua`, termasuk script, ValueBase, environment/effect, UI, audio, animation, visual effect, prompt, tool, attachment, dan constraint. `BasePart`/`Model` hanya menjadi anchor path; descendant yang didukung di bawahnya tetap diekspor.

- Script menulis `Enabled` dan, untuk `Script`, `RunContext`; `Disabled` lama hanya dipakai sebagai input kompatibilitas.
- `ObjectValue` dan property reference lain memakai `$type: "InstanceRef"`; `RayValue` memakai `$type: "Ray"`.
- Attributes dan tags ikut snapshot. Source script tetap dimiliki file `.server.luau`, `.client.luau`, atau `.module.luau`.
- Gunakan plugin dan server versi 4.1 bersama ketika project memakai `InstanceRef`.
