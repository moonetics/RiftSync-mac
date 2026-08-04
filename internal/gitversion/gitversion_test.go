package gitversion

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/records"
	"riftsync/internal/state"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func testConfig(t *testing.T, root string) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.SyncRoot = root
	cfg.GitVersioningEnabled = true
	if err := cfg.NormalizeAndValidate(); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	return cfg
}

func TestEnsureRepositoryCreatesGit(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	cfg := testConfig(t, root)
	appState := state.New(cfg)
	service := New(cfg, appState, Options{CommandTimeout: 5 * time.Second})

	gitState, err := service.EnsureRepository(context.Background())
	if err != nil {
		t.Fatalf("EnsureRepository returned error: %v", err)
	}
	if !gitState.Available {
		t.Fatalf("gitState.Available = false, want true: %#v", gitState)
	}
	if gitState.LastError != "" {
		t.Fatalf("gitState.LastError = %q, want empty for repo without HEAD", gitState.LastError)
	}
	if gitState.Status != state.GitStatusReady {
		t.Fatalf("gitState.Status = %q, want %q", gitState.Status, state.GitStatusReady)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatalf(".git missing: %v", err)
	}
	for key, want := range map[string]string{
		"core.longpaths": "true",
		"core.autocrlf":  "false",
		"core.safecrlf":  "false",
	} {
		got, _, err := service.runGit(context.Background(), "config", "--get", key)
		if err != nil {
			t.Fatalf("read git config %s: %v", key, err)
		}
		if got = strings.TrimSpace(got); got != want {
			t.Fatalf("git config %s = %q, want %q", key, got, want)
		}
	}
}

func TestDisabledGitDoesNotDeleteRepository(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	cfg := testConfig(t, root)
	cfg.GitVersioningEnabled = false
	appState := state.New(cfg)
	service := New(cfg, appState, Options{CommandTimeout: 5 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service.Start(ctx)
	defer service.Close()

	if _, err := os.Stat(gitDir); err != nil {
		t.Fatalf(".git missing after disabling git: %v", err)
	}
	gitState := appState.GitState()
	if gitState.Enabled || gitState.Status != state.GitStatusDisabled {
		t.Fatalf("gitState = %#v, want disabled status", gitState)
	}
}

func TestNotifyRevisionDebouncedCommit(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	cfg := testConfig(t, root)
	appState := state.New(cfg)
	service := New(cfg, appState, Options{
		Debounce:       20 * time.Millisecond,
		CommandTimeout: 5 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx)
	defer service.Close()

	target := filepath.Join(root, "ServerScriptService", "Foo.server.luau")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("print(1)"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	record := records.SyncRecord{
		Entity:      records.EntityScript,
		LocalPath:   "ServerScriptService/Foo.server.luau",
		LocalDir:    "ServerScriptService",
		RbxPath:     "game.ServerScriptService.Foo",
		ClassName:   "Script",
		Source:      "print(1)",
		ContentHash: records.ContentDigest("print(1)"),
	}

	started := time.Now()
	appState.ApplySnapshot(map[string]records.SyncRecord{record.LocalPath: record}, nil, nil)
	if time.Since(started) > 200*time.Millisecond {
		t.Fatalf("ApplySnapshot/NotifyRevision took %v, want async return", time.Since(started))
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		summary, _, found := appState.RevisionDetail(1)
		if found && summary.GitCommitShort != "" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("revision did not receive git commit; git state=%#v", appState.GitState())
}

func TestTransientGitAddErrorDetection(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "short read while indexing",
			err:  errString("git add -A: error: short read while indexing .rblxsync/project-state.json error: .rblxsync/project-state.json: failed to insert into database error: unable to index file '.rblxsync/project-state.json' fatal: updating files failed"),
			want: true,
		},
		{
			name: "index lock",
			err:  errString("git add -A: fatal: Unable to create '.git/index.lock': File exists."),
			want: true,
		},
		{
			name: "ordinary pathspec error",
			err:  errString("git add -A: fatal: pathspec 'missing' did not match any files"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTransientGitAddError(tc.err); got != tc.want {
				t.Fatalf("isTransientGitAddError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

type errString string

func (e errString) Error() string {
	return string(e)
}
