package uiapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/exechistory"
	"riftsync/internal/instances"
	"riftsync/internal/profilediscovery"
	"riftsync/internal/serverapp"
	"riftsync/internal/state"

	webview "github.com/webview/webview_go"
)

const (
	defaultWindowWidth  = 1280
	defaultWindowHeight = 720
)

type appController struct {
	runner  *serverapp.Runner
	options serverapp.Options
	ctx     context.Context
	window  *windowController

	mu        sync.Mutex
	starting  bool
	startup   state.Activity
	lastError string
	quitOnce  sync.Once
	quit      chan struct{}
	terminate func()
	forceExit bool
}

type multiAppController struct {
	ctx      context.Context
	store    *instances.Store
	window   *windowController
	registry instances.Registry
	apps     map[string]*appController

	mu        sync.Mutex
	quitOnce  sync.Once
	quit      chan struct{}
	terminate func()
	forceExit bool
	warning   string
}

type windowController struct {
	native uintptr
}

type statusPayload struct {
	InstanceID            string  `json:"instance_id,omitempty"`
	InstanceName          string  `json:"instance_name,omitempty"`
	Running               bool    `json:"running"`
	Starting              bool    `json:"starting"`
	Syncing               bool    `json:"syncing"`
	Status                string  `json:"status"`
	Address               string  `json:"address"`
	Host                  string  `json:"host"`
	Port                  int     `json:"port"`
	ConfigPath            string  `json:"config_path"`
	SyncRoot              string  `json:"sync_root"`
	Mode                  string  `json:"mode"`
	Debug                 bool    `json:"debug"`
	LegacyScan            bool    `json:"legacy_scan"`
	Revision              int     `json:"revision"`
	Indexed               int     `json:"indexed"`
	Scripts               int     `json:"scripts"`
	UI                    int     `json:"ui"`
	RequestCount          int     `json:"request_count"`
	Polls                 int     `json:"polls"`
	GitEnabled            bool    `json:"git_enabled"`
	GitStatus             string  `json:"git_status"`
	GitLastError          string  `json:"git_last_error"`
	ActivityText          string  `json:"activity_text"`
	ActivityOp            string  `json:"activity_operation"`
	ActivityPhase         string  `json:"activity_phase"`
	ActivityError         bool    `json:"activity_error"`
	ActivityProgress      int     `json:"activity_progress"`
	ActivityCurrent       int     `json:"activity_current"`
	ActivityTotal         int     `json:"activity_total"`
	ActivityIndeterminate bool    `json:"activity_indeterminate"`
	ActivityClientID      string  `json:"activity_client_id"`
	ActivityRevision      int     `json:"activity_revision"`
	ActivityAt            float64 `json:"activity_at"`
	LastError             string  `json:"last_error"`
	Uptime                string  `json:"uptime"`
	RemoteExecEnabled     bool    `json:"remote_exec_enabled"`
	RemoteExecConfigured  bool    `json:"remote_exec_configured"`
	RemoteExecAvailable   bool    `json:"remote_exec_available"`
	RemoteExecToken       string  `json:"remote_exec_token"`
}

type restartRequest struct {
	ConfigPath string `json:"config_path"`
	SyncRoot   string `json:"sync_root"`
	Host       string `json:"host"`
	Port       string `json:"port"`
	GitEnabled bool   `json:"git_enabled"`
	Debug      bool   `json:"debug"`
	LegacyScan bool   `json:"legacy_scan"`
}

type execRunRequest struct {
	Source     string `json:"source"`
	TimeoutSec int    `json:"timeout_sec"`
}

type execRerunRequest struct {
	ID         string `json:"id"`
	TimeoutSec int    `json:"timeout_sec"`
}

type createInstanceRequest struct {
	Name       string `json:"name"`
	SyncRoot   string `json:"sync_root"`
	ConfigPath string `json:"config_path"`
}

type instanceIDRequest struct {
	ID string `json:"id"`
}

type updateInstanceRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type sidebarRequest struct {
	Pinned bool `json:"pinned"`
}

type appExecPrint struct {
	Level string  `json:"level"`
	Text  string  `json:"text"`
	AtMS  float64 `json:"at_ms"`
}

type appExecResult struct {
	ID         string         `json:"id"`
	State      string         `json:"state"`
	OK         bool           `json:"ok"`
	DurationMS float64        `json:"duration_ms"`
	Prints     []appExecPrint `json:"prints"`
	Returns    []any          `json:"returns"`
	Error      string         `json:"error"`
	Traceback  string         `json:"traceback"`
	LateResult bool           `json:"late_result"`
}

type appExecSubmitResponse struct {
	Status    string `json:"status"`
	CommandID string `json:"command_id"`
	Message   string `json:"message"`
}

type appExecDetailResponse struct {
	Status  string        `json:"status"`
	Command appExecResult `json:"command"`
	Message string        `json:"message"`
}

// Run starts a tiny loopback control UI and displays it in an embedded WebView2 window.
// The sync server itself remains stopped until the user clicks Start.
func Run(ctx context.Context, options serverapp.Options) error {
	registryPath, err := instances.DefaultPath()
	if err != nil {
		return err
	}
	controller, err := newMultiAppController(ctx, instances.NewStore(registryPath), options)
	if err != nil {
		return err
	}
	discovery, err := profilediscovery.Start(ctx, controller.pluginProfileSnapshot)
	if err != nil {
		return fmt.Errorf("start plugin profile discovery: %w", err)
	}
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start app UI listener: %w", err)
	}
	defer listener.Close()

	server := &http.Server{Handler: mux}
	serveErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()

	url := "http://" + listener.Addr().String() + "/app"
	w := webview.New(options.Debug)
	if w == nil {
		return errors.New("create WebView2 window failed")
	}
	defer w.Destroy()
	w.SetTitle("RiftSync")
	w.SetSize(defaultWindowWidth, defaultWindowHeight, webview.HintNone)
	window := &windowController{native: uintptr(w.Window())}
	controller.window = window
	controller.terminate = w.Terminate
	controller.mu.Lock()
	for _, app := range controller.apps {
		app.window = window
	}
	controller.mu.Unlock()
	_ = window.makeFrameless()
	_ = w.Bind("appDragWindow", func() error {
		return window.drag()
	})
	_ = w.Bind("appMinimizeWindow", func() error {
		return window.minimize()
	})
	_ = w.Bind("appCloseWindow", func() error {
		controller.requestQuit()
		return nil
	})
	w.Navigate(url)

	monitorErr := make(chan error, 1)
	go func() {
		var err error
		select {
		case <-ctx.Done():
		case <-controller.quit:
		case err = <-serveErr:
		}
		monitorErr <- err
		w.Terminate()
	}()

	w.Run()

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	controller.stopAll(stopCtx)
	_ = discovery.Close(stopCtx)
	_ = server.Shutdown(stopCtx)

	var result error
	select {
	case result = <-monitorErr:
	default:
	}
	select {
	case err := <-serveErr:
		if result == nil {
			result = err
		}
	case <-stopCtx.Done():
	}
	return result
}

func (m *multiAppController) pluginProfileSnapshot() profilediscovery.Snapshot {
	m.mu.Lock()
	registry := m.registry
	type pair struct {
		entry instances.Entry
		app   *appController
	}
	pairs := make([]pair, 0, len(registry.Instances))
	for _, entry := range registry.Instances {
		pairs = append(pairs, pair{entry: entry, app: m.apps[entry.ID]})
	}
	m.mu.Unlock()

	profiles := make([]profilediscovery.Profile, 0, len(pairs))
	for _, item := range pairs {
		if item.app == nil {
			continue
		}
		status := item.app.status()
		profiles = append(profiles, profilediscovery.Profile{
			ID:      "local:" + item.entry.ID,
			Name:    item.entry.Name,
			Host:    fallback(status.Host, "127.0.0.1"),
			Port:    status.Port,
			Token:   status.RemoteExecToken,
			Running: status.Running,
		})
	}
	selectedID := ""
	if registry.SelectedInstanceID != "" {
		selectedID = "local:" + registry.SelectedInstanceID
	}
	return profilediscovery.Snapshot{Profiles: profiles, SelectedProfileID: selectedID}
}

func newMultiAppController(ctx context.Context, store *instances.Store, options serverapp.Options) (*multiAppController, error) {
	_, registryStatErr := os.Stat(store.Path())
	registryExisted := registryStatErr == nil
	registry, warning, err := store.Load()
	if err != nil {
		return nil, err
	}
	seeded := false
	seedConfigPath := fallback(options.ConfigPath, "sync_config.json")
	_, seedConfigErr := os.Stat(seedConfigPath)
	if len(registry.Instances) == 0 && (!registryExisted || warning != "") && seedConfigErr == nil {
		registry, seeded, err = instances.EnsureSeed(registry, seedConfigPath)
		if err != nil {
			return nil, err
		}
	}
	if seeded {
		if err := store.Save(registry); err != nil {
			return nil, err
		}
	}
	controller := &multiAppController{
		ctx:       ctx,
		store:     store,
		registry:  registry,
		apps:      map[string]*appController{},
		quit:      make(chan struct{}),
		forceExit: true,
		warning:   warning,
	}
	seedConfigAbs, _ := filepath.Abs(seedConfigPath)
	for _, entry := range registry.Instances {
		entryOptions := options
		entryOptions.ConfigPath = entry.ConfigPath
		if !strings.EqualFold(filepath.Clean(entry.ConfigPath), filepath.Clean(seedConfigAbs)) {
			entryOptions.HostOverride = ""
			entryOptions.PortOverride = -1
			entryOptions.SyncRootOverride = ""
		}
		controller.apps[entry.ID] = controller.newInstanceController(entryOptions)
	}
	return controller, nil
}

func (m *multiAppController) newInstanceController(options serverapp.Options) *appController {
	options = normalizeUIOptions(options)
	return &appController{
		runner:    serverapp.New(options),
		options:   options,
		ctx:       m.ctx,
		window:    m.window,
		quit:      m.quit,
		forceExit: false,
	}
}

func (m *multiAppController) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/app", m.handleApp)
	mux.HandleFunc("/app/instances", m.handleInstances)
	mux.HandleFunc("/app/instances/select", m.handleSelectInstance)
	mux.HandleFunc("/app/instances/update", m.handleUpdateInstance)
	mux.HandleFunc("/app/instances/remove", m.handleRemoveInstance)
	mux.HandleFunc("/app/instances/pick-folder", m.handleGlobalPickFolder)
	mux.HandleFunc("/app/sidebar", m.handleSidebar)
	mux.HandleFunc("/app/start-all", m.handleStartAll)
	mux.HandleFunc("/app/status", m.delegate(func(a *appController, w http.ResponseWriter, r *http.Request) { a.handleStatus(w, r) }))
	mux.HandleFunc("/app/start", m.handleStartInstance)
	mux.HandleFunc("/app/stop", m.delegate(func(a *appController, w http.ResponseWriter, r *http.Request) { a.handleStop(w, r) }))
	mux.HandleFunc("/app/restart", m.handleScopedConfigMutation(true))
	mux.HandleFunc("/app/config/apply", m.handleScopedConfigMutation(false))
	mux.HandleFunc("/app/open-folder", m.delegate(func(a *appController, w http.ResponseWriter, r *http.Request) { a.handleOpenFolder(w, r) }))
	mux.HandleFunc("/app/pick-folder", m.delegate(func(a *appController, w http.ResponseWriter, r *http.Request) { a.handlePickFolder(w, r) }))
	mux.HandleFunc("/app/history", m.delegate(func(a *appController, w http.ResponseWriter, r *http.Request) { a.handleHistory(w, r) }))
	mux.HandleFunc("/app/exec/history", m.delegate(func(a *appController, w http.ResponseWriter, r *http.Request) { a.handleExecHistory(w, r) }))
	mux.HandleFunc("/app/exec/run", m.delegate(func(a *appController, w http.ResponseWriter, r *http.Request) { a.handleExecRun(w, r) }))
	mux.HandleFunc("/app/exec/rerun", m.delegate(func(a *appController, w http.ResponseWriter, r *http.Request) { a.handleExecRerun(w, r) }))
	mux.HandleFunc("/app/logs", m.delegate(func(a *appController, w http.ResponseWriter, r *http.Request) { a.handleLogs(w, r) }))
	mux.HandleFunc("/app/window/minimize", m.handleWindowMinimize)
	mux.HandleFunc("/app/window/close", m.handleWindowClose)
	mux.HandleFunc("/app/quit", m.handleWindowClose)
}

