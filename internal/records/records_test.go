package records

import (
	"strings"
	"testing"
)

func TestApplyScriptPayloadCopiesStableID(t *testing.T) {
	record := buildScriptRecord("ServerScriptService/Foo.Script/Foo.server.luau", "game.ServerScriptService.Foo", "Script", "print(1)")
	payload := map[string]any{"id": "script-stable-id", "className": "Script", "name": "Foo"}
	if err := record.ApplyScriptPayload(payload); err != nil {
		t.Fatalf("ApplyScriptPayload returned error: %v", err)
	}
	if record.StableID != "script-stable-id" {
		t.Fatalf("StableID = %q, want script-stable-id", record.StableID)
	}
}

var managedServices = map[string]bool{
	"ServerScriptService": true,
	"StarterGui":          true,
	"Workspace":           true,
}

func TestNamedScriptPathMapsToRbxPath(t *testing.T) {
	record, err := NewScriptRecord("ServerScriptService/Foo.server.luau", "print('hi')", managedServices, nil)
	if err != nil {
		t.Fatalf("NewScriptRecord returned error: %v", err)
	}
	if record == nil {
		t.Fatal("record is nil")
	}
	if record.RbxPath != "game.ServerScriptService.Foo" {
		t.Fatalf("RbxPath = %q, want game.ServerScriptService.Foo", record.RbxPath)
	}
	if record.ClassName != "Script" {
		t.Fatalf("ClassName = %q, want Script", record.ClassName)
	}
}

func TestCanonicalScriptPathMapsToRbxPath(t *testing.T) {
	record, err := NewScriptRecord("StarterGui/Main.ScreenGui/Loader.LocalScript/source.client.luau", "print('hi')", managedServices, nil)
	if err != nil {
		t.Fatalf("NewScriptRecord returned error: %v", err)
	}
	if record == nil {
		t.Fatal("record is nil")
	}
	if record.RbxPath != "game.StarterGui.Main.Loader" {
		t.Fatalf("RbxPath = %q, want game.StarterGui.Main.Loader", record.RbxPath)
	}
	if record.ClassName != "LocalScript" {
		t.Fatalf("ClassName = %q, want LocalScript", record.ClassName)
	}
}

func TestLegacyScriptPathMapsToFolderRbxPath(t *testing.T) {
	record, err := NewScriptRecord("Workspace/Part.Script/Part.server.lua", "print('hi')", managedServices, nil)
	if err != nil {
		t.Fatalf("NewScriptRecord returned error: %v", err)
	}
	if record == nil {
		t.Fatal("record is nil")
	}
	if record.RbxPath != "game.Workspace.Part" {
		t.Fatalf("RbxPath = %q, want game.Workspace.Part", record.RbxPath)
	}
}

func TestIgnoredScriptReturnsNil(t *testing.T) {
	record, err := NewScriptRecord(
		"ServerScriptService/RiftSyncPlugin.server.luau",
		"print('ignore')",
		managedServices,
		[]string{"game.ServerScriptService.RiftSyncPlugin"},
	)
	if err != nil {
		t.Fatalf("NewScriptRecord returned error: %v", err)
	}
	if record != nil {
		t.Fatalf("record = %#v, want nil", record)
	}
}

func TestPayloadHelpers(t *testing.T) {
	record, err := NewScriptRecord("ServerScriptService/Foo.server.luau", "print('hi')", managedServices, nil)
	if err != nil {
		t.Fatalf("NewScriptRecord returned error: %v", err)
	}
	if err := record.ApplyScriptPayload(map[string]any{"properties": map[string]any{"Disabled": true}}); err != nil {
		t.Fatalf("ApplyScriptPayload returned error: %v", err)
	}

	upsert := record.ToUpsert()
	if upsert["op"] != "upsert" || upsert["source"] != "print('hi')" {
		t.Fatalf("upsert payload = %#v", upsert)
	}
	if upsert["payload"] == nil {
		t.Fatalf("upsert payload missing script properties: %#v", upsert)
	}
	if !strings.HasPrefix(upsert["content_hash"].(string), "sha256:") {
		t.Fatalf("content_hash = %q, want sha256 prefix", upsert["content_hash"])
	}

	deleted := record.ToDelete()
	if deleted["op"] != "delete" || deleted["rbx_path"] != "game.ServerScriptService.Foo" {
		t.Fatalf("delete payload = %#v", deleted)
	}

	renamed := *record
	renamed.LocalPath = "ServerScriptService/Bar.server.luau"
	renamed.RbxPath = "game.ServerScriptService.Bar"
	renamePayload := BuildRenameChange(*record, renamed)
	if renamePayload["op"] != "rename" || renamePayload["old_rbx_path"] != "game.ServerScriptService.Foo" || renamePayload["new_rbx_path"] != "game.ServerScriptService.Bar" {
		t.Fatalf("rename payload = %#v", renamePayload)
	}
}

