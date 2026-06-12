package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/records"
	"riftsync/internal/scanner"
	"riftsync/internal/state"
	"riftsync/internal/watcher"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	handler, _ := newTestHandlerWithState(t)
	return handler
}

func newTestHandlerWithState(t *testing.T) (http.Handler, *state.AppState) {
	t.Helper()
	cfg := config.Default()
	cfg.SyncRoot = t.TempDir()
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	appState := state.New(cfg)
	return New(appState, "3.0.0"), appState
}

func newTestHandlerWithConfig(t *testing.T, cfg config.Config) (http.Handler, *state.AppState) {
	t.Helper()
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	appState := state.New(cfg)
	return New(appState, "3.0.0"), appState
}

func newRemoteExecTestHandler(t *testing.T, enabled bool, token string) (http.Handler, *state.AppState) {
	t.Helper()
	cfg := config.Default()
	cfg.SyncRoot = t.TempDir()
	cfg.RemoteExecEnabled = enabled
	cfg.RemoteExecToken = token
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	appState := state.New(cfg)
	return New(appState, "3.0.0"), appState
}

func addExecAuth(request *http.Request, token string) *http.Request {
	request.Header.Set("Authorization", "Bearer "+token)
	return request
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload
}

func TestHealth(t *testing.T) {
	handler := newTestHandler(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	payload := decodeResponse(t, recorder)
	if payload["status"] != "ok" {
		t.Fatalf("status = %v, want ok", payload["status"])
	}
	if payload["protocol"] != protocol {
		t.Fatalf("protocol = %v, want %s", payload["protocol"], protocol)
	}
	if payload["indexed_entry_count"] != float64(0) {
		t.Fatalf("indexed_entry_count = %v, want 0", payload["indexed_entry_count"])
	}
}

func TestHandshake(t *testing.T) {
	handler := newTestHandler(t)
	body := bytes.NewBufferString(`{"protocol":"rbxsync/2.0.0","client_id":"test-client","last_applied_rev":0,"accept_encodings":["compact-json-v1","verbose-json-v1"]}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/handshake", body))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	payload := decodeResponse(t, recorder)
	if payload["status"] != "ok" {
		t.Fatalf("status = %v, want ok", payload["status"])
	}
	if payload["session_id"] == "" {
		t.Fatal("session_id is empty")
	}
	if payload["selected_change_encoding"] != "compact-json-v1" {
		t.Fatalf("selected_change_encoding = %v, want compact-json-v1", payload["selected_change_encoding"])
	}
	managedRoots, ok := payload["managed_roots"].([]any)
	if !ok || len(managedRoots) == 0 {
		t.Fatalf("managed_roots = %#v, want non-empty array", payload["managed_roots"])
	}
	ignored, ok := payload["ignored_rbx_paths"].([]any)
	if !ok || len(ignored) == 0 {
		t.Fatalf("ignored_rbx_paths = %#v, want non-empty array", payload["ignored_rbx_paths"])
	}
}

func TestHandshakeLegacyProtocolUsesVerbose(t *testing.T) {
	handler := newTestHandler(t)
	body := bytes.NewBufferString(`{"protocol":"rbxsync/1.0.0","client_id":"legacy-client","last_applied_rev":0}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/handshake", body))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	payload := decodeResponse(t, recorder)
	if payload["selected_change_encoding"] != "verbose-json-v1" {
		t.Fatalf("selected_change_encoding = %v, want verbose-json-v1", payload["selected_change_encoding"])
	}
}

func TestHandshakeProtocolMismatch(t *testing.T) {
	handler := newTestHandler(t)
	body := bytes.NewBufferString(`{"protocol":"wrong"}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/handshake", body))

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	payload := decodeResponse(t, recorder)
	if payload["status"] != "error" {
		t.Fatalf("status = %v, want error", payload["status"])
	}
}

func TestAckAndDebugMetrics(t *testing.T) {
	handler, appState := newTestHandlerWithState(t)
	appState.RecordPerformance(2, 120*time.Millisecond, 3*time.Millisecond, time.Millisecond, 4, 1)
	ackBody := bytes.NewBufferString(`{"client_id":"test-client","status":"error","applied_rev":12,"errors":["boom"]}`)
	ackRecorder := httptest.NewRecorder()
	handler.ServeHTTP(ackRecorder, httptest.NewRequest(http.MethodPost, "/ack", ackBody))

	if ackRecorder.Code != http.StatusOK {
		t.Fatalf("ack status = %d, want 200", ackRecorder.Code)
	}
	ackPayload := decodeResponse(t, ackRecorder)
	if ackPayload["status"] != "ok" {
		t.Fatalf("ack status = %v, want ok", ackPayload["status"])
	}

	debugRecorder := httptest.NewRecorder()
	handler.ServeHTTP(debugRecorder, httptest.NewRequest(http.MethodGet, "/debug/state", nil))
	debugPayload := decodeResponse(t, debugRecorder)
	metrics, ok := debugPayload["metrics"].(map[string]any)
	if !ok {
		t.Fatalf("metrics = %#v, want object", debugPayload["metrics"])
	}
	if metrics["ack_count"] != float64(1) {
		t.Fatalf("ack_count = %v, want 1", metrics["ack_count"])
	}
	if metrics["ack_error_count"] != float64(1) {
		t.Fatalf("ack_error_count = %v, want 1", metrics["ack_error_count"])
	}
	if metrics["last_event_batch_size"] != float64(2) || metrics["last_cache_hit_count"] != float64(4) || metrics["last_cache_miss_count"] != float64(1) {
		t.Fatalf("performance metrics = %#v", metrics)
	}
}

func TestActivityEndpointAndDebugState(t *testing.T) {
	handler, appState := newTestHandlerWithState(t)
	body := bytes.NewBufferString(`{"client_id":"studio-1","text":"Syncing snapshot... (55%)","operation":"sync","progress":55,"revision":7}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/activity", body))
	if recorder.Code != http.StatusOK {
		t.Fatalf("activity status = %d body=%q, want 200", recorder.Code, recorder.Body.String())
	}

	activity := appState.Activity()
	if activity.Text != "Syncing snapshot... (55%)" || activity.Operation != "sync" || activity.Progress != 55 || activity.Revision != 7 {
		t.Fatalf("activity = %#v", activity)
	}

	debugRecorder := httptest.NewRecorder()
	handler.ServeHTTP(debugRecorder, httptest.NewRequest(http.MethodGet, "/debug/state", nil))
	debugPayload := decodeResponse(t, debugRecorder)
	debugActivity, ok := debugPayload["activity"].(map[string]any)
	if !ok || debugActivity["text"] != "Syncing snapshot... (55%)" {
		t.Fatalf("debug activity = %#v", debugPayload["activity"])
	}
}

func TestActivityRejectsMissingText(t *testing.T) {
	handler := newTestHandler(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/activity", bytes.NewBufferString(`{"operation":"sync"}`)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("activity status = %d body=%q, want 400", recorder.Code, recorder.Body.String())
	}
}

func TestRemoteExecRejectsDisabled(t *testing.T) {
	handler, _ := newRemoteExecTestHandler(t, false, "secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/exec/commands", bytes.NewBufferString(`{"source":"print(1)"}`)))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%q, want 403", recorder.Code, recorder.Body.String())
	}
}

func TestRemoteExecRejectsEnabledWithEmptyToken(t *testing.T) {
	handler, _ := newRemoteExecTestHandler(t, true, "")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/exec/commands", bytes.NewBufferString(`{"source":"print(1)"}`)))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%q, want 403", recorder.Code, recorder.Body.String())
	}
}

