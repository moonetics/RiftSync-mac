package state

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"riftsync/internal/config"
	"riftsync/internal/debuglog"
	"riftsync/internal/records"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type IndexedCounts struct {
	Entry  int
	Script int
	UI     int
}

type HandshakeInfo struct {
	ClientID       string `json:"client_id"`
	SessionID      string `json:"session_id"`
	LastAppliedRev any    `json:"last_applied_rev"`
	At             int64  `json:"at"`
}

type AckInfo struct {
	ClientID   string   `json:"client_id"`
	Status     string   `json:"status"`
	AppliedRev any      `json:"applied_rev"`
	Errors     []string `json:"errors"`
	At         int64    `json:"at"`
}

type Metrics struct {
	RequestCount            int            `json:"request_count"`
	HandshakeCount          int            `json:"handshake_count"`
	SnapshotRequests        int            `json:"snapshot_requests"`
	AckCount                int            `json:"ack_count"`
	AckErrorCount           int            `json:"ack_error_count"`
	ChangesRequests         int            `json:"changes_requests"`
	ChangesTimeouts         int            `json:"changes_timeouts"`
	ScanCycles              int            `json:"scan_cycles"`
	LastScanSec             float64        `json:"last_scan_sec"`
	LastScanEntryCount      int            `json:"last_scan_entry_count"`
	LastScanScriptCount     int            `json:"last_scan_script_count"`
	LastScanUICount         int            `json:"last_scan_ui_count"`
	LastScanInvalidJSON     int            `json:"last_scan_invalid_json"`
	LastRevisionChangeCount int            `json:"last_revision_change_count"`
	LastRevisionBumpAt      float64        `json:"last_revision_bump_at"`
	LastEventBatchSize      int            `json:"last_event_batch_size"`
	LastDebounceMS          float64        `json:"last_debounce_ms"`
	LastParseDurationMS     float64        `json:"last_parse_duration_ms"`
	LastPublishDurationMS   float64        `json:"last_publish_duration_ms"`
	LastCacheHitCount       int            `json:"last_cache_hit_count"`
	LastCacheMissCount      int            `json:"last_cache_miss_count"`
	LastChangeEncoding      string         `json:"last_change_encoding"`
	LastPayloadBytes        int            `json:"last_payload_bytes"`
	LastHandshake           HandshakeInfo  `json:"last_handshake"`
	LastAck                 AckInfo        `json:"last_ack"`
	LastChanges             map[string]any `json:"last_changes"`
	LastError               string         `json:"last_error"`
}

type GitState struct {
	Enabled         bool   `json:"enabled"`
	Available       bool   `json:"available"`
	Status          string `json:"status"`
	RepoPath        string `json:"repo_path"`
	LastCommit      string `json:"last_commit"`
	LastCommitShort string `json:"last_commit_short"`
	LastError       string `json:"last_error"`
}

const (
	GitStatusDisabled      = "disabled"
	GitStatusPending       = "pending"
	GitStatusChecking      = "checking"
	GitStatusInitializing  = "initializing"
	GitStatusInitialCommit = "initial_commit"
	GitStatusReady         = "ready"
	GitStatusCommitting    = "committing"
	GitStatusError         = "error"
)

type Activity struct {
	Text          string         `json:"text"`
	Operation     string         `json:"operation"`
	Phase         string         `json:"phase,omitempty"`
	Error         bool           `json:"error"`
	Progress      int            `json:"progress"`
	Current       int            `json:"current,omitempty"`
	Total         int            `json:"total,omitempty"`
	Indeterminate bool           `json:"indeterminate,omitempty"`
	ClientID      string         `json:"client_id"`
	Revision      int            `json:"revision"`
	At            float64        `json:"at"`
	Details       map[string]any `json:"details,omitempty"`
}

type RemoteExecState string

const (
	RemoteExecQueued    RemoteExecState = "queued"
	RemoteExecRunning   RemoteExecState = "running"
	RemoteExecDone      RemoteExecState = "done"
	RemoteExecError     RemoteExecState = "error"
	RemoteExecTimeout   RemoteExecState = "timeout"
	RemoteExecExpired   RemoteExecState = "expired"
	RemoteExecCancelled RemoteExecState = "cancelled"
)

const RemoteExecRunningGrace = 5 * time.Second

