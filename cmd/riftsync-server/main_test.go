package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/records"
	"riftsync/internal/scanner"
	"riftsync/internal/serverapp"
	"riftsync/internal/state"
)

func TestParseOptionsDefaults(t *testing.T) {
	options, err := parseOptions(nil)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if options.configPath != "sync_config.json" || options.portOverride != -1 || options.debug || options.legacyScan || options.headless {
		t.Fatalf("options = %#v", options)
	}
}

func TestParseOptionsCustom(t *testing.T) {
	options, err := parseOptions([]string{
		"--config", "custom.json",
		"--host", "localhost",
		"--port", "8766",
		"--sync-root", "C:\\Project\\Game",
		"--debug",
		"--legacy-scan",
		"--headless",
	})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if options.configPath != "custom.json" || options.hostOverride != "localhost" || options.portOverride != 8766 || options.syncRootOverride != "C:\\Project\\Game" || !options.debug || !options.legacyScan || !options.headless {
		t.Fatalf("options = %#v", options)
	}
}

func TestParseOptionsInvalidPort(t *testing.T) {
	if _, err := parseOptions([]string{"--port", "0"}); err == nil {
		t.Fatal("parseOptions returned nil error, want invalid port error")
	}
	if _, err := parseOptions([]string{"--port", "70000"}); err == nil {
		t.Fatal("parseOptions returned nil error, want invalid port error")
	}
}

func TestParseOptionsInvalidHost(t *testing.T) {
	if _, err := parseOptions([]string{"--host", "0.0.0.0"}); err == nil {
		t.Fatal("parseOptions returned nil error, want invalid host error")
	}
}

func TestParseOptionsHelp(t *testing.T) {
	if _, err := parseOptions([]string{"--help"}); err != flag.ErrHelp {
		t.Fatalf("err = %v, want flag.ErrHelp", err)
	}
}

func TestLegacyScanOncePublishesRevision(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(t, root)
	cache := scanner.NewCache()
	appState := state.New(cfg)
	appState.SetSnapshot(map[string]records.SyncRecord{}, nil, 0)

	writeFile(t, root, "ServerScriptService/Foo.server.luau", "print(1)")
	event, err := serverapp.LegacyScanOnce(cfg, cache, appState, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("legacyScanOnce returned error: %v", err)
	}
	if event.Rev != 1 || appState.Revision() != 1 {
		t.Fatalf("event rev=%d state rev=%d, want 1", event.Rev, appState.Revision())
	}
	changes, target, needsSnapshot := appState.FlattenChanges(0)
	if needsSnapshot || target != 1 || len(changes) != 1 || changes[0]["op"] != "upsert" {
		t.Fatalf("changes=%#v target=%d needsSnapshot=%v", changes, target, needsSnapshot)
	}
}

func TestStartLegacyScannerStopsOnContextCancel(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(t, root)
	cfg.ScanIntervalSec = 0.01
	cache := scanner.NewCache()
	appState := state.New(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	serverapp.StartLegacyScanner(ctx, cfg, cache, appState, false, nil)
	cancel()

	writeFile(t, root, "ServerScriptService/Foo.server.luau", "print(1)")
	time.Sleep(50 * time.Millisecond)
	if appState.Revision() != 0 {
		t.Fatalf("revision = %d, want 0 after cancelled legacy scanner", appState.Revision())
	}
}

func testConfig(t *testing.T, root string) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.SyncRoot = root
	cfg.GitVersioningEnabled = false
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	return cfg
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
