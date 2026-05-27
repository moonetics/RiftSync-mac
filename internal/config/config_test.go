package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExistingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "sync_config.json")
	body := `{
  "host": "localhost",
  "port": 9876,
  "sync_root": "src/game",
  "scan_interval_sec": 0.5,
  "poll_timeout_sec": 15,
  "change_retention": 100,
  "debug_event_retention": 75,
  "invalid_json_grace_sec": 1.5,
  "git_versioning_enabled": false,
  "managed_roots": [" game.Workspace ", ""],
  "ignored_rbx_paths": [" game.ServerScriptService.RiftSyncPlugin ", ""]
}`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := Load("sync_config.json")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Port != 9876 {
		t.Fatalf("Port = %d, want 9876", cfg.Port)
	}
	if cfg.Host != "localhost" {
		t.Fatalf("Host = %q, want localhost", cfg.Host)
	}
	if cfg.GitVersioningEnabled {
		t.Fatal("GitVersioningEnabled = true, want false")
	}
	if cfg.SyncRootAbs != filepath.Join(dir, "src", "game") {
		t.Fatalf("SyncRootAbs = %q, want %q", cfg.SyncRootAbs, filepath.Join(dir, "src", "game"))
	}
	if len(cfg.ManagedRoots) != 1 || cfg.ManagedRoots[0] != "game.Workspace" {
		t.Fatalf("ManagedRoots = %#v, want normalized single value", cfg.ManagedRoots)
	}
	if len(cfg.IgnoredRbxPaths) != 1 || cfg.IgnoredRbxPaths[0] != "game.ServerScriptService.RiftSyncPlugin" {
		t.Fatalf("IgnoredRbxPaths = %#v, want normalized single value", cfg.IgnoredRbxPaths)
	}
}

func TestLoadMissingConfigUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := Load("sync_config.json")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Port != 8765 {
		t.Fatalf("Port = %d, want default 8765", cfg.Port)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want default 127.0.0.1", cfg.Host)
	}
	if cfg.SyncRoot != "src/game" {
		t.Fatalf("SyncRoot = %q, want src/game", cfg.SyncRoot)
	}
	if len(cfg.ManagedRoots) != 11 {
		t.Fatalf("ManagedRoots length = %d, want 11", len(cfg.ManagedRoots))
	}
}

func TestInvalidPortFails(t *testing.T) {
	cfg := Default()
	cfg.Port = 70000
	if err := cfg.NormalizeAndValidate(); err == nil {
		t.Fatal("NormalizeAndValidate returned nil, want invalid port error")
	}
}

func TestInvalidHostFails(t *testing.T) {
	cfg := Default()
	cfg.Host = "0.0.0.0"
	if err := cfg.NormalizeAndValidate(); err == nil {
		t.Fatal("NormalizeAndValidate returned nil, want invalid host error")
	}
}

func TestEmptySyncRootFails(t *testing.T) {
	cfg := Default()
	cfg.SyncRoot = " "
	if err := cfg.NormalizeAndValidate(); err == nil {
		t.Fatal("NormalizeAndValidate returned nil, want empty sync_root error")
	}
}

func TestResolveInsideSyncRootAcceptsValidPath(t *testing.T) {
	root := t.TempDir()
	cfg := Default()
	cfg.SyncRoot = root
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}

	resolved, err := cfg.ResolveInsideSyncRoot("StarterGui/Main.ScreenGui/properties.init.json")
	if err != nil {
		t.Fatalf("ResolveInsideSyncRoot returned error: %v", err)
	}
	want := filepath.Join(root, "StarterGui", "Main.ScreenGui", "properties.init.json")
	if resolved != want {
		t.Fatalf("resolved = %q, want %q", resolved, want)
	}
}

func TestResolveInsideSyncRootRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	cfg := Default()
	cfg.SyncRoot = root
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}

	if _, err := cfg.ResolveInsideSyncRoot("../outside"); err == nil {
		t.Fatal("ResolveInsideSyncRoot returned nil error, want traversal rejection")
	}
}