var (
	ErrRemoteExecCommandNotFound = errors.New("remote exec command not found")
	ErrRemoteExecTerminalState   = errors.New("remote exec command is already terminal")
)

type RemoteExecPrint struct {
	Level string  `json:"level"`
	Text  string  `json:"text"`
	AtMS  float64 `json:"at_ms"`
}

type RemoteExecCommand struct {
	ID          string            `json:"id"`
	Source      string            `json:"source,omitempty"`
	TimeoutSec  int               `json:"timeout_sec"`
	Client      string            `json:"client"`
	Mode        string            `json:"mode"`
	State       RemoteExecState   `json:"state"`
	CreatedAt   float64           `json:"created_at"`
	ClaimedAt   float64           `json:"claimed_at,omitempty"`
	CompletedAt float64           `json:"completed_at,omitempty"`
	ClaimedBy   string            `json:"claimed_by,omitempty"`
	OK          bool              `json:"ok"`
	DurationMS  float64           `json:"duration_ms,omitempty"`
	Prints      []RemoteExecPrint `json:"prints,omitempty"`
	Returns     []any             `json:"returns,omitempty"`
	Error       string            `json:"error,omitempty"`
	Traceback   string            `json:"traceback,omitempty"`
	LateResult  bool              `json:"late_result,omitempty"`
}

type RemoteExecResult struct {
	CommandID  string
	ClientID   string
	OK         bool
	DurationMS float64
	Prints     []RemoteExecPrint
	Returns    []any
	Error      string
	Traceback  string
}

type RemoteExecDebug struct {
	Enabled          bool    `json:"remote_exec_enabled"`
	QueueCount       int     `json:"remote_exec_queue_count"`
	RunningCount     int     `json:"remote_exec_running_count"`
	CompletedCount   int     `json:"remote_exec_completed_count"`
	ErrorCount       int     `json:"remote_exec_error_count"`
	LastCommandAt    float64 `json:"remote_exec_last_command_at"`
	LastResultStatus string  `json:"remote_exec_last_result_status"`
}

type RevisionEvent struct {
	Rev          int              `json:"rev"`
	TS           float64          `json:"ts"`
	ChangeCount  int              `json:"change_count"`
	OpCounts     map[string]int   `json:"op_counts"`
	EntityCounts map[string]int   `json:"entity_counts"`
	GitCommit    string           `json:"git_commit"`
	GitShort     string           `json:"git_commit_short"`
	Changes      []map[string]any `json:"changes"`
	Compact      []map[string]any `json:"-"`
}

type RevisionSummary struct {
	Rev            int            `json:"rev"`
	TS             float64        `json:"ts"`
	ChangeCount    int            `json:"change_count"`
	OpCounts       map[string]int `json:"op_counts"`
	EntityCounts   map[string]int `json:"entity_counts"`
	GitCommit      string         `json:"git_commit"`
	GitCommitShort string         `json:"git_commit_short"`
}

type Session struct {
	ClientID  string
	CreatedAt time.Time
}

type AppState struct {
	mu                   sync.Mutex
	reconcileMu          sync.Mutex
	cfg                  config.Config
	bootTime             time.Time
	revision             int
	counts               IndexedCounts
	records              map[string]records.SyncRecord
	snapshot             []map[string]any
	compactSnapshot      []map[string]any
	warnings             []string
	changeLog            []RevisionEvent
	sessions             map[string]Session
	metrics              Metrics
	git                  GitState
	activity             Activity
	events               *debuglog.Ring
	changed              chan struct{}
	execCommands         map[string]*RemoteExecCommand
	execOrder            []string
	execChanged          chan struct{}
	execCompletedCount   int
	execErrorCount       int
	execLastCommandAt    float64
	execLastResultStatus string
	listeners            []func(RevisionEvent)
	projectInitialized   bool
	tombstones           map[string]DeletionTombstone
}

type persistentHistory struct {
	Version   int             `json:"version"`
	Revision  int             `json:"revision"`
	ChangeLog []RevisionEvent `json:"change_log"`
}

