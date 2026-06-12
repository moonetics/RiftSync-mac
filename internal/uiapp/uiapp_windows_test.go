package uiapp

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"riftsync/internal/exechistory"
	"riftsync/internal/serverapp"
	"riftsync/internal/state"
)

func TestFormatHistory(t *testing.T) {
	text := formatHistory([]state.RevisionSummary{
		{
			Rev:            2,
			ChangeCount:    3,
			OpCounts:       map[string]int{"upsert": 2, "delete": 1},
			GitCommitShort: "abc1234",
		},
	})
	if !strings.Contains(text, "rev 2") || !strings.Contains(text, "3 changes") || !strings.Contains(text, "delete:1") || !strings.Contains(text, "upsert:2") {
		t.Fatalf("history text = %q", text)
	}
}

func TestFormatEventsAndWarnings(t *testing.T) {
	if formatWarnings(nil) != "No warnings." {
		t.Fatalf("formatWarnings(nil) = %q", formatWarnings(nil))
	}
	events := formatEvents(serverapp.Status{})
	if events != "No events yet." {
		t.Fatalf("formatEvents(empty) = %q", events)
	}
}

func TestPortTextAndFallback(t *testing.T) {
	if portText(-1) != "" || portText(8765) != "8765" {
		t.Fatalf("portText unexpected: %q %q", portText(-1), portText(8765))
	}
	if fallback("", "x") != "x" || fallback("ok", "x") != "ok" {
		t.Fatal("fallback returned unexpected values")
	}
}

func TestOptionsFromInput(t *testing.T) {
	gitEnabled := false
	options, err := optionsFromInput(serverapp.Options{}, "sync_config.json", "C:\\Project\\Game", "127.0.0.1", "8766", gitEnabled, true, true)
	if err != nil {
		t.Fatalf("optionsFromInput returned error: %v", err)
	}
	if options.ConfigPath != "sync_config.json" || options.SyncRootOverride != "C:\\Project\\Game" || options.HostOverride != "127.0.0.1" || options.PortOverride != 8766 || !options.Debug || options.LegacyScan {
		t.Fatalf("options = %#v", options)
	}
	if options.GitVersioningOverride == nil || !*options.GitVersioningOverride {
		t.Fatalf("GitVersioningOverride = %#v, want true pointer", options.GitVersioningOverride)
	}
}

func TestOptionsFromInputRejectsInvalidHost(t *testing.T) {
	if _, err := optionsFromInput(serverapp.Options{}, "sync_config.json", "src/game", "0.0.0.0", "8765", true, false, false); err == nil {
		t.Fatal("optionsFromInput returned nil error, want invalid host error")
	}
}