func TestScriptPropertiesPathHelpers(t *testing.T) {
	sourcePath, ok, err := ScriptSourcePathForPropertiesPath("ServerScriptService/Foo.Script/properties.init.json")
	if err != nil {
		t.Fatalf("ScriptSourcePathForPropertiesPath returned error: %v", err)
	}
	if !ok || sourcePath != "ServerScriptService/Foo.Script/Foo.server.luau" {
		t.Fatalf("sourcePath=%q ok=%t, want Foo.server.luau true", sourcePath, ok)
	}

	propertiesPath, ok, err := ScriptPropertiesPathForSourcePath("ServerScriptService/Foo.Script/Foo.server.luau")
	if err != nil {
		t.Fatalf("ScriptPropertiesPathForSourcePath returned error: %v", err)
	}
	if !ok || propertiesPath != "ServerScriptService/Foo.Script/properties.init.json" {
		t.Fatalf("propertiesPath=%q ok=%t, want properties.init.json true", propertiesPath, ok)
	}
}

func TestDisambiguatedScriptPathKeepsRobloxNameAndStableID(t *testing.T) {
	localPath := "Workspace/Model~rid_parent-id.Model/LightConfig~rid_script-id.Script/LightConfig~rid_script-id.server.luau"
	record, err := NewScriptRecord(localPath, "print('duplicate-safe')", managedServices, nil)
	if err != nil {
		t.Fatalf("NewScriptRecord returned error: %v", err)
	}
	if record == nil {
		t.Fatal("record is nil")
	}
	if record.RbxPath != "game.Workspace.Model.LightConfig" || record.StableID != "script-id" {
		t.Fatalf("record path/id = %q/%q", record.RbxPath, record.StableID)
	}
	propertiesPath, ok, err := ScriptPropertiesPathForSourcePath(localPath)
	if err != nil || !ok {
		t.Fatalf("ScriptPropertiesPathForSourcePath = %q/%t/%v", propertiesPath, ok, err)
	}
	wantProperties := "Workspace/Model~rid_parent-id.Model/LightConfig~rid_script-id.Script/properties.init.json"
	if propertiesPath != wantProperties {
		t.Fatalf("propertiesPath = %q, want %q", propertiesPath, wantProperties)
	}
	sourcePath, ok, err := ScriptSourcePathForPropertiesPath(propertiesPath)
	if err != nil || !ok || sourcePath != localPath {
		t.Fatalf("source path roundtrip = %q/%t/%v, want %q", sourcePath, ok, err, localPath)
	}
}

func TestStableID(t *testing.T) {
	tests := []map[string]any{
		{"id": " main "},
		{"$id": "dollar"},
		{"syncId": "sync"},
	}
	for _, test := range tests {
		if got := StableID(test); got == "" || got == " main " {
			t.Fatalf("StableID(%#v) = %q, want trimmed non-empty", test, got)
		}
	}
}

func TestUIPropertiesRecordDefaultsNameAndClass(t *testing.T) {
	record, err := NewUIRecord("StarterGui/Main.ScreenGui/properties.init.json", `{"properties":{"ResetOnSpawn":false}}`, managedServices, nil)
	if err != nil {
		t.Fatalf("NewUIRecord returned error: %v", err)
	}
	if record == nil {
		t.Fatal("record is nil")
	}
	if record.Entity != EntityUIInstance {
		t.Fatalf("Entity = %q, want %s", record.Entity, EntityUIInstance)
	}
	if record.RbxPath != "game.StarterGui.Main" {
		t.Fatalf("RbxPath = %q, want game.StarterGui.Main", record.RbxPath)
	}
	if record.ClassName != "ScreenGui" {
		t.Fatalf("ClassName = %q, want ScreenGui", record.ClassName)
	}
	if record.Payload["name"] != "Main" {
		t.Fatalf("payload name = %v, want Main", record.Payload["name"])
	}
}

func TestDisambiguatedUIPathKeepsRobloxName(t *testing.T) {
	record, err := NewUIRecord(
		"Workspace/RaceWallDecal~rid_part-id.Part/Decal~rid_decal-id.Decal/properties.init.json",
		`{"id":"decal-id","className":"Decal","name":"Decal","properties":{"Face":{"$type":"Enum","enumType":"NormalId","name":"Back"}}}`,
		managedServices,
		nil,
	)
	if err != nil {
		t.Fatalf("NewUIRecord returned error: %v", err)
	}
	if record == nil {
		t.Fatal("record is nil")
	}
	if record.RbxPath != "game.Workspace.RaceWallDecal.Decal" || record.StableID != "decal-id" {
		t.Fatalf("record path/id = %q/%q", record.RbxPath, record.StableID)
	}
}

func TestIdentityKeyUsesStableIDForDuplicateRobloxPaths(t *testing.T) {
	left := SyncRecord{Entity: EntityUIInstance, LocalPath: "Workspace/Same~rid_left.Frame/properties.init.json", RbxPath: "game.Workspace.Same", StableID: "left"}
	right := SyncRecord{Entity: EntityUIInstance, LocalPath: "Workspace/Same~rid_right.Frame/properties.init.json", RbxPath: "game.Workspace.Same", StableID: "right"}
	if IdentityKey(left) == IdentityKey(right) {
		t.Fatal("duplicate Roblox paths with distinct stable IDs must have distinct identities")
	}
}