func New(cfg config.Config) *AppState {
	events := debuglog.New(cfg.DebugEventRetention)
	events.Add("boot", "server booted")
	return &AppState{
		cfg:          cfg,
		bootTime:     time.Now(),
		records:      map[string]records.SyncRecord{},
		sessions:     map[string]Session{},
		changed:      make(chan struct{}),
		execCommands: map[string]*RemoteExecCommand{},
		execChanged:  make(chan struct{}),
		tombstones:   map[string]DeletionTombstone{},
		metrics: Metrics{
			LastChanges: map[string]any{},
		},
		git: GitState{
			Enabled:  cfg.GitVersioningEnabled,
			Status:   initialGitStatus(cfg.GitVersioningEnabled),
			RepoPath: cfg.SyncRootAbs,
		},
		events: events,
	}
}

func initialGitStatus(enabled bool) string {
	if !enabled {
		return GitStatusDisabled
	}
	return GitStatusPending
}

func (s *AppState) Config() config.Config {
	return s.cfg
}

func (s *AppState) LockReconciliation() {
	s.reconcileMu.Lock()
}

func (s *AppState) UnlockReconciliation() {
	s.reconcileMu.Unlock()
}

func (s *AppState) Revision() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revision
}

func (s *AppState) IndexedCounts() IndexedCounts {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts
}

func (s *AppState) SetSnapshot(snapshotRecords map[string]records.SyncRecord, warnings []string, invalidJSONCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.replaceRecordsLocked(snapshotRecords, warnings, invalidJSONCount)
}

func (s *AppState) ApplySnapshot(snapshotRecords map[string]records.SyncRecord, warnings []string, invalidPaths []string) RevisionEvent {
	s.mu.Lock()
	oldRecords := cloneRecords(s.records)
	invalidSet := map[string]bool{}
	for _, path := range invalidPaths {
		invalidSet[path] = true
	}
	changes := records.DetectChanges(oldRecords, snapshotRecords, invalidSet)
	s.replaceRecordsLocked(snapshotRecords, warnings, len(invalidPaths))
	if len(changes) == 0 {
		s.mu.Unlock()
		return RevisionEvent{}
	}

	s.revision++
	event := buildRevisionEvent(s.revision, changes)
	s.applyChangesToTombstonesLocked(changes, s.revision)
	s.changeLog = append(s.changeLog, event)
	if len(s.changeLog) > s.cfg.ChangeRetention {
		s.changeLog = s.changeLog[len(s.changeLog)-s.cfg.ChangeRetention:]
	}
	s.metrics.LastRevisionBumpAt = nowSeconds()
	s.metrics.LastRevisionChangeCount = len(changes)
	persistErr := s.saveHistoryLocked()
	projectStateErr := s.saveProjectStateLocked()
	listeners := append([]func(RevisionEvent){}, s.listeners...)
	s.broadcastLocked()
	s.mu.Unlock()

	if persistErr != nil {
		s.events.Add("history", persistErr.Error())
	}
	if projectStateErr != nil {
		s.events.Add("project-state", projectStateErr.Error())
	}
	s.events.Add("revision", "revision "+strconv.Itoa(event.Rev)+" changes="+strconv.Itoa(event.ChangeCount))
	for _, listener := range listeners {
		listener(event)
	}
	return event
}

func (s *AppState) SnapshotUpserts() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	return cloneChangeList(s.snapshot)
}

func (s *AppState) CompactSnapshotUpserts() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	return cloneChangeList(s.compactSnapshot)
}

func (s *AppState) FlattenChanges(sinceRev int) ([]map[string]any, int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flattenChangesLocked(sinceRev)
}

func (s *AppState) FlattenCompactChanges(sinceRev int) ([]map[string]any, int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flattenCompactChangesLocked(sinceRev)
}

func (s *AppState) ClearChangeLog() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.changeLog = nil
	if err := s.saveHistoryLocked(); err != nil {
		s.events.Add("history", err.Error())
	}
}

func (s *AppState) RecordsCopy() map[string]records.SyncRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneRecords(s.records)
}

func (s *AppState) EnqueueRemoteExecCommand(source, client, mode string, timeoutSec int) (RemoteExecCommand, error) {
	id, err := newRemoteExecID()
	if err != nil {
		return RemoteExecCommand{}, err
	}
	now := nowSeconds()
	command := &RemoteExecCommand{
		ID:         id,
		Source:     source,
		TimeoutSec: timeoutSec,
		Client:     strings.TrimSpace(client),
		Mode:       strings.TrimSpace(mode),
		State:      RemoteExecQueued,
		CreatedAt:  now,
	}
	if command.Client == "" {
		command.Client = "unknown"
	}
	if command.Mode == "" {
		command.Mode = "edit"
	}

	s.mu.Lock()
	s.expireRemoteExecLocked(time.Now())
	s.execCommands[command.ID] = command
	s.execOrder = append(s.execOrder, command.ID)
	s.execLastCommandAt = now
	s.broadcastExecLocked()
	copy := cloneRemoteExecCommand(command)
	s.mu.Unlock()

	s.events.Add("exec", fmt.Sprintf("exec queued id=%s bytes=%d timeout=%d", command.ID, len(source), timeoutSec))
	return copy, nil
}