func TestMakeStatusPayloadStarting(t *testing.T) {
	payload := makeStatusPayload(serverapp.Status{Host: "127.0.0.1", Port: 8765}, true, "")
	if !payload.Starting || payload.Address != "127.0.0.1:8765" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestGitStatusText(t *testing.T) {
	cases := []struct {
		name   string
		status serverapp.Status
		want   string
	}{
		{name: "stopped default", status: serverapp.Status{}, want: "Pending, will initialize on Start"},
		{name: "stopped enabled", status: serverapp.Status{Git: state.GitState{Enabled: true}}, want: "Pending, will initialize on Start"},
		{name: "checking", status: serverapp.Status{Running: true, Git: state.GitState{Enabled: true, Status: state.GitStatusChecking}}, want: "Checking Git"},
		{name: "initializing", status: serverapp.Status{Running: true, Git: state.GitState{Enabled: true, Status: state.GitStatusInitializing}}, want: "Initializing repo"},
		{name: "initial commit", status: serverapp.Status{Running: true, Git: state.GitState{Enabled: true, Status: state.GitStatusInitialCommit}}, want: "Creating initial commit"},
		{name: "committing", status: serverapp.Status{Running: true, Git: state.GitState{Enabled: true, Status: state.GitStatusCommitting}}, want: "Committing"},
		{name: "ready", status: serverapp.Status{Running: true, Git: state.GitState{Enabled: true, Status: state.GitStatusReady}}, want: "Ready"},
		{name: "error", status: serverapp.Status{Running: true, Git: state.GitState{Enabled: true, Status: state.GitStatusError}}, want: "Error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gitStatusText(tc.status); got != tc.want {
				t.Fatalf("gitStatusText = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHTMLDocumentHasAppControls(t *testing.T) {
	document := htmlDocument()
	for _, needle := range []string{
		`color-scheme: dark`,
		`startBtn`,
		`stopBtn`,
		`overviewTab`,
		`historyTab`,
		`execTab`,
		`configTab`,
		`overviewPanel`,
		`historyPanel`,
		`execPanel`,
		`configPanel`,
		`metricStrip`,
		`workspacePanel`,
		`healthPanel`,
		`configPathPanel`,
		`serverSettingsPanel`,
		`appHeader`,
		`brand-title`,
		`brand-icon`,
		`aria-hidden="true"`,
		`minimizeBtn`,
		`closeBtn`,
		`overflow: hidden`,
		`changePathBtn`,
		`configApplyStatus`,
		`/app/config/apply`,
		`configInput`,
		`syncRootInput`,
		`historyRefreshBtn`,
		`historyTimeline`,
		`history-layout`,
		`history-toolbar-panel`,
		`logsBtn`,
		`logsModal`,
		`/app/logs`,
		`activityText`,
		`activityFill`,
		`errorDetailsBtn`,
		`errorDetailText`,
		`copyErrorBtn`,
		`activity_progress`,
		`aria-live="polite"`,
		`aria-busy="false"`,
		`role="progressbar"`,
		`setConfigLocked`,
		`Stop sync before editing config.`,
		`syncRootPickerBtn`,
		`toggle-switch`,
		`historyDetail`,
		`execSourceInput`,
		`execTimeoutInput`,
		`execRunBtn`,
		`execLatestBtn`,
		`execOutputText`,
		`execHistoryList`,
		`/app/exec/run`,
		`/app/exec/rerun`,
		`/app/exec/history`,
	} {
		if !strings.Contains(document, needle) {
			t.Fatalf("htmlDocument missing %q", needle)
		}
	}
	if strings.Contains(document, `historyList`) {
		t.Fatal("htmlDocument still contains historyList button-list UI")
	}
	if strings.Contains(document, `historyRevisionSelect`) {
		t.Fatal("htmlDocument still contains native history select UI")
	}
	if strings.Contains(document, `historyLimitInput`) || strings.Contains(document, `limit-option`) || strings.Contains(document, `Limit`) {
		t.Fatal("htmlDocument still contains history limit UI")
	}
	if strings.Contains(document, `historyRevisionButton`) || strings.Contains(document, `historyRevisionMenu`) {
		t.Fatal("htmlDocument still contains dropdown-centric history UI")
	}
	if strings.Contains(document, `historyJumpInput`) || strings.Contains(document, `historyJumpBtn`) || strings.Contains(document, `Open Rev`) {
		t.Fatal("htmlDocument still contains jump/open revision UI")
	}
	if strings.Contains(document, `Fsnotify`) || strings.Contains(document, `Protocol`) || strings.Contains(document, `Legacy scan`) {
		t.Fatal("htmlDocument still contains technical overview/config labels")
	}
	if strings.Contains(document, `gitInput`) || strings.Contains(document, `Git versioning`) {
		t.Fatal("htmlDocument still contains Git versioning toggle UI")
	}
	if strings.Contains(document, `quitBtn`) {
		t.Fatal("htmlDocument still contains quit button")
	}
	if strings.Contains(document, `Save & Restart`) || strings.Contains(document, `saveBtn`) {
		t.Fatal("htmlDocument still contains manual save/restart UI")
	}
}

func TestAppRoutesServeStatusAndHTML(t *testing.T) {
	controller := &appController{
		runner:  serverapp.New(serverapp.Options{ConfigPath: "sync_config.json", PortOverride: -1}),
		options: serverapp.Options{ConfigPath: "sync_config.json", PortOverride: -1},
		quit:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	htmlResponse := httptest.NewRecorder()
	mux.ServeHTTP(htmlResponse, httptest.NewRequest(http.MethodGet, "/app", nil))
	if htmlResponse.Code != http.StatusOK || !strings.Contains(htmlResponse.Body.String(), "RiftSync") {
		t.Fatalf("/app code=%d body=%q", htmlResponse.Code, htmlResponse.Body.String())
	}

	statusResponse := httptest.NewRecorder()
	mux.ServeHTTP(statusResponse, httptest.NewRequest(http.MethodGet, "/app/status", nil))
	if statusResponse.Code != http.StatusOK || !strings.Contains(statusResponse.Body.String(), `"status":"ok"`) {
		t.Fatalf("/app/status code=%d body=%q", statusResponse.Code, statusResponse.Body.String())
	}

	historyResponse := httptest.NewRecorder()
	mux.ServeHTTP(historyResponse, httptest.NewRequest(http.MethodGet, "/app/history?limit=50", nil))
	if historyResponse.Code != http.StatusOK || !strings.Contains(historyResponse.Body.String(), `"revisions":[]`) {
		t.Fatalf("/app/history code=%d body=%q", historyResponse.Code, historyResponse.Body.String())
	}

	detailResponse := httptest.NewRecorder()
	mux.ServeHTTP(detailResponse, httptest.NewRequest(http.MethodGet, "/app/history?rev=1", nil))
	if detailResponse.Code != http.StatusOK || !strings.Contains(detailResponse.Body.String(), `"found":false`) {
		t.Fatalf("/app/history detail code=%d body=%q", detailResponse.Code, detailResponse.Body.String())
	}

	logsResponse := httptest.NewRecorder()
	mux.ServeHTTP(logsResponse, httptest.NewRequest(http.MethodGet, "/app/logs", nil))
	if logsResponse.Code != http.StatusOK || !strings.Contains(logsResponse.Body.String(), `"events"`) || !strings.Contains(logsResponse.Body.String(), `"metrics"`) {
		t.Fatalf("/app/logs code=%d body=%q", logsResponse.Code, logsResponse.Body.String())
	}

	execHistoryResponse := httptest.NewRecorder()
	mux.ServeHTTP(execHistoryResponse, httptest.NewRequest(http.MethodGet, "/app/exec/history", nil))
	if execHistoryResponse.Code != http.StatusOK || !strings.Contains(execHistoryResponse.Body.String(), `"entries"`) {
		t.Fatalf("/app/exec/history code=%d body=%q", execHistoryResponse.Code, execHistoryResponse.Body.String())
	}
}

func TestAppExecHistoryRouteHandlesCorruptFile(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sync_config.json")
	syncRoot := filepath.Join(root, "game")
	configBody, err := json.Marshal(map[string]any{
		"host":      "127.0.0.1",
		"port":      8765,
		"sync_root": syncRoot,
	})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(exechistory.Path(syncRoot)), 0o755); err != nil {
		t.Fatalf("mkdir history: %v", err)
	}
	if err := os.WriteFile(exechistory.Path(syncRoot), []byte("{bad"), 0o644); err != nil {
		t.Fatalf("write corrupt history: %v", err)
	}
	controller := &appController{
		runner:  serverapp.New(serverapp.Options{ConfigPath: configPath, PortOverride: -1}),
		options: serverapp.Options{ConfigPath: configPath, PortOverride: -1},
		quit:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/app/exec/history", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "ignored corrupt exec history") || !strings.Contains(response.Body.String(), `"entries":[]`) {
		t.Fatalf("/app/exec/history code=%d body=%q", response.Code, response.Body.String())
	}
}

func TestAppExecRunStoppedIsRejected(t *testing.T) {
	controller := &appController{
		runner:  serverapp.New(serverapp.Options{ConfigPath: "sync_config.json", PortOverride: -1}),
		options: serverapp.Options{ConfigPath: "sync_config.json", PortOverride: -1},
		quit:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/app/exec/run", bytes.NewBufferString(`{"source":"print(1)","timeout_sec":1}`)))
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "Start RiftSync first") {
		t.Fatalf("/app/exec/run code=%d body=%q", response.Code, response.Body.String())
	}
}

func TestAppExecRunAndRerunWithRunningServer(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sync_config.json")
	syncRoot := filepath.Join(root, "game")
	port := freePort(t)
	configBody, err := json.Marshal(map[string]any{
		"host":                "127.0.0.1",
		"port":                port,
		"sync_root":           syncRoot,
		"remote_exec_enabled": true,
		"remote_exec_token":   "secret",
	})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	runner := serverapp.New(serverapp.Options{ConfigPath: configPath, PortOverride: -1})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := runner.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = runner.Stop(stopCtx)
	})
	workerDone := startExecWorker(t, port, "secret", 2)

	controller := &appController{
		runner:  runner,
		options: serverapp.Options{ConfigPath: configPath, PortOverride: -1},
		ctx:     ctx,
		quit:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	runResponse := httptest.NewRecorder()
	mux.ServeHTTP(runResponse, httptest.NewRequest(http.MethodPost, "/app/exec/run", bytes.NewBufferString(`{"source":"print('app')","timeout_sec":3}`)))
	if runResponse.Code != http.StatusOK || !strings.Contains(runResponse.Body.String(), `[print] hello`) {
		t.Fatalf("/app/exec/run code=%d body=%q", runResponse.Code, runResponse.Body.String())
	}
	history, _, err := exechistory.Load(syncRoot)
	if err != nil {
		t.Fatalf("load history: %v", err)
	}
	if len(history.Entries) != 1 || history.Entries[0].SubmittedBy != "app" || history.Entries[0].SourceKind != "app" {
		t.Fatalf("history after run = %#v", history.Entries)
	}

	rerunResponse := httptest.NewRecorder()
	mux.ServeHTTP(rerunResponse, httptest.NewRequest(http.MethodPost, "/app/exec/rerun", bytes.NewBufferString(`{}`)))
	if rerunResponse.Code != http.StatusOK || !strings.Contains(rerunResponse.Body.String(), `[print] hello`) {
		t.Fatalf("/app/exec/rerun code=%d body=%q", rerunResponse.Code, rerunResponse.Body.String())
	}
	history, _, err = exechistory.Load(syncRoot)
	if err != nil {
		t.Fatalf("load history after rerun: %v", err)
	}
	if len(history.Entries) != 2 || history.Entries[0].RerunOfID != history.Entries[1].ID {
		t.Fatalf("history after rerun = %#v", history.Entries)
	}

	select {
	case <-workerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("exec worker did not finish")
	}
}

func TestConfigApplyStoppedSavesWithoutStarting(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sync_config.json")
	syncRoot := filepath.Join(root, "game")
	controller := &appController{
		runner:  serverapp.New(serverapp.Options{ConfigPath: configPath, PortOverride: -1}),
		options: serverapp.Options{ConfigPath: configPath, PortOverride: -1},
		quit:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	body, err := json.Marshal(map[string]any{
		"config_path": configPath,
		"sync_root":   syncRoot,
		"host":        "127.0.0.1",
		"port":        "8765",
		"git_enabled": false,
		"debug":       true,
		"legacy_scan": true,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/app/config/apply", bytes.NewReader(body)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"result":"saved_only"`) {
		t.Fatalf("/app/config/apply code=%d body=%q", response.Code, response.Body.String())
	}
	if controller.runner.Status(1).Running {
		t.Fatal("config apply started stopped runner")
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config was not saved: %v", err)
	}
	cfg, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(cfg), `"git_versioning_enabled": true`) {
		t.Fatalf("saved config did not force git on: %s", string(cfg))
	}
}

func TestConfigApplyStoppedStatusUsesNewPathAfterPreviousRun(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sync_config.json")
	oldSyncRoot := filepath.Join(root, "old-game")
	newSyncRoot := filepath.Join(root, "new-game")
	port := freePort(t)
	configBody, err := json.Marshal(map[string]any{
		"host":      "127.0.0.1",
		"port":      port,
		"sync_root": oldSyncRoot,
	})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runner := serverapp.New(serverapp.Options{ConfigPath: configPath, PortOverride: -1})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := runner.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := runner.Stop(stopCtx); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	controller := &appController{
		runner:  runner,
		options: serverapp.Options{ConfigPath: configPath, PortOverride: -1},
		ctx:     ctx,
		quit:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	body, err := json.Marshal(map[string]any{
		"config_path": configPath,
		"sync_root":   newSyncRoot,
		"host":        "127.0.0.1",
		"port":        strconv.Itoa(port),
		"git_enabled": true,
		"debug":       false,
		"legacy_scan": false,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/app/config/apply", bytes.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("/app/config/apply code=%d body=%q", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"sync_root":"`+strings.ReplaceAll(newSyncRoot, `\`, `\\`)+`"`) {
		t.Fatalf("response kept stale sync root: %s", response.Body.String())
	}
}

func TestConfigApplyRunningIsRejected(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sync_config.json")
	syncRoot := filepath.Join(root, "game")
	port := freePort(t)
	configBody, err := json.Marshal(map[string]any{
		"host":                   "127.0.0.1",
		"port":                   port,
		"sync_root":              syncRoot,
		"git_versioning_enabled": false,
	})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runner := serverapp.New(serverapp.Options{ConfigPath: configPath, PortOverride: -1})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := runner.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = runner.Stop(stopCtx)
	})

	controller := &appController{
		runner:  runner,
		options: serverapp.Options{ConfigPath: configPath, PortOverride: -1},
		ctx:     ctx,
		quit:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	body, err := json.Marshal(map[string]any{
		"config_path": configPath,
		"sync_root":   filepath.Join(root, "other"),
		"host":        "127.0.0.1",
		"port":        strconv.Itoa(port),
		"git_enabled": true,
		"debug":       true,
		"legacy_scan": true,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/app/config/apply", bytes.NewReader(body)))
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "stop sync before changing config") {
		t.Fatalf("/app/config/apply code=%d body=%q, want conflict", response.Code, response.Body.String())
	}
}

func TestQuitRoutesSignalClose(t *testing.T) {
	for _, route := range []string{"/app/window/close", "/app/quit"} {
		controller := &appController{
			runner:  serverapp.New(serverapp.Options{ConfigPath: "sync_config.json", PortOverride: -1}),
			options: serverapp.Options{ConfigPath: "sync_config.json", PortOverride: -1},
			quit:    make(chan struct{}),
		}
		mux := http.NewServeMux()
		controller.registerRoutes(mux)

		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, route, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s code=%d body=%q", route, response.Code, response.Body.String())
		}
		select {
		case <-controller.quit:
		default:
			t.Fatalf("%s did not signal quit", route)
		}
	}
}

func TestStatusText(t *testing.T) {
	if statusText(statusPayload{}) != "Stopped" {
		t.Fatalf("stopped text = %q", statusText(statusPayload{}))
	}
	if statusText(statusPayload{Starting: true}) != "Starting" {
		t.Fatalf("starting text = %q", statusText(statusPayload{Starting: true}))
	}
	if statusText(statusPayload{Running: true}) != "Running" {
		t.Fatalf("running text = %q", statusText(statusPayload{Running: true}))
	}
}

func TestNoWalkDependency(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if strings.Contains(string(body), "github.com/lxn/walk") {
		t.Fatal("go.mod still contains walk dependency")
	}
}

func startExecWorker(t *testing.T, port int, token string, count int) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		client := http.Client{Timeout: 10 * time.Second}
		for index := 0; index < count; index++ {
			req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/exec/commands/next?client_id=test-worker&timeout=5", nil)
			if err != nil {
				t.Errorf("create next request: %v", err)
				return
			}
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("claim command: %v", err)
				return
			}
			var payload map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				_ = resp.Body.Close()
				t.Errorf("decode next: %v", err)
				return
			}
			_ = resp.Body.Close()
			command, ok := payload["command"].(map[string]any)
			if !ok || command == nil {
				t.Errorf("missing command payload: %#v", payload)
				return
			}
			commandID, _ := command["id"].(string)
			resultBody, err := json.Marshal(map[string]any{
				"command_id":  commandID,
				"client_id":   "test-worker",
				"ok":          true,
				"duration_ms": 6,
				"prints":      []map[string]any{{"level": "print", "text": "hello", "at_ms": 1}},
				"returns":     []any{"ok"},
			})
			if err != nil {
				t.Errorf("marshal result: %v", err)
				return
			}
			resultReq, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+strconv.Itoa(port)+"/exec/commands/result", bytes.NewReader(resultBody))
			if err != nil {
				t.Errorf("create result request: %v", err)
				return
			}
			resultReq.Header.Set("Authorization", "Bearer "+token)
			resultReq.Header.Set("Content-Type", "application/json")
			resultResp, err := client.Do(resultReq)
			if err != nil {
				t.Errorf("post result: %v", err)
				return
			}
			if resultResp.StatusCode != http.StatusOK {
				t.Errorf("post result status = %d", resultResp.StatusCode)
				_ = resultResp.Body.Close()
				return
			}
			_ = resultResp.Body.Close()
		}
	}()
	return done
}

