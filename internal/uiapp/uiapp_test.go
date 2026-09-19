package uiapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/exechistory"
	"riftsync/internal/instances"
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

func TestDefaultWindowSize(t *testing.T) {
	if defaultWindowWidth != 1280 || defaultWindowHeight != 720 {
		t.Fatalf("default window size = %dx%d, want 1280x720", defaultWindowWidth, defaultWindowHeight)
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
	payload := makeStatusPayload(serverapp.Status{
		Host: "127.0.0.1",
		Port: 8765,
		Activity: state.Activity{
			Phase:         "properties",
			Progress:      88,
			Current:       44,
			Total:         50,
			Indeterminate: true,
		},
	}, true, "")
	if !payload.Starting || payload.Syncing || payload.Status != "Starting" || payload.Address != "127.0.0.1:8765" {
		t.Fatalf("payload = %#v", payload)
	}
	if payload.LastError != "" {
		t.Fatalf("last error = %q, want empty healthy state", payload.LastError)
	}
	if payload.ActivityPhase != "properties" ||
		payload.ActivityProgress != 88 ||
		payload.ActivityCurrent != 44 ||
		payload.ActivityTotal != 50 ||
		!payload.ActivityIndeterminate {
		t.Fatalf("activity payload = %#v", payload)
	}
}

func TestMakeStatusPayloadSyncing(t *testing.T) {
	payload := makeStatusPayload(serverapp.Status{
		Running: true,
		Activity: state.Activity{
			Phase:    "writing",
			Progress: 76,
			Current:  5500,
			Total:    16595,
		},
	}, false, "")
	if !payload.Syncing || payload.Status != "Syncing" {
		t.Fatalf("payload = %#v, want syncing status", payload)
	}
}

func TestMakeStatusPayloadKeepsGitErrorSeparate(t *testing.T) {
	payload := makeStatusPayload(serverapp.Status{
		Running: true,
		Git: state.GitState{
			Enabled:   true,
			Status:    state.GitStatusError,
			LastError: "git add -A: fatal: index.lock exists",
		},
	}, false, "")
	if payload.LastError != "" {
		t.Fatalf("LastError = %q, want empty for git-only error", payload.LastError)
	}
	if payload.GitStatus != "Error" || payload.GitLastError == "" {
		t.Fatalf("payload git fields = status %q error %q", payload.GitStatus, payload.GitLastError)
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
		`color-scheme:dark`,
		`class="brand-logo"`,
		`class="brand-version">v` + serverapp.Version + `</span>`,
		`data:image/png;base64,`,
		`ROBLOX STUDIO LIVE SYNC`,
		`--space-1:4px`,
		`--radius-md:12px`,
		`--blur:14px`,
		`backdrop-filter:blur(var(--blur))`,
		`@supports`,
		`@media(prefers-reduced-motion:reduce)`,
		`sidebar-pinned`,
		`force-collapsed`,
		`width:var(--rail);transition:none`,
		`addEventListener("mouseleave"`,
		`projectList`,
		`startAllBtn`,
		`addProjectBtn`,
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
		`minimizeBtn`,
		`closeBtn`,
		`overflow:hidden`,
		`scrollbar-width:none`,
		`-ms-overflow-style:none`,
		`*::-webkit-scrollbar{display:none;width:0;height:0}`,
		`changePathBtn`,
		`configApplyStatus`,
		`/app/config/apply`,
		`configInput`,
		`syncRootInput`,
		`configDirty=false`,
		`configApplying=false`,
		`!configDirty&&!configApplying`,
		`markConfigDirty`,
		`Unsaved changes`,
		`git_last_error`,
		`$("gitText").title=app.git_last_error||""`,
		`historyRefreshBtn`,
		`historyTimeline`,
		`history-shell`,
		`logsBtn`,
		`logsModal`,
		`removeModal`,
		`role="alertdialog"`,
		`removeKeepBtn`,
		`removeDeleteBtn`,
		`pendingRemoveProjectId`,
		`function requestRemoveProject(`,
		`function confirmRemoveProject(`,
		`Choose whether RiftSync should delete the project folder`,
		`delete_files`,
		`/app/logs`,
		`activityText`,
		`activityProgress`,
		`activity_indeterminate`,
		`showStartPending`,
		`phaseLabel`,
		`Scanning files`,
		`role="progressbar"`,
		`activity-slide`,
		`errorText`,
		`✓ No issues`,
		`activity-notice success`,
		`.lamp.syncing`,
		`app&&app.syncing?"syncing"`,
		`.toast.success`,
		`.toast.error`,
		`.toast.warning`,
		`.toast.info`,
		`aria-atomic="true"`,
		`ok?"success":"error"`,
		`syncRootPickerBtn`,
		`historyDetail`,
		`execSourceInput`,
		`execTimeoutInput`,
		`execRunBtn`,
		`execLatestBtn`,
		`execTokenDisplay`,
		`execTokenCopyBtn`,
		`readonly`,
		`execOutputText`,
		`execHistoryList`,
		`exec-history-item`,
		`grid-template-columns:minmax(0,1fr) auto auto`,
		`grid-template-columns:46px minmax(0,1fr)`,
		`grid-template-columns:40px 0;justify-content:center`,
		`.sidebar-wrap.force-collapsed .project-button`,
		`class="side-icon"`,
		`min-height:46px`,
		`justify-self:start`,
		`.config-side .panel{width:100%`,
		`align-items:stretch`,
		`overflow-x:hidden`,
		`text-overflow:ellipsis`,
		`overflow-wrap:anywhere`,
		`data-view`,
		`viewModal`,
		`View`,
		`/app/exec/run`,
		`/app/exec/rerun`,
		`/app/exec/history`,
		`/app/instances`,
		`/app/start-all`,
		`role="tablist"`,
		`role="tab"`,
		`role="tabpanel"`,
		`aria-selected="true"`,
		`aria-current`,
		`aria-expanded="false"`,
		`aria-live="polite"`,
		`role="dialog"`,
		`aria-modal="true"`,
		`diagnosticsToggle`,
		`advancedToggle`,
		`modalReturnFocus`,
		`inert=true`,
		`e.key==="Escape"`,
	} {
		if !strings.Contains(document, needle) {
			t.Fatalf("htmlDocument missing %q", needle)
		}
	}
	if strings.Contains(document, `background:linear-gradient`) || strings.Contains(document, `box-shadow:inset`) {
		t.Fatal("htmlDocument still contains beveled control styling")
	}
	if strings.Contains(document, `confirm("Remove this project`) {
		t.Fatal("htmlDocument still relies on the WebView JavaScript confirm dialog")
	}
	if strings.Contains(document, `Clear`) {
		t.Fatal("htmlDocument still exposes the internal Clear sentinel")
	}
	if strings.Contains(document, `MULTI-INSTANCE`) {
		t.Fatal("htmlDocument still contains the implementation-focused brand subtitle")
	}
	if strings.Contains(document, `border-radius:999px`) {
		t.Fatal("htmlDocument still contains oversized capsule status styling")
	}
	if strings.Contains(document, `<span>▶</span>`) || strings.Contains(document, `<span>＋</span>`) {
		t.Fatal("htmlDocument still contains uncentered Unicode sidebar action icons")
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

func TestMultiAppControllerCreatesSelectsAndRemovesProjectsWithoutDeletingFiles(t *testing.T) {
	root := t.TempDir()
	firstRoot := filepath.Join(root, "first")
	firstConfig := filepath.Join(root, "first-config.json")
	first := config.Default()
	first.SyncRoot = firstRoot
	first.Port = freePort(t)
	if err := config.Save(firstConfig, first); err != nil {
		t.Fatalf("save first config: %v", err)
	}

	store := instances.NewStore(filepath.Join(root, "appdata", "instances.json"))
	controller, err := newMultiAppController(context.Background(), store, serverapp.Options{
		ConfigPath:   firstConfig,
		PortOverride: -1,
	})
	if err != nil {
		t.Fatalf("newMultiAppController: %v", err)
	}
	controller.forceExit = false
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	secondRoot := filepath.Join(root, "second")
	createBody, _ := json.Marshal(map[string]any{"name": "Second", "sync_root": secondRoot})
	createResponse := httptest.NewRecorder()
	mux.ServeHTTP(createResponse, httptest.NewRequest(http.MethodPost, "/app/instances", bytes.NewReader(createBody)))
	if createResponse.Code != http.StatusOK {
		t.Fatalf("create code=%d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var created struct {
		Instances          []map[string]any `json:"instances"`
		SelectedInstanceID string           `json:"selected_instance_id"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if len(created.Instances) != 2 || created.SelectedInstanceID == "" {
		t.Fatalf("create response = %#v", created)
	}
	secondConfig := filepath.Join(secondRoot, config.MetadataDir, "sync_config.json")
	if _, err := os.Stat(secondConfig); err != nil {
		t.Fatalf("new project config was not created: %v", err)
	}

	duplicateBody, _ := json.Marshal(map[string]any{"name": "Duplicate", "sync_root": secondRoot})
	duplicateResponse := httptest.NewRecorder()
	mux.ServeHTTP(duplicateResponse, httptest.NewRequest(http.MethodPost, "/app/instances", bytes.NewReader(duplicateBody)))
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("duplicate code=%d body=%s", duplicateResponse.Code, duplicateResponse.Body.String())
	}

	marker := filepath.Join(secondRoot, "keep-me.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	removeBody, _ := json.Marshal(map[string]any{"id": created.SelectedInstanceID})
	removeResponse := httptest.NewRecorder()
	mux.ServeHTTP(removeResponse, httptest.NewRequest(http.MethodPost, "/app/instances/remove", bytes.NewReader(removeBody)))
	if removeResponse.Code != http.StatusOK {
		t.Fatalf("remove code=%d body=%s", removeResponse.Code, removeResponse.Body.String())
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "keep" {
		t.Fatalf("project marker was removed or changed: body=%q err=%v", body, err)
	}
	registry, _, err := store.Load()
	if err != nil || len(registry.Instances) != 1 {
		t.Fatalf("registry after remove=%#v err=%v", registry, err)
	}

	deleteRoot := filepath.Join(root, "delete-me")
	deleteCreateBody, _ := json.Marshal(map[string]any{"name": "Delete me", "sync_root": deleteRoot})
	deleteCreateResponse := httptest.NewRecorder()
	mux.ServeHTTP(deleteCreateResponse, httptest.NewRequest(http.MethodPost, "/app/instances", bytes.NewReader(deleteCreateBody)))
	if deleteCreateResponse.Code != http.StatusOK {
		t.Fatalf("delete-project create code=%d body=%s", deleteCreateResponse.Code, deleteCreateResponse.Body.String())
	}
	var deleteCreated struct {
		SelectedInstanceID string `json:"selected_instance_id"`
	}
	if err := json.Unmarshal(deleteCreateResponse.Body.Bytes(), &deleteCreated); err != nil {
		t.Fatalf("decode delete-project response: %v", err)
	}
	markerToDelete := filepath.Join(deleteRoot, "remove-me.txt")
	if err := os.WriteFile(markerToDelete, []byte("remove"), 0o644); err != nil {
		t.Fatalf("write delete marker: %v", err)
	}
	deleteBody, _ := json.Marshal(map[string]any{"id": deleteCreated.SelectedInstanceID, "delete_files": true})
	deleteResponse := httptest.NewRecorder()
	mux.ServeHTTP(deleteResponse, httptest.NewRequest(http.MethodPost, "/app/instances/remove", bytes.NewReader(deleteBody)))
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete-project remove code=%d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	var deleteResult struct {
		FilesDeleted bool `json:"files_deleted"`
	}
	if err := json.Unmarshal(deleteResponse.Body.Bytes(), &deleteResult); err != nil || !deleteResult.FilesDeleted {
		t.Fatalf("delete-project response=%s err=%v", deleteResponse.Body.String(), err)
	}
	if _, err := os.Stat(deleteRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("project root still exists after delete: err=%v", err)
	}
}

func TestMultiAppControllerDoesNotReimportSeedAfterValidRegistryWasEmptied(t *testing.T) {
	root := t.TempDir()
	store := instances.NewStore(filepath.Join(root, "appdata", "instances.json"))
	if err := store.Save(instances.Registry{Version: instances.RegistryVersion, Instances: []instances.Entry{}}); err != nil {
		t.Fatal(err)
	}
	controller, err := newMultiAppController(context.Background(), store, serverapp.Options{
		ConfigPath:   filepath.Join(root, "sync_config.json"),
		PortOverride: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(controller.registry.Instances) != 0 || len(controller.apps) != 0 {
		t.Fatalf("valid empty registry was unexpectedly reseeded: %#v", controller.registry)
	}
}

func TestMultiAppControllerStartsEmptyWhenDefaultConfigIsMissing(t *testing.T) {
	root := t.TempDir()
	store := instances.NewStore(filepath.Join(root, "appdata", "instances.json"))
	controller, err := newMultiAppController(context.Background(), store, serverapp.Options{
		ConfigPath:   filepath.Join(root, "missing-sync-config.json"),
		PortOverride: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(controller.registry.Instances) != 0 || len(controller.apps) != 0 {
		t.Fatalf("missing default config created a phantom project: %#v", controller.registry)
	}
}

func TestMultiAppStartAllRunsTwoProjectsConcurrently(t *testing.T) {
	root := t.TempDir()
	portA := freePort(t)
	portB := freePort(t)
	for portB == portA {
		portB = freePort(t)
	}
	entries := make([]instances.Entry, 0, 2)
	for index, port := range []int{portA, portB} {
		name := fmt.Sprintf("Project %d", index+1)
		configPath := filepath.Join(root, fmt.Sprintf("project-%d.json", index+1))
		cfg := config.Default()
		cfg.SyncRoot = filepath.Join(root, fmt.Sprintf("game-%d", index+1))
		if err := os.MkdirAll(cfg.SyncRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		cfg.Port = port
		cfg.GitVersioningEnabled = false
		if err := config.Save(configPath, cfg); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, instances.Entry{
			ID:         fmt.Sprintf("project-%d", index+1),
			Name:       name,
			ConfigPath: configPath,
		})
	}
	store := instances.NewStore(filepath.Join(root, "appdata", "instances.json"))
	if err := store.Save(instances.Registry{
		Version:            instances.RegistryVersion,
		SelectedInstanceID: entries[0].ID,
		Instances:          entries,
	}); err != nil {
		t.Fatal(err)
	}
	controller, err := newMultiAppController(context.Background(), store, serverapp.Options{PortOverride: -1})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		controller.stopAll(stopCtx)
	}()
	response := httptest.NewRecorder()
	controller.handleStartAll(response, httptest.NewRequest(http.MethodPost, "/app/start-all", nil))
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"result":"starting"`) {
		t.Fatalf("start-all code=%d body=%s", response.Code, response.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for _, entry := range entries {
		app, _, ok := controller.resolveApp(entry.ID)
		for ok && !app.runner.Status(1).Running && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if !ok || !app.runner.Status(1).Running {
			t.Fatalf("%s was not running after Start All", entry.ID)
		}
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
	if err := os.MkdirAll(syncRoot, 0o755); err != nil {
		t.Fatalf("mkdir sync root: %v", err)
	}
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
	if !strings.Contains(string(cfg), `"remote_exec_enabled": true`) {
		t.Fatalf("saved config did not enable remote exec: %s", string(cfg))
	}
	var saved config.Config
	if err := json.Unmarshal(cfg, &saved); err != nil {
		t.Fatalf("decode saved config: %v", err)
	}
	if len(saved.RemoteExecToken) != config.RemoteExecTokenLength {
		t.Fatalf("RemoteExecToken length = %d, want %d", len(saved.RemoteExecToken), config.RemoteExecTokenLength)
	}
}

func TestAppStatusIncludesRemoteExecToken(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sync_config.json")
	syncRoot := filepath.Join(root, "game")
	configBody, err := json.Marshal(map[string]any{
		"host":                "127.0.0.1",
		"port":                8765,
		"sync_root":           syncRoot,
		"remote_exec_enabled": true,
		"remote_exec_token":   "displaytoken123456",
	})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	controller := &appController{
		runner:  serverapp.New(serverapp.Options{ConfigPath: configPath, PortOverride: -1}),
		options: serverapp.Options{ConfigPath: configPath, PortOverride: -1},
		quit:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/app/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("/app/status code=%d body=%q", response.Code, response.Body.String())
	}
	for _, needle := range []string{
		`"remote_exec_enabled":true`,
		`"remote_exec_configured":true`,
		`"remote_exec_available":true`,
		`"remote_exec_token":"displaytoken123456"`,
	} {
		if !strings.Contains(response.Body.String(), needle) {
			t.Fatalf("/app/status missing %q in %s", needle, response.Body.String())
		}
	}
}

func TestConfigApplyStoppedStatusUsesNewPathAfterPreviousRun(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sync_config.json")
	oldSyncRoot := filepath.Join(root, "old-game")
	newSyncRoot := filepath.Join(root, "new-game")
	if err := os.MkdirAll(oldSyncRoot, 0o755); err != nil {
		t.Fatalf("mkdir old sync root: %v", err)
	}
	if err := os.MkdirAll(newSyncRoot, 0o755); err != nil {
		t.Fatalf("mkdir new sync root: %v", err)
	}
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
	if err := os.MkdirAll(syncRoot, 0o755); err != nil {
		t.Fatalf("mkdir sync root: %v", err)
	}
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
	cases := []struct {
		name    string
		payload statusPayload
		want    string
	}{
		{name: "stopped", payload: statusPayload{}, want: "Stopped"},
		{name: "error", payload: statusPayload{LastError: "bind failed"}, want: "Error"},
		{name: "running before error", payload: statusPayload{Running: true, LastError: "previous warning"}, want: "Running"},
		{name: "syncing before running", payload: statusPayload{Running: true, Syncing: true}, want: "Syncing"},
		{name: "starting before syncing", payload: statusPayload{Running: true, Starting: true, Syncing: true}, want: "Starting"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusText(tc.payload); got != tc.want {
				t.Fatalf("statusText = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSyncingStatus(t *testing.T) {
	cases := []struct {
		name    string
		payload statusPayload
		want    bool
	}{
		{name: "determinate", payload: statusPayload{Running: true, ActivityPhase: "properties", ActivityProgress: 85}, want: true},
		{name: "indeterminate", payload: statusPayload{Running: true, ActivityPhase: "uploading", ActivityIndeterminate: true}, want: true},
		{name: "complete", payload: statusPayload{Running: true, ActivityPhase: "complete", ActivityProgress: 100, ActivityIndeterminate: true}, want: false},
		{name: "activity error", payload: statusPayload{Running: true, ActivityPhase: "properties", ActivityProgress: 85, ActivityError: true}, want: false},
		{name: "error phase", payload: statusPayload{Running: true, ActivityPhase: "error", ActivityProgress: 90, ActivityIndeterminate: true}, want: false},
		{name: "stopped", payload: statusPayload{ActivityPhase: "uploading", ActivityIndeterminate: true}, want: false},
		{name: "starting", payload: statusPayload{Running: true, Starting: true, ActivityPhase: "uploading", ActivityIndeterminate: true}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := syncingStatus(tc.payload); got != tc.want {
				t.Fatalf("syncingStatus = %v, want %v", got, tc.want)
			}
		})
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
		`local snapshotButton = button(actionGrid, "Resync"`,
		`AutomaticCanvasSize = Enum.AutomaticSize.Y`,
		`AutomaticSize = Enum.AutomaticSize.Y`,
		`label(connectionPanel, "CONNECTION"`,
		`profileEditor.Visible = false`,
		`debugBody.Visible = false`,
		`profileEditorToggleButton`,
		`debugDisclosureButton`,
		`value.Selectable = true`,
		`UDim2.new(0, 104, 0, 40)`,
		`listConnectionProfiles`,
		`selectConnectionProfile`,
		`upsertConnectionProfile`,
		`removeConnectionProfile`,
		`local refreshProfilesButton = button(profileSwitcher, ""`,
		`refreshProfilesIcon.Image = "rbxasset://studio_svg_textures/Lua/FileSync/Light/Standard/Refresh.png"`,
		`refreshProfilesButton.MouseButton1Click:Connect`,
		`refreshLocalProfilesFromApp(false)`,
		`local startSourcePanel = panel(scroll, 5)`,
		`"CHOOSE START SOURCE"`,
		`local startFromStudioButton = button(startSourceActions, "Use Studio"`,
		`local startFromLocalButton = button(startSourceActions, "Use Local"`,
		`local forceIdentityRepairButton = button(startSourcePanel, "Force ID repair: OFF"`,
		`forceIdentityRepairButton.MouseButton1Click:Connect`,
		`client:setForceIdentityRepair(forceIdentityRepairEnabled)`,
		`startWithMode(START_MODES.StudioToFolder)`,
		`startWithMode(START_MODES.FolderToStudio)`,
		`Start cancelled. Studio and local files were not changed.`,
	} {
		if !strings.Contains(document, needle) {
			t.Fatalf("plugin UI missing %q", needle)
		}
	}
	if strings.Contains(document, `snapshotButton.Text = "Snapshot"`) {
		t.Fatal("plugin still labels snapshot button as Snapshot")
	}
	if strings.Contains(document, `Instance.new("UIGradient")`) {
		t.Fatal("plugin still contains beveled gradient controls")
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
		`TypeList.VERSION = "4.2.04"`,
	} {
		if !strings.Contains(typeListDocument, needle) {
			t.Fatalf("TypeList missing %q", needle)
		}
	}
	latestPushStart := strings.LastIndex(apiDocument, "function SyncAPI:pushStudioSnapshot")
	if latestPushStart < 0 {
		t.Fatal("latest pushStudioSnapshot implementation not found")
	}
	latestPush := apiDocument[latestPushStart:]
	for _, needle := range []string{
		`Bootstrap completed with`,
		`skipInfo.ownership`,
		`markManagedInstance(`,
	} {
		if !strings.Contains(latestPush, needle) {
			t.Fatalf("latest pushStudioSnapshot missing %q", needle)
		}
	}
	for _, needle := range []string{
		`previewPullStudioToLocal`,
		`confirmPullStudioToLocal`,
		`cancelPullStudioToLocal`,
		`TypeList.ENDPOINTS.BootstrapPreview`,
		`pendingPullPreview`,
		`self:pushStudioSnapshot(`,
		`self.debugState.lastError = ""`,
		`self:refreshServerDebugState(true, true)`,
		`quiet = quiet == true`,
		`self:requestJson("POST", TypeList.ENDPOINTS.BootstrapPreview, payload, nil)`,
		`self:requestJson("POST", TypeList.ENDPOINTS.Bootstrap, payload, nil)`,
		`riftsync_connection_profiles_v1`,
		`riftsync_place_profile_map_v1`,
		`currentPlaceProfileKey`,
		`listConnectionProfiles`,
		`selectConnectionProfile`,
		`upsertConnectionProfile`,
		`removeConnectionProfile`,
		`serverSyncRootInitialized`,
		`response.deletion_tombstones`,
		`repair_tombstone = true`,
		`Repaired legacy delete`,
		`skipped stale repair tombstone`,
		`Refused unmanaged delete without exact RiftSync ownership`,
		`skipInfo.ownership`,
	} {
		if !strings.Contains(apiDocument, needle) {
			t.Fatalf("API missing %q", needle)
		}
	}
	for _, needle := range []string{
		`pullPreviewPanel`,
		`PULL STUDIO PREVIEW`,
		`confirmPullButton`,
		`cancelPullButton`,
		`client:previewPullStudioToLocal()`,
		`client:confirmPullStudioToLocal()`,
		`client:cancelPullStudioToLocal()`,
		`http_server_reachable`,
		`connected_to_server`,
		`token_valid`,
		`sync_active`,
		`exec_active`,
		`edit mode`,
		`progressTrack`,
		`progressFill`,
		`updateProgress`,
		`TweenInfo.new`,
	} {
		if !strings.Contains(pluginDocument, needle) {
			t.Fatalf("plugin UI missing %q", needle)
		}
	}
	if strings.Contains(pluginDocument, `table.insert(bits, string.gsub`) {
		t.Fatal("plugin passes string.gsub's second return value into table.insert")
	}
}

func TestPluginBroadPropertiesSchemaAndReferenceApply(t *testing.T) {
	apiBody, err := os.ReadFile(filepath.Join("..", "..", "plugin", "API.lua"))
	if err != nil {
		t.Fatalf("read API: %v", err)
	}
	typeListBody, err := os.ReadFile(filepath.Join("..", "..", "plugin", "TypeList.lua"))
	if err != nil {
		t.Fatalf("read TypeList: %v", err)
	}
	apiDocument := string(apiBody)
	typeListDocument := string(typeListBody)
	for _, needle := range []string{
		`Script = { "Enabled", "RunContext" }`,
		`Terrain = {`,
		`Atmosphere = {`,
		`DepthOfFieldEffect = {`,
		`IntValue = { "Value" }`,
		`ObjectValue = { "Value" }`,
		`AudioAnalyzer = { "SpectrumEnabled", "WindowSize" }`,
		`ParticleEmitter = {`,
		`WeldConstraint = { "Enabled", "Part0", "Part1" }`,
		`Decal = { "Color3", "Face",`,
		`Texture = { "Color3", "Face",`,
		`TypeList.GEOMETRY_ANCESTOR_CLASSES`,
		`TypeList.INSTANCE_REFERENCE_PROPERTIES`,
	} {
		if !strings.Contains(typeListDocument, needle) {
			t.Fatalf("broad property schema missing %q", needle)
		}
	}
	for _, forbidden := range []string{
		`BasePart = {`,
		`Model = {`,
		`"Source",`,
		`"FlipbookIncompatible",`,
		`"UICorner",`,
		`"UIScale",`,
	} {
		if strings.Contains(typeListDocument, forbidden) {
			t.Fatalf("broad property schema contains excluded entry %q", forbidden)
		}
	}
	for _, needle := range []string{
		`rootInstance:GetDescendants()`,
		`instance:IsA("BasePart") or instance:IsA("Model") or instance:IsA("Camera")`,
		`["$type"] = "InstanceRef"`,
		`["$type"] = "Ray"`,
		`buildReferenceIndex(self.managedRoots, self.ignoredRbxPaths)`,
		`properties.Enabled == nil and legacyDisabled ~= nil`,
		`ORPHAN_PARENT: metadata target`,
		`hasGeometryAncestorLocalPath(change.local_path)`,
		`resolveLocalChangeParent`,
		`isActualInstanceProperty`,
		`propertyCandidateCache`,
		`local function findUniqueChild`,
		`local LOCAL_ID_MARKER = "~rid_"`,
		`local function hasDuplicateSiblingIdentity`,
		`buildLocalIdentityName(current, ensureIdentity)`,
		`findChildByStableId(current, expectedStableId)`,
		`AMBIGUOUS_SIBLING: multiple children named`,
		`fatal_errors = fatalErrors`,
		`Studio snapshot dibatalkan agar object tidak tertukar`,
		`beginInitialLocalApplyRecording`,
		`changeHistoryService:TryBeginRecording(`,
		`Enum.FinishRecordingOperation.Commit`,
		`local visited = {}`,
		`duplicateStudioIds`,
		`local function isStudioRollbackPath`,
		`local serverStoragePrefix = "ServerStorage."`,
		`string.sub(rootFolderName, 1, 2) == "__"`,
		`string.find(string.lower(rootFolderName), "rollback", 1, true)`,
		`noteSkip("ignored_subtree", candidatePath)`,
		`buildManagedIndexes(self.managedRoots, self.ignoredRbxPaths)`,
		`Identity.canAdoptExactTarget(change, {`,
		`local function findAdoptableUnmanagedChild(`,
		`local canAdoptUnmanaged = resolvedTarget == nil`,
		`Exact same-class upsert dapat diadopsi`,
		`Refused replacing unmanaged instance at`,
	} {
		if !strings.Contains(apiDocument, needle) {
			t.Fatalf("broad property implementation missing %q", needle)
		}
	}
	for _, vendorSpecificRollbackName := range []string{`WeAreDevsRollback`, `WrexDevRollback`} {
		if strings.Contains(apiDocument, vendorSpecificRollbackName) {
			t.Fatalf("rollback filter must remain vendor-neutral; found %q", vendorSpecificRollbackName)
		}
	}
	stableLookup := strings.Index(apiDocument, `local byId = indexes.byStableId[stableId]`)
	localPathLookup := strings.Index(apiDocument, `local byLocalPath = indexes.byLocalPath[localPath]`)
	if stableLookup < 0 || localPathLookup < 0 || stableLookup > localPathLookup {
		t.Fatal("managed target lookup must prefer stable ID before local/name paths")
	}
	runLoopStart := strings.Index(apiDocument, "function SyncAPI:runLoop")
	if runLoopStart < 0 {
		t.Fatal("runLoop implementation not found")
	}
	runLoopEnd := strings.Index(apiDocument[runLoopStart:], "function SyncAPI:start")
	if runLoopEnd < 0 {
		t.Fatal("runLoop end not found")
	}
	runLoop := apiDocument[runLoopStart : runLoopStart+runLoopEnd]
	if strings.Count(runLoop, "self:pushStudioSnapshot()") != 1 {
		t.Fatal("runLoop must keep exactly one direct StudioToFolder bootstrap")
	}
	if strings.Contains(runLoop, `self:pushStudioSnapshot("replace", "Auto Pull Studio")`) ||
		strings.Count(runLoop, "acceptBootstrapRevision(bootstrapResponse)") != 1 {
		t.Fatal("explicit Local start must not silently switch to Studio auto-pull")
	}
	if strings.Contains(runLoop, `self.serverSyncRootEmpty == true and self.serverSyncRootInitialized ~= true`) {
		t.Fatal("explicit start source must not be overridden based on local folder state")
	}
	if strings.Contains(runLoop, "use Pull Studio to export Studio explicitly") {
		t.Fatal("empty FolderToStudio still requires a manual Pull Studio")
	}
	for _, needle := range []string{
		`function SyncAPI:setForceIdentityRepair`,
		`restoreStudioStableIds = function`,
		`clearStudioStableIds = function`,
		`isIgnoredPath(gamePath, self.ignoredRbxPaths or {})`,
		`finishForceIdentityRepairRecording`,
		`ID lama sudah dikembalikan`,
	} {
		if !strings.Contains(apiDocument, needle) {
			t.Fatalf("force identity repair missing %q", needle)
		}
	}
	if strings.Count(runLoop, "clearStudioStableIds(self)") != 2 {
		t.Fatal("force identity repair must clear Studio IDs once in either explicit start direction")
	}
	if strings.Count(runLoop, "identityRepairPending = false") != 2 {
		t.Fatal("force identity repair must remain a one-shot bootstrap operation")
	}
	lastApplyStart := strings.LastIndex(apiDocument, "function SyncAPI:applyChanges")
	if lastApplyStart < 0 {
		t.Fatal("latest applyChanges implementation not found")
	}
	latestApply := apiDocument[lastApplyStart:]
	declaration := strings.Index(latestApply, "local skippedCount = 0")
	firstIncrement := strings.Index(latestApply, "skippedCount += 1")
	if declaration < 0 || firstIncrement < 0 || declaration > firstIncrement {
		t.Fatal("latest applyChanges must initialize skippedCount before orphan handling")
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

func TestAppOpenIDERoute(t *testing.T) {
	tempDir := t.TempDir()
	opts := serverapp.Options{
		HostOverride:     "127.0.0.1",
		PortOverride:     freePort(t),
		SyncRootOverride: tempDir,
	}
	controller := &appController{
		runner:  serverapp.New(opts),
		options: opts,
		quit:    make(chan struct{}),
	}

	getReq := httptest.NewRequest(http.MethodGet, "/app/open-ide", nil)
	getRec := httptest.NewRecorder()
	controller.handleOpenIDE(getRec, getReq)
	if getRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 Method Not Allowed, got %d", getRec.Code)
	}

	body, _ := json.Marshal(openIDERequest{IDE: "invalid_ide_test"})
	postReq := httptest.NewRequest(http.MethodPost, "/app/open-ide", bytes.NewReader(body))
	postRec := httptest.NewRecorder()
	controller.handleOpenIDE(postRec, postReq)
	if postRec.Code != http.StatusOK && postRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status %d", postRec.Code)
	}
}