func TestRemoteExecRejectsMissingAndInvalidBearerToken(t *testing.T) {
	handler, _ := newRemoteExecTestHandler(t, true, "secret")

	missingRecorder := httptest.NewRecorder()
	handler.ServeHTTP(missingRecorder, httptest.NewRequest(http.MethodPost, "/exec/commands", bytes.NewBufferString(`{"source":"print(1)"}`)))
	if missingRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d body=%q, want 401", missingRecorder.Code, missingRecorder.Body.String())
	}

	invalidRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/exec/commands", bytes.NewBufferString(`{"source":"print(1)"}`))
	request.Header.Set("Authorization", "Bearer wrong")
	handler.ServeHTTP(invalidRecorder, request)
	if invalidRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token status = %d body=%q, want 401", invalidRecorder.Code, invalidRecorder.Body.String())
	}
}

func TestRemoteExecSubmitValidation(t *testing.T) {
	handler, _ := newRemoteExecTestHandler(t, true, "secret")

	emptyRecorder := httptest.NewRecorder()
	handler.ServeHTTP(emptyRecorder, addExecAuth(httptest.NewRequest(http.MethodPost, "/exec/commands", bytes.NewBufferString(`{"source":"   "}`)), "secret"))
	if emptyRecorder.Code != http.StatusBadRequest {
		t.Fatalf("empty status = %d body=%q, want 400", emptyRecorder.Code, emptyRecorder.Body.String())
	}

	cfg := config.Default()
	cfg.SyncRoot = t.TempDir()
	cfg.RemoteExecEnabled = true
	cfg.RemoteExecToken = "secret"
	cfg.RemoteExecMaxSourceBytes = 4
	limitedHandler, _ := newTestHandlerWithConfig(t, cfg)
	largeRecorder := httptest.NewRecorder()
	limitedHandler.ServeHTTP(largeRecorder, addExecAuth(httptest.NewRequest(http.MethodPost, "/exec/commands", bytes.NewBufferString(`{"source":"print(1)"}`)), "secret"))
	if largeRecorder.Code != http.StatusBadRequest {
		t.Fatalf("large status = %d body=%q, want 400", largeRecorder.Code, largeRecorder.Body.String())
	}
}

