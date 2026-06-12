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
	"sort"
	"strings"
	"syscall"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/exechistory"
	"riftsync/internal/records"
	"riftsync/internal/scanner"
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
	validate         validateOptions
}

type execOptions struct {
	useStdin        bool
	useLast         bool
	filePath        string
	timeoutSec      int
	timeoutExplicit bool
	jsonOutput      bool
	rawOutput       bool
	inline          string
}

type execSourceRequest struct {
	Source     string
	SourceKind string
	FilePath   string
	RerunOfID  string
	TimeoutSec int
}

type validateOptions struct {
	jsonOutput bool
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
	if options.command == "validate" {
		os.Exit(runValidateCommand(options, os.Stdout, os.Stderr))
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
		switch remaining[0] {
		case "exec":
			execOptions, err := parseExecOptions(remaining[1:])
			if err != nil {
				return cliOptions{}, err
			}
			options.command = "exec"
			options.exec = execOptions
		case "validate":
			validateOptions, err := parseValidateOptions(remaining[1:])
			if err != nil {
				return cliOptions{}, err
			}
			options.command = "validate"
			options.validate = validateOptions
		default:
			return cliOptions{}, fmt.Errorf("unknown command: %s", remaining[0])
		}
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
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Validate:")
	fmt.Fprintln(out, "  riftsync-server [global flags] validate [--json]")
}

func parseValidateOptions(args []string) (validateOptions, error) {
	options := validateOptions{}
	flags := flag.NewFlagSet("riftsync-server validate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&options.jsonOutput, "json", false, "print JSON validation report")
	if err := flags.Parse(args); err != nil {
		return validateOptions{}, err
	}
	if len(flags.Args()) > 0 {
		return validateOptions{}, fmt.Errorf("validate does not accept positional arguments")
	}
	return options, nil
}

func parseExecOptions(args []string) (execOptions, error) {
	options := execOptions{}
	flags := flag.NewFlagSet("riftsync-server exec", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&options.useStdin, "stdin", false, "read Luau source from stdin")
	flags.BoolVar(&options.useLast, "last", false, "rerun the latest Remote Exec command from history")
	flags.StringVar(&options.filePath, "file", "", "read Luau source from file")
	flags.IntVar(&options.timeoutSec, "timeout", 0, "command timeout in seconds")
	flags.BoolVar(&options.jsonOutput, "json", false, "print JSON output")
	flags.BoolVar(&options.rawOutput, "raw", false, "print raw output")
	options.timeoutExplicit = hasTimeoutArg(args)
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
	if options.useLast {
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
		if arg == "--stdin" || arg == "--last" || arg == "--file" || strings.HasPrefix(arg, "--file=") {
			return true
		}
	}
	return false
}

func hasTimeoutArg(args []string) bool {
	for _, arg := range args {
		if arg == "--timeout" || strings.HasPrefix(arg, "--timeout=") {
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

type validateReport struct {
	Status     string         `json:"status"`
	ConfigPath string         `json:"config_path"`
	SyncRoot   string         `json:"sync_root"`
	Errors     []string       `json:"errors"`
	Warnings   []string       `json:"warnings"`
	Counts     map[string]int `json:"counts"`
}

func runValidateCommand(options cliOptions, stdout, stderr io.Writer) int {
	report, err := validateProject(options)
	if err != nil {
		if options.validate.jsonOutput {
			_ = json.NewEncoder(stdout).Encode(validateReport{
				Status: "cli_error",
				Errors: []string{err.Error()},
				Counts: map[string]int{},
			})
		} else {
			fmt.Fprintf(stderr, "validation setup error: %v\n", err)
		}
		return exitArgumentError
	}
	if options.validate.jsonOutput {
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			fmt.Fprintf(stderr, "validation output error: %v\n", err)
			return exitArgumentError
		}
	} else {
		writeValidateHuman(stdout, report)
	}
	if len(report.Errors) > 0 {
		return exitCommandError
	}
	return exitOK
}

func validateProject(options cliOptions) (validateReport, error) {
	cfg, err := config.Load(options.configPath)
	if err != nil {
		return validateReport{}, fmt.Errorf("config error: %w", err)
	}
	if options.hostOverride != "" {
		cfg.Host = options.hostOverride
	}
	if options.portOverride > 0 {
		cfg.Port = options.portOverride
	}
	if options.syncRootOverride != "" {
		cfg.SyncRoot = options.syncRootOverride
	}
	if err := cfg.NormalizeAndValidate(); err != nil {
		return validateReport{}, fmt.Errorf("config error: %w", err)
	}

	report := validateReport{
		Status:     "ok",
		ConfigPath: cfg.ConfigPathAbs,
		SyncRoot:   cfg.SyncRootAbs,
		Counts: map[string]int{
			"scripts":                  0,
			"ui":                       0,
			"entries":                  0,
			"invalid_json":             0,
			"duplicate_stable_ids":     0,
			"unsupported_paths":        0,
			"ignored_metadata_folders": countIgnoredMetadataFolders(cfg.SyncRootAbs),
			"warnings":                 0,
			"errors":                   0,
		},
	}
	if strings.TrimSpace(report.ConfigPath) == "" {
		if abs, absErr := filepath.Abs(options.configPath); absErr == nil {
			report.ConfigPath = filepath.Clean(abs)
		}
	}

	if info, statErr := os.Stat(cfg.SyncRootAbs); statErr != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("sync_root is not accessible: %v", statErr))
		finalizeValidateReport(&report)
		return report, nil
	} else if !info.IsDir() {
		report.Errors = append(report.Errors, "sync_root is not a directory")
		finalizeValidateReport(&report)
		return report, nil
	}
	if _, traversalErr := cfg.ResolveInsideSyncRoot("../__riftsync_validate_outside__"); traversalErr == nil {
		report.Errors = append(report.Errors, "path traversal safety check failed")
	}

	cache := scanner.NewCache()
	snapshot, scanErr := cache.Scan(cfg)
	if scanErr != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("scanner failed: %v", scanErr))
		finalizeValidateReport(&report)
		return report, nil
	}
	report.Counts["scripts"] = snapshot.ScriptCount
	report.Counts["ui"] = snapshot.UICount
	report.Counts["entries"] = len(snapshot.Records)

	for _, invalidPath := range snapshot.InvalidPaths {
		report.Errors = append(report.Errors, "invalid metadata JSON: "+invalidPath)
	}
	for _, warning := range snapshot.Warnings {
		if strings.Contains(strings.ToLower(warning), "duplicate stable id") {
			report.Errors = append(report.Errors, warning)
			continue
		}
		if strings.Contains(strings.ToLower(warning), "invalid json") {
			continue
		}
		report.Warnings = append(report.Warnings, warning)
	}

	unsupported, err := findUnsupportedSyncPaths(cfg, snapshot)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("unsupported path scan incomplete: %v", err))
	}
	for _, path := range unsupported {
		report.Warnings = append(report.Warnings, "unsupported sync path: "+path)
	}
	report.Counts["invalid_json"] = len(snapshot.InvalidPaths)
	report.Counts["duplicate_stable_ids"] = countDuplicateStableIDErrors(report.Errors)
	report.Counts["unsupported_paths"] = len(unsupported)
	finalizeValidateReport(&report)
	return report, nil
}

