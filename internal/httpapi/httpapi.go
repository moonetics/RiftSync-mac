package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/scanner"
	"riftsync/internal/state"
)

const (
	protocol            = "rbxsync/2.0.0"
	legacyProtocol      = "rbxsync/1.0.0"
	encodingCompactJSON = "compact-json-v1"
	encodingVerboseJSON = "verbose-json-v1"
)

var createReplaceBackup = createReplaceBackupOnDisk

type API struct {
	state          *state.AppState
	version        string
	mux            *http.ServeMux
	scanCache      *scanner.Cache
	refreshWatches func() error
}

func New(appState *state.AppState, version string) http.Handler {
	return NewWithScannerCache(appState, version, scanner.NewCache())
}

func NewWithScannerCache(appState *state.AppState, version string, scanCache *scanner.Cache) http.Handler {
	return NewWithScannerCacheAndWatchRefresh(appState, version, scanCache, nil)
}

func NewWithScannerCacheAndWatchRefresh(appState *state.AppState, version string, scanCache *scanner.Cache, refreshWatches func() error) http.Handler {
	if scanCache == nil {
		scanCache = scanner.NewCache()
	}
	api := &API{
		state:          appState,
		version:        version,
		mux:            http.NewServeMux(),
		scanCache:      scanCache,
		refreshWatches: refreshWatches,
	}
	api.routes()
	return api
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.state.RecordRequest()
	a.mux.ServeHTTP(w, r)
}

func (a *API) routes() {
	a.mux.HandleFunc("/health", a.method(http.MethodGet, a.health))
	a.mux.HandleFunc("/handshake", a.method(http.MethodPost, a.handshake))
	a.mux.HandleFunc("/snapshot", a.method(http.MethodGet, a.snapshot))
	a.mux.HandleFunc("/changes", a.method(http.MethodGet, a.changes))
	a.mux.HandleFunc("/history", a.method(http.MethodGet, a.history))
	a.mux.HandleFunc("/history/", a.method(http.MethodGet, a.historyDetailPath))
	a.mux.HandleFunc("/bootstrap/preview", a.method(http.MethodPost, a.bootstrapPreview))
	a.mux.HandleFunc("/bootstrap", a.method(http.MethodPost, a.bootstrap))
	a.mux.HandleFunc("/ack", a.method(http.MethodPost, a.ack))
	a.mux.HandleFunc("/activity", a.method(http.MethodPost, a.activity))
	a.mux.HandleFunc("/exec/commands", a.method(http.MethodPost, a.execCreate))
	a.mux.HandleFunc("/exec/commands/next", a.method(http.MethodGet, a.execNext))
	a.mux.HandleFunc("/exec/commands/result", a.method(http.MethodPost, a.execResult))
	a.mux.HandleFunc("/exec/commands/", a.method(http.MethodGet, a.execDetailPath))
	a.mux.HandleFunc("/debug/state", a.method(http.MethodGet, a.debugState))
}

func (a *API) method(method string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handler(w, r)
	}
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	cfg := a.state.Config()
	counts := a.state.IndexedCounts()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                    "ok",
		"version":                   a.version,
		"protocol":                  protocol,
		"server_rev":                a.state.Revision(),
		"sync_root":                 cfg.SyncRootAbs,
		"indexed_entry_count":       counts.Entry,
		"indexed_script_count":      counts.Script,
		"indexed_ui_count":          counts.UI,
		"indexed_props_count":       counts.UI,
		"strict_property_whitelist": cfg.StrictPropertyWhitelist,
		"extra_allowed_class_count": len(cfg.ExtraAllowedProperties),
		"git":                       a.state.GitState(),
	})
}