func TestRemoteExecSubmitClaimAndDetail(t *testing.T) {
	handler, _ := newRemoteExecTestHandler(t, true, "secret")
	submitRecorder := httptest.NewRecorder()
	handler.ServeHTTP(submitRecorder, addExecAuth(httptest.NewRequest(http.MethodPost, "/exec/commands", bytes.NewBufferString(`{"source":"print(workspace.Name)","client":"cli","mode":"edit","timeout_sec":15}`)), "secret"))
	if submitRecorder.Code != http.StatusOK {
		t.Fatalf("submit status = %d body=%q, want 200", submitRecorder.Code, submitRecorder.Body.String())
	}
	submitPayload := decodeResponse(t, submitRecorder)
	commandID, ok := submitPayload["command_id"].(string)
	if !ok || !strings.HasPrefix(commandID, "cmd_") {
		t.Fatalf("command_id = %#v, want cmd_ prefix", submitPayload["command_id"])
	}

	nextRecorder := httptest.NewRecorder()
	handler.ServeHTTP(nextRecorder, addExecAuth(httptest.NewRequest(http.MethodGet, "/exec/commands/next?client_id=studio&timeout=1", nil), "secret"))
	if nextRecorder.Code != http.StatusOK {
		t.Fatalf("next status = %d body=%q, want 200", nextRecorder.Code, nextRecorder.Body.String())
	}
	nextPayload := decodeResponse(t, nextRecorder)
	command := nextPayload["command"].(map[string]any)
	if command["id"] != commandID || command["source"] != "print(workspace.Name)" || command["timeout_sec"] != float64(15) {
		t.Fatalf("claimed command = %#v", command)
	}

	cancelledContext, cancel := context.WithCancel(context.Background())
	cancel()
	secondRecorder := httptest.NewRecorder()
	secondRequest := addExecAuth(httptest.NewRequest(http.MethodGet, "/exec/commands/next?client_id=studio&timeout=60", nil).WithContext(cancelledContext), "secret")
	handler.ServeHTTP(secondRecorder, secondRequest)
	secondPayload := decodeResponse(t, secondRecorder)
	if secondRecorder.Code != http.StatusOK || secondPayload["command"] != nil {
		t.Fatalf("second claim status=%d payload=%#v, want command nil", secondRecorder.Code, secondPayload)
	}

	detailRecorder := httptest.NewRecorder()
	handler.ServeHTTP(detailRecorder, addExecAuth(httptest.NewRequest(http.MethodGet, "/exec/commands/"+commandID, nil), "secret"))
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("detail status = %d body=%q, want 200", detailRecorder.Code, detailRecorder.Body.String())
	}
	detailPayload := decodeResponse(t, detailRecorder)
	detail := detailPayload["command"].(map[string]any)
	if detail["state"] != string(state.RemoteExecRunning) || detail["source"] != nil {
		t.Fatalf("detail command = %#v, want running without source", detail)
	}
}

