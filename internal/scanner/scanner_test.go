package scanner

import (
	"os"
	"path/filepath"
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