func (a *API) handshake(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		payload = map[string]any{}
	}

	gotProtocol, _ := payload["protocol"].(string)
	if gotProtocol != protocol && gotProtocol != legacyProtocol {
		writeError(w, http.StatusConflict, fmt.Sprintf("Protocol mismatch. expected=%s got=%s", protocol, gotProtocol))
		return
	}
	selectedEncoding := selectChangeEncoding(gotProtocol, payload["accept_encodings"])

	clientID, _ := payload["client_id"].(string)
	if clientID == "" {
		clientID = "unknown"
	}
	sessionID := newSessionID()
	a.state.RecordHandshake(clientID, sessionID, payload["last_applied_rev"])

	cfg := a.state.Config()
	counts := a.state.IndexedCounts()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                     "ok",
		"version":                    a.version,
		"protocol":                   protocol,
		"legacy_protocol":            legacyProtocol,
		"selected_change_encoding":   selectedEncoding,
		"supported_change_encodings": []string{encodingCompactJSON, encodingVerboseJSON},
		"session_id":                 sessionID,
		"server_rev":                 a.state.Revision(),
		"poll_timeout_sec":           cfg.PollTimeoutSec,
		"managed_roots":              cfg.ManagedRoots,
		"ignored_rbx_paths":          cfg.IgnoredRbxPaths,
		"indexed_script_count":       counts.Script,
		"indexed_ui_count":           counts.UI,
		"indexed_props_count":        counts.UI,
		"indexed_entry_count":        counts.Entry,
		"strict_property_whitelist":  cfg.StrictPropertyWhitelist,
		"extra_allowed_properties":   cfg.ExtraAllowedProperties,
	})
}

func (a *API) ack(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		payload = map[string]any{}
	}

	clientID, _ := payload["client_id"].(string)
	if clientID == "" {
		clientID = "unknown"
	}
	statusText, _ := payload["status"].(string)
	if statusText == "" {
		statusText = "ok"
	}
	a.state.RecordAck(clientID, statusText, payload["applied_rev"], sanitizeErrors(payload["errors"]))
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (a *API) activity(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	text := firstNonEmpty(stringValue(payload["text"]), stringValue(payload["message"]))
	if strings.TrimSpace(text) == "" {
		writeError(w, http.StatusBadRequest, "activity text is required")
		return
	}
	if len(text) > 300 {
		text = text[:300]
	}

	activity := a.state.RecordActivity(state.Activity{
		Text:      text,
		Operation: stringValue(payload["operation"]),
		Error:     boolValue(payload["error"]),
		Progress:  intValue(payload["progress"], 0),
		ClientID:  stringValue(payload["client_id"]),
		Revision:  intValue(payload["revision"], 0),
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "activity": activity})
}

func (a *API) execCreate(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRemoteExec(w, r) {
		return
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	source, _ := payload["source"].(string)
	if strings.TrimSpace(source) == "" {
		writeError(w, http.StatusBadRequest, "source is required")
		return
	}

	cfg := a.state.Config()
	if len([]byte(source)) > cfg.RemoteExecMaxSourceBytes {
		writeError(w, http.StatusBadRequest, "source exceeds remote_exec_max_source_bytes")
		return
	}

	timeoutSec := intValue(payload["timeout_sec"], cfg.RemoteExecDefaultTimeoutSec)
	timeoutSec = clampInt(timeoutSec, 1, cfg.RemoteExecMaxTimeoutSec)
	command, err := a.state.EnqueueRemoteExecCommand(
		source,
		stringValue(payload["client"]),
		stringValue(payload["mode"]),
		timeoutSec,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to create command id")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"command_id": command.ID,
		"queued_at":  command.CreatedAt,
	})
}

func (a *API) execNext(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRemoteExec(w, r) {
		return
	}

	clientID := firstNonEmpty(strings.TrimSpace(r.URL.Query().Get("client_id")), "unknown")
	timeoutSec := 25
	if raw := r.URL.Query().Get("timeout"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			timeoutSec = parsed
		}
	}
	timeoutSec = clampInt(timeoutSec, 1, 60)

	command, ok := a.state.ClaimRemoteExecCommand(r.Context(), clientID, time.Duration(timeoutSec)*time.Second)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"command": nil,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"command": map[string]any{
			"id":          command.ID,
			"source":      command.Source,
			"timeout_sec": command.TimeoutSec,
			"created_at":  command.CreatedAt,
		},
	})
}

