package state

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/records"
)

func testState(t *testing.T, retention int) *AppState {
	t.Helper()
	cfg := config.Default()
	cfg.SyncRoot = t.TempDir()
	cfg.ChangeRetention = retention
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	return New(cfg)
}

func testRecord(localPath, name, source string) records.SyncRecord {
	return records.SyncRecord{
		Entity:      records.EntityScript,
		LocalPath:   localPath,
		LocalDir:    "ServerScriptService",
		RbxPath:     fmt.Sprintf("game.ServerScriptService.%s", name),
		ClassName:   "Script",
		Source:      source,
		ContentHash: records.ContentDigest(source),
	}
}

func TestApplySnapshotBumpsRevisionOnChange(t *testing.T) {
	appState := testState(t, 10)
	record := testRecord("ServerScriptService/A.server.luau", "A", "print(1)")

	event := appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)
	if event.Rev != 1 {
		t.Fatalf("Rev = %d, want 1", event.Rev)
	}
	if appState.Revision() != 1 {
		t.Fatalf("Revision = %d, want 1", appState.Revision())
	}
}

func TestApplySnapshotNoopDoesNotBumpRevision(t *testing.T) {
	appState := testState(t, 10)
	record := testRecord("ServerScriptService/A.server.luau", "A", "print(1)")
	appState.SetSnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, 0)

	event := appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)
	if event.Rev != 0 {
		t.Fatalf("Rev = %d, want zero event", event.Rev)
	}
	if appState.Revision() != 0 {
		t.Fatalf("Revision = %d, want 0", appState.Revision())
	}
}

func TestFlattenChangesAndRetention(t *testing.T) {
	appState := testState(t, 2)
	a := testRecord("ServerScriptService/A.server.luau", "A", "print(1)")
	b := testRecord("ServerScriptService/B.server.luau", "B", "print(2)")
	c := testRecord("ServerScriptService/C.server.luau", "C", "print(3)")

	appState.ApplySnapshot(map[string]records.SyncRecord{a.LocalPath: a}, nil, nil)
	appState.ApplySnapshot(map[string]records.SyncRecord{a.LocalPath: a, b.LocalPath: b}, nil, nil)
	appState.ApplySnapshot(map[string]records.SyncRecord{a.LocalPath: a, b.LocalPath: b, c.LocalPath: c}, nil, nil)

	changes, target, needsSnapshot := appState.FlattenChanges(1)
	if needsSnapshot {
		t.Fatal("needsSnapshot = true, want false")
	}
	if target != 3 {
		t.Fatalf("target = %d, want 3", target)
	}
	if len(changes) != 2 {
		t.Fatalf("changes len = %d, want 2", len(changes))
	}

	_, _, needsSnapshot = appState.FlattenChanges(0)
	if !needsSnapshot {
		t.Fatal("needsSnapshot = false, want true for trimmed revision")
	}
}

func TestWaitForRevisionAfter(t *testing.T) {
	appState := testState(t, 10)
	done := make(chan struct{})
	go func() {
		appState.WaitForRevisionAfter(0, time.Second)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("wait returned before revision")
	case <-time.After(25 * time.Millisecond):
	}

	record := testRecord("ServerScriptService/A.server.luau", "A", "print(1)")
	appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("wait did not unblock")
	}
}

func TestSnapshotUpsertsUsesCurrentSnapshotCache(t *testing.T) {
	appState := testState(t, 10)
	a := testRecord("ServerScriptService/A.server.luau", "A", "print(1)")
	appState.SetSnapshot(map[string]records.SyncRecord{a.LocalPath: a}, nil, 0)

	upserts := appState.SnapshotUpserts()
	if len(upserts) != 1 || upserts[0]["local_path"] != a.LocalPath {
		t.Fatalf("upserts = %#v, want cached A snapshot", upserts)
	}

	b := testRecord("ServerScriptService/B.server.luau", "B", "print(2)")
	appState.ApplySnapshot(map[string]records.SyncRecord{b.LocalPath: b}, nil, nil)
	upserts = appState.SnapshotUpserts()
	if len(upserts) != 1 || upserts[0]["local_path"] != b.LocalPath {
		t.Fatalf("upserts = %#v, want rebuilt B snapshot", upserts)
	}
}