func (s *AppState) ClaimRemoteExecCommand(ctx context.Context, clientID string, timeout time.Duration) (RemoteExecCommand, bool) {
	deadline := time.Now().Add(timeout)
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = "unknown"
	}

	for {
		s.mu.Lock()
		s.expireRemoteExecLocked(time.Now())
		if command := s.claimQueuedRemoteExecLocked(clientID); command != nil {
			copy := cloneRemoteExecCommand(command)
			s.mu.Unlock()
			s.events.Add("exec", "exec claimed id="+copy.ID+" client="+clientID)
			return copy, true
		}
		changed := s.execChanged
		s.mu.Unlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return RemoteExecCommand{}, false
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return RemoteExecCommand{}, false
		case <-changed:
			timer.Stop()
		case <-timer.C:
			return RemoteExecCommand{}, false
		}
	}
}

func (s *AppState) RecordRemoteExecResult(result RemoteExecResult) (RemoteExecCommand, error) {
	commandID := strings.TrimSpace(result.CommandID)
	if commandID == "" {
		return RemoteExecCommand{}, ErrRemoteExecCommandNotFound
	}

	now := nowSeconds()
	var eventMessage string
	s.mu.Lock()
	s.expireRemoteExecLocked(time.Now())
	command := s.execCommands[commandID]
	if command == nil {
		s.mu.Unlock()
		return RemoteExecCommand{}, ErrRemoteExecCommandNotFound
	}

	if isRemoteExecTerminal(command.State) {
		command.LateResult = true
		command.CompletedAt = now
		s.execLastResultStatus = "late"
		copy := cloneRemoteExecCommand(command)
		s.mu.Unlock()
		s.events.Add("exec", "exec late result id="+commandID+" state="+string(copy.State))
		return copy, ErrRemoteExecTerminalState
	}

	command.OK = result.OK
	if result.OK {
		command.State = RemoteExecDone
		s.execCompletedCount++
		eventMessage = "exec done id=" + command.ID + " ok=true"
	} else {
		command.State = RemoteExecError
		s.execErrorCount++
		eventMessage = "exec error id=" + command.ID
	}
	command.CompletedAt = now
	command.DurationMS = result.DurationMS
	command.Prints = append([]RemoteExecPrint(nil), result.Prints...)
	command.Returns = append([]any(nil), result.Returns...)
	command.Error = strings.TrimSpace(result.Error)
	command.Traceback = strings.TrimSpace(result.Traceback)
	if strings.TrimSpace(result.ClientID) != "" {
		command.ClaimedBy = strings.TrimSpace(result.ClientID)
	}
	s.execLastResultStatus = string(command.State)
	copy := cloneRemoteExecCommand(command)
	s.mu.Unlock()

	s.events.Add("exec", eventMessage)
	return copy, nil
}

func (s *AppState) RemoteExecCommand(id string) (RemoteExecCommand, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireRemoteExecLocked(time.Now())
	command := s.execCommands[strings.TrimSpace(id)]
	if command == nil {
		return RemoteExecCommand{}, false
	}
	return cloneRemoteExecCommand(command), true
}

func (s *AppState) RemoteExecDebug() RemoteExecDebug {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireRemoteExecLocked(time.Now())

	debug := RemoteExecDebug{
		Enabled:          s.cfg.RemoteExecEnabled,
		CompletedCount:   s.execCompletedCount,
		ErrorCount:       s.execErrorCount,
		LastCommandAt:    s.execLastCommandAt,
		LastResultStatus: s.execLastResultStatus,
	}
	for _, command := range s.execCommands {
		switch command.State {
		case RemoteExecQueued:
			debug.QueueCount++
		case RemoteExecRunning:
			debug.RunningCount++
		}
	}
	return debug
}

func (s *AppState) AddRevisionListener(listener func(RevisionEvent)) {
	if listener == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, listener)
}

