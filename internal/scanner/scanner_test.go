package scanner

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"riftsync/internal/config"
	"riftsync/internal/records"
)

func testConfig(t *testing.T, root string) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.SyncRoot = root
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	return cfg
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestScanScriptsAndUITree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ServerScriptService/Foo.server.luau", "print('hi')")
	writeFile(t, root, "StarterGui/Main.ScreenGui/properties.init.json", `{"properties":{"ResetOnSpawn":false}}`)
	writeFile(t, root, ".git/ignored.server.luau", "print('ignore')")
	writeFile(t, root, ".rblxsync/history.server.luau", "print('ignore')")
	writeFile(t, root, ".guidebook/ignored.server.luau", "print('ignore')")

	snapshot, err := Scan(testConfig(t, root))
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(snapshot.Records) != 2 {
		t.Fatalf("record count = %d, want 2 (%#v)", len(snapshot.Records), snapshot.Records)
	}
	if snapshot.ScriptCount != 1 || snapshot.UICount != 1 {
		t.Fatalf("counts script=%d ui=%d, want 1/1", snapshot.ScriptCount, snapshot.UICount)
	}
}

func TestScanWithProgressReportsEnumerationAndExactFileCount(t *testing.T) {
	root := t.TempDir()
	for index := 0; index < 25; index++ {
		writeFile(t, root, filepath.ToSlash(filepath.Join("ServerScriptService", "Script"+strconv.Itoa(index)+".server.luau")), "print('hi')")
	}
	cfg := testConfig(t, root)
	updates := []Progress{}
	snapshot, err := NewCache().ScanWithProgress(cfg, func(progress Progress) {
		updates = append(updates, progress)
	})
	if err != nil {
		t.Fatalf("ScanWithProgress returned error: %v", err)
	}
	if len(snapshot.Records) != 25 {
		t.Fatalf("record count = %d, want 25", len(snapshot.Records))
	}
	foundEnumeration := false
	foundCompleteScan := false
	for _, update := range updates {
		if update.Phase == "enumerating" && update.Indeterminate {
			foundEnumeration = true
		}
		if update.Phase == "scanning" && update.Current == 25 && update.Total == 25 && !update.Indeterminate {
			foundCompleteScan = true
		}
	}
	if !foundEnumeration || !foundCompleteScan {
		t.Fatalf("progress updates = %#v", updates)
	}
}

func TestScanScriptPropertiesMergeIntoScriptRecord(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ServerScriptService/Foo.Script/Foo.server.luau", "print('hi')")
	writeFile(t, root, "ServerScriptService/Foo.Script/properties.init.json", `{"properties":{"Disabled":true}}`)

	snapshot, err := Scan(testConfig(t, root))
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(snapshot.Records) != 1 {
		t.Fatalf("record count = %d, want 1 (%#v)", len(snapshot.Records), snapshot.Records)
	}
	record, ok := snapshot.Records["ServerScriptService/Foo.Script/Foo.server.luau"]
	if !ok {
		t.Fatalf("records = %#v, want script source local_path", snapshot.Records)
	}
	if record.Entity != records.EntityScript || record.RbxPath != "game.ServerScriptService.Foo" {
		t.Fatalf("record = %#v, want script record", record)
	}
	properties, ok := record.Payload["properties"].(map[string]any)
	if !ok || properties["Disabled"] != true {
		t.Fatalf("payload = %#v, want Disabled true", record.Payload)
	}
	upsert := record.ToUpsert()
	if upsert["payload"] == nil {
		t.Fatalf("upsert = %#v, want script payload", upsert)
	}
}

func TestScanScriptPropertiesChangeUpdatesHash(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ServerScriptService/Foo.Script/Foo.server.luau", "print('hi')")
	writeFile(t, root, "ServerScriptService/Foo.Script/properties.init.json", `{"properties":{"Disabled":true}}`)
	cfg := testConfig(t, root)
	cache := NewCache()

	first, err := cache.Scan(cfg)
	if err != nil {
		t.Fatalf("first Scan returned error: %v", err)
	}
	firstRecord := first.Records["ServerScriptService/Foo.Script/Foo.server.luau"]

	writeFile(t, root, "ServerScriptService/Foo.Script/properties.init.json", `{"properties":{"Disabled":false}}`)
	second, err := cache.Scan(cfg)
	if err != nil {
		t.Fatalf("second Scan returned error: %v", err)
	}
	secondRecord := second.Records["ServerScriptService/Foo.Script/Foo.server.luau"]
	if firstRecord.ContentHash == secondRecord.ContentHash {
		t.Fatalf("ContentHash did not change after script properties edit: %q", firstRecord.ContentHash)
	}
	properties, ok := secondRecord.Payload["properties"].(map[string]any)
	if !ok || properties["Disabled"] != false {
		t.Fatalf("payload = %#v, want Disabled false", secondRecord.Payload)
	}
}

