package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/serverapp"
	"riftsync/internal/uiapp"
)

const (
	exitOK              = 0
	exitCommandError    = 1
	exitArgumentError   = 2
	exitServerError     = 3
	exitCommandTimedOut = 4
)

type cliOptions struct {
	configPath       string
	hostOverride     string
	portOverride     int
	syncRootOverride string
	debug            bool
	legacyScan       bool
	headless         bool
	command          string
	exec             execOptions
}

type execOptions struct {
	useStdin   bool
	filePath   string
	timeoutSec int
	jsonOutput bool
	rawOutput  bool
	inline     string
}

func main() {
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		if err == flag.ErrHelp {
			printUsage(os.Stdout)
			return
		}
		fmt.Fprintf(os.Stderr, "argument error: %v\n", err)
		os.Exit(2)
	}
	if options.command == "exec" {
		os.Exit(runExecCommand(context.Background(), options, os.Stdin, os.Stdout, os.Stderr, nil))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var runErr error
	if options.headless {
		runErr = runHeadless(ctx, options, os.Stdout)
	} else {
		runErr = uiapp.Run(ctx, toServerOptions(options, os.Stdout))
	}
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "%v\n", runErr)
		os.Exit(1)
	}
}

func parseOptions(args []string) (cliOptions, error) {
	options := cliOptions{
		configPath:   "sync_config.json",
		portOverride: -1,
	}
	flags := flag.NewFlagSet("riftsync-server", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.configPath, "config", options.configPath, "path to sync_config.json")
	flags.StringVar(&options.hostOverride, "host", "", "override local bind host: 127.0.0.1 or localhost")
	flags.IntVar(&options.portOverride, "port", options.portOverride, "override server port")
	flags.StringVar(&options.syncRootOverride, "sync-root", "", "override sync root path")
	flags.BoolVar(&options.debug, "debug", false, "print verbose startup and runtime diagnostics")
	flags.BoolVar(&options.legacyScan, "legacy-scan", false, "use periodic full scan instead of fsnotify watcher")
	flags.BoolVar(&options.headless, "headless", false, "run console server without native UI")
	if err := flags.Parse(args); err != nil {
		return cliOptions{}, err
	}
	remaining := flags.Args()
	if len(remaining) > 0 {
		if remaining[0] != "exec" {
			return cliOptions{}, fmt.Errorf("unknown command: %s", remaining[0])
		}
		execOptions, err := parseExecOptions(remaining[1:])
		if err != nil {
			return cliOptions{}, err
		}
		options.command = "exec"
		options.exec = execOptions
	}
	if options.configPath == "" {
		return cliOptions{}, fmt.Errorf("--config must not be empty")
	}
	if options.portOverride != -1 && (options.portOverride < 1 || options.portOverride > 65535) {
		return cliOptions{}, fmt.Errorf("--port must be between 1 and 65535, got %d", options.portOverride)
	}
	if options.hostOverride != "" && options.hostOverride != "127.0.0.1" && options.hostOverride != "localhost" {
		return cliOptions{}, fmt.Errorf("--host must be 127.0.0.1 or localhost")
	}
	return options, nil
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: riftsync-server [--headless] [--config sync_config.json] [--host 127.0.0.1] [--port 8765] [--sync-root src/game] [--debug] [--legacy-scan]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Flags:")
	fmt.Fprintln(out, "  --headless     Run console server without native UI")
	fmt.Fprintln(out, "  --config       Path to sync_config.json (default sync_config.json)")
	fmt.Fprintln(out, "  --host         Override local bind host")
	fmt.Fprintln(out, "  --port         Override server port from config")
	fmt.Fprintln(out, "  --sync-root    Override sync root from config")
	fmt.Fprintln(out, "  --debug        Print verbose startup and runtime diagnostics")
	fmt.Fprintln(out, "  --legacy-scan  Use periodic full scan instead of fsnotify watcher")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Exec:")
	fmt.Fprintln(out, "  riftsync-server [global flags] exec [--stdin | --file path | path.lua | inline source] [--timeout seconds] [--json | --raw]")
}

func parseExecOptions(args []string) (execOptions, error) {
	options := execOptions{}
	flags := flag.NewFlagSet("riftsync-server exec", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&options.useStdin, "stdin", false, "read Luau source from stdin")
	flags.StringVar(&options.filePath, "file", "", "read Luau source from file")
	flags.IntVar(&options.timeoutSec, "timeout", 0, "command timeout in seconds")
	flags.BoolVar(&options.jsonOutput, "json", false, "print JSON output")
	flags.BoolVar(&options.rawOutput, "raw", false, "print raw output")
	if err := flags.Parse(normalizeExecShortcutArgs(args)); err != nil {
		return execOptions{}, err
	}
	if options.timeoutSec < 0 {
		return execOptions{}, fmt.Errorf("--timeout must be positive")
	}
	if options.jsonOutput && options.rawOutput {
		return execOptions{}, fmt.Errorf("--json and --raw cannot be used together")
	}

	inlineArgs := flags.Args()
	if !options.useStdin && strings.TrimSpace(options.filePath) == "" && len(inlineArgs) == 1 && isLuaExecFilePath(inlineArgs[0]) {
		options.filePath = inlineArgs[0]
		inlineArgs = nil
	}
	hasInline := len(inlineArgs) > 0
	sourceModes := 0
	if options.useStdin {
		sourceModes++
	}
	if strings.TrimSpace(options.filePath) != "" {
		sourceModes++
	}
	if hasInline {
		sourceModes++
	}
	if sourceModes != 1 {
		return execOptions{}, fmt.Errorf("provide exactly one source: inline command, --stdin, or --file")
	}
	if hasInline {
		options.inline = strings.Join(inlineArgs, " ")
	}
	return options, nil
}