func (m *multiAppController) handleApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(htmlDocument()))
}

func (m *multiAppController) delegate(handler func(*appController, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		app, _, ok := m.resolveApp(r.URL.Query().Get("instance_id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": "instance not found"})
			return
		}
		handler(app, w, r)
	}
}

func (m *multiAppController) resolveApp(id string) (*appController, instances.Entry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id = strings.TrimSpace(id)
	if id == "" {
		id = m.registry.SelectedInstanceID
	}
	app := m.apps[id]
	for _, entry := range m.registry.Instances {
		if entry.ID == id && app != nil {
			return app, entry, true
		}
	}
	return nil, instances.Entry{}, false
}

func (m *multiAppController) handleInstances(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		m.writeInstances(w)
	case http.MethodPost:
		m.createInstance(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
	}
}

func (m *multiAppController) handleGlobalPickFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	owner := uintptr(0)
	if m.window != nil {
		owner = m.window.native
	}
	path, selected, err := pickFolder(owner)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "selected": selected, "path": path})
}

func (m *multiAppController) writeInstances(w http.ResponseWriter) {
	m.mu.Lock()
	registry := m.registry
	warning := m.warning
	type pair struct {
		entry instances.Entry
		app   *appController
	}
	pairs := make([]pair, 0, len(registry.Instances))
	for _, entry := range registry.Instances {
		pairs = append(pairs, pair{entry: entry, app: m.apps[entry.ID]})
	}
	m.mu.Unlock()

	items := make([]map[string]any, 0, len(pairs))
	for _, item := range pairs {
		status := item.app.status()
		status.InstanceID = item.entry.ID
		status.InstanceName = item.entry.Name
		items = append(items, map[string]any{
			"id":          item.entry.ID,
			"name":        item.entry.Name,
			"config_path": item.entry.ConfigPath,
			"app":         status,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":               "ok",
		"instances":            items,
		"selected_instance_id": registry.SelectedInstanceID,
		"sidebar_pinned":       registry.SidebarPinned,
		"warning":              warning,
	})
}

func (m *multiAppController) createInstance(w http.ResponseWriter, r *http.Request) {
	var request createInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON"})
		return
	}
	syncRoot := strings.TrimSpace(request.SyncRoot)
	configPath := strings.TrimSpace(request.ConfigPath)
	if syncRoot == "" && configPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "sync_root or config_path is required"})
		return
	}
	if syncRoot != "" {
		abs, err := filepath.Abs(syncRoot)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		syncRoot = filepath.Clean(abs)
	}
	if configPath == "" {
		configPath = filepath.Join(syncRoot, config.MetadataDir, "sync_config.json")
	}
	configAbs, err := filepath.Abs(configPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	configPath = filepath.Clean(configAbs)

	_, statErr := os.Stat(configPath)
	configExists := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": statErr.Error()})
		return
	}
	cfg := config.Default()
	if configExists {
		cfg, err = config.Load(configPath)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
	} else {
		cfg = config.Default()
	}
	if !configExists {
		cfg.SyncRoot = syncRoot
		cfg.Port = m.nextAvailablePort()
		cfg.RemoteExecEnabled = true
		if _, err := config.EnsureRemoteExecToken(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		if err := config.Save(configPath, cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
	} else {
		if syncRoot == "" {
			syncRoot = cfg.SyncRootAbs
		} else if !strings.EqualFold(filepath.Clean(syncRoot), filepath.Clean(cfg.SyncRootAbs)) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"status": "error",
				"error":  "selected sync root does not match the existing config sync_root",
			})
			return
		}
	}
	if err := cfg.NormalizeAndValidate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = filepath.Base(filepath.Clean(syncRoot))
	}
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "Project"
	}
	entry := instances.Entry{ID: instances.NewID(), Name: name, ConfigPath: configPath}
	if err := m.addEntry(entry, cfg); err != nil {
		if !configExists {
			// Keep the project folder and config recoverable; only report the registry conflict.
		}
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	m.writeInstances(w)
}

func (m *multiAppController) handleScopedConfigMutation(restart bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
			return
		}
		app, entry, ok := m.resolveApp(r.URL.Query().Get("instance_id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": "instance not found"})
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		var request restartRequest
		if err := json.Unmarshal(body, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON"})
			return
		}
		requestConfigAbs, err := filepath.Abs(fallback(request.ConfigPath, entry.ConfigPath))
		if err != nil || !strings.EqualFold(filepath.Clean(requestConfigAbs), filepath.Clean(entry.ConfigPath)) {
			writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "registered config path cannot be changed; import it as another project"})
			return
		}
		next, err := optionsFromInput(app.runner.Options(), entry.ConfigPath, request.SyncRoot, request.Host, request.Port, request.GitEnabled, request.Debug, request.LegacyScan)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		if err := m.validateOptionsUnique(entry.ID, next); err != nil {
			writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		if restart {
			app.handleRestart(w, r)
		} else {
			app.handleConfigApply(w, r)
		}
	}
}

func (m *multiAppController) validateOptionsUnique(instanceID string, options serverapp.Options) error {
	cfg, err := config.Load(options.ConfigPath)
	if err != nil {
		return err
	}
	if options.HostOverride != "" {
		cfg.Host = options.HostOverride
	}
	if options.PortOverride > 0 {
		cfg.Port = options.PortOverride
	}
	if options.SyncRootOverride != "" {
		cfg.SyncRoot = options.SyncRootOverride
	}
	if err := cfg.NormalizeAndValidate(); err != nil {
		return err
	}
	m.mu.Lock()
	entries := append([]instances.Entry{}, m.registry.Instances...)
	m.mu.Unlock()
	for _, entry := range entries {
		if entry.ID == instanceID {
			continue
		}
		other, err := config.Load(entry.ConfigPath)
		if err != nil {
			continue
		}
		if strings.EqualFold(other.SyncRootAbs, cfg.SyncRootAbs) {
			return fmt.Errorf("sync root is already used by %s", entry.Name)
		}
		if other.Host == cfg.Host && other.Port == cfg.Port {
			return fmt.Errorf("port %d is already configured by %s", cfg.Port, entry.Name)
		}
	}
	return nil
}

func (m *multiAppController) addEntry(entry instances.Entry, cfg config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.registry.Instances {
		if strings.EqualFold(filepath.Clean(existing.ConfigPath), filepath.Clean(entry.ConfigPath)) {
			return fmt.Errorf("config path is already registered by %s", existing.Name)
		}
		existingCfg, err := config.Load(existing.ConfigPath)
		if err == nil {
			if strings.EqualFold(existingCfg.SyncRootAbs, cfg.SyncRootAbs) {
				return fmt.Errorf("sync root is already registered by %s", existing.Name)
			}
			if existingCfg.Host == cfg.Host && existingCfg.Port == cfg.Port {
				return fmt.Errorf("port %d is already configured by %s", cfg.Port, existing.Name)
			}
		}
	}
	next := m.registry
	next.Instances = append(append([]instances.Entry{}, next.Instances...), entry)
	next.SelectedInstanceID = entry.ID
	if err := m.store.Save(next); err != nil {
		return err
	}
	options := serverapp.Options{ConfigPath: entry.ConfigPath, PortOverride: -1}
	m.apps[entry.ID] = m.newInstanceController(options)
	m.registry = next
	return nil
}

func (m *multiAppController) nextAvailablePort() int {
	m.mu.Lock()
	used := map[int]bool{}
	for _, entry := range m.registry.Instances {
		if cfg, err := config.Load(entry.ConfigPath); err == nil {
			used[cfg.Port] = true
		}
	}
	m.mu.Unlock()
	for port := 8765; port <= 65535; port++ {
		if used[port] {
			continue
		}
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			_ = listener.Close()
			return port
		}
	}
	return 8765
}

func (m *multiAppController) handleSelectInstance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	var request instanceIDRequest
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON"})
		return
	}
	m.mu.Lock()
	if m.apps[strings.TrimSpace(request.ID)] == nil {
		m.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": "instance not found"})
		return
	}
	next := m.registry
	next.SelectedInstanceID = strings.TrimSpace(request.ID)
	if err := m.store.Save(next); err != nil {
		m.mu.Unlock()
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	m.registry = next
	m.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "selected_instance_id": request.ID})
}

func (m *multiAppController) handleUpdateInstance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	var request updateInstanceRequest
	if json.NewDecoder(r.Body).Decode(&request) != nil || strings.TrimSpace(request.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "id and name are required"})
		return
	}
	m.mu.Lock()
	next := m.registry
	found := false
	for index := range next.Instances {
		if next.Instances[index].ID == strings.TrimSpace(request.ID) {
			next.Instances[index].Name = strings.TrimSpace(request.Name)
			found = true
			break
		}
	}
	if !found {
		m.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": "instance not found"})
		return
	}
	if err := m.store.Save(next); err != nil {
		m.mu.Unlock()
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	m.registry = next
	m.mu.Unlock()
	m.writeInstances(w)
}

func (m *multiAppController) handleRemoveInstance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	var request instanceIDRequest
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON"})
		return
	}
	id := strings.TrimSpace(request.ID)
	m.mu.Lock()
	app := m.apps[id]
	if app == nil {
		m.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": "instance not found"})
		return
	}
	if app.configLocked() {
		m.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "stop this instance before removing it"})
		return
	}
	next := m.registry
	filtered := make([]instances.Entry, 0, len(next.Instances)-1)
	for _, entry := range next.Instances {
		if entry.ID != id {
			filtered = append(filtered, entry)
		}
	}
	next.Instances = filtered
	if next.SelectedInstanceID == id {
		next.SelectedInstanceID = ""
		if len(filtered) > 0 {
			next.SelectedInstanceID = filtered[0].ID
		}
	}
	if err := m.store.Save(next); err != nil {
		m.mu.Unlock()
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	delete(m.apps, id)
	m.registry = next
	m.mu.Unlock()
	m.writeInstances(w)
}

func (m *multiAppController) handleSidebar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	var request sidebarRequest
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON"})
		return
	}
	m.mu.Lock()
	next := m.registry
	next.SidebarPinned = request.Pinned
	if err := m.store.Save(next); err != nil {
		m.mu.Unlock()
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	m.registry = next
	m.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "sidebar_pinned": request.Pinned})
}

func (m *multiAppController) handleStartAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	m.mu.Lock()
	apps := make(map[string]*appController, len(m.apps))
	for id, app := range m.apps {
		apps[id] = app
	}
	m.mu.Unlock()
	results := map[string]any{}
	for id, app := range apps {
		options := app.runner.Options()
		err := m.validateOptionsUnique(id, options)
		if err == nil {
			err = ensureRemoteExecConfig(options)
		}
		if err != nil {
			results[id] = map[string]any{"status": "error", "error": err.Error(), "app": app.status()}
			continue
		}
		started := app.startAsync(m.ctx)
		result := "already_running"
		if started {
			result = "starting"
		}
		results[id] = map[string]any{"status": "ok", "result": result, "app": app.status()}
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "ok", "results": results})
}

func (m *multiAppController) handleStartInstance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	app, entry, ok := m.resolveApp(r.URL.Query().Get("instance_id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": "instance not found"})
		return
	}
	if err := m.validateOptionsUnique(entry.ID, app.runner.Options()); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": err.Error(), "app": app.status()})
		return
	}
	app.handleStart(w, r)
}

func (m *multiAppController) handleWindowMinimize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	if m.window != nil {
		_ = m.window.minimize()
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (m *multiAppController) handleWindowClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	m.requestQuit()
}

func (m *multiAppController) requestQuit() {
	m.quitOnce.Do(func() {
		close(m.quit)
		if m.window != nil {
			_ = m.window.close()
		}
		if m.terminate != nil {
			go m.terminate()
		}
		if m.forceExit {
			time.AfterFunc(1500*time.Millisecond, func() { os.Exit(0) })
		}
	})
}

