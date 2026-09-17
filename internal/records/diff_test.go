package records

import "testing"

func scriptRecord(localPath, rbxPath, source string) SyncRecord {
	return SyncRecord{
		Entity:      EntityScript,
		LocalPath:   localPath,
		LocalDir:    "ServerScriptService",
		RbxPath:     rbxPath,
		ClassName:   "Script",
		Source:      source,
		ContentHash: ContentDigest(source),
	}
}

func uiRecord(localPath, rbxPath, stableID string) SyncRecord {
	return SyncRecord{
		Entity:      EntityUIInstance,
		LocalPath:   localPath,
		LocalDir:    "StarterGui",
		RbxPath:     rbxPath,
		ClassName:   "ScreenGui",
		StableID:    stableID,
		ContentHash: ContentDigest(stableID),
		Payload: map[string]any{
			"id":        stableID,
			"className": "ScreenGui",
			"name":      "Main",
		},
	}
}

func TestDetectChangesAdd(t *testing.T) {
	changes := DetectChanges(nil, map[string]SyncRecord{
		"ServerScriptService/Foo.server.luau": scriptRecord("ServerScriptService/Foo.server.luau", "game.ServerScriptService.Foo", "print(1)"),
	}, nil)
	if len(changes) != 1 || changes[0]["op"] != "upsert" {
		t.Fatalf("changes = %#v, want one upsert", changes)
	}
}

func TestDetectChangesModify(t *testing.T) {
	oldRecord := scriptRecord("ServerScriptService/Foo.server.luau", "game.ServerScriptService.Foo", "print(1)")
	newRecord := scriptRecord("ServerScriptService/Foo.server.luau", "game.ServerScriptService.Foo", "print(2)")
	changes := DetectChanges(map[string]SyncRecord{oldRecord.LocalPath: oldRecord}, map[string]SyncRecord{newRecord.LocalPath: newRecord}, nil)
	if len(changes) != 1 || changes[0]["op"] != "upsert" {
		t.Fatalf("changes = %#v, want one upsert", changes)
	}
}

func TestDetectChangesDelete(t *testing.T) {
	oldRecord := scriptRecord("ServerScriptService/Foo.server.luau", "game.ServerScriptService.Foo", "print(1)")
	changes := DetectChanges(map[string]SyncRecord{oldRecord.LocalPath: oldRecord}, nil, nil)
	if len(changes) != 1 || changes[0]["op"] != "delete" {
		t.Fatalf("changes = %#v, want one delete", changes)
	}
}

func TestDetectChangesStableIDRename(t *testing.T) {
	oldRecord := uiRecord("StarterGui/Main.ScreenGui/properties.init.json", "game.StarterGui.Main", "main")
	newRecord := uiRecord("StarterGui/Menu.ScreenGui/properties.init.json", "game.StarterGui.Menu", "main")
	changes := DetectChanges(map[string]SyncRecord{oldRecord.LocalPath: oldRecord}, map[string]SyncRecord{newRecord.LocalPath: newRecord}, nil)
	if len(changes) != 1 || changes[0]["op"] != "rename" {
		t.Fatalf("changes = %#v, want one rename", changes)
	}
}

func TestDetectChangesHashRename(t *testing.T) {
	oldRecord := scriptRecord("ServerScriptService/Foo.server.luau", "game.ServerScriptService.Foo", "print(1)")
	newRecord := scriptRecord("ServerScriptService/Bar.server.luau", "game.ServerScriptService.Bar", "print(1)")
	changes := DetectChanges(map[string]SyncRecord{oldRecord.LocalPath: oldRecord}, map[string]SyncRecord{newRecord.LocalPath: newRecord}, nil)
	if len(changes) != 1 || changes[0]["op"] != "rename" {
		t.Fatalf("changes = %#v, want one rename", changes)
	}
}

func TestDetectChangesUnchanged(t *testing.T) {
	record := scriptRecord("ServerScriptService/Foo.server.luau", "game.ServerScriptService.Foo", "print(1)")
	changes := DetectChanges(map[string]SyncRecord{record.LocalPath: record}, map[string]SyncRecord{record.LocalPath: record}, nil)
	if len(changes) != 0 {
		t.Fatalf("changes = %#v, want none", changes)
	}
}

func TestDetectChangesPreservesDuplicateRobloxPathsByStableID(t *testing.T) {
	left := uiRecord(
		"StarterGui/Same~rid_left.ScreenGui/properties.init.json",
		"game.StarterGui.Same",
		"left",
	)
	right := uiRecord(
		"StarterGui/Same~rid_right.ScreenGui/properties.init.json",
		"game.StarterGui.Same",
		"right",
	)
	changes := DetectChanges(nil, map[string]SyncRecord{
		left.LocalPath:  left,
		right.LocalPath: right,
	}, nil)
	if len(changes) != 2 {
		t.Fatalf("changes = %#v, want two duplicate-name upserts", changes)
	}
}