func TestRecordPerformance(t *testing.T) {
	appState := testState(t, 10)
	appState.RecordPerformance(3, 120*time.Millisecond, 2*time.Millisecond, time.Millisecond, 4, 5)

	metrics := appState.Metrics()
	if metrics.LastEventBatchSize != 3 || metrics.LastDebounceMS != 120 || metrics.LastCacheHitCount != 4 || metrics.LastCacheMissCount != 5 {
		t.Fatalf("metrics = %#v", metrics)
	}
	if metrics.LastParseDurationMS <= 0 || metrics.LastPublishDurationMS <= 0 {
		t.Fatalf("duration metrics = parse %v publish %v", metrics.LastParseDurationMS, metrics.LastPublishDurationMS)
	}
}

func TestHistoryPersistsAndLoads(t *testing.T) {
	appState := testState(t, 10)
	a := testRecord("ServerScriptService/A.server.luau", "A", "print(1)")
	b := testRecord("ServerScriptService/B.server.luau", "B", "print(2)")

	appState.ApplySnapshot(map[string]records.SyncRecord{a.LocalPath: a}, nil, nil)
	appState.ApplySnapshot(map[string]records.SyncRecord{a.LocalPath: a, b.LocalPath: b}, nil, nil)

	reloaded := New(appState.Config())
	reloaded.SetSnapshot(appState.RecordsCopy(), nil, 0)
	if err := reloaded.LoadHistory(); err != nil {
		t.Fatalf("LoadHistory returned error: %v", err)
	}
	if reloaded.Revision() != 2 {
		t.Fatalf("Revision = %d, want 2", reloaded.Revision())
	}
	summaries := reloaded.RevisionSummaries(10)
	if len(summaries) != 2 || summaries[0].Rev != 2 || summaries[1].Rev != 1 {
		t.Fatalf("summaries = %#v, want rev 2 then 1", summaries)
	}
	_, changes, found := reloaded.RevisionDetail(2)
	if !found || len(changes) != 1 {
		t.Fatalf("RevisionDetail found=%v changes=%#v, want one change", found, changes)
	}
}

func TestHistoryRetentionAndRevisionContinuation(t *testing.T) {
	appState := testState(t, 1)
	a := testRecord("ServerScriptService/A.server.luau", "A", "print(1)")
	b := testRecord("ServerScriptService/B.server.luau", "B", "print(2)")
	c := testRecord("ServerScriptService/C.server.luau", "C", "print(3)")

	appState.ApplySnapshot(map[string]records.SyncRecord{a.LocalPath: a}, nil, nil)
	appState.ApplySnapshot(map[string]records.SyncRecord{a.LocalPath: a, b.LocalPath: b}, nil, nil)

	reloaded := New(appState.Config())
	reloaded.SetSnapshot(map[string]records.SyncRecord{a.LocalPath: a, b.LocalPath: b}, nil, 0)
	if err := reloaded.LoadHistory(); err != nil {
		t.Fatalf("LoadHistory returned error: %v", err)
	}
	summaries := reloaded.RevisionSummaries(10)
	if reloaded.Revision() != 2 || len(summaries) != 1 || summaries[0].Rev != 2 {
		t.Fatalf("revision=%d summaries=%#v, want retained rev 2", reloaded.Revision(), summaries)
	}

	event := reloaded.ApplySnapshot(map[string]records.SyncRecord{a.LocalPath: a, b.LocalPath: b, c.LocalPath: c}, nil, nil)
	if event.Rev != 3 {
		t.Fatalf("next Rev = %d, want 3", event.Rev)
	}
}

func TestCorruptedHistoryIsIgnored(t *testing.T) {
	appState := testState(t, 10)
	historyPath := filepath.Join(appState.Config().SyncRootAbs, config.MetadataDir, "history.json")
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		t.Fatalf("mkdir history dir: %v", err)
	}
	if err := os.WriteFile(historyPath, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write history: %v", err)
	}
	if err := appState.LoadHistory(); err != nil {
		t.Fatalf("LoadHistory returned error: %v", err)
	}
	if appState.Revision() != 0 || len(appState.RevisionSummaries(10)) != 0 {
		t.Fatalf("corrupted history changed state: rev=%d summaries=%#v", appState.Revision(), appState.RevisionSummaries(10))
	}
}