func finalizeValidateReport(report *validateReport) {
	report.Counts["warnings"] = len(report.Warnings)
	report.Counts["errors"] = len(report.Errors)
	switch {
	case len(report.Errors) > 0:
		report.Status = "error"
	case len(report.Warnings) > 0:
		report.Status = "warning"
	default:
		report.Status = "ok"
	}
}

func writeValidateHuman(out io.Writer, report validateReport) {
	fmt.Fprintln(out, "RiftSync validate")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "OK:")
	if report.ConfigPath != "" {
		fmt.Fprintf(out, "- Config: %s\n", report.ConfigPath)
	}
	if report.SyncRoot != "" {
		fmt.Fprintf(out, "- Sync root: %s\n", report.SyncRoot)
	}
	fmt.Fprintf(out, "- Scripts: %d\n", report.Counts["scripts"])
	fmt.Fprintf(out, "- UI metadata: %d\n", report.Counts["ui"])
	fmt.Fprintf(out, "- Ignored metadata folders: %d\n", report.Counts["ignored_metadata_folders"])

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Warnings:")
	if len(report.Warnings) == 0 {
		fmt.Fprintln(out, "- None")
	} else {
		for _, warning := range report.Warnings {
			fmt.Fprintln(out, "- "+warning)
		}
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Errors:")
	if len(report.Errors) == 0 {
		fmt.Fprintln(out, "- None")
	} else {
		for _, validationError := range report.Errors {
			fmt.Fprintln(out, "- "+validationError)
		}
	}

	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Summary: status=%s entries=%d warnings=%d errors=%d\n",
		report.Status,
		report.Counts["entries"],
		report.Counts["warnings"],
		report.Counts["errors"],
	)
}