func (a *API) execResult(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRemoteExec(w, r) {
		return
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	commandID := strings.TrimSpace(stringValue(payload["command_id"]))
	if commandID == "" {
		writeError(w, http.StatusBadRequest, "command_id is required")
		return
	}

	command, err := a.state.RecordRemoteExecResult(state.RemoteExecResult{
		CommandID:  commandID,
		ClientID:   stringValue(payload["client_id"]),
		OK:         boolValue(payload["ok"]),
		DurationMS: floatValue(payload["duration_ms"], 0),
		Prints:     sanitizeRemoteExecPrints(payload["prints"]),
		Returns:    sliceValue(payload["returns"]),
		Error:      stringValue(payload["error"]),
		Traceback:  stringValue(payload["traceback"]),
	})
	if err != nil && errors.Is(err, state.ErrRemoteExecCommandNotFound) {
		writeError(w, http.StatusNotFound, "command not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"command_id":  command.ID,
		"state":       command.State,
		"late_result": command.LateResult,
	})
}

func (a *API) execDetailPath(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRemoteExec(w, r) {
		return
	}

	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/exec/commands/"))
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusBadRequest, "command id is required")
		return
	}
	command, ok := a.state.RemoteExecCommand(id)
	if !ok {
		writeError(w, http.StatusNotFound, "command not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"command": remoteExecCommandPayload(command, false),
	})
}

func (a *API) snapshot(w http.ResponseWriter, r *http.Request) {
	encoding := requestedChangeEncoding(r.URL.Query().Get("encoding"))
	var changes []map[string]any
	if encoding == encodingCompactJSON {
		changes = a.state.CompactSnapshotUpserts()
	} else {
		changes = a.state.SnapshotUpserts()
	}
	payload := map[string]any{
		"status":          "ok",
		"server_rev":      a.state.Revision(),
		"change_encoding": encoding,
		"changes":         changes,
	}
	payloadBytes := encodedPayloadBytes(payload)
	a.state.RecordSnapshotRequest(len(changes), encoding, payloadBytes)
	writeJSON(w, http.StatusOK, payload)
}

func (a *API) changes(w http.ResponseWriter, r *http.Request) {
	requestStarted := time.Now()

	sinceRev, err := strconv.Atoi(firstNonEmpty(r.URL.Query().Get("since_rev"), "0"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "since_rev must be an integer")
		return
	}

	timeoutSeconds := a.state.Config().PollTimeoutSec
	if raw := r.URL.Query().Get("timeout"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			timeoutSeconds = parsed
		}
	}
	if timeoutSeconds < 1 {
		timeoutSeconds = 1
	}
	if timeoutSeconds > 60 {
		timeoutSeconds = 60
	}

	a.state.WaitForRevisionAfter(sinceRev, time.Duration(timeoutSeconds)*time.Second)
	encoding := requestedChangeEncoding(r.URL.Query().Get("encoding"))
	var changesPayload []map[string]any
	var targetRev int
	var needsSnapshot bool
	if encoding == encodingCompactJSON {
		changesPayload, targetRev, needsSnapshot = a.state.FlattenCompactChanges(sinceRev)
	} else {
		changesPayload, targetRev, needsSnapshot = a.state.FlattenChanges(sinceRev)
	}
	waitedSec := time.Since(requestStarted).Seconds()
	timedOut := waitedSec >= float64(timeoutSeconds) && a.state.Revision() <= sinceRev
	payload := map[string]any{
		"status":          "ok",
		"base_rev":        sinceRev,
		"target_rev":      targetRev,
		"server_rev":      a.state.Revision(),
		"needs_snapshot":  needsSnapshot,
		"change_encoding": encoding,
		"changes":         changesPayload,
	}
	payloadBytes := encodedPayloadBytes(payload)
	a.state.RecordChangesRequest(sinceRev, targetRev, needsSnapshot, len(changesPayload), timedOut, waitedSec, encoding, payloadBytes)

	writeJSON(w, http.StatusOK, payload)
}

func (a *API) history(w http.ResponseWriter, r *http.Request) {
	if revArg := r.URL.Query().Get("rev"); revArg != "" {
		revision, err := strconv.Atoi(revArg)
		if err != nil {
			writeError(w, http.StatusBadRequest, "rev must be an integer")
			return
		}
		a.writeHistoryDetail(w, revision)
		return
	}

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"server_rev": a.state.Revision(),
		"git":        a.state.GitState(),
		"revisions":  a.state.RevisionSummaries(limit),
	})
}

func (a *API) historyDetailPath(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimPrefix(r.URL.Path, "/history/")
	revision, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "rev must be an integer")
		return
	}
	a.writeHistoryDetail(w, revision)
}

