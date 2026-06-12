package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"riftsync/internal/config"
	"riftsync/internal/records"
	"riftsync/internal/state"
)

func testConfig(t *testing.T, root string) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.SyncRoot = root
	cfg.GitVersioningEnabled = false
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	return cfg
}

func writeFile(t *testing.T, root, rel, body string) string {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return target
}

func newTestService(t *testing.T, root string) (*Service, *state.AppState) {
	t.Helper()
	cfg := testConfig(t, root)
	appState := state.New(cfg)
	service, err := New(cfg, appState, Options{Debounce: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})
	return service, appState
}

func TestProcessBatchCreateEditDelete(t *testing.T) {
	root := t.TempDir()
	service, appState := newTestService(t, root)
	target := writeFile(t, root, "ServerScriptService/Foo.server.luau", "print(1)")

	service.processBatch(map[string]fsnotify.Op{target: fsnotify.Create})
	if appState.Revision() != 1 {
		t.Fatalf("revision after create = %d, want 1", appState.Revision())
	}
	metrics := appState.Metrics()
	if metrics.LastEventBatchSize != 1 || metrics.LastDebounceMS != 10 || metrics.LastCacheMissCount != 1 {
		t.Fatalf("metrics after create = %#v", metrics)
	}
	changes, _, _ := appState.FlattenChanges(0)
	if len(changes) != 1 || changes[0]["op"] != "upsert" {
		t.Fatalf("create changes = %#v, want upsert", changes)
	}

	writeFile(t, root, "ServerScriptService/Foo.server.luau", "print(2)")
	service.processBatch(map[string]fsnotify.Op{target: fsnotify.Write})
	if appState.Revision() != 2 {
		t.Fatalf("revision after edit = %d, want 2", appState.Revision())
	}

	if err := os.Remove(target); err != nil {
		t.Fatalf("remove: %v", err)
	}
	service.processBatch(map[string]fsnotify.Op{target: fsnotify.Remove})
	if appState.Revision() != 3 {
		t.Fatalf("revision after delete = %d, want 3", appState.Revision())
	}
	changes, _, _ = appState.FlattenChanges(2)
	if len(changes) != 1 || changes[0]["op"] != "delete" {
		t.Fatalf("delete changes = %#v, want delete", changes)
	}
}

func TestProcessBatchWaitsForReconciliationLock(t *testing.T) {
	root := t.TempDir()
	service, appState := newTestService(t, root)
	target := writeFile(t, root, "ServerScriptService/Foo.server.luau", "print(1)")

	appState.LockReconciliation()
	locked := true
	defer func() {
		if locked {
			appState.UnlockReconciliation()
		}
	}()

	done := make(chan struct{})
	go func() {
		service.processBatch(map[string]fsnotify.Op{target: fsnotify.Create})
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("processBatch completed while reconciliation lock was held")
	case <-time.After(25 * time.Millisecond):
	}

	appState.UnlockReconciliation()
	locked = false
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("processBatch did not resume after reconciliation lock was released")
	}
	if appState.Revision() != 1 {
		t.Fatalf("revision = %d, want 1", appState.Revision())
	}
}

func TestProcessBatchRenameByHash(t *testing.T) {
	root := t.TempDir()
	service, appState := newTestService(t, root)
	oldTarget := writeFile(t, root, "ServerScriptService/Foo.server.luau", "print(1)")
	oldRecord := records.SyncRecord{
		Entity:      records.EntityScript,
		LocalPath:   "ServerScriptService/Foo.server.luau",
		LocalDir:    "ServerScriptService",
		RbxPath:     "game.ServerScriptService.Foo",
		ClassName:   "Script",
		Source:      "print(1)",
		ContentHash: records.ContentDigest("print(1)"),
	}
	appState.SetSnapshot(map[string]records.SyncRecord{oldRecord.LocalPath: oldRecord}, nil, 0)

	if err := os.Remove(oldTarget); err != nil {
		t.Fatalf("remove old: %v", err)
	}
	newTarget := writeFile(t, root, "ServerScriptService/Bar.server.luau", "print(1)")
	service.processBatch(map[string]fsnotify.Op{
		oldTarget: fsnotify.Remove,
		newTarget: fsnotify.Create,
	})

	changes, _, _ := appState.FlattenChanges(0)
	if len(changes) != 1 || changes[0]["op"] != "rename" {
		t.Fatalf("changes = %#v, want rename", changes)
	}
}

func TestProcessBatchNewFolderAndGitIgnored(t *testing.T) {
	root := t.TempDir()
	service, appState := newTestService(t, root)

	folder := filepath.Join(root, "ServerScriptService", "Nested")
	target := writeFile(t, root, "ServerScriptService/Nested/Child.server.luau", "print(1)")
	service.processBatch(map[string]fsnotify.Op{folder: fsnotify.Create})
	if appState.Revision() != 1 {
		t.Fatalf("revision after folder scan = %d, want 1", appState.Revision())
	}

	gitTarget := writeFile(t, root, ".git/config", "ignored")
	service.processBatch(map[string]fsnotify.Op{gitTarget: fsnotify.Write})
	if appState.Revision() != 1 {
		t.Fatalf("revision after git event = %d, want unchanged 1", appState.Revision())
	}

	metadataTarget := writeFile(t, root, ".rblxsync/history.json", "{}")
	service.processBatch(map[string]fsnotify.Op{metadataTarget: fsnotify.Write})
	if appState.Revision() != 1 {
		t.Fatalf("revision after metadata event = %d, want unchanged 1", appState.Revision())
	}

	guidebookTarget := writeFile(t, root, ".guidebook/README.md", "# guide")
	service.processBatch(map[string]fsnotify.Op{guidebookTarget: fsnotify.Write})
	if appState.Revision() != 1 {
		t.Fatalf("revision after guidebook event = %d, want unchanged 1", appState.Revision())
	}

	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected scanned child to exist: %v", err)
	}
}

func TestSyncTreeAddsFoldersCreatedWithoutFsnotifyEvent(t *testing.T) {
	root := t.TempDir()
	service, _ := newTestService(t, root)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := service.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	initialCount := service.WatchCount()
	if initialCount == 0 {
		t.Fatal("expected root watch after Start")
	}

	nested := filepath.Join(root, "ServerScriptService", "Pulled", "Deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	result, err := service.SyncTree()
	if err != nil {
		t.Fatalf("SyncTree returned error: %v", err)
	}
	if service.WatchCount() < initialCount+3 {
		t.Fatalf("watch count = %d, want at least %d (SyncTree added=%d)", service.WatchCount(), initialCount+3, result.Added)
	}
}
