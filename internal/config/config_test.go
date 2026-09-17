package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	expectedRoot := filepath.Join(resolvedDir, "src", "game")
	if cfg.SyncRootAbs != expectedRoot {
		t.Fatalf("SyncRootAbs = %q, want %q", cfg.SyncRootAbs, expectedRoot)
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
	if cfg.RemoteExecEnabled {
		t.Fatal("RemoteExecEnabled = true, want false")
	}
	if cfg.RemoteExecToken != "" {
		t.Fatalf("RemoteExecToken = %q, want empty", cfg.RemoteExecToken)
	}
	if cfg.RemoteExecMaxSourceBytes != 262144 || cfg.RemoteExecDefaultTimeoutSec != 10 || cfg.RemoteExecMaxTimeoutSec != 120 {
		t.Fatalf("remote exec defaults = maxBytes %d defaultTimeout %d maxTimeout %d", cfg.RemoteExecMaxSourceBytes, cfg.RemoteExecDefaultTimeoutSec, cfg.RemoteExecMaxTimeoutSec)
	}
}

func TestGenerateRemoteExecToken(t *testing.T) {
	token, err := GenerateRemoteExecToken()
	if err != nil {
		t.Fatalf("GenerateRemoteExecToken returned error: %v", err)
	}
	if len(token) != RemoteExecTokenLength {
		t.Fatalf("token length = %d, want %d", len(token), RemoteExecTokenLength)
	}
	for _, char := range token {
		if !strings.ContainsRune(remoteExecTokenAlphabet, char) {
			t.Fatalf("token contains unsupported char %q in %q", char, token)
		}
	}
}

func TestEnsureRemoteExecTokenPreservesExisting(t *testing.T) {
	cfg := Default()
	cfg.RemoteExecToken = "existing-token"
	generated, err := EnsureRemoteExecToken(&cfg)
	if err != nil {
		t.Fatalf("EnsureRemoteExecToken returned error: %v", err)
	}
	if generated {
		t.Fatal("EnsureRemoteExecToken generated new token for existing token")
	}
	if cfg.RemoteExecToken != "existing-token" {
		t.Fatalf("RemoteExecToken = %q, want existing-token", cfg.RemoteExecToken)
	}
}

func TestEnsureRemoteExecTokenGeneratesMissing(t *testing.T) {
	cfg := Default()
	generated, err := EnsureRemoteExecToken(&cfg)
	if err != nil {
		t.Fatalf("EnsureRemoteExecToken returned error: %v", err)
	}
	if !generated {
		t.Fatal("EnsureRemoteExecToken generated=false, want true")
	}
	if len(cfg.RemoteExecToken) != RemoteExecTokenLength {
		t.Fatalf("RemoteExecToken length = %d, want %d", len(cfg.RemoteExecToken), RemoteExecTokenLength)
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

func TestInvalidRemoteExecConfigFails(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "max source bytes",
			mutate: func(cfg *Config) {
				cfg.RemoteExecMaxSourceBytes = 0
			},
		},
		{
			name: "default timeout",
			mutate: func(cfg *Config) {
				cfg.RemoteExecDefaultTimeoutSec = 0
			},
		},
		{
			name: "max timeout",
			mutate: func(cfg *Config) {
				cfg.RemoteExecMaxTimeoutSec = 0
			},
		},
		{
			name: "default exceeds max",
			mutate: func(cfg *Config) {
				cfg.RemoteExecDefaultTimeoutSec = 30
				cfg.RemoteExecMaxTimeoutSec = 10
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Default()
			tc.mutate(&cfg)
			if err := cfg.NormalizeAndValidate(); err == nil {
				t.Fatal("NormalizeAndValidate returned nil, want remote exec config error")
			}
		})
	}
}

func TestEnsureSyncRootScaffoldCreatesFolders(t *testing.T) {
	root := t.TempDir()
	cfg := Default()
	cfg.SyncRoot = root
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	existing := filepath.Join(root, "ServerScriptService", "Existing.server.luau")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatalf("mkdir existing: %v", err)
	}
	if err := os.WriteFile(existing, []byte("print(1)"), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	if err := EnsureSyncRootScaffold(cfg); err != nil {
		t.Fatalf("EnsureSyncRootScaffold returned error: %v", err)
	}
	for _, dir := range BootstrapServiceDirs {
		if info, err := os.Stat(filepath.Join(root, dir)); err != nil || !info.IsDir() {
			t.Fatalf("service dir %s missing or not dir: info=%#v err=%v", dir, info, err)
		}
	}
	if _, err := os.Stat(existing); err != nil {
		t.Fatalf("existing file was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, GuidebookDir)); !os.IsNotExist(err) {
		t.Fatalf("expected .guidebook to not exist after EnsureSyncRootScaffold, err=%v", err)
	}
	vscodeSettings := filepath.Join(root, VSCodeDir, "settings.json")
	if body, err := os.ReadFile(vscodeSettings); err != nil {
		t.Fatalf("expected .vscode/settings.json to exist after scaffold: %v", err)
	} else if !strings.Contains(string(body), "luau-lsp.platform.type") || !strings.Contains(string(body), "roblox") {
		t.Fatalf(".vscode/settings.json content = %q", string(body))
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

func TestPortabilityWarningDetectsWindowsPathOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows paths are native on Windows")
	}
	cfg := Default()
	cfg.SyncRoot = `C:\Users\Example\game`
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	if warning := cfg.PortabilityWarning(); warning == "" {
		t.Fatal("PortabilityWarning is empty for a Windows path on Unix")
	}
}