func TestRemoteExecLongPollTimeoutReturnsNullCommand(t *testing.T) {
	handler, _ := newRemoteExecTestHandler(t, true, "secret")
	recorder := httptest.NewRecorder()
	started := time.Now()
	handler.ServeHTTP(recorder, addExecAuth(httptest.NewRequest(http.MethodGet, "/exec/commands/next?client_id=studio&timeout=1", nil), "secret"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q, want 200", recorder.Code, recorder.Body.String())
	}
	if time.Since(started) < 900*time.Millisecond {
		t.Fatalf("long poll returned too quickly: %v", time.Since(started))
	}
	payload := decodeResponse(t, recorder)
	if payload["command"] != nil {
		t.Fatalf("command = %#v, want nil", payload["command"])
	}
}

func TestRemoteExecPostResultDoneAndError(t *testing.T) {
	handler, _ := newRemoteExecTestHandler(t, true, "secret")

	submit := func(source string) string {
		t.Helper()
		submitRecorder := httptest.NewRecorder()
		handler.ServeHTTP(submitRecorder, addExecAuth(httptest.NewRequest(http.MethodPost, "/exec/commands", bytes.NewBufferString(`{"source":`+strconv.Quote(source)+`}`)), "secret"))
		if submitRecorder.Code != http.StatusOK {
			t.Fatalf("submit status = %d body=%q", submitRecorder.Code, submitRecorder.Body.String())
		}
		return decodeResponse(t, submitRecorder)["command_id"].(string)
	}
	claim := func() {
		t.Helper()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, addExecAuth(httptest.NewRequest(http.MethodGet, "/exec/commands/next?client_id=studio&timeout=1", nil), "secret"))
		if recorder.Code != http.StatusOK {
			t.Fatalf("claim status = %d body=%q", recorder.Code, recorder.Body.String())
		}
	}

	okID := submit("print('ok')")
	claim()
	okBody := `{"command_id":"` + okID + `","client_id":"studio","ok":true,"duration_ms":12,"prints":[{"level":"print","text":"hello","at_ms":1}],"returns":["Workspace"],"error":"","traceback":""}`
	okRecorder := httptest.NewRecorder()
	handler.ServeHTTP(okRecorder, addExecAuth(httptest.NewRequest(http.MethodPost, "/exec/commands/result", bytes.NewBufferString(okBody)), "secret"))
	if okRecorder.Code != http.StatusOK {
		t.Fatalf("ok result status = %d body=%q", okRecorder.Code, okRecorder.Body.String())
	}

	okDetailRecorder := httptest.NewRecorder()
	handler.ServeHTTP(okDetailRecorder, addExecAuth(httptest.NewRequest(http.MethodGet, "/exec/commands/"+okID, nil), "secret"))
	okDetail := decodeResponse(t, okDetailRecorder)["command"].(map[string]any)
	if okDetail["state"] != string(state.RemoteExecDone) || okDetail["ok"] != true || len(okDetail["prints"].([]any)) != 1 || okDetail["duration_ms"] != float64(12) {
		t.Fatalf("ok detail = %#v", okDetail)
	}

	errID := submit("error('boom')")
	claim()
	errBody := `{"command_id":"` + errID + `","client_id":"studio","ok":false,"duration_ms":4,"prints":[],"returns":[],"error":"boom","traceback":"trace"}`
	errRecorder := httptest.NewRecorder()
	handler.ServeHTTP(errRecorder, addExecAuth(httptest.NewRequest(http.MethodPost, "/exec/commands/result", bytes.NewBufferString(errBody)), "secret"))
	if errRecorder.Code != http.StatusOK {
		t.Fatalf("error result status = %d body=%q", errRecorder.Code, errRecorder.Body.String())
	}
	errDetailRecorder := httptest.NewRecorder()
	handler.ServeHTTP(errDetailRecorder, addExecAuth(httptest.NewRequest(http.MethodGet, "/exec/commands/"+errID, nil), "secret"))
	errDetail := decodeResponse(t, errDetailRecorder)["command"].(map[string]any)
	if errDetail["state"] != string(state.RemoteExecError) || errDetail["error"] != "boom" || errDetail["traceback"] != "trace" {
		t.Fatalf("error detail = %#v", errDetail)
	}

	debugRecorder := httptest.NewRecorder()
	handler.ServeHTTP(debugRecorder, httptest.NewRequest(http.MethodGet, "/debug/state", nil))
	debugPayload := decodeResponse(t, debugRecorder)
	if debugPayload["remote_exec_completed_count"] != float64(1) || debugPayload["remote_exec_error_count"] != float64(1) || debugPayload["remote_exec_last_result_status"] != string(state.RemoteExecError) {
		t.Fatalf("debug payload = %#v", debugPayload)
	}
}

func TestRemoteExecMissingResultAndDetailReturn404(t *testing.T) {
	handler, _ := newRemoteExecTestHandler(t, true, "secret")

	resultRecorder := httptest.NewRecorder()
	handler.ServeHTTP(resultRecorder, addExecAuth(httptest.NewRequest(http.MethodPost, "/exec/commands/result", bytes.NewBufferString(`{"command_id":"cmd_missing","ok":true}`)), "secret"))
	if resultRecorder.Code != http.StatusNotFound {
		t.Fatalf("result status = %d body=%q, want 404", resultRecorder.Code, resultRecorder.Body.String())
	}

	detailRecorder := httptest.NewRecorder()
	handler.ServeHTTP(detailRecorder, addExecAuth(httptest.NewRequest(http.MethodGet, "/exec/commands/cmd_missing", nil), "secret"))
	if detailRecorder.Code != http.StatusNotFound {
		t.Fatalf("detail status = %d body=%q, want 404", detailRecorder.Code, detailRecorder.Body.String())
	}
}

