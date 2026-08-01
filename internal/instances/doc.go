// Package instances persists the desktop application's project registry.
//
// It deliberately stores only desktop metadata and config paths. Each project
// keeps its existing sync_config.json contract so headless and CLI workflows
// remain independent from the multi-instance UI.
package instances