func TestPluginUISimplifiedActions(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "plugin", "RiftSyncPlugin.lua"))
	if err != nil {
		t.Fatalf("read plugin: %v", err)
	}
	document := string(body)
	for _, needle := range []string{
		`snapshotButton.Text = "Resync"`,
		`historyPanel.Visible = false`,
		`metaFrame.Visible = false`,
		`debugFrame.Visible = false`,
		`connectionLayout.CellSize = UDim2.new(0.5, -4, 1, 0)`,
	} {
		if !strings.Contains(document, needle) {
			t.Fatalf("plugin UI missing %q", needle)
		}
	}
	if strings.Contains(document, `snapshotButton.Text = "Snapshot"`) {
		t.Fatal("plugin still labels snapshot button as Snapshot")
	}
}

func TestPluginPullStudioPreviewAndHealthChecklist(t *testing.T) {
	pluginBody, err := os.ReadFile(filepath.Join("..", "..", "plugin", "RiftSyncPlugin.lua"))
	if err != nil {
		t.Fatalf("read plugin: %v", err)
	}
	apiBody, err := os.ReadFile(filepath.Join("..", "..", "plugin", "API.lua"))
	if err != nil {
		t.Fatalf("read API: %v", err)
	}
	typeListBody, err := os.ReadFile(filepath.Join("..", "..", "plugin", "TypeList.lua"))
	if err != nil {
		t.Fatalf("read TypeList: %v", err)
	}

	pluginDocument := string(pluginBody)
	apiDocument := string(apiBody)
	typeListDocument := string(typeListBody)
	for _, needle := range []string{
		`BootstrapPreview = "/bootstrap/preview"`,
		`TypeList.VERSION = "3.7.0"`,
	} {
		if !strings.Contains(typeListDocument, needle) {
			t.Fatalf("TypeList missing %q", needle)
		}
	}
	for _, needle := range []string{
		`previewPullStudioToLocal`,
		`confirmPullStudioToLocal`,
		`cancelPullStudioToLocal`,
		`TypeList.ENDPOINTS.BootstrapPreview`,
		`pendingPullPreview`,
		`self:pushStudioSnapshot(`,
	} {
		if !strings.Contains(apiDocument, needle) {
			t.Fatalf("API missing %q", needle)
		}
	}
	for _, needle := range []string{
		`pullPreviewFrame`,
		`Pull Studio preview`,
		`confirmPullButton`,
		`cancelPullButton`,
		`client:previewPullStudioToLocal()`,
		`client:confirmPullStudioToLocal()`,
		`client:cancelPullStudioToLocal()`,
		`HTTP/server reachable`,
		`connected to server`,
		`token valid`,
		`sync active`,
		`exec active`,
		`edit mode`,
	} {
		if !strings.Contains(pluginDocument, needle) {
			t.Fatalf("plugin UI missing %q", needle)
		}
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