func TestSnapshotUsesCachedState(t *testing.T) {
	handler, appState := newTestHandlerWithState(t)
	appState.SetSnapshot(map[string]records.SyncRecord{
		"ServerScriptService/B.server.luau": {
			Entity:      records.EntityScript,
			LocalPath:   "ServerScriptService/B.server.luau",
			LocalDir:    "ServerScriptService",
			RbxPath:     "game.ServerScriptService.B",
			ClassName:   "Script",
			Source:      "print('b')",
			ContentHash: "sha256:b",
		},
		"ServerScriptService/A.server.luau": {
			Entity:      records.EntityScript,
			LocalPath:   "ServerScriptService/A.server.luau",
			LocalDir:    "ServerScriptService",
			RbxPath:     "game.ServerScriptService.A",
			ClassName:   "Script",
			Source:      "print('a')",
			ContentHash: "sha256:a",
		},
	}, []string{"sample warning"}, 0)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/snapshot", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	payload := decodeResponse(t, recorder)
	changes, ok := payload["changes"].([]any)
	if !ok || len(changes) != 2 {
		t.Fatalf("changes = %#v, want two changes", payload["changes"])
	}
	first := changes[0].(map[string]any)
	if first["local_path"] != "ServerScriptService/A.server.luau" {
		t.Fatalf("first local_path = %v, want sorted A first", first["local_path"])
	}

	debugRecorder := httptest.NewRecorder()
	handler.ServeHTTP(debugRecorder, httptest.NewRequest(http.MethodGet, "/debug/state", nil))
	debugPayload := decodeResponse(t, debugRecorder)
	metrics := debugPayload["metrics"].(map[string]any)
	if metrics["snapshot_requests"] != float64(1) {
		t.Fatalf("snapshot_requests = %v, want 1", metrics["snapshot_requests"])
	}
	if debugPayload["indexed_entry_count"] != float64(2) {
		t.Fatalf("indexed_entry_count = %v, want 2", debugPayload["indexed_entry_count"])
	}
}

func TestSnapshotCompactEncoding(t *testing.T) {
	handler, appState := newTestHandlerWithState(t)
	record := records.SyncRecord{
		Entity:      records.EntityScript,
		LocalPath:   "ServerScriptService/A.server.luau",
		LocalDir:    "ServerScriptService",
		RbxPath:     "game.ServerScriptService.A",
		ClassName:   "Script",
		Source:      "print('a')",
		ContentHash: "sha256:a",
	}
	appState.SetSnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, 0)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/snapshot?encoding=compact-json-v1", nil))
	payload := decodeResponse(t, recorder)
	if payload["change_encoding"] != "compact-json-v1" {
		t.Fatalf("change_encoding = %v, want compact-json-v1", payload["change_encoding"])
	}
	changes := payload["changes"].([]any)
	if len(changes) != 1 {
		t.Fatalf("changes len = %d, want 1", len(changes))
	}
	compact := changes[0].(map[string]any)
	if compact["o"] != "u" || compact["e"] != "s" || compact["l"] != record.LocalPath || compact["rp"] != record.RbxPath || compact["src"] != record.Source {
		t.Fatalf("compact change = %#v", compact)
	}
	expanded := records.ExpandCompactChange(compact)
	if expanded["op"] != "upsert" || expanded["entity"] != records.EntityScript || expanded["local_path"] != record.LocalPath {
		t.Fatalf("expanded change = %#v", expanded)
	}
}