func (a *API) writeHistoryDetail(w http.ResponseWriter, revision int) {
	summary, changes, found := a.state.RevisionDetail(revision)
	payload := map[string]any{
		"status":     "ok",
		"found":      found,
		"rev":        revision,
		"server_rev": a.state.Revision(),
		"git":        a.state.GitState(),
		"changes":    changes,
	}
	if found {
		payload["ts"] = summary.TS
		payload["change_count"] = summary.ChangeCount
		payload["op_counts"] = summary.OpCounts
		payload["entity_counts"] = summary.EntityCounts
		payload["git_commit"] = summary.GitCommit
		payload["git_commit_short"] = summary.GitCommitShort
	}
	writeJSON(w, http.StatusOK, payload)
}

type bootstrapPreviewResult struct {
	Status         string   `json:"status"`
	Mode           string   `json:"mode"`
	AddCount       int      `json:"add_count"`
	UpdateCount    int      `json:"update_count"`
	DeleteCount    int      `json:"delete_count"`
	UnchangedCount int      `json:"unchanged_count"`
	FilesToAdd     []string `json:"files_to_add"`
	FilesToUpdate  []string `json:"files_to_update"`
	FilesToDelete  []string `json:"files_to_delete"`
	Errors         []string `json:"errors"`
}

func (a *API) bootstrapPreview(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		payload = map[string]any{}
	}
	mode, _ := payload["mode"].(string)
	if mode == "" {
		mode = "replace"
	}
	if mode != "replace" && mode != "merge" {
		writeError(w, http.StatusBadRequest, "Unsupported bootstrap mode")
		return
	}
	files, ok := payload["files"].([]any)
	if !ok {
		writeError(w, http.StatusBadRequest, "files must be an array")
		return
	}
	preview := calculateBootstrapPreview(a.state.Config(), mode, files)
	writeJSON(w, http.StatusOK, preview)
}

func calculateBootstrapPreview(cfg config.Config, mode string, files []any) bootstrapPreviewResult {
	preview := bootstrapPreviewResult{
		Status:        "ok",
		Mode:          mode,
		FilesToAdd:    []string{},
		FilesToUpdate: []string{},
		FilesToDelete: []string{},
		Errors:        []string{},
	}
	incoming := map[string]string{}
	for _, rawItem := range files {
		item, ok := rawItem.(map[string]any)
		if !ok {
			preview.Errors = append(preview.Errors, "Invalid file payload entry")
			continue
		}
		localPath := fmt.Sprint(item["local_path"])
		source, ok := item["source"].(string)
		if !ok {
			preview.Errors = append(preview.Errors, fmt.Sprintf("Invalid source for %s", localPath))
			continue
		}
		target, err := cfg.ResolveInsideSyncRoot(localPath)
		if err != nil {
			preview.Errors = append(preview.Errors, fmt.Sprintf("Invalid local_path %s", localPath))
			continue
		}
		relative, err := filepath.Rel(cfg.SyncRootAbs, target)
		if err != nil {
			preview.Errors = append(preview.Errors, fmt.Sprintf("Invalid local_path %s", localPath))
			continue
		}
		relPosix := filepath.ToSlash(relative)
		incoming[relPosix] = source

		info, statErr := os.Stat(target)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				preview.FilesToAdd = append(preview.FilesToAdd, relPosix)
				continue
			}
			preview.Errors = append(preview.Errors, fmt.Sprintf("Failed stat %s: %v", relPosix, statErr))
			continue
		}
		if info.IsDir() {
			preview.FilesToAdd = append(preview.FilesToAdd, relPosix)
			continue
		}
		existingSource, readErr := os.ReadFile(target)
		if readErr != nil {
			preview.Errors = append(preview.Errors, fmt.Sprintf("Failed reading existing %s: %v", relPosix, readErr))
			continue
		}
		if normalizePullText(string(existingSource)) == normalizePullText(source) {
			preview.UnchangedCount++
			continue
		}
		preview.FilesToUpdate = append(preview.FilesToUpdate, relPosix)
	}

	if mode == "replace" {
		localFiles, err := listSyncRootFiles(cfg.SyncRootAbs)
		if err != nil {
			preview.Errors = append(preview.Errors, fmt.Sprintf("Failed walking sync_root: %v", err))
		}
		for _, relPosix := range localFiles {
			if _, exists := incoming[relPosix]; !exists {
				preview.FilesToDelete = append(preview.FilesToDelete, relPosix)
			}
		}
	}

	sort.Strings(preview.FilesToAdd)
	sort.Strings(preview.FilesToUpdate)
	sort.Strings(preview.FilesToDelete)
	preview.AddCount = len(preview.FilesToAdd)
	preview.UpdateCount = len(preview.FilesToUpdate)
	preview.DeleteCount = len(preview.FilesToDelete)
	return preview
}