func findUnsupportedSyncPaths(cfg config.Config, snapshot scanner.Snapshot) ([]string, error) {
	unsupported := []string{}
	err := filepath.WalkDir(cfg.SyncRootAbs, func(absPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == config.MetadataDir || entry.Name() == config.GuidebookDir {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(cfg.SyncRootAbs, absPath)
		if err != nil {
			return nil
		}
		relPosix := filepath.ToSlash(relative)
		if _, ok := snapshot.Records[relPosix]; ok {
			return nil
		}
		if containsString(snapshot.InvalidPaths, relPosix) {
			return nil
		}
		if looksLikeUnsupportedSyncPath(relPosix) {
			unsupported = append(unsupported, relPosix)
		}
		return nil
	})
	sort.Strings(unsupported)
	return unsupported, err
}

func looksLikeUnsupportedSyncPath(relPosix string) bool {
	filename := filepath.Base(filepath.FromSlash(relPosix))
	lower := strings.ToLower(filename)
	if lower == records.UIPropertiesFilename || lower == records.UIInitMetaFilename || strings.HasSuffix(lower, records.UIModelJSONSuffix) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(lower))
	if ext == ".lua" || ext == ".luau" {
		return true
	}
	return false
}

func countIgnoredMetadataFolders(syncRoot string) int {
	count := 0
	for _, name := range []string{".git", config.MetadataDir, config.GuidebookDir} {
		if info, err := os.Stat(filepath.Join(syncRoot, name)); err == nil && info.IsDir() {
			count++
		}
	}
	return count
}