func TestChangesTimeout(t *testing.T) {
	handler := newTestHandler(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/changes?since_rev=0&timeout=1", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	payload := decodeResponse(t, recorder)
	if payload["status"] != "ok" || payload["target_rev"] != float64(0) {
		t.Fatalf("payload = %#v", payload)
	}
	changes := payload["changes"].([]any)
	if len(changes) != 0 {
		t.Fatalf("changes len = %d, want 0", len(changes))
	}
}

func TestChangesReturnsExistingRevision(t *testing.T) {
	handler, appState := newTestHandlerWithState(t)
	record := records.SyncRecord{
		Entity:      records.EntityScript,
		LocalPath:   "ServerScriptService/A.server.luau",
		LocalDir:    "ServerScriptService",
		RbxPath:     "game.ServerScriptService.A",
		ClassName:   "Script",
		Source:      "print(1)",
		ContentHash: records.ContentDigest("print(1)"),
	}
	appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/changes?since_rev=0&timeout=1", nil))
	payload := decodeResponse(t, recorder)
	if payload["target_rev"] != float64(1) {
		t.Fatalf("target_rev = %v, want 1", payload["target_rev"])
	}
	changes := payload["changes"].([]any)
	if len(changes) != 1 {
		t.Fatalf("changes len = %d, want 1", len(changes))
	}
}

func TestChangesCompactEncoding(t *testing.T) {
	handler, appState := newTestHandlerWithState(t)
	record := records.SyncRecord{
		Entity:      records.EntityScript,
		LocalPath:   "ServerScriptService/A.server.luau",
		LocalDir:    "ServerScriptService",
		RbxPath:     "game.ServerScriptService.A",
		ClassName:   "Script",
		Source:      "print(1)",
		ContentHash: records.ContentDigest("print(1)"),
	}
	appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/changes?since_rev=0&timeout=1&encoding=compact-json-v1", nil))
	payload := decodeResponse(t, recorder)
	if payload["change_encoding"] != "compact-json-v1" || payload["target_rev"] != float64(1) {
		t.Fatalf("payload = %#v", payload)
	}
	changes := payload["changes"].([]any)
	if len(changes) != 1 {
		t.Fatalf("changes len = %d, want 1", len(changes))
	}
	compact := changes[0].(map[string]any)
	if compact["o"] != "u" || compact["e"] != "s" {
		t.Fatalf("compact change = %#v", compact)
	}
}

func TestChangesPendingUnblocks(t *testing.T) {
	handler, appState := newTestHandlerWithState(t)
	done := make(chan map[string]any)
	go func() {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/changes?since_rev=0&timeout=5", nil))
		done <- decodeResponse(t, recorder)
	}()

	time.Sleep(50 * time.Millisecond)
	record := records.SyncRecord{
		Entity:      records.EntityScript,
		LocalPath:   "ServerScriptService/A.server.luau",
		LocalDir:    "ServerScriptService",
		RbxPath:     "game.ServerScriptService.A",
		ClassName:   "Script",
		Source:      "print(1)",
		ContentHash: records.ContentDigest("print(1)"),
	}
	appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)

	select {
	case payload := <-done:
		if payload["target_rev"] != float64(1) {
			t.Fatalf("payload = %#v, want target_rev 1", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("pending /changes did not unblock")
	}
}

func TestHistoryListAndDetail(t *testing.T) {
	handler, appState := newTestHandlerWithState(t)
	record := records.SyncRecord{
		Entity:      records.EntityScript,
		LocalPath:   "ServerScriptService/A.server.luau",
		LocalDir:    "ServerScriptService",
		RbxPath:     "game.ServerScriptService.A",
		ClassName:   "Script",
		Source:      "print(1)",
		ContentHash: records.ContentDigest("print(1)"),
	}
	appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)

	listRecorder := httptest.NewRecorder()
	handler.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/history?limit=15", nil))
	listPayload := decodeResponse(t, listRecorder)
	revisions := listPayload["revisions"].([]any)
	if len(revisions) != 1 {
		t.Fatalf("revisions len = %d, want 1", len(revisions))
	}

	detailRecorder := httptest.NewRecorder()
	handler.ServeHTTP(detailRecorder, httptest.NewRequest(http.MethodGet, "/history?rev=1", nil))
	detailPayload := decodeResponse(t, detailRecorder)
	if detailPayload["found"] != true {
		t.Fatalf("found = %v, want true", detailPayload["found"])
	}
	if len(detailPayload["changes"].([]any)) != 1 {
		t.Fatalf("changes = %#v, want one compact change", detailPayload["changes"])
	}

	pathRecorder := httptest.NewRecorder()
	handler.ServeHTTP(pathRecorder, httptest.NewRequest(http.MethodGet, "/history/1", nil))
	pathPayload := decodeResponse(t, pathRecorder)
	if pathPayload["found"] != true {
		t.Fatalf("path detail found = %v, want true", pathPayload["found"])
	}

	missingRecorder := httptest.NewRecorder()
	handler.ServeHTTP(missingRecorder, httptest.NewRequest(http.MethodGet, "/history?rev=404", nil))
	missingPayload := decodeResponse(t, missingRecorder)
	if missingPayload["found"] != false {
		t.Fatalf("missing found = %v, want false", missingPayload["found"])
	}
}

func TestBootstrapValidation(t *testing.T) {
	handler := newTestHandler(t)

	modeRecorder := httptest.NewRecorder()
	handler.ServeHTTP(modeRecorder, httptest.NewRequest(http.MethodPost, "/bootstrap", bytes.NewBufferString(`{"mode":"bad","files":[]}`)))
	if modeRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid mode status = %d, want 400", modeRecorder.Code)
	}

	filesRecorder := httptest.NewRecorder()
	handler.ServeHTTP(filesRecorder, httptest.NewRequest(http.MethodPost, "/bootstrap", bytes.NewBufferString(`{"mode":"merge","files":{}}`)))
	if filesRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid files status = %d, want 400", filesRecorder.Code)
	}
}