func (s *AppState) RevisionSummaries(limit int) []RevisionSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 || limit > len(s.changeLog) {
		limit = len(s.changeLog)
	}
	if limit > 1000 {
		limit = 1000
	}
	start := len(s.changeLog) - limit
	if start < 0 {
		start = 0
	}
	result := make([]RevisionSummary, 0, len(s.changeLog)-start)
	for i := len(s.changeLog) - 1; i >= start; i-- {
		result = append(result, revisionSummary(s.changeLog[i]))
	}
	return result
}

func (s *AppState) RevisionDetail(revision int) (RevisionSummary, []map[string]any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := len(s.changeLog) - 1; i >= 0; i-- {
		event := s.changeLog[i]
		if event.Rev == revision {
			changes := make([]map[string]any, 0, len(event.Changes))
			for _, change := range event.Changes {
				changes = append(changes, compactChange(change))
			}
			return revisionSummary(event), changes, true
		}
	}
	return RevisionSummary{Rev: revision}, []map[string]any{}, false
}

func (s *AppState) WaitForRevisionAfter(sinceRev int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		if s.revision > sinceRev {
			s.mu.Unlock()
			return
		}
		changed := s.changed
		s.mu.Unlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		timer := time.NewTimer(remaining)
		select {
		case <-changed:
			timer.Stop()
		case <-timer.C:
			return
		}
	}
}

func (s *AppState) SessionsActive() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

func (s *AppState) GitState() GitState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.git
}

func (s *AppState) SetGitState(git GitState) {
	if git.Status == "" {
		git.Status = initialGitStatus(git.Enabled)
	}
	s.mu.Lock()
	s.git = git
	s.mu.Unlock()
	if git.LastError != "" {
		s.events.Add("git", git.LastError)
	} else {
		s.events.Add("git", "git state updated")
	}
}

func (s *AppState) UpdateRevisionGitCommit(revision int, commit, short string) {
	s.mu.Lock()
	var persistErr error
	for i := range s.changeLog {
		if s.changeLog[i].Rev == revision {
			s.changeLog[i].GitCommit = commit
			s.changeLog[i].GitShort = short
			persistErr = s.saveHistoryLocked()
			break
		}
	}
	s.mu.Unlock()
	if persistErr != nil {
		s.events.Add("history", persistErr.Error())
	}
}