func isLuaExecFilePath(value string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(value)))
	return ext == ".lua" || ext == ".luau"
}

func normalizeExecShortcutArgs(args []string) []string {
	if hasExplicitExecSourceFlag(args) {
		return args
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return args
		case arg == "--timeout":
			i++
			continue
		case strings.HasPrefix(arg, "--timeout="), arg == "--json", arg == "--raw":
			continue
		case strings.HasPrefix(arg, "-"):
			return args
		case isLuaExecFilePath(arg):
			normalized := make([]string, 0, len(args)+1)
			normalized = append(normalized, args[:i]...)
			normalized = append(normalized, "--file", arg)
			normalized = append(normalized, args[i+1:]...)
			return normalized
		default:
			return args
		}
	}
	return args
}

func hasExplicitExecSourceFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--stdin" || arg == "--file" || strings.HasPrefix(arg, "--file=") {
			return true
		}
	}
	return false
}

func runHeadless(ctx context.Context, options cliOptions, stdout io.Writer) error {
	runner := serverapp.New(toServerOptions(options, stdout))
	fmt.Fprintf(stdout, "RiftSync Go server %s\n", serverapp.Version)
	if err := runner.Start(ctx); err != nil {
		return err
	}
	status := runner.Status(5)
	fmt.Fprintf(stdout, "[sync] server listening on http://%s:%d\n", status.Host, status.Port)
	fmt.Fprintf(stdout, "[sync] sync root: %s\n", status.SyncRoot)
	if status.LegacyScan {
		fmt.Fprintln(stdout, "[sync] legacy scan active")
	} else {
		fmt.Fprintln(stdout, "[sync] watcher active")
	}
	<-ctx.Done()
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return runner.Stop(stopCtx)
}

type execHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type remoteExecClient struct {
	baseURL      string
	token        string
	httpClient   execHTTPClient
	pollInterval time.Duration
}

type execPrint struct {
	Level string  `json:"level"`
	Text  string  `json:"text"`
	AtMS  float64 `json:"at_ms"`
}

type execCommandResult struct {
	ID         string      `json:"id"`
	State      string      `json:"state"`
	OK         bool        `json:"ok"`
	DurationMS float64     `json:"duration_ms"`
	Prints     []execPrint `json:"prints"`
	Returns    []any       `json:"returns"`
	Error      string      `json:"error"`
	Traceback  string      `json:"traceback"`
	LateResult bool        `json:"late_result"`
}

type execSubmitResponse struct {
	Status    string `json:"status"`
	CommandID string `json:"command_id"`
	Message   string `json:"message"`
}

type execDetailResponse struct {
	Status  string            `json:"status"`
	Command execCommandResult `json:"command"`
	Message string            `json:"message"`
}

func runExecCommand(ctx context.Context, options cliOptions, stdin io.Reader, stdout, stderr io.Writer, client execHTTPClient) int {
	source, err := readExecSource(options.exec, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "argument error: %v\n", err)
		return exitArgumentError
	}

	cfg, err := config.Load(options.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return exitArgumentError
	}
	if options.hostOverride != "" {
		cfg.Host = options.hostOverride
	}
	if options.portOverride > 0 {
		cfg.Port = options.portOverride
	}
	if err := cfg.NormalizeAndValidate(); err != nil {
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return exitArgumentError
	}

	timeoutSec := options.exec.timeoutSec
	if timeoutSec <= 0 {
		timeoutSec = cfg.RemoteExecDefaultTimeoutSec
	}
	if timeoutSec < 1 {
		timeoutSec = 1
	}
	if timeoutSec > cfg.RemoteExecMaxTimeoutSec {
		timeoutSec = cfg.RemoteExecMaxTimeoutSec
	}

	if client == nil {
		client = http.DefaultClient
	}
	execClient := remoteExecClient{
		baseURL:      fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port),
		token:        cfg.RemoteExecToken,
		httpClient:   client,
		pollInterval: 200 * time.Millisecond,
	}
	commandID, err := execClient.submit(ctx, source, timeoutSec)
	if err != nil {
		fmt.Fprintf(stderr, "exec submit failed: %v\n", err)
		return exitServerError
	}

	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec+5)*time.Second)
	defer cancel()
	result, err := execClient.wait(waitCtx, commandID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			fmt.Fprintln(stderr, "exec timed out waiting for result")
			return exitCommandTimedOut
		}
		fmt.Fprintf(stderr, "exec wait failed: %v\n", err)
		return exitServerError
	}

	if err := writeExecOutput(stdout, result, options.exec); err != nil {
		fmt.Fprintf(stderr, "exec output failed: %v\n", err)
		return exitServerError
	}
	return execExitCode(result)
}