func TestValidateRecordsRejectsDuplicateStableIDAndAmbiguousFallback(t *testing.T) {
	snapshot := map[string]SyncRecord{
		"Workspace/A~rid_same.Part/properties.init.json": {
			Entity: EntityUIInstance, LocalPath: "Workspace/A~rid_same.Part/properties.init.json", RbxPath: "game.Workspace.A", StableID: "same",
		},
		"Workspace/B~rid_same.Part/properties.init.json": {
			Entity: EntityUIInstance, LocalPath: "Workspace/B~rid_same.Part/properties.init.json", RbxPath: "game.Workspace.B", StableID: "same",
		},
		"Workspace/UnnamedA.Part/properties.init.json": {
			Entity: EntityUIInstance, LocalPath: "Workspace/UnnamedA.Part/properties.init.json", RbxPath: "game.Workspace.Duplicate", StableID: "path:one",
		},
		"Workspace/UnnamedB.Part/properties.init.json": {
			Entity: EntityUIInstance, LocalPath: "Workspace/UnnamedB.Part/properties.init.json", RbxPath: "game.Workspace.Duplicate", StableID: "path:two",
		},
	}
	conflicts := ValidateRecords(snapshot)
	seen := map[ConflictKind]bool{}
	for _, conflict := range conflicts {
		seen[conflict.Kind] = true
	}
	if !seen[ConflictDuplicateStableID] || !seen[ConflictAmbiguousTarget] {
		t.Fatalf("conflicts = %#v, want duplicate stable ID and ambiguous target", conflicts)
	}
}

func TestBroadMetadataRecordNestedBelowGeometry(t *testing.T) {
	source := `{
  "id":"health-id",
  "className":"ObjectValue",
  "name":"HealthTarget",
  "properties":{"Value":{"$type":"InstanceRef","stableId":"target-id","path":"game.Workspace.Map.Target"}},
  "attributes":{"Purpose":"test"},
  "tags":["metadata"]
}`
	record, err := NewUIRecord(
		"Workspace/Map.Model/Trigger.Part/HealthTarget.ObjectValue/properties.init.json",
		source,
		managedServices,
		nil,
	)
	if err != nil {
		t.Fatalf("NewUIRecord returned error: %v", err)
	}
	if record == nil {
		t.Fatal("record is nil")
	}
	if record.RbxPath != "game.Workspace.Map.Trigger.HealthTarget" {
		t.Fatalf("RbxPath = %q, want nested path with geometry as anchors", record.RbxPath)
	}
	if record.ClassName != "ObjectValue" || record.StableID != "health-id" {
		t.Fatalf("record class/id = %q/%q", record.ClassName, record.StableID)
	}
	properties, ok := record.Payload["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v, want object", record.Payload["properties"])
	}
	reference, ok := properties["Value"].(map[string]any)
	if !ok || reference["$type"] != "InstanceRef" || reference["stableId"] != "target-id" {
		t.Fatalf("Value = %#v, want preserved InstanceRef", properties["Value"])
	}
}

func TestRojoInitMetaRecordNormalizesPayload(t *testing.T) {
	record, err := NewRojoInitMetaRecord("StarterGui/Menu/init.meta.json", `{"$className":"ScreenGui","properties":{"ResetOnSpawn":false}}`, managedServices, nil)
	if err != nil {
		t.Fatalf("NewRojoInitMetaRecord returned error: %v", err)
	}
	if record == nil {
		t.Fatal("record is nil")
	}
	if record.StableID != "path:StarterGui/Menu/init.meta.json" {
		t.Fatalf("StableID = %q, want path fallback", record.StableID)
	}
	if record.Payload["className"] != "ScreenGui" || record.Payload["name"] != "Menu" {
		t.Fatalf("payload = %#v", record.Payload)
	}
}

func TestRojoModelRecordMovesTopLevelProperties(t *testing.T) {
	record, err := NewRojoModelRecord("StarterGui/Menu.model.json", `{"$className":"ScreenGui","ResetOnSpawn":false,"attributes":{"Theme":"Dark"},"tags":["ui"]}`, managedServices, nil)
	if err != nil {
		t.Fatalf("NewRojoModelRecord returned error: %v", err)
	}
	if record == nil {
		t.Fatal("record is nil")
	}
	properties, ok := record.Payload["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v, want object", record.Payload["properties"])
	}
	if properties["ResetOnSpawn"] != false {
		t.Fatalf("ResetOnSpawn = %#v, want false", properties["ResetOnSpawn"])
	}
	if record.RbxPath != "game.StarterGui.Menu" {
		t.Fatalf("RbxPath = %q, want game.StarterGui.Menu", record.RbxPath)
	}
}
