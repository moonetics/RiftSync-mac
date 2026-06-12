package config

import (
	"os"
	"path/filepath"
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

func TestEnsureSyncRootScaffoldCreatesFoldersAndGuidebook(t *testing.T) {
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
	if body, err := os.ReadFile(filepath.Join(root, GuidebookDir, "README.md")); err != nil {
		t.Fatalf("guidebook missing: %v", err)
	} else if !strings.Contains(string(body), "RiftSync Guidebook") {
		t.Fatalf("guidebook content = %q, want RiftSync Guidebook", string(body))
	}
}

func TestEnsureSyncRootScaffoldDoesNotOverwriteGuidebook(t *testing.T) {
	root := t.TempDir()
	cfg := Default()
	cfg.SyncRoot = root
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	guidebook := filepath.Join(root, GuidebookDir, "README.md")
	if err := os.MkdirAll(filepath.Dir(guidebook), 0o755); err != nil {
		t.Fatalf("mkdir guidebook: %v", err)
	}
	if err := os.WriteFile(guidebook, []byte("custom guide"), 0o644); err != nil {
		t.Fatalf("write guidebook: %v", err)
	}
	if err := EnsureSyncRootScaffold(cfg); err != nil {
		t.Fatalf("EnsureSyncRootScaffold returned error: %v", err)
	}
	body, err := os.ReadFile(guidebook)
	if err != nil {
		t.Fatalf("read guidebook: %v", err)
	}
	if string(body) != "custom guide" {
		t.Fatalf("guidebook overwritten = %q", string(body))
	}
}

func TestEnsureSyncRootScaffoldWritesExecLauncherAndOverwritesIt(t *testing.T) {
	root := t.TempDir()
	cfg := Default()
	cfg.SyncRoot = root
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	serverPath := filepath.Join(root, "tools", "riftsync-server.exe")
	if err := os.MkdirAll(filepath.Dir(serverPath), 0o755); err != nil {
		t.Fatalf("mkdir server dir: %v", err)
	}
	if err := os.WriteFile(serverPath, []byte("exe"), 0o644); err != nil {
		t.Fatalf("write server: %v", err)
	}
	configPath := filepath.Join(root, "sync_config.json")

	if err := EnsureSyncRootScaffold(cfg, ScaffoldOptions{ConfigPath: configPath, ExecutablePath: serverPath}); err != nil {
		t.Fatalf("EnsureSyncRootScaffold returned error: %v", err)
	}
	launcher := filepath.Join(root, GuidebookDir, GuidebookExecLauncher)
	body, err := os.ReadFile(launcher)
	if err != nil {
		t.Fatalf("read launcher: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "$RiftSyncServer = '"+serverPath+"'") {
		t.Fatalf("launcher missing server path: %s", text)
	}
	if !strings.Contains(text, "$input | & $RiftSyncServer --config $RiftSyncConfig exec @execArgs") {
		t.Fatalf("launcher missing stdin forwarding: %s", text)
	}
	if !strings.Contains(text, "$execArgs = @('--file', $firstArg) + $remainingArgs") {
		t.Fatalf("launcher missing lua file shortcut rewrite: %s", text)
	}
	if err := os.WriteFile(launcher, []byte("old launcher"), 0o644); err != nil {
		t.Fatalf("overwrite launcher setup: %v", err)
	}
	nextServerPath := filepath.Join(root, "next", "riftsync-server.exe")
	if err := os.MkdirAll(filepath.Dir(nextServerPath), 0o755); err != nil {
		t.Fatalf("mkdir next server dir: %v", err)
	}
	if err := os.WriteFile(nextServerPath, []byte("exe"), 0o644); err != nil {
		t.Fatalf("write next server: %v", err)
	}
	if err := EnsureSyncRootScaffold(cfg, ScaffoldOptions{ConfigPath: configPath, ExecutablePath: nextServerPath}); err != nil {
		t.Fatalf("EnsureSyncRootScaffold second call returned error: %v", err)
	}
	body, err = os.ReadFile(launcher)
	if err != nil {
		t.Fatalf("read launcher after overwrite: %v", err)
	}
	if !strings.Contains(string(body), nextServerPath) || strings.Contains(string(body), "old launcher") {
		t.Fatalf("launcher was not regenerated: %s", string(body))
	}
}

func TestGuidebookExecLauncherEscapesPowerShellPaths(t *testing.T) {
	launcher := buildGuidebookExecLauncher(`C:\Users\O'Brien\Rift Sync\riftsync-server.exe`, `D:\Game Config\sync_config.json`, true)
	if !strings.Contains(launcher, "$RiftSyncServer = 'C:\\Users\\O''Brien\\Rift Sync\\riftsync-server.exe'") {
		t.Fatalf("server path not single-quote escaped: %s", launcher)
	}
	if !strings.Contains(launcher, "$RiftSyncConfig = 'D:\\Game Config\\sync_config.json'") {
		t.Fatalf("config path not single-quote escaped: %s", launcher)
	}
}

func TestResolveRiftSyncServerPath(t *testing.T) {
	root := t.TempDir()
	serverPath := filepath.Join(root, "riftsync-server.exe")
	if err := os.WriteFile(serverPath, []byte("exe"), 0o644); err != nil {
		t.Fatalf("write server: %v", err)
	}
	resolved, exists := resolveRiftSyncServerPath(serverPath)
	if resolved != filepath.Clean(serverPath) || !exists {
		t.Fatalf("server resolve = %q %t, want %q true", resolved, exists, filepath.Clean(serverPath))
	}

	appPath := filepath.Join(root, "riftsync.exe")
	if err := os.WriteFile(appPath, []byte("exe"), 0o644); err != nil {
		t.Fatalf("write app: %v", err)
	}
	resolved, exists = resolveRiftSyncServerPath(appPath)
	if resolved != serverPath || !exists {
		t.Fatalf("app resolve = %q %t, want sibling %q true", resolved, exists, serverPath)
	}

	missingAppPath := filepath.Join(t.TempDir(), "riftsync.exe")
	resolved, exists = resolveRiftSyncServerPath(missingAppPath)
	if resolved != filepath.Join(filepath.Dir(missingAppPath), "riftsync-server.exe") || exists {
		t.Fatalf("missing app resolve = %q %t, want sibling candidate false", resolved, exists)
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
