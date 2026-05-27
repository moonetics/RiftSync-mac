package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"riftsync/internal/serverapp"
	"riftsync/internal/uiapp"
)

type appOptions struct {
	configPath       string
	hostOverride     string
	portOverride     int
	syncRootOverride string
	debug            bool
	legacyScan       bool
}

func main() {
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		if err == flag.ErrHelp {
			printUsage(os.Stdout)
			return
		}
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := uiapp.Run(ctx, toServerOptions(options)); err != nil {
		os.Exit(1)
	}
}

func parseOptions(args []string) (appOptions, error) {
	options := appOptions{
		configPath:   "sync_config.json",
		portOverride: -1,
	}
	flags := flag.NewFlagSet("riftsync", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.configPath, "config", options.configPath, "path to sync_config.json")
	flags.StringVar(&options.hostOverride, "host", "", "override local bind host: 127.0.0.1 or localhost")
	flags.IntVar(&options.portOverride, "port", options.portOverride, "override server port")
	flags.StringVar(&options.syncRootOverride, "sync-root", "", "override sync root path")
	flags.BoolVar(&options.debug, "debug", false, "enable debug mode")
	flags.BoolVar(&options.legacyScan, "legacy-scan", false, "use periodic full scan instead of fsnotify watcher")
	if err := flags.Parse(args); err != nil {
		return appOptions{}, err
	}
	if options.configPath == "" {
		return appOptions{}, fmt.Errorf("--config must not be empty")
	}
	if options.portOverride != -1 && (options.portOverride < 1 || options.portOverride > 65535) {
		return appOptions{}, fmt.Errorf("--port must be between 1 and 65535, got %d", options.portOverride)
	}
	if options.hostOverride != "" && options.hostOverride != "127.0.0.1" && options.hostOverride != "localhost" {
		return appOptions{}, fmt.Errorf("--host must be 127.0.0.1 or localhost")
	}
	return options, nil
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: riftsync [--config sync_config.json] [--host 127.0.0.1] [--port 8765] [--sync-root src/game] [--debug] [--legacy-scan]")
}

func toServerOptions(options appOptions) serverapp.Options {
	return serverapp.Options{
		ConfigPath:       options.configPath,
		HostOverride:     options.hostOverride,
		PortOverride:     options.portOverride,
		SyncRootOverride: options.syncRootOverride,
		Debug:            options.debug,
		LegacyScan:       options.legacyScan,
		Stdout:           io.Discard,
	}
}
