package serverapp

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/debuglog"
	"riftsync/internal/gitversion"
	"riftsync/internal/httpapi"
	"riftsync/internal/scanner"
	"riftsync/internal/state"
	"riftsync/internal/watcher"
)

const Version = "4.2.04"

type Options struct {
	ConfigPath            string
	HostOverride          string
	PortOverride          int
	SyncRootOverride      string
	GitVersioningOverride *bool
	Debug                 bool
	LegacyScan            bool
	Stdout                io.Writer
}

type Runner struct {
	mu         sync.Mutex
	cfg        config.Config
	options    Options
	appState   *state.AppState
	scanCache  *scanner.Cache
	server     *http.Server
	gitService *gitversion.Service
	watcher    *watcher.Service
	cancel     context.CancelFunc
	done       chan error
	safetyDone <-chan struct{}
	running    bool
	startedAt  time.Time
}

type Status struct {
	Running           bool
	StartedAt         time.Time
	Host              string
	Port              int
	ConfigPath        string
	SyncRoot          string
	LegacyScan        bool
	Debug             bool
	Revision          int
	Counts            state.IndexedCounts
	Metrics           state.Metrics
	Git               state.GitState
	Activity          state.Activity
	Warnings          []string
	Events            []debuglog.Event
	History           []state.RevisionSummary
	LastError         string
	RemoteExecEnabled bool
	RemoteExecToken   string
}

func New(options Options) *Runner {
	if options.ConfigPath == "" {
		options.ConfigPath = "sync_config.json"
	}
	if options.PortOverride == 0 {
		options.PortOverride = -1
	}
	return &Runner{options: options}
}

func (r *Runner) Start(parent context.Context) error {
	return r.StartWithProgress(parent, nil)
}

func (r *Runner) StartWithProgress(parent context.Context, onProgress func(state.Activity)) error {
	report := func(text, phase string, progress, current, total int, indeterminate bool) {
		if onProgress == nil {
			return
		}
		onProgress(state.Activity{
			Text:          text,
			Operation:     "startup",
			Phase:         phase,
			Progress:      progress,
			Current:       current,
			Total:         total,
			Indeterminate: indeterminate,
			At:            float64(time.Now().UnixNano()) / float64(time.Second),
		})
	}

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		report("RiftSync is already running.", "complete", 100, 0, 0, false)
		return nil
	}
	options := r.options
	r.mu.Unlock()

	report("Loading project configuration...", "loading_config", 4, 0, 0, true)
	cfg, err := config.Load(options.ConfigPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	if options.PortOverride != -1 {
		cfg.Port = options.PortOverride
	}
	if options.HostOverride != "" {
		cfg.Host = options.HostOverride
	}
	if options.SyncRootOverride != "" {
		cfg.SyncRoot = options.SyncRootOverride
	}
	if options.GitVersioningOverride != nil {
		cfg.GitVersioningEnabled = *options.GitVersioningOverride
	}
	cfg.GitVersioningEnabled = true
	if err := cfg.NormalizeAndValidate(); err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	if warning := cfg.PortabilityWarning(); warning != "" {
		return fmt.Errorf("sync root unavailable: %s; choose a folder with Open Folder and retry", warning)
	}
	report("Reserving the local sync port...", "reserving_port", 9, 0, 0, true)
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	if err != nil {
		return fmt.Errorf("listen on %s:%d: %w", cfg.Host, cfg.Port, err)
	}
	closeListener := true
	defer func() {
		if closeListener {
			_ = listener.Close()
		}
	}()
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("executable path error: %w", err)
	}
	report("Preparing the project folder...", "preparing_project", 14, 0, 0, true)
	if err := config.EnsureSyncRootScaffold(cfg, config.ScaffoldOptions{
		ConfigPath:      options.ConfigPath,
		ExecutablePath:  executablePath,
		Version:         Version,
		WriteStatusJSON: true,
	}); err != nil {
		return fmt.Errorf("sync root setup error: %w", err)
	}

	appState := state.New(cfg)
	scanCache := scanner.NewCache()
	scanStarted := time.Now()
	snapshot, err := scanCache.ScanWithProgress(cfg, func(scanProgress scanner.Progress) {
		progress := 20
		text := "Finding project files..."
		if scanProgress.Phase == "scanning" {
			progress = 25
			text = "Scanning project files..."
			if scanProgress.Total > 0 {
				progress += scanProgress.Current * 47 / scanProgress.Total
				text = fmt.Sprintf("Scanning project files... %d/%d", scanProgress.Current, scanProgress.Total)
			}
		} else if scanProgress.Current > 0 {
			text = fmt.Sprintf("Finding project files... %d found", scanProgress.Current)
		}
		report(
			text,
			scanProgress.Phase,
			progress,
			scanProgress.Current,
			scanProgress.Total,
			scanProgress.Indeterminate,
		)
	})
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}
	report(fmt.Sprintf("Indexed %d sync records.", len(snapshot.Records)), "indexing_state", 76, len(snapshot.Records), len(snapshot.Records), false)
	appState.SetSnapshot(snapshot.Records, snapshot.Warnings, len(snapshot.InvalidPaths))
	report("Loading revision history...", "loading_history", 80, 0, 0, true)
	if err := appState.LoadHistory(); err != nil && options.Debug && options.Stdout != nil {
		fmt.Fprintf(options.Stdout, "[debug] load history error: %v\n", err)
	}
	report("Loading project state...", "loading_project_state", 84, 0, 0, true)
	if err := appState.LoadProjectState(); err != nil && options.Debug && options.Stdout != nil {
		fmt.Fprintf(options.Stdout, "[debug] load project state error: %v\n", err)
	}
	appState.RecordPerformance(0, 0, time.Since(scanStarted), 0, snapshot.CacheHits, snapshot.CacheMisses)

	ctx, cancel := context.WithCancel(parent)
	report("Starting Git integration...", "starting_git", 88, 0, 0, true)
	gitService := gitversion.New(cfg, appState, gitversion.Options{})
	gitService.Start(ctx)

	var watchService *watcher.Service
	var safetyDone <-chan struct{}
	if options.LegacyScan {
		report("Starting legacy periodic scanner...", "starting_scanner", 92, 0, 0, true)
		safetyDone = StartLegacyScanner(ctx, cfg, scanCache, appState, options.Debug, options.Stdout)
	} else {
		report("Starting filesystem watcher...", "starting_watcher", 92, 0, 0, true)
		watchService, err = watcher.New(cfg, appState, watcher.Options{Cache: scanCache})
		if err != nil {
			cancel()
			gitService.Close()
			return fmt.Errorf("watcher error: %w", err)
		}
		if err := watchService.Start(ctx); err != nil {
			cancel()
			gitService.Close()
			_ = watchService.Close()
			return fmt.Errorf("watcher error: %w", err)
		}
		if cfg.SafetyScanIntervalSec > 0 {
			safetyDone = StartSafetyScanner(ctx, cfg, scanCache, appState, watchService, options.Debug, options.Stdout)
		}
	}

	report("Starting local HTTP server...", "starting_server", 97, 0, 0, true)
	var refreshWatches func() error
	if watchService != nil {
		refreshWatches = func() error {
			_, err := watchService.SyncTree()
			return err
		}
	}
	server := &http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: httpapi.NewWithScannerCacheAndWatchRefresh(appState, Version, scanCache, refreshWatches),
	}
	done := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err == http.ErrServerClosed {
			err = nil
		}
		done <- err
	}()

	r.mu.Lock()
	r.cfg = cfg
	r.appState = appState
	r.scanCache = scanCache
	r.server = server
	r.gitService = gitService
	r.watcher = watchService
	r.cancel = cancel
	r.done = done
	r.safetyDone = safetyDone
	r.running = true
	r.startedAt = time.Now()
	r.mu.Unlock()
	closeListener = false
	appState.RecordActivity(state.Activity{
		Text:      fmt.Sprintf("Startup complete: %d records indexed.", len(snapshot.Records)),
		Operation: "startup",
		Phase:     "complete",
		Progress:  100,
		Current:   len(snapshot.Records),
		Total:     len(snapshot.Records),
	})
	report("RiftSync is ready.", "complete", 100, len(snapshot.Records), len(snapshot.Records), false)
	return nil
}