func (a *API) bootstrap(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		payload = map[string]any{}
	}

	mode, _ := payload["mode"].(string)
	if mode == "" {
		mode = "replace"
	}
	if mode != "replace" && mode != "merge" {
		writeError(w, http.StatusBadRequest, "Unsupported bootstrap mode")
		return
	}
	files, ok := payload["files"].([]any)
	if !ok {
		writeError(w, http.StatusBadRequest, "files must be an array")
		return
	}

	cfg := a.state.Config()
	writeErrors := []string{}
	writtenCount := 0
	newCount := 0
	updatedCount := 0
	unchangedCount := 0
	deletedCount := 0
	backupPath := ""
	backupCount := 0

	a.state.LockReconciliation()
	if mode == "replace" {
		path, count, err := createReplaceBackup(cfg.SyncRootAbs, time.Now())
		if err != nil {
			a.state.UnlockReconciliation()
			writeError(w, http.StatusInternalServerError, "Unable to backup sync_root: "+err.Error())
			return
		}
		backupPath = path
		backupCount = count
		count, err = clearSyncRootContents(cfg.SyncRootAbs)
		if err != nil {
			a.state.UnlockReconciliation()
			writeError(w, http.StatusInternalServerError, "Unable to reset sync_root: "+err.Error())
			return
		}
		a.scanCache.InvalidateSubtree(".")
		deletedCount = count
	}

	for _, rawItem := range files {
		item, ok := rawItem.(map[string]any)
		if !ok {
			writeErrors = append(writeErrors, "Invalid file payload entry")
			continue
		}
		localPath := fmt.Sprint(item["local_path"])
		source, ok := item["source"].(string)
		if !ok {
			writeErrors = append(writeErrors, fmt.Sprintf("Invalid source for %s", localPath))
			continue
		}
		target, err := cfg.ResolveInsideSyncRoot(localPath)
		if err != nil {
			writeErrors = append(writeErrors, fmt.Sprintf("Invalid local_path %s", localPath))
			continue
		}

		targetExists := false
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			targetExists = true
		}
		if mode == "merge" && targetExists {
			existingSource, err := os.ReadFile(target)
			if err != nil {
				writeErrors = append(writeErrors, fmt.Sprintf("Failed reading existing %s: %v", localPath, err))
				continue
			}
			if normalizePullText(string(existingSource)) == normalizePullText(source) {
				unchangedCount++
				continue
			}
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			writeErrors = append(writeErrors, fmt.Sprintf("Failed creating parent for %s: %v", localPath, err))
			continue
		}
		if err := os.WriteFile(target, []byte(source), 0o644); err != nil {
			writeErrors = append(writeErrors, fmt.Sprintf("Failed writing %s: %v", localPath, err))
			continue
		}
		a.scanCache.Invalidate(localPath)
		writtenCount++
		if targetExists {
			updatedCount++
		} else {
			newCount++
		}
	}
	if err := config.EnsureSyncRootScaffold(cfg); err != nil {
		a.state.UnlockReconciliation()
		writeError(w, http.StatusInternalServerError, "Unable to setup sync_root: "+err.Error())
		return
	}
	a.state.UnlockReconciliation()

	if a.refreshWatches != nil {
		if err := a.refreshWatches(); err != nil {
			writeErrors = append(writeErrors, fmt.Sprintf("Failed refreshing file watcher: %v", err))
		}
	}

	a.state.LockReconciliation()
	scanStarted := time.Now()
	snapshot, scanErr := a.scanCache.Scan(cfg)
	if scanErr != nil {
		writeErrors = append(writeErrors, scanErr.Error())
	}
	a.state.ApplySnapshot(snapshot.Records, snapshot.Warnings, snapshot.InvalidPaths)
	a.state.RecordPerformance(len(files), 0, time.Since(scanStarted), 0, snapshot.CacheHits, snapshot.CacheMisses)
	if err := config.WriteGuidebookStatus(cfg, config.ScaffoldOptions{
		Version:           a.version,
		LastKnownRevision: a.state.Revision(),
	}); err != nil {
		writeErrors = append(writeErrors, fmt.Sprintf("Failed writing guidebook status: %v", err))
	}
	a.state.RecordBootstrap(writtenCount, updatedCount, unchangedCount, deletedCount, len(writeErrors))
	a.state.UnlockReconciliation()

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"written_count":   writtenCount,
		"new_count":       newCount,
		"updated_count":   updatedCount,
		"unchanged_count": unchangedCount,
		"deleted_count":   deletedCount,
		"backup_path":     backupPath,
		"backup_count":    backupCount,
		"error_count":     len(writeErrors),
		"errors":          writeErrors,
		"scan_warnings":   firstWarnings(snapshot.Warnings, 20),
		"server_rev":      a.state.Revision(),
	})
}

