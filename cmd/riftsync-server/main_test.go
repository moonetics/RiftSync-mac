package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestParseOptionsExecInline(t *testing.T) {
	options, err := parseOptions([]string{"--config", "custom.json", "--port", "8766", "exec", "print(workspace.Name)"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if options.command != "exec" || options.configPath != "custom.json" || options.portOverride != 8766 || options.exec.inline != "print(workspace.Name)" {
		t.Fatalf("options = %#v", options)
	}
}

func TestParseOptionsExecStdin(t *testing.T) {
	options, err := parseOptions([]string{"exec", "--stdin", "--timeout", "15", "--raw"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if options.command != "exec" || !options.exec.useStdin || options.exec.timeoutSec != 15 || !options.exec.rawOutput {
		t.Fatalf("options = %#v", options)
	}
}

func TestParseOptionsExecFile(t *testing.T) {
	options, err := parseOptions([]string{"exec", "--file", "studio-command.luau", "--json"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if options.command != "exec" || options.exec.filePath != "studio-command.luau" || !options.exec.jsonOutput {
		t.Fatalf("options = %#v", options)
	}
}

func TestParseOptionsExecLuaFileShortcut(t *testing.T) {
	options, err := parseOptions([]string{"exec", "studio-command.lua", "--json"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if options.command != "exec" || options.exec.filePath != "studio-command.lua" || options.exec.inline != "" {
		t.Fatalf("options = %#v", options)
	}

	options, err = parseOptions([]string{"exec", "studio-command.luau"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if options.exec.filePath != "studio-command.luau" {
		t.Fatalf("filePath = %q, want studio-command.luau", options.exec.filePath)
	}
}

func TestParseOptionsExecRejectsAmbiguousSource(t *testing.T) {
	cases := [][]string{
		{"exec", "--stdin", "--file", "x.luau"},
		{"exec", "--stdin", "print(1)"},
		{"exec", "--file", "x.luau", "print(1)"},
		{"exec"},
		{"exec", "--json", "--raw", "print(1)"},
	}
	for _, args := range cases {
		if _, err := parseOptions(args); err == nil {
			t.Fatalf("parseOptions(%v) returned nil error, want ambiguous source error", args)
		}
	}
}

func TestReadExecSourceFromStdinAndFile(t *testing.T) {
	source, err := readExecSource(execOptions{useStdin: true}, strings.NewReader("print(1)\nreturn 2"))
	if err != nil {
		t.Fatalf("read stdin source: %v", err)
	}
	if source != "print(1)\nreturn 2" {
		t.Fatalf("stdin source = %q", source)
	}

	path := filepath.Join(t.TempDir(), "studio-command.luau")
	if err := os.WriteFile(path, []byte("print('file')"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	source, err = readExecSource(execOptions{filePath: path}, nil)
	if err != nil {
		t.Fatalf("read file source: %v", err)
	}
	if source != "print('file')" {
		t.Fatalf("file source = %q", source)
	}
}

func TestRemoteExecClientSubmitAndWait(t *testing.T) {
	pollCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("Authorization = %q, want bearer token", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/exec/commands":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode submit payload: %v", err)
			}
			if payload["source"] != "print(1)" || payload["timeout_sec"] != float64(15) || payload["client"] != "riftsync-cli" || payload["mode"] != "edit" {
				t.Fatalf("submit payload = %#v", payload)
			}
			_, _ = w.Write([]byte(`{"status":"ok","command_id":"cmd_test"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/exec/commands/cmd_test":
			pollCount++
			if pollCount == 1 {
				_, _ = w.Write([]byte(`{"status":"ok","command":{"id":"cmd_test","state":"running","ok":false}}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"ok","command":{"id":"cmd_test","state":"done","ok":true,"duration_ms":12,"prints":[{"level":"print","text":"hello","at_ms":1}],"returns":["Workspace"],"error":"","traceback":"","late_result":false}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := remoteExecClient{
		baseURL:      server.URL,
		token:        "secret",
		httpClient:   server.Client(),
		pollInterval: time.Millisecond,
	}
	commandID, err := client.submit(context.Background(), "print(1)", 15)
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
	if commandID != "cmd_test" {
		t.Fatalf("commandID = %q, want cmd_test", commandID)
	}
	result, err := client.wait(context.Background(), commandID)
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}
	if result.State != "done" || !result.OK || len(result.Prints) != 1 || len(result.Returns) != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestRemoteExecClientServerErrorAndWaitTimeout(t *testing.T) {
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"status":"error","message":"remote exec is disabled"}`, http.StatusForbidden)
	}))
	defer errorServer.Close()
	client := remoteExecClient{baseURL: errorServer.URL, token: "secret", httpClient: errorServer.Client(), pollInterval: time.Millisecond}
	if _, err := client.submit(context.Background(), "print(1)", 10); err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("submit error = %v, want HTTP 403", err)
	}

	runningServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","command":{"id":"cmd_wait","state":"running","ok":false}}`))
	}))
	defer runningServer.Close()
	client = remoteExecClient{baseURL: runningServer.URL, token: "secret", httpClient: runningServer.Client(), pollInterval: time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, err := client.wait(ctx, "cmd_wait"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait error = %v, want context deadline exceeded", err)
	}
}

func TestWriteExecOutputFormats(t *testing.T) {
	result := execCommandResult{
		ID:         "cmd_test",
		State:      "done",
		OK:         true,
		DurationMS: 12,
		Prints:     []execPrint{{Level: "print", Text: "hello"}, {Level: "warn", Text: "careful"}},
		Returns:    []any{"Workspace", float64(3)},
	}

	var human bytes.Buffer
	if err := writeExecOutput(&human, result, execOptions{}); err != nil {
		t.Fatalf("write human: %v", err)
	}
	if got := human.String(); got != "[print] hello\n[warn] careful\n[return] Workspace, 3\n[ok] 12ms\n" {
		t.Fatalf("human output = %q", got)
	}

	var raw bytes.Buffer
	if err := writeExecOutput(&raw, result, execOptions{rawOutput: true}); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	if got := raw.String(); got != "hello\ncareful\nWorkspace, 3\n" {
		t.Fatalf("raw output = %q", got)
	}

	var jsonOut bytes.Buffer
	if err := writeExecOutput(&jsonOut, result, execOptions{jsonOutput: true}); err != nil {
		t.Fatalf("write json: %v", err)
	}
	var decoded execCommandResult
	if err := json.Unmarshal(jsonOut.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json output: %v", err)
	}
	if decoded.ID != result.ID || decoded.State != result.State || !decoded.OK {
		t.Fatalf("json output decoded = %#v", decoded)
	}
}

func TestExecExitCodeMapping(t *testing.T) {
	cases := []struct {
		result execCommandResult
		want   int
	}{
		{result: execCommandResult{State: "done", OK: true}, want: exitOK},
		{result: execCommandResult{State: "done", OK: false}, want: exitCommandError},
		{result: execCommandResult{State: "error"}, want: exitCommandError},
		{result: execCommandResult{State: "expired"}, want: exitCommandTimedOut},
		{result: execCommandResult{State: "running"}, want: exitServerError},
	}
	for _, tc := range cases {
		if got := execExitCode(tc.result); got != tc.want {
			t.Fatalf("execExitCode(%#v) = %d, want %d", tc.result, got, tc.want)
		}
	}
}

func TestRunExecCommandAgainstMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/exec/commands":
			if r.Header.Get("Authorization") != "Bearer secret" {
				t.Fatalf("Authorization = %q, want bearer token", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"status":"ok","command_id":"cmd_run"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/exec/commands/cmd_run":
			_, _ = w.Write([]byte(`{"status":"ok","command":{"id":"cmd_run","state":"done","ok":true,"duration_ms":1,"prints":[],"returns":["ok"],"error":"","traceback":"","late_result":false}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	configPath := writeExecTestConfig(t, server.URL, "secret")
	options, err := parseOptions([]string{"--config", configPath, "exec", "return", `"ok"`})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := runExecCommand(context.Background(), options, strings.NewReader(""), &stdout, &stderr, server.Client())
	if code != exitOK {
		t.Fatalf("code = %d stderr=%q stdout=%q, want 0", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "[return] ok") {
		t.Fatalf("stdout = %q, want return output", stdout.String())
	}
}

func TestRunExecCommandMapsSubmitFailureToServerExitCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"status":"error","message":"remote exec is disabled"}`, http.StatusForbidden)
	}))
	defer server.Close()

	configPath := writeExecTestConfig(t, server.URL, "secret")
	options, err := parseOptions([]string{"--config", configPath, "exec", "print(1)"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := runExecCommand(context.Background(), options, strings.NewReader(""), &stdout, &stderr, server.Client())
	if code != exitServerError {
		t.Fatalf("code = %d stderr=%q stdout=%q, want server error exit", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "HTTP 403") {
		t.Fatalf("stderr = %q, want HTTP 403", stderr.String())
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

func writeExecTestConfig(t *testing.T, serverURL, token string) string {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}
	body := fmt.Sprintf(`{
  "host": %q,
  "port": %d,
  "sync_root": %q,
  "remote_exec_enabled": true,
  "remote_exec_token": %q
}`, parsed.Hostname(), port, filepath.ToSlash(t.TempDir()), token)
	path := filepath.Join(t.TempDir(), "sync_config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write exec test config: %v", err)
	}
	return path
}