func countDuplicateStableIDErrors(errors []string) int {
	count := 0
	for _, item := range errors {
		if strings.Contains(strings.ToLower(item), "duplicate stable id") {
			count++
		}
	}
	return count
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
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
	if options.syncRootOverride != "" {
		cfg.SyncRoot = options.syncRootOverride
	}
	if err := cfg.NormalizeAndValidate(); err != nil {
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return exitArgumentError
	}

	sourceRequest, err := readExecSourceRequest(options.exec, stdin, cfg.SyncRootAbs)
	if err != nil {
		fmt.Fprintf(stderr, "argument error: %v\n", err)
		return exitArgumentError
	}

	timeoutSec := options.exec.timeoutSec
	if options.exec.useLast && !options.exec.timeoutExplicit {
		timeoutSec = sourceRequest.TimeoutSec
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

	if client == nil {
		client = http.DefaultClient
	}
	execClient := remoteExecClient{
		baseURL:      fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port),
		token:        cfg.RemoteExecToken,
		httpClient:   client,
		pollInterval: 200 * time.Millisecond,
	}
	commandStarted := time.Now()
	commandID, err := execClient.submit(ctx, sourceRequest.Source, timeoutSec)
	if err != nil {
		recordExecHistory(cfg.SyncRootAbs, sourceRequest, timeoutSec, execCommandResult{
			State: "submit_failed",
			OK:    false,
			Error: err.Error(),
		}, "")
		fmt.Fprintf(stderr, "exec submit failed: %v\n", err)
		return exitServerError
	}

	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec+5)*time.Second)
	defer cancel()
	result, err := execClient.wait(waitCtx, commandID)
	if err != nil {
		state := "wait_failed"
		exitCode := exitServerError
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			state = "timeout"
			exitCode = exitCommandTimedOut
		}
		recordExecHistory(cfg.SyncRootAbs, sourceRequest, timeoutSec, execCommandResult{
			ID:         commandID,
			State:      state,
			OK:         false,
			Error:      err.Error(),
			DurationMS: float64(time.Since(commandStarted).Milliseconds()),
		}, commandID)
		if exitCode == exitCommandTimedOut {
			fmt.Fprintln(stderr, "exec timed out waiting for result")
			return exitCommandTimedOut
		}
		fmt.Fprintf(stderr, "exec wait failed: %v\n", err)
		return exitServerError
	}

	recordExecHistory(cfg.SyncRootAbs, sourceRequest, timeoutSec, result, commandID)
	if err := writeExecOutput(stdout, result, options.exec); err != nil {
		fmt.Fprintf(stderr, "exec output failed: %v\n", err)
		return exitServerError
	}
	return execExitCode(result)
}

func readExecSourceRequest(options execOptions, stdin io.Reader, syncRoot string) (execSourceRequest, error) {
	if options.useLast {
		file, warning, err := exechistory.Load(syncRoot)
		if err != nil {
			return execSourceRequest{}, err
		}
		if warning != "" {
			return execSourceRequest{}, fmt.Errorf("%s", warning)
		}
		entry, ok := exechistory.LatestUsable(file)
		if !ok {
			return execSourceRequest{}, fmt.Errorf("no usable Remote Exec history")
		}
		return execSourceRequest{
			Source:     entry.Source,
			SourceKind: firstNonEmpty(entry.SourceKind, "inline"),
			FilePath:   entry.FilePath,
			RerunOfID:  entry.ID,
			TimeoutSec: entry.TimeoutSec,
		}, nil
	}

	source, err := readExecSource(options, stdin)
	if err != nil {
		return execSourceRequest{}, err
	}
	sourceKind := "inline"
	filePath := ""
	switch {
	case options.useStdin:
		sourceKind = "stdin"
	case strings.TrimSpace(options.filePath) != "":
		sourceKind = "file"
		filePath = options.filePath
	}
	return execSourceRequest{
		Source:     source,
		SourceKind: sourceKind,
		FilePath:   filePath,
	}, nil
}

func recordExecHistory(syncRoot string, sourceRequest execSourceRequest, timeoutSec int, result execCommandResult, commandID string) {
	if strings.TrimSpace(syncRoot) == "" || strings.TrimSpace(sourceRequest.Source) == "" {
		return
	}
	if strings.TrimSpace(commandID) == "" {
		commandID = result.ID
	}
	_, _, _ = exechistory.Append(syncRoot, exechistory.Entry{
		SubmittedBy: "cli",
		SourceKind:  sourceRequest.SourceKind,
		Source:      sourceRequest.Source,
		FilePath:    sourceRequest.FilePath,
		TimeoutSec:  timeoutSec,
		ResultState: result.State,
		OK:          result.OK,
		Error:       result.Error,
		DurationMS:  result.DurationMS,
		CommandID:   commandID,
		RerunOfID:   sourceRequest.RerunOfID,
	}, exechistory.DefaultLimit)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
