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