func TestBootstrapMergeWriteAndChanges(t *testing.T) {
	cfg := config.Default()
	root := t.TempDir()
	cfg.SyncRoot = root
	handler, _ := newTestHandlerWithConfig(t, cfg)

	body := `{"mode":"merge","files":[{"local_path":"ServerScriptService/Foo.server.luau","source":"print(1)"}]}`
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/bootstrap", bytes.NewBufferString(body)))
	payload := decodeResponse(t, recorder)
	if payload["written_count"] != float64(1) || payload["new_count"] != float64(1) {
		t.Fatalf("bootstrap payload = %#v", payload)
	}
	if payload["server_rev"] != float64(1) {
		t.Fatalf("server_rev = %v, want 1", payload["server_rev"])
	}
	if _, err := os.Stat(filepath.Join(root, "ServerScriptService", "Foo.server.luau")); err != nil {
		t.Fatalf("written file missing: %v", err)
	}

	changesRecorder := httptest.NewRecorder()
	handler.ServeHTTP(changesRecorder, httptest.NewRequest(http.MethodGet, "/changes?since_rev=0&timeout=1", nil))
	changesPayload := decodeResponse(t, changesRecorder)
	if changesPayload["target_rev"] != float64(1) || len(changesPayload["changes"].([]any)) != 1 {
		t.Fatalf("changes payload = %#v", changesPayload)
	}

	debugRecorder := httptest.NewRecorder()
	handler.ServeHTTP(debugRecorder, httptest.NewRequest(http.MethodGet, "/debug/state?limit=20", nil))
	debugPayload := decodeResponse(t, debugRecorder)
	events := debugPayload["events"].([]any)
	foundBootstrap := false
	foundChanges := false
	for _, rawEvent := range events {
		event := rawEvent.(map[string]any)
		message := event["message"].(string)
		if strings.Contains(message, "/bootstrap written=1 updated=0 unchanged=0") {
			foundBootstrap = true
		}
		if strings.Contains(message, "/changes since=0 target=1 changes=1") {
			foundChanges = true
		}
	}
	if !foundBootstrap || !foundChanges {
		t.Fatalf("events missing bootstrap=%t changes=%t: %#v", foundBootstrap, foundChanges, events)
	}
}

func TestBootstrapWaitsForReconciliationLock(t *testing.T) {
	cfg := config.Default()
	root := t.TempDir()
	cfg.SyncRoot = root
	handler, appState := newTestHandlerWithConfig(t, cfg)
	target := filepath.Join(root, "ServerScriptService", "Foo.server.luau")

	appState.LockReconciliation()
	locked := true
	defer func() {
		if locked {
			appState.UnlockReconciliation()
		}
	}()

	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		body := `{"mode":"merge","files":[{"local_path":"ServerScriptService/Foo.server.luau","source":"print(1)"}]}`
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/bootstrap", bytes.NewBufferString(body)))
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("bootstrap completed while reconciliation lock was held")
	case <-time.After(25 * time.Millisecond):
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("bootstrap wrote target while reconciliation lock was held: %v", err)
	}

	appState.UnlockReconciliation()
	locked = false
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("bootstrap did not resume after reconciliation lock was released")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("written file missing: %v", err)
	}
}

func TestBootstrapRefreshesWatches(t *testing.T) {
	cfg := config.Default()
	root := t.TempDir()
	cfg.SyncRoot = root
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	appState := state.New(cfg)
	refreshCount := 0
	handler := NewWithScannerCacheAndWatchRefresh(appState, "3.0.0", scanner.NewCache(), func() error {
		appState.LockReconciliation()
		defer appState.UnlockReconciliation()
		refreshCount++
		return nil
	})

	body := `{"mode":"merge","files":[{"local_path":"ServerScriptService/Pulled/Foo.server.luau","source":"print(1)"}]}`
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/bootstrap", bytes.NewBufferString(body)))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("bootstrap deadlocked while refreshing watches")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if refreshCount != 1 {
		t.Fatalf("refreshCount = %d, want 1", refreshCount)
	}
}