func (r *Runner) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	server := r.server
	cancel := r.cancel
	done := r.done
	safetyDone := r.safetyDone
	gitService := r.gitService
	watchService := r.watcher
	r.running = false
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	var shutdownErr error
	if server != nil {
		shutdownErr = server.Shutdown(ctx)
	}
	if safetyDone != nil {
		select {
		case <-safetyDone:
		case <-ctx.Done():
			shutdownErr = ctx.Err()
		}
	}
	if watchService != nil {
		_ = watchService.Close()
	}
	if gitService != nil {
		gitService.Close()
	}
	if done != nil {
		select {
		case err := <-done:
			if shutdownErr == nil {
				shutdownErr = err
			}
		case <-ctx.Done():
			if shutdownErr == nil {
				shutdownErr = ctx.Err()
			}
		}
	}
	return shutdownErr
}

func (r *Runner) Restart(ctx context.Context, options Options) error {
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Stop(stopCtx); err != nil {
		return err
	}
	r.mu.Lock()
	r.options = normalizeOptions(options)
	r.mu.Unlock()
	return r.Start(ctx)
}

func (r *Runner) ApplyOptions(ctx context.Context, options Options) (bool, error) {
	r.mu.Lock()
	running := r.running
	if !running {
		r.options = normalizeOptions(options)
		r.mu.Unlock()
		return false, nil
	}
	r.mu.Unlock()
	return true, r.Restart(ctx, options)
}

