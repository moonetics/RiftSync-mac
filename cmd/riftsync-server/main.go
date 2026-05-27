package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"riftsync/internal/serverapp"
	"riftsync/internal/uiapp"
)

type cliOptions struct {
	configPath       string
	hostOverride     string
	portOverride     int
	syncRootOverride string
	debug            bool
	legacyScan       bool
	headless         bool
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