func (m *multiAppController) stopAll(ctx context.Context) {
	m.mu.Lock()
	apps := make([]*appController, 0, len(m.apps))
	for _, app := range m.apps {
		apps = append(apps, app)
	}
	m.mu.Unlock()
	var group sync.WaitGroup
	for _, app := range apps {
		group.Add(1)
		go func(app *appController) {
			defer group.Done()
			_ = app.runner.Stop(ctx)
		}(app)
	}
	group.Wait()
}

func (a *appController) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/app", a.handleApp)
	mux.HandleFunc("/app/status", a.handleStatus)
	mux.HandleFunc("/app/start", a.handleStart)
	mux.HandleFunc("/app/stop", a.handleStop)
	mux.HandleFunc("/app/restart", a.handleRestart)
	mux.HandleFunc("/app/config/apply", a.handleConfigApply)
	mux.HandleFunc("/app/open-folder", a.handleOpenFolder)
	mux.HandleFunc("/app/pick-folder", a.handlePickFolder)
	mux.HandleFunc("/app/history", a.handleHistory)
	mux.HandleFunc("/app/exec/history", a.handleExecHistory)
	mux.HandleFunc("/app/exec/run", a.handleExecRun)
	mux.HandleFunc("/app/exec/rerun", a.handleExecRerun)
	mux.HandleFunc("/app/logs", a.handleLogs)
	mux.HandleFunc("/app/window/minimize", a.handleWindowMinimize)
	mux.HandleFunc("/app/window/close", a.handleWindowClose)
	mux.HandleFunc("/app/quit", a.handleQuit)
}

func (a *appController) handleApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(htmlDocument()))
}

func (a *appController) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "app": a.status()})
}

func (a *appController) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	if err := ensureRemoteExecConfig(a.options); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error(), "app": a.status()})
		return
	}
	started := a.startAsync(a.ctx)
	result := "already_running"
	statusCode := http.StatusOK
	if started {
		result = "starting"
		statusCode = http.StatusAccepted
	}
	writeJSON(w, statusCode, map[string]any{"status": "ok", "result": result, "app": a.status()})
}

func (a *appController) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.runner.Stop(stopCtx); err != nil {
		a.setLastError(err.Error())
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error(), "app": a.status()})
		return
	}
	a.setLastError("")
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "app": a.status()})
}

func (a *appController) handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	if a.configLocked() {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "stop sync before changing config", "app": a.status()})
		return
	}
	next, err := a.optionsFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error(), "app": a.status()})
		return
	}
	if err := a.saveAndSetOptions(next); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error(), "app": a.status()})
		return
	}

	a.mu.Lock()
	a.starting = true
	a.lastError = ""
	a.mu.Unlock()

	err = a.runner.Restart(a.ctx, next)
	a.mu.Lock()
	a.starting = false
	if err != nil {
		a.lastError = err.Error()
	}
	a.mu.Unlock()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error(), "app": a.status()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "result": "restarted", "app": a.status()})
}

func (a *appController) handleConfigApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	if a.configLocked() {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "stop sync before changing config", "app": a.status()})
		return
	}
	next, err := a.optionsFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error(), "app": a.status()})
		return
	}
	if err := a.saveAndSetOptions(next); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error(), "app": a.status()})
		return
	}

	a.mu.Lock()
	a.starting = true
	a.lastError = ""
	a.mu.Unlock()

	restarted, err := a.runner.ApplyOptions(a.ctx, next)
	a.mu.Lock()
	a.starting = false
	if err != nil {
		a.lastError = err.Error()
	}
	a.mu.Unlock()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error(), "app": a.status()})
		return
	}
	result := "saved_only"
	if restarted {
		result = "restarted"
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "result": result, "app": a.status()})
}

func (a *appController) handleOpenFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	payload := a.status()
	target := strings.TrimSpace(payload.SyncRoot)
	if target == "" {
		target = "src/game"
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error(), "app": payload})
		return
	}
	if err := openFolder(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error(), "app": payload})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "app": payload})
}

func (a *appController) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	owner := uintptr(0)
	if a.window != nil {
		owner = a.window.native
	}
	path, selected, err := pickFolder(owner)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error(), "app": a.status()})
		return
	}
	payload := a.status()
	if selected && strings.TrimSpace(path) != "" {
		payload.SyncRoot = path
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"selected": selected,
		"path":     path,
		"app":      payload,
	})
}

func (a *appController) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	if revArg := r.URL.Query().Get("rev"); revArg != "" {
		revision, err := strconv.Atoi(revArg)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "rev must be an integer"})
			return
		}
		serverRev, gitState, summary, changes, found := a.runner.HistoryDetail(revision)
		payload := map[string]any{
			"status":     "ok",
			"found":      found,
			"rev":        revision,
			"server_rev": serverRev,
			"git":        gitState,
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
		return
	}

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	serverRev, gitState, revisions := a.runner.HistorySummaries(limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"server_rev": serverRev,
		"git":        gitState,
		"revisions":  revisions,
	})
}

func (a *appController) handleExecHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	syncRoot := a.status().SyncRoot
	file, warning, err := exechistory.Load(syncRoot)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"warning": warning,
		"entries": file.Entries,
	})
}

func (a *appController) handleExecRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	var request execRunRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON"})
		return
	}
	if strings.TrimSpace(request.Source) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "source is required"})
		return
	}
	a.runExecAndWrite(w, request.Source, request.TimeoutSec, "")
}

func (a *appController) handleExecRerun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	var request execRerunRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON"})
		return
	}
	syncRoot := a.status().SyncRoot
	file, warning, err := exechistory.Load(syncRoot)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	var entry exechistory.Entry
	found := false
	if strings.TrimSpace(request.ID) != "" {
		for _, candidate := range file.Entries {
			if candidate.ID == request.ID && strings.TrimSpace(candidate.Source) != "" {
				entry = candidate
				found = true
				break
			}
		}
	} else {
		entry, found = exechistory.LatestUsable(file)
	}
	if !found {
		message := "no usable Remote Exec history"
		if warning != "" {
			message = warning
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": message})
		return
	}
	timeoutSec := request.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = entry.TimeoutSec
	}
	a.runExecAndWrite(w, entry.Source, timeoutSec, entry.ID)
}

func (a *appController) runExecAndWrite(w http.ResponseWriter, source string, timeoutSec int, rerunOfID string) {
	status := a.runner.Status(5)
	if !status.Running {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "Start RiftSync first"})
		return
	}
	cfg, err := config.Load(fallback(status.ConfigPath, a.options.ConfigPath))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	if timeoutSec <= 0 {
		timeoutSec = cfg.RemoteExecDefaultTimeoutSec
	}
	if timeoutSec < 1 {
		timeoutSec = 1
	}
	if timeoutSec > cfg.RemoteExecMaxTimeoutSec {
		timeoutSec = cfg.RemoteExecMaxTimeoutSec
	}

	result, commandID, runErr := runAppRemoteExec(a.ctx, status.Host, status.Port, cfg.RemoteExecToken, source, timeoutSec)
	if runErr != nil {
		result = appExecResult{
			ID:    commandID,
			State: "submit_failed",
			OK:    false,
			Error: runErr.Error(),
		}
	}
	entry, warning, historyErr := exechistory.Append(status.SyncRoot, exechistory.Entry{
		SubmittedBy: "app",
		SourceKind:  "app",
		Source:      source,
		TimeoutSec:  timeoutSec,
		ResultState: result.State,
		OK:          result.OK,
		Error:       result.Error,
		Output:      formatAppExecOutput(result),
		Traceback:   result.Traceback,
		DurationMS:  result.DurationMS,
		CommandID:   fallback(commandID, result.ID),
		RerunOfID:   rerunOfID,
	}, exechistory.DefaultLimit)
	if historyErr != nil && runErr == nil {
		runErr = historyErr
	}
	payload := map[string]any{
		"status":          "ok",
		"result":          result,
		"output":          formatAppExecOutput(result),
		"entry":           entry,
		"history_warning": warning,
	}
	if runErr != nil {
		payload["status"] = "error"
		payload["error"] = runErr.Error()
		writeJSON(w, http.StatusBadRequest, payload)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func runAppRemoteExec(ctx context.Context, host string, port int, token, source string, timeoutSec int) (appExecResult, string, error) {
	baseURL := fmt.Sprintf("http://%s:%d", host, port)
	commandID, err := submitAppRemoteExec(ctx, baseURL, token, source, timeoutSec)
	if err != nil {
		return appExecResult{}, "", err
	}
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec+5)*time.Second)
	defer cancel()
	result, err := waitAppRemoteExec(waitCtx, baseURL, token, commandID)
	return result, commandID, err
}

func submitAppRemoteExec(ctx context.Context, baseURL, token, source string, timeoutSec int) (string, error) {
	payload := map[string]any{
		"source":      source,
		"timeout_sec": timeoutSec,
		"client":      "riftsync-app",
		"mode":        "edit",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/exec/commands", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	applyAppExecAuth(req, token)
	var response appExecSubmitResponse
	if err := doAppExecJSON(req, &response); err != nil {
		return "", err
	}
	if response.CommandID == "" {
		return "", errors.New("server returned empty command_id")
	}
	return response.CommandID, nil
}

func waitAppRemoteExec(ctx context.Context, baseURL, token, commandID string) (appExecResult, error) {
	for {
		result, err := detailAppRemoteExec(ctx, baseURL, token, commandID)
		if err != nil {
			return appExecResult{}, err
		}
		if isAppExecTerminalState(result.State) {
			return result, nil
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return appExecResult{ID: commandID, State: "timeout", OK: false, Error: ctx.Err().Error()}, ctx.Err()
		case <-timer.C:
		}
	}
}

func detailAppRemoteExec(ctx context.Context, baseURL, token, commandID string) (appExecResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/exec/commands/"+commandID, nil)
	if err != nil {
		return appExecResult{}, err
	}
	applyAppExecAuth(req, token)
	var response appExecDetailResponse
	if err := doAppExecJSON(req, &response); err != nil {
		return appExecResult{}, err
	}
	if response.Command.ID == "" {
		response.Command.ID = commandID
	}
	return response.Command, nil
}

func doAppExecJSON(req *http.Request, target any) error {
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		var payload map[string]any
		if json.Unmarshal(body, &payload) == nil {
			if value, ok := payload["message"].(string); ok && value != "" {
				message = value
			}
		}
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, message)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}
	return nil
}

func applyAppExecAuth(req *http.Request, token string) {
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
}

func isAppExecTerminalState(state string) bool {
	switch state {
	case "done", "error", "timeout", "expired", "cancelled":
		return true
	default:
		return false
	}
}

func formatAppExecOutput(result appExecResult) string {
	lines := []string{}
	for _, entry := range result.Prints {
		level := strings.TrimSpace(entry.Level)
		if level == "" {
			level = "print"
		}
		lines = append(lines, "["+level+"] "+entry.Text)
	}
	if len(result.Returns) > 0 {
		values := make([]string, 0, len(result.Returns))
		for _, value := range result.Returns {
			values = append(values, fmt.Sprint(value))
		}
		lines = append(lines, "[return] "+strings.Join(values, ", "))
	}
	if result.Error != "" {
		lines = append(lines, "[error] "+result.Error)
	}
	if result.Traceback != "" {
		lines = append(lines, "[traceback] "+result.Traceback)
	}
	if result.State == "done" && result.OK {
		lines = append(lines, "[ok] "+formatAppMillis(result.DurationMS)+"ms")
	}
	if len(lines) == 0 {
		lines = append(lines, "[state] "+fallback(result.State, "unknown"))
	}
	return strings.Join(lines, "\n")
}

func formatAppMillis(value float64) string {
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d", int64(value))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
}

func (a *appController) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	a.mu.Lock()
	options := a.options
	a.mu.Unlock()
	status := a.runner.Status(120)
	status = hydrateStoppedStatus(status, options)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"running":       status.Running,
		"revision":      status.Revision,
		"events":        status.Events,
		"metrics":       status.Metrics,
		"git":           status.Git,
		"activity":      status.Activity,
		"scan_warnings": status.Warnings,
	})
}

func (a *appController) handleWindowMinimize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	if a.window != nil {
		_ = a.window.minimize()
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (a *appController) handleWindowClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	a.requestQuit()
}