func (a *API) debugState(w http.ResponseWriter, r *http.Request) {
	limit := 40
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			if parsed < 1 {
				parsed = 1
			}
			if parsed > 200 {
				parsed = 200
			}
			limit = parsed
		}
	}

	cfg := a.state.Config()
	counts := a.state.IndexedCounts()
	execDebug := a.state.RemoteExecDebug()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                         "ok",
		"server_rev":                     a.state.Revision(),
		"indexed_entry_count":            counts.Entry,
		"indexed_script_count":           counts.Script,
		"indexed_ui_count":               counts.UI,
		"indexed_props_count":            counts.UI,
		"sessions_active":                a.state.SessionsActive(),
		"sync_root":                      cfg.SyncRootAbs,
		"scan_interval_sec":              cfg.ScanIntervalSec,
		"strict_property_whitelist":      cfg.StrictPropertyWhitelist,
		"extra_allowed_properties":       cfg.ExtraAllowedProperties,
		"extra_allowed_class_count":      len(cfg.ExtraAllowedProperties),
		"git":                            a.state.GitState(),
		"metrics":                        a.state.Metrics(),
		"activity":                       a.state.Activity(),
		"events":                         a.state.Events(limit),
		"scan_warnings":                  a.state.ScanWarnings(limit),
		"remote_exec_enabled":            execDebug.Enabled,
		"remote_exec_queue_count":        execDebug.QueueCount,
		"remote_exec_running_count":      execDebug.RunningCount,
		"remote_exec_completed_count":    execDebug.CompletedCount,
		"remote_exec_error_count":        execDebug.ErrorCount,
		"remote_exec_last_command_at":    execDebug.LastCommandAt,
		"remote_exec_last_result_status": execDebug.LastResultStatus,
	})
}

func (a *API) authorizeRemoteExec(w http.ResponseWriter, r *http.Request) bool {
	cfg := a.state.Config()
	if !cfg.RemoteExecEnabled {
		writeError(w, http.StatusForbidden, "remote exec is disabled")
		return false
	}
	if strings.TrimSpace(cfg.RemoteExecToken) == "" {
		writeError(w, http.StatusForbidden, "remote exec token is not configured")
		return false
	}

	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		writeError(w, http.StatusUnauthorized, "remote exec token is required")
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	if subtle.ConstantTimeCompare([]byte(got), []byte(cfg.RemoteExecToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "remote exec token is invalid")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func encodedPayloadBytes(payload any) int {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0
	}
	return len(body)
}

func requestedChangeEncoding(raw string) string {
	if raw == encodingCompactJSON {
		return encodingCompactJSON
	}
	return encodingVerboseJSON
}

func selectChangeEncoding(protocolValue string, value any) string {
	raw, ok := value.([]any)
	if !ok {
		if protocolValue == protocol {
			return encodingCompactJSON
		}
		return encodingVerboseJSON
	}
	for _, item := range raw {
		if text, ok := item.(string); ok && text == encodingCompactJSON {
			return encodingCompactJSON
		}
	}
	return encodingVerboseJSON
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]any{
		"status":  "error",
		"message": message,
	})
}

func sanitizeErrors(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return []string{}
	}
	limit := len(raw)
	if limit > 5 {
		limit = 5
	}
	result := make([]string, 0, limit)
	for _, item := range raw[:limit] {
		result = append(result, fmt.Sprint(item))
	}
	return result
}