func readExecSource(options execOptions, stdin io.Reader) (string, error) {
	var body []byte
	var err error
	switch {
	case options.useStdin:
		body, err = io.ReadAll(stdin)
	case options.filePath != "":
		body, err = os.ReadFile(options.filePath)
	default:
		body = []byte(options.inline)
	}
	if err != nil {
		return "", err
	}
	source := string(body)
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("source must not be empty")
	}
	return source, nil
}

func (c remoteExecClient) submit(ctx context.Context, source string, timeoutSec int) (string, error) {
	payload := map[string]any{
		"source":      source,
		"timeout_sec": timeoutSec,
		"client":      "riftsync-cli",
		"mode":        "edit",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/exec/commands", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyAuth(req)

	var response execSubmitResponse
	if err := c.doJSON(req, &response); err != nil {
		return "", err
	}
	if response.CommandID == "" {
		return "", fmt.Errorf("server returned empty command_id")
	}
	return response.CommandID, nil
}

func (c remoteExecClient) wait(ctx context.Context, commandID string) (execCommandResult, error) {
	interval := c.pollInterval
	if interval <= 0 {
		interval = 200 * time.Millisecond
	}
	for {
		result, err := c.detail(ctx, commandID)
		if err != nil {
			return execCommandResult{}, err
		}
		if isExecTerminalState(result.State) {
			return result, nil
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return execCommandResult{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func (c remoteExecClient) detail(ctx context.Context, commandID string) (execCommandResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/exec/commands/"+commandID, nil)
	if err != nil {
		return execCommandResult{}, err
	}
	c.applyAuth(req)

	var response execDetailResponse
	if err := c.doJSON(req, &response); err != nil {
		return execCommandResult{}, err
	}
	if response.Command.ID == "" {
		response.Command.ID = commandID
	}
	return response.Command, nil
}

func (c remoteExecClient) doJSON(req *http.Request, target any) error {
	response, err := c.httpClient.Do(req)
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

func (c remoteExecClient) applyAuth(req *http.Request) {
	if strings.TrimSpace(c.token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.token))
	}
}

func writeExecOutput(out io.Writer, result execCommandResult, options execOptions) error {
	if options.jsonOutput {
		return json.NewEncoder(out).Encode(result)
	}
	if options.rawOutput {
		writeExecRaw(out, result)
		return nil
	}
	writeExecHuman(out, result)
	return nil
}

func writeExecHuman(out io.Writer, result execCommandResult) {
	for _, entry := range result.Prints {
		level := strings.TrimSpace(entry.Level)
		if level == "" {
			level = "print"
		}
		fmt.Fprintf(out, "[%s] %s\n", level, entry.Text)
	}
	if len(result.Returns) > 0 {
		fmt.Fprintf(out, "[return] %s\n", joinAny(result.Returns))
	}
	if result.Error != "" {
		fmt.Fprintf(out, "[error] %s\n", result.Error)
	}
	if result.Traceback != "" {
		fmt.Fprintf(out, "[traceback] %s\n", result.Traceback)
	}
	if result.State == "done" && result.OK {
		fmt.Fprintf(out, "[ok] %sms\n", formatMillis(result.DurationMS))
	}
}

func writeExecRaw(out io.Writer, result execCommandResult) {
	for _, entry := range result.Prints {
		fmt.Fprintln(out, entry.Text)
	}
	if len(result.Returns) > 0 {
		fmt.Fprintln(out, joinAny(result.Returns))
	}
	if result.Error != "" {
		fmt.Fprintln(out, result.Error)
	}
	if result.Traceback != "" {
		fmt.Fprintln(out, result.Traceback)
	}
}

func execExitCode(result execCommandResult) int {
	switch result.State {
	case "done":
		if result.OK {
			return exitOK
		}
		return exitCommandError
	case "error":
		return exitCommandError
	case "timeout", "expired", "cancelled":
		return exitCommandTimedOut
	default:
		return exitServerError
	}
}

func isExecTerminalState(state string) bool {
	switch state {
	case "done", "error", "timeout", "expired", "cancelled":
		return true
	default:
		return false
	}
}

func joinAny(values []any) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprint(value))
	}
	return strings.Join(parts, ", ")
}

func formatMillis(value float64) string {
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d", int64(value))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
}

func toServerOptions(options cliOptions, stdout io.Writer) serverapp.Options {
	return serverapp.Options{
		ConfigPath:       options.configPath,
		HostOverride:     options.hostOverride,
		PortOverride:     options.portOverride,
		SyncRootOverride: options.syncRootOverride,
		Debug:            options.debug,
		LegacyScan:       options.legacyScan,
		Stdout:           stdout,
	}
}
