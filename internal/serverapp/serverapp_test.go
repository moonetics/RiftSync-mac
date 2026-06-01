package serverapp

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/scanner"
	"riftsync/internal/state"
)

func TestRunnerStartStopAndHealth(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sync_config.json")
	syncRoot := filepath.Join(root, "game")
	port := freePort(t)
	body := map[string]any{
		"host":                   "127.0.0.1",
		"port":                   port,
		"sync_root":              syncRoot,
		"git_versioning_enabled": false,
	}
	configBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runner := New(Options{ConfigPath: configPath})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := runner.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	status := runner.Status(5)
	if !status.Running || status.Host != "127.0.0.1" || status.Port != port || status.SyncRoot == "" {
		t.Fatalf("status = %#v", status)
	}

	resp, err := http.Get("http://" + status.Host + ":" + strconv.Itoa(status.Port) + "/health")
	if err != nil {
		t.Fatalf("GET health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d, want 200", resp.StatusCode)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := runner.Stop(stopCtx); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if runner.Status(1).Running {
		t.Fatal("runner still running after Stop")
	}
}

func TestRunnerApplyOptionsStoppedDoesNotStart(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sync_config.json")
	options := Options{
		ConfigPath:       configPath,
		HostOverride:     "127.0.0.1",
		PortOverride:     8765,
		SyncRootOverride: filepath.Join(root, "game"),
		LegacyScan:       true,
		Debug:            true,
	}
	runner := New(Options{ConfigPath: configPath, PortOverride: -1})
	restarted, err := runner.ApplyOptions(context.Background(), options)
	if err != nil {
		t.Fatalf("ApplyOptions returned error: %v", err)
	}
	if restarted {
		t.Fatal("ApplyOptions returned restarted=true for stopped runner")
	}
	status := runner.Status(1)
	if status.Running || status.Port != 8765 || status.SyncRoot != options.SyncRootOverride || status.LegacyScan || !status.Debug {
		t.Fatalf("status = %#v", status)
	}
}

func TestHybridSafetyScannerDetectsNestedLocalSave(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sync_config.json")
	syncRoot := filepath.Join(root, "game")
	port := freePort(t)
	configBody, err := json.Marshal(map[string]any{
		"host":                   "127.0.0.1",
		"port":                   port,
		"sync_root":              syncRoot,
		"scan_interval_sec":      0.1,
		"git_versioning_enabled": true,
	})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runner := New(Options{ConfigPath: configPath})
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

	target := filepath.Join(syncRoot, "ServerScriptService", "Nested", "Foo.server.luau")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("print(1)"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if runner.Status(1).Revision > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("safety scanner did not detect nested save; status=%#v", runner.Status(5))
}

func TestLegacyScanOnceWaitsForReconciliationLock(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.SyncRoot = root
	cfg.GitVersioningEnabled = false
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	cache := scanner.NewCache()
	appState := state.New(cfg)
	target := filepath.Join(root, "ServerScriptService", "Foo.server.luau")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("print(1)"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	appState.LockReconciliation()
	locked := true
	defer func() {
		if locked {
			appState.UnlockReconciliation()
		}
	}()

	done := make(chan error, 1)
	go func() {
		_, err := LegacyScanOnce(cfg, cache, appState, 10*time.Millisecond)
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("LegacyScanOnce completed while reconciliation lock was held: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	appState.UnlockReconciliation()
	locked = false
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("LegacyScanOnce returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("LegacyScanOnce did not resume after reconciliation lock was released")
	}
	if appState.Revision() != 1 {
		t.Fatalf("revision = %d, want 1", appState.Revision())
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