func TestScanCanonicalEnabledAndNestedBroadMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Workspace/Map.Model/Trigger.Part/Runner.Script/Runner.server.luau", "print('nested')")
	writeFile(t, root, "Workspace/Map.Model/Trigger.Part/Runner.Script/properties.init.json", `{"properties":{"Enabled":false,"RunContext":{"$type":"Enum","enumType":"RunContext","value":"Server"}}}`)
	writeFile(t, root, "Workspace/Map.Model/Trigger.Part/Counter.IntValue/properties.init.json", `{"id":"counter","className":"IntValue","name":"Counter","properties":{"Value":12},"attributes":{"Unit":"rounds"},"tags":["state"]}`)

	snapshot, err := Scan(testConfig(t, root))
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if snapshot.ScriptCount != 1 || snapshot.UICount != 1 {
		t.Fatalf("counts script=%d metadata=%d, want 1/1", snapshot.ScriptCount, snapshot.UICount)
	}
	scriptRecord, ok := snapshot.Records["Workspace/Map.Model/Trigger.Part/Runner.Script/Runner.server.luau"]
	if !ok {
		t.Fatalf("script record missing: %#v", snapshot.Records)
	}
	properties := scriptRecord.Payload["properties"].(map[string]any)
	if properties["Enabled"] != false {
		t.Fatalf("script properties = %#v, want canonical Enabled false", properties)
	}
	valueRecord, ok := snapshot.Records["Workspace/Map.Model/Trigger.Part/Counter.IntValue/properties.init.json"]
	if !ok || valueRecord.RbxPath != "game.Workspace.Map.Trigger.Counter" || valueRecord.ClassName != "IntValue" {
		t.Fatalf("value record = %#v", valueRecord)
	}
}

func TestDeletingScriptMetadataKeepsSourceOwnedScript(t *testing.T) {
	root := t.TempDir()
	sourcePath := "ServerScriptService/Foo.Script/Foo.server.luau"
	metadataPath := "ServerScriptService/Foo.Script/properties.init.json"
	writeFile(t, root, sourcePath, "print('still owned by source')")
	writeFile(t, root, metadataPath, `{"properties":{"Enabled":false}}`)
	cfg := testConfig(t, root)
	cache := NewCache()

	withMetadata, err := cache.Scan(cfg)
	if err != nil {
		t.Fatalf("scan with metadata: %v", err)
	}
	before := withMetadata.Records[sourcePath]
	if before.Payload == nil {
		t.Fatal("script payload missing before metadata deletion")
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(metadataPath))); err != nil {
		t.Fatalf("remove metadata: %v", err)
	}

	withoutMetadata, err := cache.Scan(cfg)
	if err != nil {
		t.Fatalf("scan without metadata: %v", err)
	}
	after, ok := withoutMetadata.Records[sourcePath]
	if !ok || after.Entity != records.EntityScript {
		t.Fatalf("source-owned script disappeared: %#v", withoutMetadata.Records)
	}
	if after.Payload != nil {
		t.Fatalf("stale metadata remained after delete: %#v", after.Payload)
	}
	if before.ContentHash == after.ContentHash {
		t.Fatal("metadata deletion did not change script content hash")
	}
}

func TestScanInvalidJSONWarning(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "StarterGui/Main.ScreenGui/properties.init.json", `{"className":`)

	snapshot, err := Scan(testConfig(t, root))
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(snapshot.Records) != 0 {
		t.Fatalf("record count = %d, want 0", len(snapshot.Records))
	}
	if len(snapshot.InvalidPaths) != 1 {
		t.Fatalf("InvalidPaths = %#v, want one path", snapshot.InvalidPaths)
	}
	if len(snapshot.Warnings) == 0 {
		t.Fatal("Warnings empty, want invalid JSON warning")
	}
}

func TestScanDeduplicatesByIdentity(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "StarterGui/Main.ScreenGui/properties.init.json", `{"id":"main","properties":{}}`)
	writeFile(t, root, "StarterGui/Main/init.meta.json", `{"$className":"ScreenGui","id":"main-rojo"}`)

	snapshot, err := Scan(testConfig(t, root))
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(snapshot.Records) != 1 {
		t.Fatalf("record count = %d, want 1", len(snapshot.Records))
	}
	record, ok := snapshot.Records["StarterGui/Main/init.meta.json"]
	if !ok {
		t.Fatalf("records = %#v, want Rojo init.meta preferred", snapshot.Records)
	}
	if record.Entity != records.EntityUIInstance || record.RbxPath != "game.StarterGui.Main" {
		t.Fatalf("record = %#v", record)
	}
}

func TestParseRelativeFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ServerScriptService/Foo.server.luau", "print('hi')")

	result := ParseRelativeFile(testConfig(t, root), "ServerScriptService/Foo.server.luau")
	if result.Warning != "" || result.InvalidPath != "" {
		t.Fatalf("result warning=%q invalid=%q", result.Warning, result.InvalidPath)
	}
	if result.Record == nil || result.Record.RbxPath != "game.ServerScriptService.Foo" {
		t.Fatalf("record = %#v, want script record", result.Record)
	}
}

func TestParseRelativeFileInvalidJSON(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "StarterGui/Main.ScreenGui/properties.init.json", `{"className":`)

	result := ParseRelativeFile(testConfig(t, root), "StarterGui/Main.ScreenGui/properties.init.json")
	if result.Record != nil {
		t.Fatalf("record = %#v, want nil", result.Record)
	}
	if result.InvalidPath != "StarterGui/Main.ScreenGui/properties.init.json" {
		t.Fatalf("InvalidPath = %q, want json path", result.InvalidPath)
	}
	if result.Warning == "" {
		t.Fatal("Warning empty, want invalid JSON warning")
	}
}

func TestScanSubtree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ServerScriptService/Foo.server.luau", "print('hi')")
	writeFile(t, root, "Workspace/Part.server.luau", "print('other')")

	snapshot, err := ScanSubtree(testConfig(t, root), "ServerScriptService")
	if err != nil {
		t.Fatalf("ScanSubtree returned error: %v", err)
	}
	if len(snapshot.Records) != 1 {
		t.Fatalf("record count = %d, want 1", len(snapshot.Records))
	}
	if _, ok := snapshot.Records["ServerScriptService/Foo.server.luau"]; !ok {
		t.Fatalf("records = %#v, want ServerScriptService/Foo", snapshot.Records)
	}
}

func TestCacheHitForUnchangedFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ServerScriptService/Foo.server.luau", "print('hi')")
	cfg := testConfig(t, root)
	cache := NewCache()

	first := cache.ParseRelativeFile(cfg, "ServerScriptService/Foo.server.luau")
	if first.CacheHit {
		t.Fatal("first parse CacheHit = true, want false")
	}
	second := cache.ParseRelativeFile(cfg, "ServerScriptService/Foo.server.luau")
	if !second.CacheHit {
		t.Fatal("second parse CacheHit = false, want true")
	}
	if second.Record == nil || second.Record.ContentHash != first.Record.ContentHash {
		t.Fatalf("second record = %#v, want cached same hash", second.Record)
	}
}

func TestCacheMissAfterFileChange(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ServerScriptService/Foo.server.luau", "print(1)")
	cfg := testConfig(t, root)
	cache := NewCache()
	first := cache.ParseRelativeFile(cfg, "ServerScriptService/Foo.server.luau")

	writeFile(t, root, "ServerScriptService/Foo.server.luau", "print(22)")
	second := cache.ParseRelativeFile(cfg, "ServerScriptService/Foo.server.luau")
	if second.CacheHit {
		t.Fatal("second parse CacheHit = true, want false after content change")
	}
	if second.Record == nil || second.Record.ContentHash == first.Record.ContentHash {
		t.Fatalf("second hash = %v, want changed from %v", second.Record, first.Record)
	}
}

func TestCacheCachesInvalidJSONUntilChange(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "StarterGui/Main.ScreenGui/properties.init.json", `{"className":`)
	cfg := testConfig(t, root)
	cache := NewCache()

	first := cache.ParseRelativeFile(cfg, "StarterGui/Main.ScreenGui/properties.init.json")
	second := cache.ParseRelativeFile(cfg, "StarterGui/Main.ScreenGui/properties.init.json")
	if first.InvalidPath == "" || second.InvalidPath == "" || !second.CacheHit {
		t.Fatalf("first=%#v second=%#v, want cached invalid JSON", first, second)
	}

	writeFile(t, root, "StarterGui/Main.ScreenGui/properties.init.json", `{"className":"ScreenGui"}`)
	third := cache.ParseRelativeFile(cfg, "StarterGui/Main.ScreenGui/properties.init.json")
	if third.CacheHit || third.InvalidPath != "" || third.Record == nil {
		t.Fatalf("third=%#v, want cache miss with valid record", third)
	}
}

func TestCacheInvalidateSubtree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ServerScriptService/Nested/Foo.server.luau", "print(1)")
	cfg := testConfig(t, root)
	cache := NewCache()
	cache.ParseRelativeFile(cfg, "ServerScriptService/Nested/Foo.server.luau")

	cache.InvalidateSubtree("ServerScriptService/Nested")
	result := cache.ParseRelativeFile(cfg, "ServerScriptService/Nested/Foo.server.luau")
	if result.CacheHit {
		t.Fatal("CacheHit = true, want false after subtree invalidation")
	}
}