func (a *appController) handleQuit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	a.requestQuit()
}

func (a *appController) requestQuit() {
	a.quitOnce.Do(func() {
		close(a.quit)
		if a.window != nil {
			_ = a.window.close()
		}
		if a.terminate != nil {
			go a.terminate()
		}
		if a.forceExit {
			time.AfterFunc(1500*time.Millisecond, func() {
				os.Exit(0)
			})
		}
	})
}

func (a *appController) optionsFromRequest(r *http.Request) (serverapp.Options, error) {
	var request restartRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return serverapp.Options{}, fmt.Errorf("invalid JSON: %w", err)
	}
	current := a.runner.Options()
	return optionsFromInput(current, request.ConfigPath, request.SyncRoot, request.Host, request.Port, request.GitEnabled, request.Debug, request.LegacyScan)
}

func (a *appController) saveAndSetOptions(options serverapp.Options) error {
	if err := saveConfigFromOptions(options); err != nil {
		a.setLastError(err.Error())
		return err
	}
	a.mu.Lock()
	a.options = options
	a.lastError = ""
	a.mu.Unlock()
	return nil
}

func (a *appController) start(ctx context.Context) error {
	if !a.beginStart() {
		return nil
	}
	return a.runStart(ctx)
}

func (a *appController) startAsync(ctx context.Context) bool {
	if !a.beginStart() {
		return false
	}
	go func() {
		_ = a.runStart(ctx)
	}()
	return true
}

func (a *appController) beginStart() bool {
	if a.runner.Status(1).Running {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.starting {
		return false
	}
	a.starting = true
	a.lastError = ""
	a.startup = state.Activity{
		Text:          "Starting RiftSync...",
		Operation:     "startup",
		Phase:         "queued",
		Progress:      1,
		Indeterminate: true,
		At:            float64(time.Now().UnixNano()) / float64(time.Second),
	}
	return true
}

func (a *appController) runStart(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	err := a.runner.StartWithProgress(ctx, func(activity state.Activity) {
		a.mu.Lock()
		a.startup = activity
		a.mu.Unlock()
	})
	a.mu.Lock()
	a.starting = false
	if err != nil {
		a.lastError = err.Error()
		a.startup.Text = "Startup failed: " + err.Error()
		a.startup.Phase = "error"
		a.startup.Error = true
		a.startup.Indeterminate = false
		a.startup.At = float64(time.Now().UnixNano()) / float64(time.Second)
	}
	a.mu.Unlock()
	return err
}

func (a *appController) status() statusPayload {
	a.mu.Lock()
	starting := a.starting
	startup := a.startup
	lastError := a.lastError
	options := a.options
	a.mu.Unlock()

	status := a.runner.Status(12)
	status = hydrateStoppedStatus(status, options)
	if starting || (!status.Running && startup.Operation == "startup") {
		status.Activity = startup
	}
	return makeStatusPayload(status, starting, lastError)
}

func (a *appController) setLastError(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastError = message
}

func (a *appController) configLocked() bool {
	a.mu.Lock()
	starting := a.starting
	a.mu.Unlock()
	return starting || a.runner.Status(1).Running
}

func hydrateStoppedStatus(status serverapp.Status, options serverapp.Options) serverapp.Status {
	if status.Running && status.Port > 0 && status.SyncRoot != "" && status.ConfigPath != "" {
		return status
	}
	cfg, err := config.Load(fallback(options.ConfigPath, "sync_config.json"))
	if err == nil {
		if options.HostOverride != "" {
			cfg.Host = options.HostOverride
		}
		if options.PortOverride > 0 {
			cfg.Port = options.PortOverride
		}
		if options.SyncRootOverride != "" {
			cfg.SyncRoot = options.SyncRootOverride
			_ = cfg.NormalizeAndValidate()
		}
		if options.GitVersioningOverride != nil {
			cfg.GitVersioningEnabled = *options.GitVersioningOverride
		}
		status.RemoteExecEnabled = cfg.RemoteExecEnabled
		status.RemoteExecToken = cfg.RemoteExecToken
		if !status.Running {
			status.Host = cfg.Host
			status.Port = cfg.Port
			status.SyncRoot = cfg.SyncRootAbs
			if status.SyncRoot == "" {
				status.SyncRoot = cfg.SyncRoot
			}
		} else if status.Host == "" {
			status.Host = cfg.Host
		}
		if status.Port == 0 {
			status.Port = cfg.Port
		}
		if status.SyncRoot == "" {
			status.SyncRoot = cfg.SyncRootAbs
			if status.SyncRoot == "" {
				status.SyncRoot = cfg.SyncRoot
			}
		}
		status.Git.Enabled = true
	} else {
		defaults := config.Default()
		if !status.Running {
			status.Host = fallback(options.HostOverride, defaults.Host)
			if options.PortOverride > 0 {
				status.Port = options.PortOverride
			} else {
				status.Port = defaults.Port
			}
			status.SyncRoot = fallback(options.SyncRootOverride, defaults.SyncRoot)
		} else if status.Host == "" {
			status.Host = fallback(options.HostOverride, defaults.Host)
		}
		if status.Port == 0 {
			if options.PortOverride > 0 {
				status.Port = options.PortOverride
			} else {
				status.Port = defaults.Port
			}
		}
		if status.SyncRoot == "" {
			status.SyncRoot = fallback(options.SyncRootOverride, defaults.SyncRoot)
		}
		status.Git.Enabled = true
	}
	if status.ConfigPath == "" {
		status.ConfigPath = fallback(options.ConfigPath, "sync_config.json")
	}
	status.Debug = options.Debug
	status.LegacyScan = false
	return status
}

func makeStatusPayload(status serverapp.Status, starting bool, lastError string) statusPayload {
	host := fallback(status.Host, "127.0.0.1")
	port := status.Port
	address := host
	if port > 0 {
		address = fmt.Sprintf("%s:%d", host, port)
	}
	mode := "Live sync"
	gitStatus := gitStatusText(status)
	if lastError == "" {
		lastError = status.LastError
	}
	uptime := ""
	if status.Running && !status.StartedAt.IsZero() {
		uptime = time.Since(status.StartedAt).Round(time.Second).String()
	}
	payload := statusPayload{
		Running:               status.Running,
		Starting:              starting,
		Address:               address,
		Host:                  host,
		Port:                  port,
		ConfigPath:            fallback(status.ConfigPath, "sync_config.json"),
		SyncRoot:              status.SyncRoot,
		Mode:                  mode,
		Debug:                 status.Debug,
		LegacyScan:            status.LegacyScan,
		Revision:              status.Revision,
		Indexed:               status.Counts.Entry,
		Scripts:               status.Counts.Script,
		UI:                    status.Counts.UI,
		RequestCount:          status.Metrics.RequestCount,
		Polls:                 status.Metrics.ChangesRequests,
		GitEnabled:            status.Git.Enabled,
		GitStatus:             gitStatus,
		GitLastError:          strings.TrimSpace(status.Git.LastError),
		ActivityText:          fallback(status.Activity.Text, "No Studio activity yet."),
		ActivityOp:            status.Activity.Operation,
		ActivityPhase:         status.Activity.Phase,
		ActivityError:         status.Activity.Error,
		ActivityProgress:      status.Activity.Progress,
		ActivityCurrent:       status.Activity.Current,
		ActivityTotal:         status.Activity.Total,
		ActivityIndeterminate: status.Activity.Indeterminate,
		ActivityClientID:      status.Activity.ClientID,
		ActivityRevision:      status.Activity.Revision,
		ActivityAt:            status.Activity.At,
		LastError:             strings.TrimSpace(lastError),
		Uptime:                uptime,
		RemoteExecEnabled:     status.RemoteExecEnabled,
		RemoteExecConfigured:  strings.TrimSpace(status.RemoteExecToken) != "",
		RemoteExecAvailable:   status.RemoteExecEnabled && strings.TrimSpace(status.RemoteExecToken) != "",
		RemoteExecToken:       status.RemoteExecToken,
	}
	payload.Syncing = syncingStatus(payload)
	payload.Status = statusText(payload)
	return payload
}

func gitStatusText(status serverapp.Status) string {
	if !status.Git.Enabled {
		if !status.Running {
			return "Pending, will initialize on Start"
		}
		return "Checking Git"
	}
	if !status.Running {
		return "Pending, will initialize on Start"
	}
	switch status.Git.Status {
	case state.GitStatusChecking:
		return "Checking Git"
	case state.GitStatusInitializing:
		return "Initializing repo"
	case state.GitStatusInitialCommit:
		return "Creating initial commit"
	case state.GitStatusCommitting:
		return "Committing"
	case state.GitStatusReady:
		return "Ready"
	case state.GitStatusError:
		return "Error"
	case state.GitStatusPending:
		return "Pending"
	case state.GitStatusDisabled:
		return "Disabled"
	default:
		if status.Git.LastError != "" {
			return "Error"
		}
		return "Checking Git"
	}
}

func statusText(payload statusPayload) string {
	if payload.Starting {
		return "Starting"
	}
	if payload.Syncing {
		return "Syncing"
	}
	if payload.Running {
		return "Running"
	}
	if strings.TrimSpace(payload.LastError) != "" {
		return "Error"
	}
	return "Stopped"
}

func syncingStatus(payload statusPayload) bool {
	if !payload.Running || payload.Starting || payload.ActivityError {
		return false
	}
	phase := strings.ToLower(strings.TrimSpace(payload.ActivityPhase))
	switch phase {
	case "complete", "error", "stopped", "idle":
		return false
	}
	return payload.ActivityIndeterminate || (payload.ActivityProgress > 0 && payload.ActivityProgress < 100)
}

func optionsFromInput(current serverapp.Options, configPath, syncRoot, host, portRaw string, gitEnabled, debug, legacyScan bool) (serverapp.Options, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return serverapp.Options{}, fmt.Errorf("config path must not be empty")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	if host != "127.0.0.1" && host != "localhost" {
		return serverapp.Options{}, fmt.Errorf("host must be 127.0.0.1 or localhost")
	}
	syncRoot = strings.TrimSpace(syncRoot)
	if syncRoot == "" {
		return serverapp.Options{}, fmt.Errorf("sync root must not be empty")
	}
	port, err := strconv.Atoi(strings.TrimSpace(portRaw))
	if err != nil || port < 1 || port > 65535 {
		return serverapp.Options{}, fmt.Errorf("port must be between 1 and 65535")
	}
	options := current
	options.ConfigPath = configPath
	options.HostOverride = host
	options.PortOverride = port
	options.SyncRootOverride = syncRoot
	gitAlwaysEnabled := true
	options.GitVersioningOverride = &gitAlwaysEnabled
	options.Debug = debug
	options.LegacyScan = false
	return options, nil
}

func saveConfigFromOptions(options serverapp.Options) error {
	path := fallback(options.ConfigPath, "sync_config.json")
	cfg, err := config.Load(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		cfg = config.Default()
	}
	if options.HostOverride != "" {
		cfg.Host = options.HostOverride
	}
	if options.PortOverride > 0 {
		cfg.Port = options.PortOverride
	}
	if options.SyncRootOverride != "" {
		cfg.SyncRoot = options.SyncRootOverride
	}
	cfg.GitVersioningEnabled = true
	cfg.RemoteExecEnabled = true
	if _, err := config.EnsureRemoteExecToken(&cfg); err != nil {
		return err
	}
	return config.Save(path, cfg)
}

func ensureRemoteExecConfig(options serverapp.Options) error {
	path := fallback(options.ConfigPath, "sync_config.json")
	cfg, err := config.Load(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		cfg = config.Default()
	}
	cfg.RemoteExecEnabled = true
	if _, err := config.EnsureRemoteExecToken(&cfg); err != nil {
		return err
	}
	return config.Save(path, cfg)
}

func normalizeUIOptions(options serverapp.Options) serverapp.Options {
	if options.ConfigPath == "" {
		options.ConfigPath = "sync_config.json"
	}
	if options.PortOverride == 0 {
		options.PortOverride = -1
	}
	return options
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func portText(port int) string {
	if port < 1 {
		return ""
	}
	return strconv.Itoa(port)
}

func formatWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return "No warnings."
	}
	return strings.Join(warnings, "\n")
}

