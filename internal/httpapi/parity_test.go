package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"riftsync/internal/config"
	"riftsync/internal/records"
	"riftsync/internal/scanner"
	"riftsync/internal/state"
)

func TestFixtureBasicScriptsSnapshotParity(t *testing.T) {
	payload := snapshotFixturePayload(t, "basic-scripts")
	expected := expectedFixturePayload(t, "basic-scripts-snapshot.json")
	assertNormalizedChanges(t, payload["changes"], expected["changes"])
}

func TestFixtureUITreeSnapshotParity(t *testing.T) {
	payload := snapshotFixturePayload(t, "ui-tree")
	expected := expectedFixturePayload(t, "ui-tree-snapshot.json")
	assertNormalizedChanges(t, payload["changes"], expected["changes"])
}

func TestFixtureBootstrapMergeParity(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.SyncRoot = root
	cfg.GitVersioningEnabled = false
	handler, _ := newTestHandlerWithConfig(t, cfg)

	body := readFixtureFile(t, "bootstrap-payloads", "merge-basic.json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/bootstrap", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	payload := decodeResponse(t, recorder)
	if payload["status"] != "ok" || payload["written_count"] != float64(1) || payload["server_rev"] != float64(1) {
		t.Fatalf("bootstrap payload = %#v", payload)
	}

	changesRecorder := httptest.NewRecorder()
	handler.ServeHTTP(changesRecorder, httptest.NewRequest(http.MethodGet, "/changes?since_rev=0&timeout=1", nil))
	changesPayload := decodeResponse(t, changesRecorder)
	changes := normalizedChanges(t, changesPayload["changes"])
	if len(changes) != 1 || changes[0]["local_path"] != "ServerScriptService/Pulled.server.luau" || changes[0]["source"] != "print(\"pulled\")\n" {
		t.Fatalf("changes = %#v", changes)
	}
}

func TestFixtureInvalidJSONDoesNotDeleteExistingRecord(t *testing.T) {
	root := copyFixture(t, "invalid-json")
	cfg := testParityConfig(t, root)
	snapshot, err := scanner.NewCache().Scan(cfg)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(snapshot.InvalidPaths) != 1 || len(snapshot.Records) != 0 {
		t.Fatalf("snapshot = %#v, want invalid JSON without records", snapshot)
	}

	appState := state.New(cfg)
	oldRecord := records.SyncRecord{
		Entity:      records.EntityUIInstance,
		LocalPath:   "StarterGui/Broken.ScreenGui/properties.init.json",
		LocalDir:    "StarterGui/Broken.ScreenGui",
		RbxPath:     "game.StarterGui.Broken",
		ClassName:   "ScreenGui",
		StableID:    "broken",
		ContentHash: records.ContentDigest("old"),
		Payload: map[string]any{
			"id":        "broken",
			"className": "ScreenGui",
			"name":      "Broken",
		},
	}
	appState.SetSnapshot(map[string]records.SyncRecord{oldRecord.LocalPath: oldRecord}, nil, 0)
	event := appState.ApplySnapshot(snapshot.Records, snapshot.Warnings, snapshot.InvalidPaths)
	if event.Rev != 0 {
		t.Fatalf("Rev = %d, want no delete revision for invalid JSON", event.Rev)
	}
}

func snapshotFixturePayload(t *testing.T, fixtureName string) map[string]any {
	t.Helper()
	root := copyFixture(t, fixtureName)
	cfg := testParityConfig(t, root)
	appState := state.New(cfg)
	snapshot, err := scanner.NewCache().Scan(cfg)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	appState.SetSnapshot(snapshot.Records, snapshot.Warnings, len(snapshot.InvalidPaths))
	handler := New(appState, "3.0.0")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/snapshot", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	return decodeResponse(t, recorder)
}

func expectedFixturePayload(t *testing.T, filename string) map[string]any {
	t.Helper()
	var payload map[string]any
	body := readFixtureFile(t, "expected", filename)
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode expected fixture %s: %v", filename, err)
	}
	return payload
}

func assertNormalizedChanges(t *testing.T, actual, expected any) {
	t.Helper()
	actualChanges := normalizedChanges(t, actual)
	expectedChanges := normalizedChanges(t, expected)
	if !reflect.DeepEqual(actualChanges, expectedChanges) {
		actualBody, _ := json.MarshalIndent(actualChanges, "", "  ")
		expectedBody, _ := json.MarshalIndent(expectedChanges, "", "  ")
		t.Fatalf("changes mismatch\nactual=%s\nexpected=%s", actualBody, expectedBody)
	}
}

func normalizedChanges(t *testing.T, value any) []map[string]any {
	t.Helper()
	raw, ok := value.([]any)
	if !ok {
		t.Fatalf("changes = %#v, want array", value)
	}
	result := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		change, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("change = %#v, want object", item)
		}
		normalized := map[string]any{}
		for _, key := range []string{"op", "entity", "local_path", "rbx_path", "class_name", "source"} {
			if existing, ok := change[key]; ok {
				normalized[key] = existing
			}
		}
		if payload, ok := change["payload"].(map[string]any); ok {
			normalizedPayload := map[string]any{}
			for _, key := range []string{"id", "className", "name"} {
				if existing, ok := payload[key]; ok {
					normalizedPayload[key] = existing
				}
			}
			normalized["payload"] = normalizedPayload
		}
		result = append(result, normalized)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i]["local_path"].(string) < result[j]["local_path"].(string)
	})
	return result
}

func copyFixture(t *testing.T, fixtureName string) string {
	t.Helper()
	sourceRoot := filepath.Join("..", "..", "testdata", fixtureName)
	targetRoot := t.TempDir()
	err := filepath.WalkDir(sourceRoot, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetRoot, rel)
		body, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, body, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture %s: %v", fixtureName, err)
	}
	return targetRoot
}

func readFixtureFile(t *testing.T, parts ...string) []byte {
	t.Helper()
	pathParts := append([]string{"..", "..", "testdata"}, parts...)
	body, err := os.ReadFile(filepath.Join(pathParts...))
	if err != nil {
		t.Fatalf("read fixture %v: %v", parts, err)
	}
	return body
}

func testParityConfig(t *testing.T, root string) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.SyncRoot = root
	cfg.GitVersioningEnabled = false
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	return cfg
}