func newSessionID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "session-fallback"
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func createReplaceBackupOnDisk(syncRoot string, at time.Time) (string, int, error) {
	if err := os.MkdirAll(syncRoot, 0o755); err != nil {
		return "", 0, err
	}
	entries, err := os.ReadDir(syncRoot)
	if err != nil {
		return "", 0, err
	}
	content := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if isPreservedRootEntry(entry.Name()) {
			continue
		}
		content = append(content, entry)
	}
	if len(content) == 0 {
		return "", 0, nil
	}

	backupRoot := filepath.Join(syncRoot, config.MetadataDir, "backups")
	if err := os.MkdirAll(backupRoot, 0o755); err != nil {
		return "", 0, err
	}
	backupPath, err := uniqueBackupPath(backupRoot, at)
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(backupPath, 0o755); err != nil {
		return "", 0, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(backupPath)
		}
	}()

	for _, entry := range content {
		source := filepath.Join(syncRoot, entry.Name())
		target := filepath.Join(backupPath, entry.Name())
		if err := copyFileTree(source, target); err != nil {
			return "", 0, err
		}
	}
	success = true
	return backupPath, len(content), nil
}

func uniqueBackupPath(backupRoot string, at time.Time) (string, error) {
	if at.IsZero() {
		at = time.Now()
	}
	base := at.UTC().Format("20060102T150405.000000000Z")
	for index := 0; index < 1000; index++ {
		name := base
		if index > 0 {
			name = fmt.Sprintf("%s-%d", base, index)
		}
		candidate := filepath.Join(backupRoot, name)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("unable to choose unique backup path under %s", backupRoot)
}

func copyFileTree(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to backup symlink %s", source)
	}
	if !info.IsDir() {
		return copyRegularFile(source, target, info.Mode().Perm())
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(target, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to backup symlink %s", path)
		}
		if entry.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		return copyRegularFile(path, targetPath, info.Mode().Perm())
	})
}

func copyRegularFile(source, target string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func clearSyncRootContents(syncRoot string) (int, error) {
	if err := os.MkdirAll(syncRoot, 0o755); err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(syncRoot)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, entry := range entries {
		if isPreservedRootEntry(entry.Name()) {
			continue
		}
		target := filepath.Join(syncRoot, entry.Name())
		if err := os.RemoveAll(target); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func listSyncRootFiles(syncRoot string) ([]string, error) {
	files := []string{}
	if _, err := os.Stat(syncRoot); err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return files, err
	}
	err := filepath.WalkDir(syncRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == syncRoot {
			return nil
		}
		if entry.IsDir() && isPreservedRootEntry(entry.Name()) {
			relative, err := filepath.Rel(syncRoot, path)
			if err == nil && !strings.Contains(filepath.ToSlash(relative), "/") {
				return filepath.SkipDir
			}
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(syncRoot, path)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	sort.Strings(files)
	return files, err
}

func isPreservedRootEntry(name string) bool {
	return name == ".git" || name == config.MetadataDir || name == config.GuidebookDir
}

func normalizePullText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

func firstWarnings(warnings []string, limit int) []string {
	if len(warnings) <= limit {
		return warnings
	}
	return warnings[:limit]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "1"
	default:
		return false
	}
}

func intValue(value any, fallback int) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func floatValue(value any, fallback float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func sliceValue(value any) []any {
	raw, ok := value.([]any)
	if !ok {
		return []any{}
	}
	return append([]any(nil), raw...)
}

func sanitizeRemoteExecPrints(value any) []state.RemoteExecPrint {
	raw, ok := value.([]any)
	if !ok {
		return []state.RemoteExecPrint{}
	}
	result := make([]state.RemoteExecPrint, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		level := strings.TrimSpace(stringValue(entry["level"]))
		if level == "" {
			level = "print"
		}
		result = append(result, state.RemoteExecPrint{
			Level: level,
			Text:  stringValue(entry["text"]),
			AtMS:  floatValue(entry["at_ms"], 0),
		})
	}
	return result
}

func remoteExecCommandPayload(command state.RemoteExecCommand, includeSource bool) map[string]any {
	payload := map[string]any{
		"id":           command.ID,
		"state":        command.State,
		"timeout_sec":  command.TimeoutSec,
		"client":       command.Client,
		"mode":         command.Mode,
		"created_at":   command.CreatedAt,
		"claimed_at":   command.ClaimedAt,
		"completed_at": command.CompletedAt,
		"claimed_by":   command.ClaimedBy,
		"ok":           command.OK,
		"duration_ms":  command.DurationMS,
		"prints":       command.Prints,
		"returns":      command.Returns,
		"error":        command.Error,
		"traceback":    command.Traceback,
		"late_result":  command.LateResult,
	}
	if includeSource {
		payload["source"] = command.Source
	}
	return payload
}

func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