func formatEvents(status serverapp.Status) string {
	if len(status.Events) == 0 {
		return "No events yet."
	}
	lines := make([]string, 0, len(status.Events))
	for _, event := range status.Events {
		lines = append(lines, fmt.Sprintf("%s %s", event.Kind, event.Message))
	}
	return strings.Join(lines, "\n")
}

func formatHistory(history []state.RevisionSummary) string {
	if len(history) == 0 {
		return "No revisions yet."
	}
	lines := make([]string, 0, len(history))
	for _, revision := range history {
		lines = append(lines, fmt.Sprintf("rev %d - %d changes%s%s",
			revision.Rev,
			revision.ChangeCount,
			formatCounts(revision.OpCounts),
			formatGitShort(revision.GitCommitShort),
		))
	}
	return strings.Join(lines, "\n")
}

func formatCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", key, counts[key]))
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func formatGitShort(short string) string {
	if short == "" {
		return ""
	}
	return " [" + short + "]"
}

func legacyHTMLDocument() string {
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>RiftSync</title>
<style>
:root {
  color-scheme: dark;
  --bg: #090d18;
  --ink: #f4f7fb;
  --muted: #aab3c8;
  --panel: rgba(22, 27, 45, .68);
  --panel-strong: rgba(31, 38, 62, .78);
  --surface: rgba(6, 10, 24, .42);
  --line: rgba(210, 220, 255, .16);
  --accent: #9aa8ff;
  --accent-2: #7ee7d6;
  --good: #4ade80;
  --bad: #fb7185;
  --warn: #fbbf24;
}
* { box-sizing: border-box; }
html, body, .app { width: 100%; height: 100vh; overflow: hidden; }
body {
  margin: 0;
  background:
    radial-gradient(circle at 8% 0%, rgba(154,168,255,.20), transparent 22rem),
    radial-gradient(circle at 92% 10%, rgba(126,231,214,.14), transparent 24rem),
    radial-gradient(circle at 50% 110%, rgba(251,113,133,.10), transparent 30rem),
    var(--bg);
  color: var(--ink);
  font: 14px/1.35 "Segoe UI", Inter, system-ui, sans-serif;
}
button, input, textarea { font: inherit; }
button {
  border: 1px solid var(--line);
  color: var(--ink);
  background: var(--panel-strong);
  padding: 8px 12px;
  border-radius: 9px;
  cursor: pointer;
  min-width: 72px;
}
button.primary { background: linear-gradient(135deg, #7c8cff, #71ded0); color: #08101f; border-color: transparent; font-weight: 760; }
button.danger { border-color: rgba(251,113,133,.45); color: #ffd7df; }
button.icon { min-width: 34px; width: 34px; height: 30px; padding: 0; }
button.ghost { background: rgba(22,27,45,.52); }
button:hover { border-color: var(--accent-2); }
button:disabled { opacity: .45; cursor: not-allowed; }
button:focus-visible, input:focus-visible, textarea:focus-visible, .toggle-switch input:focus-visible + .switch-track, .custom-select-button:focus-visible { outline: 2px solid var(--accent-2); outline-offset: 2px; }
.app { display: grid; grid-template-rows: 64px 44px 1fr; }
.app-header {
  display: grid;
  grid-template-columns: minmax(190px, .9fr) auto minmax(360px, 1fr);
  align-items: center;
  gap: 14px;
  padding: 10px 18px 8px;
  border-bottom: 1px solid var(--line);
  background: rgba(11,15,28,.74);
  backdrop-filter: blur(18px);
  user-select: none;
}
.brand-title { display: inline-flex; align-items: center; gap: 8px; min-width: 0; }
.brand-icon { width: 24px; height: 24px; flex: 0 0 24px; display: block; filter: drop-shadow(0 5px 12px rgba(49,147,255,.28)); }
.brand h1 { margin: 0; font-size: 22px; letter-spacing: 0; line-height: 1; }
.brand .sub { color: var(--muted); font-size: 12px; margin-top: 1px; }
.header-status { display: flex; align-items: center; justify-content: center; gap: 11px; color: var(--muted); white-space: nowrap; }
.pill { display: inline-flex; align-items: center; gap: 8px; padding: 7px 10px; border: 1px solid var(--line); border-radius: 999px; background: var(--panel); color: var(--ink); font-weight: 750; }
.dot { width: 9px; height: 9px; border-radius: 50%; background: var(--bad); box-shadow: 0 0 0 4px rgba(251,113,133,.13); }
.running .dot { background: var(--good); box-shadow: 0 0 0 4px rgba(74,222,128,.13); }
.starting .dot { background: var(--warn); box-shadow: 0 0 0 4px rgba(251,191,36,.13); }
.header-actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; }
.window-actions { display: inline-flex; gap: 7px; margin-left: 2px; }
.tabbar { display: flex; align-items: end; gap: 8px; padding: 7px 18px 0; background: rgba(11,15,28,.44); backdrop-filter: blur(16px); }
.tab { min-width: 120px; height: 37px; border-bottom-left-radius: 0; border-bottom-right-radius: 0; background: rgba(22,27,45,.58); color: var(--muted); }
.tab.active { color: var(--ink); border-color: var(--accent-2); background: var(--panel-strong); }
.content { min-height: 0; padding: 14px 18px 18px; overflow: hidden; }
.tab-panel { display: none; height: 100%; min-height: 0; overflow: hidden; }
.tab-panel.active { display: block; }
.stack { display: grid; grid-template-rows: auto auto auto auto; gap: 12px; height: 100%; min-height: 0; }
.metric-strip { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.metric, .mini-stat { min-height: 76px; padding: 13px 14px; background: linear-gradient(145deg, rgba(255,255,255,.075), rgba(255,255,255,.025)); border: 1px solid var(--line); border-radius: 13px; backdrop-filter: blur(14px); }
.label { color: var(--muted); font-size: 11px; font-weight: 760; text-transform: uppercase; letter-spacing: .05em; }
.value { margin-top: 7px; font-size: 19px; font-weight: 820; word-break: break-word; }
.hint { color: var(--muted); font-size: 12px; margin-top: 4px; word-break: break-word; }
.panel { background: linear-gradient(145deg, rgba(255,255,255,.07), rgba(255,255,255,.025)); border: 1px solid var(--line); border-radius: 14px; padding: 15px; min-height: 0; overflow: hidden; box-shadow: 0 18px 45px rgba(0,0,0,.22); backdrop-filter: blur(18px); }
.panel h2 { margin: 0 0 12px; font-size: 16px; }
.overview-middle { display: grid; grid-template-columns: .9fr 1.1fr; gap: 12px; min-height: 0; }
.project-card { display: grid; grid-template-rows: auto auto auto auto; gap: 6px; }
.project-name { margin-top: 8px; font-size: 22px; font-weight: 820; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.row-actions { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; margin-top: 12px; }
.health-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 10px; }
.health-row { min-height: 64px; border: 1px solid var(--line); border-radius: 11px; background: rgba(4,8,20,.42); padding: 10px 11px; }
.status-band { display: grid; grid-template-columns: 120px 1fr; gap: 12px; align-items: center; min-height: 56px; padding: 12px 14px; border: 1px solid var(--line); border-radius: 13px; background: var(--panel); }
.error { color: #ffd6de; overflow-wrap: anywhere; }
.error-row { display: grid; grid-template-columns: 1fr auto; align-items: center; gap: 10px; min-width: 0; }
.error-summary { color: #ffd6de; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.error-summary.clear { color: var(--muted); }
.details-button { min-width: 72px; padding: 7px 10px; }
.activity-card { min-width: 0; }
.activity-bar { height: 7px; margin-top: 8px; border-radius: 999px; background: rgba(4,8,20,.7); border: 1px solid var(--line); overflow: hidden; }
.activity-fill { width: 0%; height: 100%; border-radius: inherit; background: linear-gradient(90deg, var(--accent), var(--accent-2)); transition: width .18s ease; }
.activity-card.busy .activity-fill { width: 42%; animation: activity-slide 1.05s ease-in-out infinite; }
@keyframes activity-slide {
  0% { transform: translateX(-110%); }
  100% { transform: translateX(250%); }
}
.modal-backdrop { position: fixed; inset: 0; z-index: 500; display: none; place-items: center; background: rgba(2,6,18,.72); padding: 22px; }
.modal-backdrop.open { display: grid; }
.modal { width: min(860px, 100%); max-height: min(620px, 88vh); display: grid; grid-template-rows: auto 1fr; gap: 12px; border: 1px solid var(--line); border-radius: 14px; background: #10162a; box-shadow: 0 24px 70px rgba(0,0,0,.48); padding: 15px; }
.modal-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.modal-actions { display: flex; gap: 8px; }
.error-detail { min-height: 220px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; border: 1px solid var(--line); border-radius: 11px; background: rgba(4,8,20,.72); color: var(--ink); padding: 12px; font-family: Consolas, "Cascadia Mono", monospace; font-size: 12px; }
.history-shell { display: grid; grid-template-rows: auto 1fr; gap: 12px; height: 100%; min-height: 0; }
.history-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.field label { display: block; color: var(--muted); font-size: 11px; font-weight: 760; text-transform: uppercase; margin-bottom: 6px; }
input, textarea { width: 100%; border: 1px solid var(--line); border-radius: 10px; background: rgba(4, 8, 20, .62); color: var(--ink); padding: 9px 11px; min-height: 39px; }
textarea { resize: none; font-family: Consolas, "Cascadia Mono", monospace; font-size: 12px; line-height: 1.45; }
.input-row { display: grid; grid-template-columns: 1fr auto; gap: 9px; align-items: end; }
.custom-select { position: relative; }
.custom-select-button { width: 100%; min-height: 39px; text-align: left; display: flex; align-items: center; justify-content: space-between; gap: 10px; background: rgba(4,8,20,.62); }
.custom-select-button::after { content: "⌄"; color: var(--accent-2); font-size: 15px; }
.history-toolbar-panel { overflow: visible; position: relative; z-index: 20; }
.custom-menu { display: none; position: absolute; z-index: 100; left: 0; right: 0; top: calc(100% + 6px); max-height: 220px; overflow: auto; border: 1px solid var(--accent-2); border-radius: 11px; background: #11182b; box-shadow: 0 18px 40px rgba(0,0,0,.34); padding: 6px; }
.custom-menu.open { display: block; }
.custom-option { width: 100%; display: block; text-align: left; border: 0; border-radius: 8px; background: transparent; color: var(--ink); padding: 9px 10px; }
.custom-option:hover, .custom-option.active { background: rgba(126,231,214,.13); }
.history-layout { display: grid; grid-template-columns: minmax(220px, .38fr) 1fr; gap: 12px; min-height: 0; height: 100%; }
.history-timeline { min-height: 0; overflow: auto; border: 1px solid var(--line); border-radius: 12px; background: var(--surface); padding: 8px; }
.history-item { width: 100%; display: grid; grid-template-columns: auto 1fr; gap: 10px; align-items: center; text-align: left; border: 0; border-radius: 10px; background: transparent; padding: 10px; margin-bottom: 6px; }
.history-item.active, .history-item:hover { background: rgba(126,231,214,.12); }
.history-dot { width: 9px; height: 9px; border-radius: 50%; background: var(--accent-2); box-shadow: 0 0 0 4px rgba(126,231,214,.10); }
.history-item-title { font-weight: 760; }
.history-item-meta { color: var(--muted); font-size: 12px; margin-top: 2px; }
.history-detail-card { display: grid; grid-template-rows: auto 1fr; min-height: 0; }
.revision-summary { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; margin-bottom: 12px; }
.history-detail { min-height: 0; overflow: auto; border: 1px solid var(--line); border-radius: 12px; background: var(--surface); padding: 11px; }
.change-row { border-bottom: 1px solid var(--line); padding: 10px 2px; }
.change-row:last-child { border-bottom: 0; }
.change-row strong { font-size: 13px; }
.change-meta { color: var(--muted); font-size: 12px; word-break: break-word; margin-top: 3px; }
.exec-shell { display: grid; grid-template-columns: minmax(320px, .9fr) minmax(360px, 1.1fr); gap: 12px; height: 100%; min-height: 0; }
.exec-editor-card { display: grid; grid-template-rows: auto auto 1fr auto; gap: 10px; min-height: 0; }
.exec-output-card { display: grid; grid-template-rows: auto 1fr auto; gap: 10px; min-height: 0; }
.exec-source { min-height: 0; height: 100%; }
.exec-token-row { display: grid; grid-template-columns: 1fr auto; gap: 9px; align-items: end; }
.exec-toolbar { display: grid; grid-template-columns: 120px 1fr auto auto; gap: 9px; align-items: end; }
.exec-output { min-height: 0; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; border: 1px solid var(--line); border-radius: 12px; background: rgba(2,5,14,.82); color: #d7ffef; padding: 12px; font-family: Consolas, "Cascadia Mono", monospace; font-size: 12px; }
.exec-history { min-height: 120px; max-height: 210px; overflow: auto; border: 1px solid var(--line); border-radius: 12px; background: var(--surface); padding: 8px; }
.exec-history-item { width: 100%; display: grid; grid-template-columns: 1fr auto; gap: 8px; align-items: center; text-align: left; border: 0; border-radius: 10px; background: transparent; padding: 9px; margin-bottom: 6px; }
.exec-history-item:hover { background: rgba(126,231,214,.12); }
.exec-history-title { font-weight: 760; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.exec-history-meta { color: var(--muted); font-size: 12px; margin-top: 2px; }
.empty { min-height: 150px; display: grid; place-items: center; color: var(--muted); text-align: center; }
.config-shell { display: grid; grid-template-rows: 1fr auto; gap: 12px; height: 100%; min-height: 0; }
.config-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; min-height: 0; }
.form-grid { display: grid; grid-template-columns: 1fr 130px; gap: 12px; }
.full { grid-column: 1 / -1; }
.switch-list { display: grid; gap: 10px; margin-top: 12px; }
.switch-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; border: 1px solid var(--line); border-radius: 11px; background: var(--surface); padding: 10px 11px; }
.switch-copy strong { display: block; }
.switch-copy span { color: var(--muted); font-size: 12px; }
.toggle-switch { position: relative; display: inline-flex; align-items: center; gap: 8px; }
.toggle-switch input { position: absolute; opacity: 0; pointer-events: none; }
.switch-track { width: 48px; height: 26px; border-radius: 999px; border: 1px solid var(--line); background: rgba(4,8,20,.7); position: relative; transition: .16s ease; }
.switch-track::before { content: ""; position: absolute; width: 20px; height: 20px; left: 2px; top: 2px; border-radius: 50%; background: var(--muted); transition: .16s ease; }
.toggle-switch input:checked + .switch-track { background: rgba(126,231,214,.22); border-color: var(--accent-2); }
.toggle-switch input:checked + .switch-track::before { transform: translateX(22px); background: var(--accent-2); }
.switch-state { min-width: 58px; color: var(--muted); font-size: 12px; font-weight: 760; text-align: right; }
.action-band { display: flex; align-items: center; justify-content: space-between; gap: 12px; border: 1px solid var(--line); border-radius: 13px; background: var(--panel); padding: 12px; }
.terminal-detail { min-height: 320px; max-height: 68vh; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; border: 1px solid var(--line); border-radius: 11px; background: rgba(2,5,14,.82); color: #d7ffef; padding: 12px; font-family: Consolas, "Cascadia Mono", monospace; font-size: 12px; }
@media (max-width: 900px) {
  .app-header { grid-template-columns: 1fr auto; }
  .header-status { justify-content: flex-start; }
  .header-actions { grid-column: 1 / -1; justify-content: flex-start; }
  .metric-strip, .overview-middle, .health-grid, .revision-summary, .config-grid, .form-grid, .history-layout, .exec-shell, .exec-toolbar { grid-template-columns: 1fr; }
}
</style>
</head>
<body>
<main class="app">
  <header id="appHeader" class="app-header">
    <div class="brand"><div class="brand-title"><svg class="brand-icon" viewBox="0 0 512 512" aria-hidden="true" focusable="false"><defs><linearGradient id="brandIconGradient" x1="76" y1="96" x2="432" y2="430" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#22d3ee"/><stop offset=".55" stop-color="#3b82f6"/><stop offset="1" stop-color="#7c3aed"/></linearGradient></defs><path fill="url(#brandIconGradient)" d="M73 172c0-53 43-96 96-96h76c70 0 127 57 127 127 0 47-26 89-66 111l78 91h-84l-97-113h43c49 0 89-40 89-89s-40-89-89-89h-77c-32 0-58 26-58 58H73Zm32 83h139c31 0 56-25 56-56h-52c0 2-2 4-4 4H105c-18 0-32 14-32 32s14 32 32 32Zm235-146c28-21 63-33 100-33 22 0 40 18 40 40v100c0 21-25 32-41 18l-50-44c-21-18-53-20-76-4-6-29-20-55-40-77h67Zm-22 229c24 17 57 14 78-7l18-18c14-14 37-14 51 0s14 37 0 51l-18 18c-52 52-136 52-188 0l-25-25 84-19Z"/></svg><h1>RiftSync</h1></div><div class="sub">Live sync for Roblox Studio</div></div>
    <div class="header-status"><span id="statusPill" class="pill" role="status" aria-live="polite"><span class="dot"></span><span id="statusText">Stopped</span></span><span id="addressText">127.0.0.1:8765</span></div>
    <div class="header-actions">
      <button id="logsBtn" class="ghost">Logs</button><button id="startBtn" class="primary">Start</button><button id="stopBtn">Stop</button>
      <span class="window-actions"><button id="minimizeBtn" class="icon" title="Minimize">_</button><button id="closeBtn" class="icon danger" title="Close">x</button></span>
    </div>
  </header>
  <nav class="tabbar"><button id="overviewTab" class="tab active" data-tab="overviewPanel">Overview</button><button id="historyTab" class="tab" data-tab="historyPanel">History</button><button id="execTab" class="tab" data-tab="execPanel">Exec</button><button id="configTab" class="tab" data-tab="configPanel">Config</button></nav>
  <section class="content">
    <section id="overviewPanel" class="tab-panel active">
      <div class="stack">
        <div id="metricStrip" class="metric-strip">
          <div class="metric"><div class="label">Sync</div><div id="overviewStatusText" class="value">Stopped</div><div id="uptimeText" class="hint"></div></div>
          <div class="metric"><div class="label">Revision</div><div id="revisionText" class="value">0</div><div id="modeText" class="hint">Live sync</div></div>
          <div class="metric"><div class="label">Studio</div><div id="indexedText" class="value">0 items</div><div id="indexedHint" class="hint">Waiting for project</div></div>
          <div class="metric"><div class="label">Git</div><div id="gitText" class="value">Pending</div><div class="hint">version history</div></div>
        </div>
        <div class="overview-middle">
          <article id="workspacePanel" class="panel project-card">
            <h2>Project</h2><div class="label">Active folder</div><div id="projectNameText" class="project-name">game</div><div class="hint">Folder path is managed in Config.</div>
            <div class="row-actions"><button id="openBtn">Open Folder</button><button id="changePathBtn">Edit Path</button></div>
          </article>
          <article id="healthPanel" class="panel">
            <h2>Live status</h2>
            <div class="health-grid">
              <div class="health-row"><div class="label">Studio checks</div><div id="pollsText" class="value">0</div><div class="hint">updates watched</div></div>
              <div class="health-row"><div class="label">App requests</div><div id="requestsText" class="value">0</div><div class="hint">local only</div></div>
              <div class="health-row"><div class="label">Sync mode</div><div id="healthModeText" class="value">Live sync</div><div class="hint">watch + safety scan</div></div>
              <div class="health-row"><div class="label">Last scan</div><div id="lastScanText" class="value">Ready</div><div class="hint">background</div></div>
            </div>
          </article>
        </div>
        <div class="status-band"><div class="label">Last error</div><div class="error-row"><div id="errorText" class="error-summary clear">Clear</div><button id="errorDetailsBtn" class="details-button ghost" type="button" disabled>Details</button></div></div>
        <div class="status-band"><div class="label">Studio activity</div><div class="activity-card" role="status" aria-live="polite" aria-busy="false"><div id="activityText" class="error">No Studio activity yet.</div><div id="activityHint" class="hint"></div><div class="activity-bar" role="progressbar" aria-label="Sync activity"><div id="activityFill" class="activity-fill"></div></div></div></div>
      </div>
    </section>
    <section id="historyPanel" class="tab-panel">
      <div class="history-shell">
        <div class="panel history-toolbar-panel">
          <div class="history-toolbar">
            <div><div class="label">Revision timeline</div><div class="hint">Showing retained project history.</div></div>
            <button id="historyRefreshBtn">Refresh</button>
          </div>
        </div>
        <div class="history-layout">
          <div id="historyTimeline" class="history-timeline"><div class="empty">History will appear after file changes publish revisions.</div></div>
          <article class="panel history-detail-card">
            <div id="revisionSummary" class="revision-summary"><div class="mini-stat"><div class="label">Revision</div><div class="value">-</div></div><div class="mini-stat"><div class="label">Changes</div><div class="value">0</div></div><div class="mini-stat"><div class="label">Ops</div><div class="hint">-</div></div><div class="mini-stat"><div class="label">Git</div><div class="hint">-</div></div></div>
            <div id="historyDetail" class="history-detail"><div class="empty">Choose a revision.</div></div>
          </article>
        </div>
      </div>
    </section>
    <section id="execPanel" class="tab-panel">
      <div class="exec-shell">
        <article class="panel exec-editor-card">
          <div><h2>Remote Exec</h2><div class="hint">Runs trusted Luau in Studio when sync and Exec ON are active.</div></div>
          <div class="field"><label for="execTokenDisplay">Exec Token</label><div class="exec-token-row"><input id="execTokenDisplay" spellcheck="false" readonly value=""><button id="execTokenCopyBtn" type="button">Copy</button></div><div id="execTokenHint" class="hint">Token is generated from sync_config.json.</div></div>
          <textarea id="execSourceInput" class="exec-source" spellcheck="false" placeholder="print(workspace.Name)"></textarea>
          <div class="exec-toolbar">
            <div class="field"><label for="execTimeoutInput">Timeout</label><input id="execTimeoutInput" type="number" min="1" max="120" value="10"></div>
            <div id="execStatusText" class="hint">Start RiftSync, connect Studio, then enable Exec ON.</div>
            <button id="execLatestBtn" type="button">Latest</button>
            <button id="execRunBtn" class="primary" type="button">Run</button>
          </div>
        </article>
        <article class="panel exec-output-card">
          <div><h2>Result</h2><div id="execResultHint" class="hint">Output will appear after a command finishes.</div></div>
          <pre id="execOutputText" class="exec-output">Ready.</pre>
          <div>
            <div class="history-toolbar"><div><div class="label">Recent commands</div><div id="execHistoryHint" class="hint">Stored in .rblxsync/exec-history.json</div></div><button id="execHistoryRefreshBtn" type="button">Refresh</button></div>
            <div id="execHistoryList" class="exec-history"><div class="empty">No Remote Exec history yet.</div></div>
          </div>
        </article>
      </div>
    </section>
    <section id="configPanel" class="tab-panel">
      <div class="config-shell">
        <div class="config-grid">
          <article id="configPathPanel" class="panel">
            <h2>Path Settings</h2>
            <div class="field"><label for="syncRootInput">Sync root</label><div class="input-row"><input id="syncRootInput" spellcheck="false"><button id="syncRootPickerBtn" type="button">Choose</button></div></div>
            <div class="field"><label for="configInput">Config path</label><input id="configInput" spellcheck="false"></div>
            <div class="hint">This is the only place to change the local folder used by pull/sync.</div>
          </article>
          <article id="serverSettingsPanel" class="panel">
            <h2>Server Settings</h2>
            <div class="form-grid"><div class="field"><label for="hostInput">Host</label><input id="hostInput" spellcheck="false"></div><div class="field"><label for="portInput">Port</label><input id="portInput" inputmode="numeric"></div></div>
            <div class="switch-list">
              <label class="switch-row"><span class="switch-copy"><strong>Debug</strong><span>Extra startup and runtime details.</span></span><span class="toggle-switch"><input id="debugInput" type="checkbox"><span class="switch-track"></span><span id="debugStateText" class="switch-state">Off</span></span></label>
            </div>
          </article>
        </div>
        <div class="action-band"><div><div class="hint">Host is local-only: 127.0.0.1 or localhost.</div><div id="configApplyStatus" class="hint">Saved</div></div><div class="row-actions" style="margin-top:0"><button id="configOpenBtn">Open Folder</button></div></div>
      </div>
    </section>
  </section>
</main>
<div id="errorModal" class="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="errorModalTitle">
  <div class="modal">
    <div class="modal-head"><div><div class="label">Last error</div><h2 id="errorModalTitle">Error details</h2></div><div class="modal-actions"><button id="copyErrorBtn" type="button">Copy</button><button id="closeErrorBtn" class="danger" type="button">Close</button></div></div>
    <pre id="errorDetailText" class="error-detail">Clear</pre>
  </div>
</div>
<div id="logsModal" class="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="logsModalTitle">
  <div class="modal">
    <div class="modal-head"><div><div class="label">Activity log</div><h2 id="logsModalTitle">Local terminal</h2></div><div class="modal-actions"><button id="refreshLogsBtn" type="button">Refresh</button><button id="closeLogsBtn" class="danger" type="button">Close</button></div></div>
    <pre id="logsText" class="terminal-detail">Logs will appear here.</pre>
  </div>
</div>
<script>
const $ = (id) => document.getElementById(id);
let latest = null;
let busy = false;
let selectedHistoryRev = null;
let historyRevisions = [];
let configDirty = false;
let applyingConfig = false;
let lastFullError = "";
let refreshTimer = null;
let logsOpen = false;
let logsTimer = null;
let execHistory = [];
const configControlIds = ["configInput","syncRootInput","hostInput","portInput","syncRootPickerBtn","debugInput"];
function setBusy(value) { busy = value; ["startBtn","stopBtn","openBtn","configOpenBtn"].forEach(id => $(id).disabled = value); setConfigLocked(value || (latest && (latest.running || latest.starting))); }
function setConfigLocked(value) { configControlIds.forEach(id => { const node = $(id); if (node) node.disabled = !!value; }); }
async function api(path, options) { const response = await fetch(path, options || {}); const body = await response.json(); if (!response.ok || body.status === "error") throw new Error(body.error || "request failed"); return body; }
function setTab(panelId) { document.querySelectorAll(".tab").forEach((tab) => tab.classList.toggle("active", tab.dataset.tab === panelId)); document.querySelectorAll(".tab-panel").forEach((panel) => panel.classList.toggle("active", panel.id === panelId)); if (panelId === "historyPanel") loadHistoryList(false); if (panelId === "execPanel") loadExecHistory(); }
function folderName(path) { const clean = String(path || "src/game").replace(/[\\/]+$/, ""); const parts = clean.split(/[\\/]/).filter(Boolean); return parts[parts.length - 1] || clean || "game"; }
function updateSwitchText() { [["debugInput","debugStateText"]].forEach(([input,text]) => { $(text).textContent = $(input).checked ? "On" : "Off"; }); }
function errorSummary(value) {
  const text = String(value || "");
  if (!text || text === "Clear") return "Clear";
  const lower = text.toLowerCase();
  if (lower.includes("filename too long")) return "Git path terlalu panjang. RiftSync akan memakai core.longpaths untuk repo ini.";
  if (lower.includes("lf will be replaced by crlf")) return "Git line-ending warning. Detail lengkap tersedia.";
  return text.split(/\r?\n/).find(Boolean) || text;
}
function showError(message) {
  lastFullError = String(message || "");
  const hasError = lastFullError !== "" && lastFullError !== "Clear";
  const summary = errorSummary(lastFullError);
  $("errorText").textContent = summary;
  $("errorText").title = hasError ? lastFullError : "";
  $("errorText").classList.toggle("clear", !hasError);
  $("errorDetailsBtn").disabled = !hasError;
  $("errorDetailText").textContent = hasError ? lastFullError : "Clear";
}
function showErrorModal() { if (lastFullError && lastFullError !== "Clear") $("errorModal").classList.add("open"); }
function hideErrorModal() { $("errorModal").classList.remove("open"); }
function setActivityProgress(value, active) {
  const progress = Math.max(0, Math.min(100, Number(value) || 0));
  const card = $("activityText").parentElement;
  card.classList.remove("busy");
  card.setAttribute("aria-busy", "false");
  $("activityFill").style.width = active ? progress + "%" : "0%";
  $("activityFill").style.transform = "";
}
function setStartupProgress(active) {
  const card = $("activityText").parentElement;
  card.classList.toggle("busy", !!active);
  card.setAttribute("aria-busy", active ? "true" : "false");
  if (active) {
    $("activityFill").style.width = "42%";
  }
}
function render(app) {
  latest = app;
  $("statusText").textContent = app.status; $("overviewStatusText").textContent = app.status; $("statusPill").className = "pill " + (app.running ? "running" : app.starting ? "starting" : "");
  $("addressText").textContent = app.address; $("uptimeText").textContent = app.uptime ? "Up " + app.uptime : ""; $("revisionText").textContent = app.revision; $("modeText").textContent = app.mode; $("healthModeText").textContent = app.mode;
  $("indexedText").textContent = app.indexed + " items"; $("indexedHint").textContent = app.scripts + " scripts · " + app.ui + " UI"; $("projectNameText").textContent = folderName(app.sync_root);
  if (!configDirty) {
    $("configInput").value = app.config_path || "sync_config.json"; $("syncRootInput").value = app.sync_root || "src/game"; $("hostInput").value = app.host || "127.0.0.1"; $("portInput").value = app.port || 8765;
    $("debugInput").checked = !!app.debug; updateSwitchText();
  }
  $("requestsText").textContent = app.request_count || 0; $("pollsText").textContent = app.polls || 0; $("gitText").textContent = app.git_status || "Pending"; $("lastScanText").textContent = app.indexed ? app.indexed + " items" : "Ready"; showError(app.last_error || "Clear");
  $("execTokenDisplay").value = app.remote_exec_token || "";
  $("execTokenCopyBtn").disabled = !(app.remote_exec_token || "").trim();
  if (app.remote_exec_available) $("execTokenHint").textContent = "Copy this token into the Studio plugin Exec Token field.";
  else if (app.remote_exec_configured) $("execTokenHint").textContent = "Token configured, but Remote Exec is disabled.";
  else $("execTokenHint").textContent = "Token will appear after config is prepared.";
  $("activityText").textContent = app.starting ? "Starting server and scanning project files..." : (app.activity_text || "No Studio activity yet."); $("activityText").style.color = app.activity_error ? "var(--bad)" : "var(--ink)";
  const activityBits = []; if (app.activity_operation) activityBits.push(app.activity_operation); if (app.activity_progress) activityBits.push(app.activity_progress + "%"); if (app.activity_revision) activityBits.push("rev " + app.activity_revision);
  $("activityHint").textContent = app.starting ? "Preparing watcher, Git, and local index" : activityBits.join(" · ");
  if (app.starting) setStartupProgress(true); else setActivityProgress(app.activity_progress, !!app.activity_progress && !app.activity_error);
  $("startBtn").disabled = busy || app.running || app.starting; $("stopBtn").disabled = busy || !app.running;
  setConfigLocked(busy || app.running || app.starting);
  if ((app.running || app.starting) && !configDirty && !applyingConfig) setConfigStatus("Stop sync before editing config.", "active");
  else if (!configDirty && !applyingConfig) setConfigStatus("Saved", "ok");
}
function gitSwitchStatus(value) { const text = String(value || "Disabled"); if (text.indexOf("Pending") === 0) return "Pending"; if (text === "Initializing repo") return "Init"; if (text === "Creating initial commit") return "Commit"; if (text === "Checking Git") return "Check"; return text; }
async function refresh() { try { const body = await api("/app/status"); render(body.app); } catch (err) { showError(err.message); } }
function nextRefreshDelay() {
  if (!latest) return 700;
  const activityRecent = latest.activity_at && ((Date.now() / 1000) - latest.activity_at) < 8;
  const activityBusy = latest.activity_progress > 0 && latest.activity_progress < 100;
  return (latest.running || latest.starting || activityRecent || activityBusy) ? 400 : 1500;
}
async function refreshLoop() {
  await refresh();
  refreshTimer = setTimeout(refreshLoop, nextRefreshDelay());
}
function escapeHtml(value) { return String(value ?? "").replace(/[&<>"']/g, (char) => ({"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#039;"}[char])); }
function countsText(counts) { if (!counts) return ""; return Object.keys(counts).sort().map((key) => key + ":" + counts[key]).join(" · "); }
function detailSummary(change) { if (change.source_summary) return (change.source_summary.line_count || 0) + " lines · " + (change.source_summary.char_count || 0) + " chars"; if (change.payload_summary) { const p = change.payload_summary; return "props=" + (p.property_count || 0) + " attrs=" + (p.attribute_count || 0) + " tags=" + (p.tag_count || 0); } return ""; }
function setRevisionSummary(body, changes) { $("revisionSummary").innerHTML = '<div class="mini-stat"><div class="label">Revision</div><div class="value">' + escapeHtml(body.rev || "-") + '</div></div><div class="mini-stat"><div class="label">Changes</div><div class="value">' + escapeHtml(body.change_count || changes.length || 0) + '</div></div><div class="mini-stat"><div class="label">Ops</div><div class="hint">' + escapeHtml(countsText(body.op_counts) || "-") + '</div></div><div class="mini-stat"><div class="label">Git</div><div class="hint">' + escapeHtml(body.git_commit_short || "-") + '</div></div>'; }
function renderHistoryTimeline() {
  const timeline = $("historyTimeline");
  if (historyRevisions.length === 0) { timeline.innerHTML = '<div class="empty">History will appear after file changes publish revisions.</div>'; return; }
  timeline.innerHTML = historyRevisions.map((rev) => '<button type="button" class="history-item' + (String(rev.rev) === String(selectedHistoryRev) ? ' active' : '') + '" data-rev="' + escapeHtml(rev.rev) + '"><span class="history-dot"></span><span><span class="history-item-title">Rev ' + escapeHtml(rev.rev) + '</span><span class="history-item-meta">' + escapeHtml(rev.change_count || 0) + ' changes' + (rev.git_commit_short ? ' · ' + escapeHtml(rev.git_commit_short) : '') + '</span></span></button>').join("");
  timeline.querySelectorAll(".history-item").forEach((item) => item.onclick = () => loadHistoryDetail(item.dataset.rev));
}
async function loadHistoryList(selectLatest) {
  try {
    const body = await api("/app/history");
    historyRevisions = body.revisions || []; renderHistoryTimeline();
    const selectedStillExists = historyRevisions.some((revision) => String(revision.rev) === String(selectedHistoryRev));
    if ((selectLatest || !selectedHistoryRev || !selectedStillExists) && historyRevisions[0]) loadHistoryDetail(historyRevisions[0].rev);
    if (historyRevisions.length === 0 && !selectedHistoryRev) { setRevisionSummary({ rev: "-", change_count: 0, op_counts: {}, git_commit_short: "-" }, []); $("historyDetail").innerHTML = '<div class="empty">History will appear after file changes publish revisions.</div>'; }
  } catch (err) { historyRevisions = []; $("historyTimeline").innerHTML = '<div class="empty">' + escapeHtml(err.message) + '</div>'; }
}
async function loadHistoryDetail(revision) {
  const rev = parseInt(revision, 10); if (!rev) { $("historyDetail").innerHTML = '<div class="empty">Choose a revision first.</div>'; return; }
  selectedHistoryRev = rev;
  try {
    const body = await api("/app/history?rev=" + encodeURIComponent(rev));
    if (!body.found) { $("historyDetail").innerHTML = '<div class="empty">Revision ' + escapeHtml(rev) + ' was not found.</div>'; return; }
    const changes = body.changes || []; setRevisionSummary(body, changes);
    $("historyDetail").innerHTML = changes.length === 0 ? '<div class="empty">No changes in this revision.</div>' : changes.map((change) => {
      const op = change.op || "upsert"; const entity = change.entity || "-"; const className = change.class_name ? " · " + change.class_name : ""; const mainPath = change.rbx_path || change.new_rbx_path || change.old_rbx_path || change.local_path || "-";
      const rename = change.old_rbx_path || change.new_rbx_path ? '<div class="change-meta">' + escapeHtml(change.old_rbx_path || "-") + ' -> ' + escapeHtml(change.new_rbx_path || change.rbx_path || "-") + '</div>' : ""; const summary = detailSummary(change);
      return '<div class="change-row"><strong>' + escapeHtml(op) + ' · ' + escapeHtml(entity) + escapeHtml(className) + '</strong><div class="change-meta">' + escapeHtml(mainPath) + '</div><div class="change-meta">' + escapeHtml(change.local_path || "") + '</div>' + rename + (summary ? '<div class="change-meta">' + escapeHtml(summary) + '</div>' : "") + '</div>';
    }).join("");
    renderHistoryTimeline();
  } catch (err) { $("historyDetail").innerHTML = '<div class="empty">' + escapeHtml(err.message) + '</div>'; }
}
function formatExecEntry(entry) {
  const summary = entry.source_summary || {};
  const title = summary.first_line || String(entry.source || "").split(/\r?\n/).find(Boolean) || "(empty)";
  const status = (entry.ok ? "ok" : (entry.result_state || "error")) + " · " + (entry.timeout_sec || 0) + "s";
  return { title, status };
}
function renderExecHistory() {
  const list = $("execHistoryList");
  if (!execHistory.length) { list.innerHTML = '<div class="empty">No Remote Exec history yet.</div>'; return; }
  list.innerHTML = execHistory.slice(0, 20).map((entry) => {
    const item = formatExecEntry(entry);
    return '<div class="exec-history-item"><div><div class="exec-history-title">' + escapeHtml(item.title) + '</div><div class="exec-history-meta">' + escapeHtml(entry.submitted_by || "-") + ' · ' + escapeHtml(item.status) + '</div></div><button type="button" data-exec-id="' + escapeHtml(entry.id || "") + '">Run</button></div>';
  }).join("");
  list.querySelectorAll("[data-exec-id]").forEach((button) => button.onclick = () => rerunExec(button.dataset.execId));
}
async function loadExecHistory() {
  try {
    const body = await api("/app/exec/history");
    execHistory = body.entries || [];
    $("execHistoryHint").textContent = body.warning || "Stored in .rblxsync/exec-history.json";
    renderExecHistory();
  } catch (err) {
    execHistory = [];
    $("execHistoryList").innerHTML = '<div class="empty">' + escapeHtml(err.message) + '</div>';
  }
}
function setExecBusy(value) {
  ["execRunBtn","execLatestBtn","execHistoryRefreshBtn"].forEach(id => { const node = $(id); if (node) node.disabled = !!value; });
  $("execStatusText").textContent = value ? "Running command..." : "Start RiftSync, connect Studio, then enable Exec ON.";
}
async function copyTextToClipboard(text) {
  try { await navigator.clipboard.writeText(text); return true; } catch (_) {}
  const area = document.createElement("textarea");
  area.value = text; area.style.position = "fixed"; area.style.left = "-9999px";
  document.body.appendChild(area); area.focus(); area.select();
  let ok = false;
  try { ok = document.execCommand("copy"); } catch (_) {}
  area.remove();
  return ok;
}
async function copyExecToken() {
  const token = $("execTokenDisplay").value || "";
  if (!token.trim()) { $("execTokenHint").textContent = "Remote Exec token is not configured yet."; return; }
  const ok = await copyTextToClipboard(token);
  $("execTokenHint").textContent = ok ? "Token copied. Paste it into the Studio plugin." : "Copy failed. Select and copy the token manually.";
}
function renderExecResult(body) {
  $("execOutputText").textContent = body.output || "";
  const result = body.result || {};
  $("execResultHint").textContent = "state=" + (result.state || "-") + " ok=" + (!!result.ok) + (result.duration_ms ? " duration=" + result.duration_ms + "ms" : "");
}
async function runExec() {
  const source = $("execSourceInput").value;
  if (!source.trim()) { $("execOutputText").textContent = "Source is required."; return; }
  setExecBusy(true);
  try {
    const body = await api("/app/exec/run", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ source, timeout_sec: Number($("execTimeoutInput").value) || 0 }) });
    renderExecResult(body);
    await loadExecHistory();
  } catch (err) {
    $("execOutputText").textContent = err.message;
    $("execResultHint").textContent = "Remote Exec failed";
  } finally {
    setExecBusy(false);
  }
}
async function rerunExec(id) {
  setExecBusy(true);
  try {
    const body = await api("/app/exec/rerun", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ id: id || "", timeout_sec: Number($("execTimeoutInput").value) || 0 }) });
    if (body.entry && body.entry.source) $("execSourceInput").value = body.entry.source;
    renderExecResult(body);
    await loadExecHistory();
  } catch (err) {
    $("execOutputText").textContent = err.message;
    $("execResultHint").textContent = "Remote Exec failed";
  } finally {
    setExecBusy(false);
  }
}
function showStartPending() {
  const app = latest || {};
  render({
    ...app,
    address: app.address || "127.0.0.1:8765",
    host: app.host || "127.0.0.1",
    port: app.port || 8765,
    sync_root: app.sync_root || $("syncRootInput").value || "src/game",
    config_path: app.config_path || $("configInput").value || "sync_config.json",
    starting: true,
    running: false,
    status: "Starting",
    activity_error: false,
    activity_text: "Starting server and scanning project files..."
  });
}
async function action(path, options) { setBusy(true); if (path === "/app/start") showStartPending(); try { const body = await api(path, options || { method: "POST" }); if (body.app) render(body.app); } catch (err) { showError(err.message); } finally { setBusy(false); refresh(); } }
function setConfigStatus(message, kind) {
  const node = $("configApplyStatus");
  node.textContent = message;
  node.style.color = kind === "error" ? "var(--bad)" : kind === "active" ? "var(--accent-2)" : "var(--muted)";
}
function configPayload() {
  return {
    config_path: $("configInput").value,
    sync_root: $("syncRootInput").value,
    host: $("hostInput").value,
    port: $("portInput").value,
    git_enabled: true,
    debug: $("debugInput").checked,
    legacy_scan: false
  };
}
async function applyConfig() {
  if (latest && (latest.running || latest.starting)) { setConfigStatus("Stop sync before editing config.", "error"); return; }
  if (applyingConfig) return;
  applyingConfig = true;
  configDirty = true;
  setConfigStatus("Applying...", "active");
  try {
    const body = await api("/app/config/apply", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(configPayload()) });
    configDirty = false;
    if (body.app) render(body.app);
    setConfigStatus(body.result === "restarted" ? "Restarted" : "Saved", "ok");
  } catch (err) {
    showError(err.message);
    setConfigStatus(err.message, "error");
  } finally {
    applyingConfig = false;
    refresh();
  }
}
async function pickFolder() {
  if (latest && (latest.running || latest.starting)) { setConfigStatus("Stop sync before editing config.", "error"); return; }
  setBusy(true);
  try {
    const body = await api("/app/pick-folder", { method: "POST" });
    if (body.selected && body.path) {
      configDirty = true;
      $("syncRootInput").value = body.path;
      setConfigStatus("Folder selected. Applying...", "active");
      await applyConfig();
    } else if (body.app && !configDirty) {
      render(body.app);
    }
  } catch (err) {
    showError(err.message);
  } finally {
    setBusy(false);
  }
}
function requestQuit(path) { fetch(path || "/app/window/close", { method: "POST", keepalive: true }).catch(() => {}); try { if (window.appCloseWindow) window.appCloseWindow(); } catch (_) {} }
function formatLogTime(seconds) { if (!seconds) return "--:--:--"; return new Date(seconds * 1000).toLocaleTimeString(); }
function formatLogsPayload(payload) {
  const lines = [];
  lines.push("RiftSync local terminal");
  lines.push("revision=" + (payload.revision || 0) + " running=" + (!!payload.running));
  if (payload.git) lines.push("git=" + (payload.git.status || "-") + (payload.git.last_commit_short ? " @" + payload.git.last_commit_short : ""));
  if (payload.metrics && payload.metrics.last_changes) {
    const c = payload.metrics.last_changes;
    lines.push("last /changes since=" + (c.since_rev ?? "-") + " target=" + (c.target_rev ?? "-") + " count=" + (c.change_count ?? 0) + " waited=" + (Number(c.waited_sec || 0).toFixed(2)) + "s");
  }
  if (payload.activity && payload.activity.text) lines.push("studio=" + payload.activity.text);
  if (payload.scan_warnings && payload.scan_warnings.length) {
    lines.push("");
    lines.push("warnings:");
    payload.scan_warnings.forEach((warning) => lines.push("  " + warning));
  }
  lines.push("");
  lines.push("events:");
  (payload.events || []).forEach((event) => lines.push("[" + formatLogTime(event.ts) + "] " + event.kind + "  " + event.message));
  return lines.join("\n");
}
async function loadLogs() {
  try {
    const body = await api("/app/logs");
    $("logsText").textContent = formatLogsPayload(body);
  } catch (err) {
    $("logsText").textContent = err.message;
  }
}
function openLogs() {
  logsOpen = true;
  $("logsModal").classList.add("open");
  loadLogs();
  if (!logsTimer) logsTimer = setInterval(() => { if (logsOpen) loadLogs(); }, 1000);
}
function closeLogs() {
  logsOpen = false;
  $("logsModal").classList.remove("open");
  if (logsTimer) { clearInterval(logsTimer); logsTimer = null; }
}
document.querySelectorAll(".tab").forEach((tab) => tab.onclick = () => setTab(tab.dataset.tab));
["configInput","syncRootInput","hostInput","portInput"].forEach((id) => {
  const input = $(id);
  input.addEventListener("input", () => { if (input.disabled) return; configDirty = true; setConfigStatus("Unsaved", "active"); });
  input.addEventListener("blur", () => { if (configDirty) applyConfig(); });
  input.addEventListener("keydown", (event) => { if (event.key === "Enter") { event.preventDefault(); input.blur(); applyConfig(); } });
});
document.querySelectorAll(".toggle-switch input").forEach((input) => input.onchange = () => { if (input.disabled) return; configDirty = true; updateSwitchText(); applyConfig(); });
$("appHeader").addEventListener("mousedown", (event) => { if (event.target.closest("button,input")) return; if (window.appDragWindow) window.appDragWindow(); });
$("minimizeBtn").onclick = (event) => { event.stopPropagation(); if (window.appMinimizeWindow) window.appMinimizeWindow(); else action("/app/window/minimize"); };
$("closeBtn").onclick = (event) => { event.stopPropagation(); requestQuit("/app/window/close"); };
$("startBtn").onclick = () => action("/app/start"); $("stopBtn").onclick = () => action("/app/stop"); $("openBtn").onclick = () => action("/app/open-folder"); $("configOpenBtn").onclick = () => action("/app/open-folder");
$("changePathBtn").onclick = () => { setTab("configPanel"); $("syncRootInput").focus(); };
$("syncRootPickerBtn").onclick = pickFolder;
$("historyRefreshBtn").onclick = () => loadHistoryList(false);
$("execRunBtn").onclick = runExec;
$("execLatestBtn").onclick = () => rerunExec("");
$("execHistoryRefreshBtn").onclick = loadExecHistory;
$("execTokenCopyBtn").onclick = copyExecToken;
$("logsBtn").onclick = openLogs;
$("refreshLogsBtn").onclick = loadLogs;
$("closeLogsBtn").onclick = closeLogs;
$("logsModal").onclick = (event) => { if (event.target.id === "logsModal") closeLogs(); };
$("errorDetailsBtn").onclick = showErrorModal;
$("closeErrorBtn").onclick = hideErrorModal;
$("errorModal").onclick = (event) => { if (event.target.id === "errorModal") hideErrorModal(); };
$("copyErrorBtn").onclick = async () => {
  const text = lastFullError || "";
  await copyTextToClipboard(text);
};
refreshLoop(); loadHistoryList(false);
</script>
</body>
</html>`
}