func TestBootstrapRefreshesRealWatcherWithoutDeadlock(t *testing.T) {
	cfg := config.Default()
	root := t.TempDir()
	cfg.SyncRoot = root
	cfg.GitVersioningEnabled = false
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	appState := state.New(cfg)
	cache := scanner.NewCache()
	watchService, err := watcher.New(cfg, appState, watcher.Options{
		Cache:    cache,
		Debounce: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("watcher.New returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := watchService.Start(ctx); err != nil {
		cancel()
		_ = watchService.Close()
		t.Fatalf("watcher.Start returned error: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = watchService.Close()
	})

	handler := NewWithScannerCacheAndWatchRefresh(appState, "3.0.0", cache, func() error {
		_, err := watchService.SyncTree()
		return err
	})
	files := make([]map[string]any, 0, 150)
	for index := 0; index < 150; index++ {
		files = append(files, map[string]any{
			"local_path": fmt.Sprintf("ServerScriptService/Nested%d/Foo.server.luau", index),
			"source":     fmt.Sprintf("print(%d)", index),
		})
	}
	body, err := json.Marshal(map[string]any{"mode": "replace", "files": files})
	if err != nil {
		t.Fatalf("marshal bootstrap payload: %v", err)
	}

	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/bootstrap", bytes.NewReader(body)))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("bootstrap deadlocked while refreshing real watcher")
	}
	payload := decodeResponse(t, recorder)
	if recorder.Code != http.StatusOK || payload["written_count"] != float64(len(files)) {
		t.Fatalf("status=%d payload=%#v", recorder.Code, payload)
	}
}

func TestBootstrapMergeUnchangedAndTraversal(t *testing.T) {
	cfg := config.Default()
	root := t.TempDir()
	cfg.SyncRoot = root
	handler, _ := newTestHandlerWithConfig(t, cfg)
	target := filepath.Join(root, "ServerScriptService", "Foo.server.luau")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("print(1)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	body := `{"mode":"merge","files":[{"local_path":"ServerScriptService/Foo.server.luau","source":"print(1)\r\n"},{"local_path":"../outside.server.luau","source":"bad"}]}`
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/bootstrap", bytes.NewBufferString(body)))
	payload := decodeResponse(t, recorder)
	if payload["unchanged_count"] != float64(1) {
		t.Fatalf("unchanged_count = %v, want 1", payload["unchanged_count"])
	}
	if payload["error_count"] != float64(1) {
		t.Fatalf("error_count = %v, want 1", payload["error_count"])
	}
}

func TestBootstrapReplaceRemovesStaleAndWritesIncoming(t *testing.T) {
	cfg := config.Default()
	root := t.TempDir()
	cfg.SyncRoot = root
	handler, _ := newTestHandlerWithConfig(t, cfg)

	stalePath := filepath.Join(root, "ServerScriptService", "Old.server.luau")
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatalf("mkdir stale parent: %v", err)
	}
	if err := os.WriteFile(stalePath, []byte("print('old')"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	body := `{"mode":"replace","files":[{"local_path":"ServerScriptService/New.server.luau","source":"print('new')"}]}`
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/bootstrap", bytes.NewBufferString(body)))
	payload := decodeResponse(t, recorder)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d payload=%#v", recorder.Code, payload)
	}
	if payload["written_count"] != float64(1) || payload["new_count"] != float64(1) {
		t.Fatalf("payload = %#v, want written/new 1", payload)
	}
	if payload["deleted_count"] != float64(1) {
		t.Fatalf("deleted_count = %v, want 1", payload["deleted_count"])
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("stale file still exists or unexpected err: %v", err)
	}
	incomingPath := filepath.Join(root, "ServerScriptService", "New.server.luau")
	if body, err := os.ReadFile(incomingPath); err != nil {
		t.Fatalf("incoming file missing: %v", err)
	} else if string(body) != "print('new')" {
		t.Fatalf("incoming body = %q, want new source", string(body))
	}
}

func TestBootstrapReplacePreservesGitAndDeletesOthers(t *testing.T) {
	cfg := config.Default()
	root := t.TempDir()
	cfg.SyncRoot = root
	handler, _ := newTestHandlerWithConfig(t, cfg)
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, config.MetadataDir), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	guidebookPath := filepath.Join(root, config.GuidebookDir, "README.md")
	if err := os.MkdirAll(filepath.Dir(guidebookPath), 0o755); err != nil {
		t.Fatalf("mkdir guidebook: %v", err)
	}
	if err := os.WriteFile(guidebookPath, []byte("custom guide"), 0o644); err != nil {
		t.Fatalf("write guidebook: %v", err)
	}
	launcherPath := filepath.Join(root, config.GuidebookDir, config.GuidebookExecLauncher)
	if err := os.WriteFile(launcherPath, []byte("custom launcher"), 0o644); err != nil {
		t.Fatalf("write launcher: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("write old: %v", err)
	}

	body := `{"mode":"replace","files":[]}`
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/bootstrap", bytes.NewBufferString(body)))
	payload := decodeResponse(t, recorder)
	if payload["deleted_count"] != float64(1) {
		t.Fatalf("deleted_count = %v, want 1", payload["deleted_count"])
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatalf(".git missing after replace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, config.MetadataDir)); err != nil {
		t.Fatalf("metadata missing after replace: %v", err)
	}
	if body, err := os.ReadFile(guidebookPath); err != nil {
		t.Fatalf("guidebook missing after replace: %v", err)
	} else if string(body) != "custom guide" {
		t.Fatalf("guidebook overwritten = %q", string(body))
	}
	if body, err := os.ReadFile(launcherPath); err != nil {
		t.Fatalf("launcher missing after replace: %v", err)
	} else if string(body) != "custom launcher" {
		t.Fatalf("launcher overwritten = %q", string(body))
	}
	for _, dir := range config.BootstrapServiceDirs {
		if info, err := os.Stat(filepath.Join(root, dir)); err != nil || !info.IsDir() {
			t.Fatalf("service dir %s missing or not dir: info=%#v err=%v", dir, info, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old.txt still exists or unexpected err: %v", err)
	}
}
