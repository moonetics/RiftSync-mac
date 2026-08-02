package state

import (
	"context"
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

func TestRecordActivityNormalizesStructuredProgress(t *testing.T) {
	appState := testState(t, 8)
	got := appState.RecordActivity(Activity{
		Text:          "  Applying properties  ",
		Operation:     " folder_to_studio ",
		Phase:         " properties ",
		Progress:      140,
		Current:       80,
		Total:         50,
		Indeterminate: true,
	})
	if got.Text != "Applying properties" ||
		got.Operation != "folder_to_studio" ||
		got.Phase != "properties" ||
		got.Progress != 100 ||
		got.Current != 50 ||
		got.Total != 50 ||
		!got.Indeterminate {
		t.Fatalf("activity = %#v", got)
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

func TestRemoteExecRunningCommandExpires(t *testing.T) {
	appState := testState(t, 10)
	command, err := appState.EnqueueRemoteExecCommand("print(1)", "cli", "edit", 1)
	if err != nil {
		t.Fatalf("EnqueueRemoteExecCommand returned error: %v", err)
	}
	claimed, ok := appState.ClaimRemoteExecCommand(context.Background(), "studio", time.Millisecond)
	if !ok || claimed.ID != command.ID {
		t.Fatalf("claimed = %#v ok=%t, want command %s", claimed, ok, command.ID)
	}

	appState.mu.Lock()
	appState.execCommands[command.ID].ClaimedAt = nowSeconds() - 7
	appState.mu.Unlock()

	expired, found := appState.RemoteExecCommand(command.ID)
	if !found {
		t.Fatal("RemoteExecCommand not found")
	}
	if expired.State != RemoteExecExpired {
		t.Fatalf("state = %s, want expired", expired.State)
	}
	debug := appState.RemoteExecDebug()
	if debug.RunningCount != 0 || debug.ErrorCount != 1 || debug.LastResultStatus != string(RemoteExecExpired) {
		t.Fatalf("debug = %#v, want expired error metrics", debug)
	}
}

func TestRemoteExecQueuesAreIsolatedPerProjectState(t *testing.T) {
	cfgA := config.Default()
	cfgA.SyncRoot = t.TempDir()
	cfgA.RemoteExecEnabled = true
	if err := cfgA.NormalizeAndValidate(); err != nil {
		t.Fatal(err)
	}
	cfgB := config.Default()
	cfgB.SyncRoot = t.TempDir()
	cfgB.RemoteExecEnabled = true
	if err := cfgB.NormalizeAndValidate(); err != nil {
		t.Fatal(err)
	}
	projectA := New(cfgA)
	projectB := New(cfgB)
	queued, err := projectA.EnqueueRemoteExecCommand("print('A')", "app", "edit", 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, claimed := projectB.ClaimRemoteExecCommand(context.Background(), "studio-b", time.Millisecond); claimed {
		t.Fatal("project B claimed a command queued in project A")
	}
	claimed, ok := projectA.ClaimRemoteExecCommand(context.Background(), "studio-a", time.Millisecond)
	if !ok || claimed.ID != queued.ID || claimed.ClaimedBy != "studio-a" {
		t.Fatalf("project A claim = %#v ok=%v", claimed, ok)
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

func TestProjectStatePersistsInitializationAndDeletionTombstone(t *testing.T) {
	appState := testState(t, 10)
	if err := appState.SetSyncRootInitialized(true); err != nil {
		t.Fatalf("SetSyncRootInitialized returned error: %v", err)
	}
	record := testRecord("ServerScriptService/A.server.luau", "A", "print(1)")
	record.StableID = "stable-a"
	appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)
	appState.ApplySnapshot(map[string]records.SyncRecord{}, nil, nil)

	reloaded := New(appState.Config())
	reloaded.SetSnapshot(map[string]records.SyncRecord{}, nil, 0)
	if err := reloaded.LoadHistory(); err != nil {
		t.Fatalf("LoadHistory returned error: %v", err)
	}
	if err := reloaded.LoadProjectState(); err != nil {
		t.Fatalf("LoadProjectState returned error: %v", err)
	}
	if !reloaded.SyncRootInitialized() {
		t.Fatal("SyncRootInitialized = false, want true")
	}
	tombstones := reloaded.DeletionTombstones()
	if len(tombstones) != 1 || tombstones[0].StableID != "stable-a" || tombstones[0].RbxPath != record.RbxPath {
		t.Fatalf("tombstones = %#v, want stable-a delete", tombstones)
	}
}

func TestProjectStateRecreatedIdentityClearsTombstone(t *testing.T) {
	appState := testState(t, 10)
	record := testRecord("ServerScriptService/A.server.luau", "A", "print(1)")
	record.StableID = "stable-a"
	appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)
	appState.ApplySnapshot(map[string]records.SyncRecord{}, nil, nil)
	if len(appState.DeletionTombstones()) != 1 {
		t.Fatalf("tombstones before recreate = %#v, want one", appState.DeletionTombstones())
	}
	record.Source = "print(2)"
	record.ContentHash = records.ContentDigest(record.Source)
	appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)
	if len(appState.DeletionTombstones()) != 0 {
		t.Fatalf("tombstones after recreate = %#v, want none", appState.DeletionTombstones())
	}
}

func TestProjectStateMigratesInitializationAndTombstonesFromHistory(t *testing.T) {
	appState := testState(t, 10)
	record := testRecord("ServerScriptService/Legacy.server.luau", "Legacy", "print(1)")
	record.StableID = "legacy-id"
	appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)
	appState.ApplySnapshot(map[string]records.SyncRecord{}, nil, nil)
	projectStatePath := filepath.Join(appState.Config().SyncRootAbs, config.MetadataDir, "project-state.json")
	if err := os.Remove(projectStatePath); err != nil {
		t.Fatalf("remove project state: %v", err)
	}

	reloaded := New(appState.Config())
	reloaded.SetSnapshot(map[string]records.SyncRecord{}, nil, 0)
	if err := reloaded.LoadHistory(); err != nil {
		t.Fatal(err)
	}
	if err := reloaded.LoadProjectState(); err != nil {
		t.Fatal(err)
	}
	if !reloaded.SyncRootInitialized() {
		t.Fatal("history migration did not mark project initialized")
	}
	tombstones := reloaded.DeletionTombstones()
	if len(tombstones) != 1 || tombstones[0].StableID != "legacy-id" {
		t.Fatalf("migrated tombstones = %#v, want legacy-id", tombstones)
	}
}

func TestCorruptedProjectStateRecoversFromExistingLocalContent(t *testing.T) {
	appState := testState(t, 10)
	root := appState.Config().SyncRootAbs
	if err := os.MkdirAll(filepath.Join(root, config.MetadataDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, config.MetadataDir, "project-state.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ServerScriptService"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ServerScriptService", "A.server.luau"), []byte("print(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appState.LoadProjectState(); err != nil {
		t.Fatalf("LoadProjectState returned error: %v", err)
	}
	if !appState.SyncRootInitialized() {
		t.Fatal("recovered project should be initialized")
	}
	matches, err := filepath.Glob(filepath.Join(root, config.MetadataDir, "project-state.json.corrupt-*"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("corrupt backup matches=%#v err=%v, want one", matches, err)
	}
}

func TestSnapshotAuthoritativeRejectsInvalidPaths(t *testing.T) {
	appState := testState(t, 10)
	appState.SetSnapshot(map[string]records.SyncRecord{}, []string{"invalid"}, 1)
	if appState.SnapshotAuthoritative() {
		t.Fatal("SnapshotAuthoritative = true with invalid paths")
	}
}