func (r *Runner) Status(limit int) Status {
	r.mu.Lock()
	running := r.running
	startedAt := r.startedAt
	cfg := r.cfg
	options := r.options
	appState := r.appState
	r.mu.Unlock()

	status := Status{
		Running:           running,
		StartedAt:         startedAt,
		Host:              cfg.Host,
		Port:              cfg.Port,
		ConfigPath:        options.ConfigPath,
		SyncRoot:          cfg.SyncRootAbs,
		LegacyScan:        running && options.LegacyScan,
		Debug:             options.Debug,
		RemoteExecEnabled: cfg.RemoteExecEnabled,
		RemoteExecToken:   cfg.RemoteExecToken,
	}
	if status.Host == "" {
		status.Host = options.HostOverride
	}
	if status.Host == "" {
		status.Host = "127.0.0.1"
	}
	if status.Port == 0 && options.PortOverride > 0 {
		status.Port = options.PortOverride
	}
	if status.SyncRoot == "" {
		status.SyncRoot = options.SyncRootOverride
	}
	if appState == nil {
		return status
	}
	status.Revision = appState.Revision()
	status.Counts = appState.IndexedCounts()
	status.Metrics = appState.Metrics()
	status.Git = appState.GitState()
	status.Activity = appState.Activity()
	status.Warnings = appState.ScanWarnings(limit)
	status.History = appState.RevisionSummaries(limit)
	status.Events = appState.Events(limit)
	status.LastError = status.Metrics.LastError
	return status
}

func (r *Runner) Options() Options {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.options
}

func (r *Runner) HistorySummaries(limit int) (int, state.GitState, []state.RevisionSummary) {
	r.mu.Lock()
	appState := r.appState
	r.mu.Unlock()
	if appState == nil {
		return 0, state.GitState{}, []state.RevisionSummary{}
	}
	return appState.Revision(), appState.GitState(), appState.RevisionSummaries(limit)
}

func (r *Runner) HistoryDetail(revision int) (int, state.GitState, state.RevisionSummary, []map[string]any, bool) {
	r.mu.Lock()
	appState := r.appState
	r.mu.Unlock()
	if appState == nil {
		return 0, state.GitState{}, state.RevisionSummary{Rev: revision}, []map[string]any{}, false
	}
	summary, changes, found := appState.RevisionDetail(revision)
	return appState.Revision(), appState.GitState(), summary, changes, found
}

func normalizeOptions(options Options) Options {
	if options.ConfigPath == "" {
		options.ConfigPath = "sync_config.json"
	}
	if options.PortOverride == 0 {
		options.PortOverride = -1
	}
	return options
}

func StartSafetyScanner(ctx context.Context, cfg config.Config, scanCache *scanner.Cache, appState *state.AppState, watchService *watcher.Service, debug bool, stdout io.Writer) <-chan struct{} {
	interval := time.Duration(cfg.SafetyScanIntervalSec * float64(time.Second))
	if interval <= 0 {
		interval = 30 * time.Second
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				event, err := LegacyScanOnce(cfg, scanCache, appState, interval)
				if watchService != nil {
					if _, watchErr := watchService.SyncTree(); watchErr != nil && debug && stdout != nil {
						fmt.Fprintf(stdout, "[debug] watcher refresh error: %v\n", watchErr)
					}
				}
				if debug && stdout != nil && err != nil {
					fmt.Fprintf(stdout, "[debug] safety scan error: %v\n", err)
				} else if debug && stdout != nil && event.Rev > 0 {
					fmt.Fprintf(stdout, "[debug] safety scan revision=%d changes=%d\n", event.Rev, event.ChangeCount)
				}
			}
		}
	}()
	return done
}

func StartLegacyScanner(ctx context.Context, cfg config.Config, scanCache *scanner.Cache, appState *state.AppState, debug bool, stdout io.Writer) <-chan struct{} {
	interval := time.Duration(cfg.ScanIntervalSec * float64(time.Second))
	if interval <= 0 {
		interval = 400 * time.Millisecond
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				event, err := LegacyScanOnce(cfg, scanCache, appState, interval)
				if debug && stdout != nil && err != nil {
					fmt.Fprintf(stdout, "[debug] legacy scan error: %v\n", err)
				} else if debug && stdout != nil && event.Rev > 0 {
					fmt.Fprintf(stdout, "[debug] legacy scan revision=%d changes=%d\n", event.Rev, event.ChangeCount)
				}
			}
		}
	}()
	return done
}

func LegacyScanOnce(cfg config.Config, scanCache *scanner.Cache, appState *state.AppState, interval time.Duration) (state.RevisionEvent, error) {
	if scanCache == nil {
		scanCache = scanner.NewCache()
	}
	appState.LockReconciliation()
	defer appState.UnlockReconciliation()

	scanStarted := time.Now()
	snapshot, err := scanCache.Scan(cfg)
	parseDuration := time.Since(scanStarted)
	if err != nil {
		return state.RevisionEvent{}, err
	}
	publishStarted := time.Now()
	event := appState.ApplySnapshot(snapshot.Records, snapshot.Warnings, snapshot.InvalidPaths)
	publishDuration := time.Since(publishStarted)
	appState.RecordPerformance(len(snapshot.Records), interval, parseDuration, publishDuration, snapshot.CacheHits, snapshot.CacheMisses)
	return event, nil
}