func (s *AppState) LoadHistory() error {
	path := s.historyPath()
	if path == "" {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var history persistentHistory
	if err := json.Unmarshal(body, &history); err != nil {
		s.events.Add("history", "ignored unreadable history")
		return nil
	}
	if history.Version != 1 {
		s.events.Add("history", "ignored unsupported history version")
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision = history.Revision
	s.changeLog = sanitizeHistoryEvents(history.ChangeLog, s.cfg.ChangeRetention)
	for _, event := range s.changeLog {
		if event.Rev > s.revision {
			s.revision = event.Rev
		}
	}
	return nil
}

func (s *AppState) Activity() Activity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activity
}

func (s *AppState) RecordActivity(activity Activity) Activity {
	activity.Text = strings.TrimSpace(activity.Text)
	activity.Operation = strings.TrimSpace(activity.Operation)
	activity.Phase = strings.TrimSpace(activity.Phase)
	activity.ClientID = strings.TrimSpace(activity.ClientID)
	if activity.Progress < 0 {
		activity.Progress = 0
	}
	if activity.Progress > 100 {
		activity.Progress = 100
	}
	if activity.Current < 0 {
		activity.Current = 0
	}
	if activity.Total < 0 {
		activity.Total = 0
	}
	if activity.Total > 0 && activity.Current > activity.Total {
		activity.Current = activity.Total
	}
	if activity.At == 0 {
		activity.At = nowSeconds()
	}

	s.mu.Lock()
	s.activity = activity
	s.mu.Unlock()
	s.events.Add("activity", activity.Text)
	return activity
}

func (s *AppState) RecordRequest() {
	s.mu.Lock()
	s.metrics.RequestCount++
	s.mu.Unlock()
}

func (s *AppState) RecordSnapshotRequest(changeCount int, encoding string, payloadBytes int) {
	s.mu.Lock()
	s.metrics.SnapshotRequests++
	s.metrics.LastChangeEncoding = encoding
	s.metrics.LastPayloadBytes = payloadBytes
	s.mu.Unlock()
	s.events.Add("snapshot", "/snapshot changes="+strconv.Itoa(changeCount)+" encoding="+encoding+" bytes="+strconv.Itoa(payloadBytes))
}

func (s *AppState) RecordChangesRequest(sinceRev, targetRev int, needsSnapshot bool, changeCount int, timedOut bool, waitedSec float64, encoding string, payloadBytes int) {
	s.mu.Lock()
	s.metrics.ChangesRequests++
	if timedOut {
		s.metrics.ChangesTimeouts++
	}
	s.metrics.LastChanges = map[string]any{
		"since_rev":      sinceRev,
		"target_rev":     targetRev,
		"server_rev":     s.revision,
		"needs_snapshot": needsSnapshot,
		"change_count":   changeCount,
		"encoding":       encoding,
		"payload_bytes":  payloadBytes,
		"timed_out":      timedOut,
		"waited_sec":     waitedSec,
		"at":             nowSeconds(),
	}
	s.metrics.LastChangeEncoding = encoding
	s.metrics.LastPayloadBytes = payloadBytes
	s.mu.Unlock()
	s.events.Add("changes", fmt.Sprintf("/changes since=%d target=%d changes=%d waited=%.2fs snapshot=%t", sinceRev, targetRev, changeCount, waitedSec, needsSnapshot))
}

func (s *AppState) RecordBootstrap(writtenCount, updatedCount, unchangedCount, deletedCount, errorCount int) {
	s.events.Add("bootstrap", fmt.Sprintf("/bootstrap written=%d updated=%d unchanged=%d deleted=%d errors=%d", writtenCount, updatedCount, unchangedCount, deletedCount, errorCount))
}

func (s *AppState) RecordWatcher(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	s.events.Add("watcher", message)
}

func (s *AppState) RecordPerformance(eventBatchSize int, debounce, parseDuration, publishDuration time.Duration, cacheHits, cacheMisses int) {
	s.mu.Lock()
	s.metrics.LastEventBatchSize = eventBatchSize
	s.metrics.LastDebounceMS = durationMillis(debounce)
	s.metrics.LastParseDurationMS = durationMillis(parseDuration)
	s.metrics.LastPublishDurationMS = durationMillis(publishDuration)
	s.metrics.LastCacheHitCount = cacheHits
	s.metrics.LastCacheMissCount = cacheMisses
	s.mu.Unlock()
}

func (s *AppState) RecordHandshake(clientID, sessionID string, lastAppliedRev any) {
	s.mu.Lock()
	s.sessions[sessionID] = Session{ClientID: clientID, CreatedAt: time.Now()}
	s.metrics.HandshakeCount++
	s.metrics.LastHandshake = HandshakeInfo{
		ClientID:       clientID,
		SessionID:      sessionID,
		LastAppliedRev: lastAppliedRev,
		At:             time.Now().Unix(),
	}
	s.mu.Unlock()
	s.events.Add("handshake", "handshake accepted client="+clientID+" session="+sessionID)
}

func (s *AppState) RecordAck(clientID, status string, appliedRev any, errors []string) {
	s.mu.Lock()
	s.metrics.AckCount++
	if status == "error" {
		s.metrics.AckErrorCount++
	}
	s.metrics.LastAck = AckInfo{
		ClientID:   clientID,
		Status:     status,
		AppliedRev: appliedRev,
		Errors:     errors,
		At:         time.Now().Unix(),
	}
	s.mu.Unlock()
	s.events.Add("ack", "ack received status="+status)
}

func (s *AppState) Metrics() Metrics {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metrics
}

func (s *AppState) saveHistoryLocked() error {
	path := s.historyPath()
	if path == "" {
		return nil
	}
	history := persistentHistory{
		Version:   1,
		Revision:  s.revision,
		ChangeLog: append([]RevisionEvent(nil), s.changeLog...),
	}
	body, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func (s *AppState) historyPath() string {
	if strings.TrimSpace(s.cfg.SyncRootAbs) == "" {
		return ""
	}
	return filepath.Join(s.cfg.SyncRootAbs, config.MetadataDir, "history.json")
}

func sanitizeHistoryEvents(events []RevisionEvent, retention int) []RevisionEvent {
	if retention < 1 {
		retention = 1
	}
	if len(events) > retention {
		events = events[len(events)-retention:]
	}
	result := make([]RevisionEvent, 0, len(events))
	for _, event := range events {
		if event.Rev <= 0 {
			continue
		}
		event.Changes = cloneChangeList(event.Changes)
		event.Compact = buildCompactPayload(event.Changes)
		result = append(result, event)
	}
	return result
}

func (s *AppState) Events(limit int) []debuglog.Event {
	events := s.events.Events()
	if limit <= 0 || limit >= len(events) {
		return events
	}
	return events[len(events)-limit:]
}

func (s *AppState) ScanWarnings(limit int) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 || limit >= len(s.warnings) {
		return append([]string(nil), s.warnings...)
	}
	return append([]string(nil), s.warnings[len(s.warnings)-limit:]...)
}

func (s *AppState) flattenChangesLocked(sinceRev int) ([]map[string]any, int, bool) {
	if len(s.changeLog) == 0 {
		return []map[string]any{}, s.revision, false
	}
	oldest := s.changeLog[0].Rev
	if sinceRev < oldest-1 {
		return []map[string]any{}, s.revision, true
	}

	collected := []map[string]any{}
	target := sinceRev
	for _, event := range s.changeLog {
		if event.Rev > sinceRev {
			collected = append(collected, cloneChangeList(event.Changes)...)
			target = event.Rev
		}
	}
	return collected, target, false
}

func (s *AppState) flattenCompactChangesLocked(sinceRev int) ([]map[string]any, int, bool) {
	if len(s.changeLog) == 0 {
		return []map[string]any{}, s.revision, false
	}
	oldest := s.changeLog[0].Rev
	if sinceRev < oldest-1 {
		return []map[string]any{}, s.revision, true
	}

	collected := []map[string]any{}
	target := sinceRev
	for _, event := range s.changeLog {
		if event.Rev > sinceRev {
			collected = append(collected, cloneChangeList(event.Compact)...)
			target = event.Rev
		}
	}
	return collected, target, false
}

func (s *AppState) replaceRecordsLocked(snapshotRecords map[string]records.SyncRecord, warnings []string, invalidJSONCount int) {
	s.records = cloneRecords(snapshotRecords)
	counts := IndexedCounts{}
	for _, record := range s.records {
		counts.Entry++
		switch record.Entity {
		case records.EntityScript:
			counts.Script++
		case records.EntityUIInstance:
			counts.UI++
		}
	}
	s.counts = counts
	s.snapshot = buildSnapshotPayload(s.records)
	s.compactSnapshot = buildCompactPayload(s.snapshot)
	s.warnings = append([]string(nil), warnings...)
	s.metrics.LastScanEntryCount = counts.Entry
	s.metrics.LastScanScriptCount = counts.Script
	s.metrics.LastScanUICount = counts.UI
	s.metrics.LastScanInvalidJSON = invalidJSONCount
	s.metrics.ScanCycles++
}

func (s *AppState) broadcastLocked() {
	close(s.changed)
	s.changed = make(chan struct{})
}

func cloneRecords(source map[string]records.SyncRecord) map[string]records.SyncRecord {
	result := make(map[string]records.SyncRecord, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func buildRevisionEvent(revision int, changes []map[string]any) RevisionEvent {
	opCounts := map[string]int{}
	entityCounts := map[string]int{}
	for _, change := range changes {
		if op, ok := change["op"].(string); ok {
			opCounts[op]++
		}
		if entity, ok := change["entity"].(string); ok {
			entityCounts[entity]++
		}
	}
	return RevisionEvent{
		Rev:          revision,
		TS:           nowSeconds(),
		ChangeCount:  len(changes),
		OpCounts:     opCounts,
		EntityCounts: entityCounts,
		Changes:      cloneChangeList(changes),
		Compact:      buildCompactPayload(changes),
	}
}

func nowSeconds() float64 {
	return float64(time.Now().UnixMilli()) / 1000
}

func durationMillis(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func buildSnapshotPayload(snapshotRecords map[string]records.SyncRecord) []map[string]any {
	keys := make([]string, 0, len(snapshotRecords))
	for key := range snapshotRecords {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	changes := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		changes = append(changes, snapshotRecords[key].ToUpsert())
	}
	return changes
}

func buildCompactPayload(changes []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(changes))
	for _, change := range changes {
		result = append(result, records.CompactChange(change))
	}
	return result
}

func revisionSummary(event RevisionEvent) RevisionSummary {
	return RevisionSummary{
		Rev:            event.Rev,
		TS:             event.TS,
		ChangeCount:    event.ChangeCount,
		OpCounts:       event.OpCounts,
		EntityCounts:   event.EntityCounts,
		GitCommit:      event.GitCommit,
		GitCommitShort: event.GitShort,
	}
}

func compactChange(change map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{
		"op",
		"entity",
		"local_path",
		"rbx_path",
		"old_local_path",
		"old_rbx_path",
		"new_rbx_path",
		"class_name",
		"stable_id",
		"content_hash",
	} {
		if value, ok := change[key]; ok {
			result[key] = value
		}
	}

	if payload, ok := change["payload"].(map[string]any); ok {
		result["payload_summary"] = map[string]any{
			"className":       firstNonEmptyString(payload["className"], payload["$className"]),
			"name":            firstNonEmptyString(payload["name"], payload["Name"]),
			"property_count":  mapLength(payload["properties"]),
			"attribute_count": mapLength(payload["attributes"]),
			"tag_count":       sliceLength(payload["tags"]),
		}
	} else if source, ok := change["source"].(string); ok {
		result["source_summary"] = map[string]any{
			"line_count": strings.Count(source, "\n") + 1,
			"char_count": len(source),
		}
	}
	return result
}

func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		if text, ok := value.(string); ok && text != "" {
			return text
		}
	}
	return ""
}

func mapLength(value any) int {
	if typed, ok := value.(map[string]any); ok {
		return len(typed)
	}
	return 0
}

func sliceLength(value any) int {
	if typed, ok := value.([]any); ok {
		return len(typed)
	}
	return 0
}

func cloneChangeList(source []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(source))
	for _, item := range source {
		result = append(result, cloneChange(item))
	}
	return result
}

func cloneChange(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func (s *AppState) claimQueuedRemoteExecLocked(clientID string) *RemoteExecCommand {
	for _, id := range s.execOrder {
		command := s.execCommands[id]
		if command == nil || command.State != RemoteExecQueued {
			continue
		}
		command.State = RemoteExecRunning
		command.ClaimedBy = clientID
		command.ClaimedAt = nowSeconds()
		return command
	}
	return nil
}

func (s *AppState) expireRemoteExecLocked(now time.Time) {
	nowSec := float64(now.UnixMilli()) / 1000
	for _, command := range s.execCommands {
		if command == nil {
			continue
		}
		timeout := time.Duration(command.TimeoutSec)*time.Second + RemoteExecRunningGrace
		if timeout <= RemoteExecRunningGrace {
			timeout = RemoteExecRunningGrace
		}

		switch command.State {
		case RemoteExecRunning:
			claimedAt := secondsToTime(command.ClaimedAt)
			if !claimedAt.IsZero() && now.Sub(claimedAt) >= timeout {
				command.State = RemoteExecExpired
				command.CompletedAt = nowSec
				s.execErrorCount++
				s.execLastResultStatus = string(RemoteExecExpired)
			}
		case RemoteExecQueued:
			createdAt := secondsToTime(command.CreatedAt)
			if !createdAt.IsZero() && now.Sub(createdAt) >= timeout {
				command.State = RemoteExecExpired
				command.CompletedAt = nowSec
				s.execErrorCount++
				s.execLastResultStatus = string(RemoteExecExpired)
			}
		}
	}
}

func (s *AppState) broadcastExecLocked() {
	close(s.execChanged)
	s.execChanged = make(chan struct{})
}

func isRemoteExecTerminal(state RemoteExecState) bool {
	switch state {
	case RemoteExecDone, RemoteExecError, RemoteExecTimeout, RemoteExecExpired, RemoteExecCancelled:
		return true
	default:
		return false
	}
}

func cloneRemoteExecCommand(command *RemoteExecCommand) RemoteExecCommand {
	if command == nil {
		return RemoteExecCommand{}
	}
	copy := *command
	copy.Prints = append([]RemoteExecPrint(nil), command.Prints...)
	copy.Returns = append([]any(nil), command.Returns...)
	return copy
}

func newRemoteExecID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return "cmd_" + hex.EncodeToString(bytes[:]), nil
}

func secondsToTime(seconds float64) time.Time {
	if seconds <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(int64(seconds * 1000))
}
