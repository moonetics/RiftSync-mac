package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Host                    string              `json:"host"`
	Port                    int                 `json:"port"`
	SyncRoot                string              `json:"sync_root"`
	SyncRootAbs             string              `json:"-"`
	ScanIntervalSec         float64             `json:"scan_interval_sec"`
	PollTimeoutSec          int                 `json:"poll_timeout_sec"`
	ChangeRetention         int                 `json:"change_retention"`
	DebugEventRetention     int                 `json:"debug_event_retention"`
	InvalidJSONGraceSec     float64             `json:"invalid_json_grace_sec"`
	GitVersioningEnabled    bool                `json:"git_versioning_enabled"`
	StrictPropertyWhitelist bool                `json:"strict_property_whitelist"`
	ExtraAllowedProperties  map[string][]string `json:"extra_allowed_properties"`
	ManagedRoots            []string            `json:"managed_roots"`
	IgnoredRbxPaths         []string            `json:"ignored_rbx_paths"`
}

const MetadataDir = ".rblxsync"

func Default() Config {
	return Config{
		Host:                    "127.0.0.1",
		Port:                    8765,
		SyncRoot:                "src/game",
		ScanIntervalSec:         0.4,
		PollTimeoutSec:          25,
		ChangeRetention:         2048,
		DebugEventRetention:     300,
		InvalidJSONGraceSec:     1.2,
		GitVersioningEnabled:    true,
		StrictPropertyWhitelist: false,
		ExtraAllowedProperties:  map[string][]string{},
		ManagedRoots: []string{
			"game.Workspace",
			"game.ReplicatedFirst",
			"game.ReplicatedStorage",
			"game.ServerScriptService",
			"game.ServerStorage",
			"game.StarterGui",
			"game.StarterPack",
			"game.StarterPlayer",
			"game.Lighting",
			"game.SoundService",
			"game.TextChatService",
		},
		IgnoredRbxPaths: []string{
			"game.ServerScriptService.RiftSyncPlugin",
			"game.ServerScriptService.RBXLSyncPlugin",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	body, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("read %s: %w", path, err)
		}
	} else if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	if err := cfg.NormalizeAndValidate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := cfg.NormalizeAndValidate(); err != nil {
		return err
	}
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (c *Config) NormalizeAndValidate() error {
	c.Host = strings.TrimSpace(c.Host)
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Host != "127.0.0.1" && c.Host != "localhost" {
		return fmt.Errorf("host must be local-only: 127.0.0.1 or localhost, got %q", c.Host)
	}

	c.SyncRoot = strings.TrimSpace(c.SyncRoot)
	if c.SyncRoot == "" {
		return errors.New("sync_root must not be empty")
	}

	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if c.ScanIntervalSec <= 0 {
		return fmt.Errorf("scan_interval_sec must be positive, got %v", c.ScanIntervalSec)
	}
	if c.PollTimeoutSec <= 0 {
		return fmt.Errorf("poll_timeout_sec must be positive, got %d", c.PollTimeoutSec)
	}
	if c.ChangeRetention <= 0 {
		return fmt.Errorf("change_retention must be positive, got %d", c.ChangeRetention)
	}
	if c.DebugEventRetention <= 0 {
		return fmt.Errorf("debug_event_retention must be positive, got %d", c.DebugEventRetention)
	}
	if c.InvalidJSONGraceSec <= 0 {
		return fmt.Errorf("invalid_json_grace_sec must be positive, got %v", c.InvalidJSONGraceSec)
	}

	if c.ExtraAllowedProperties == nil {
		c.ExtraAllowedProperties = map[string][]string{}
	}
	c.ManagedRoots = normalizeStringList(c.ManagedRoots)
	c.IgnoredRbxPaths = normalizeStringList(c.IgnoredRbxPaths)

	abs, err := filepath.Abs(c.SyncRoot)
	if err != nil {
		return fmt.Errorf("resolve sync_root: %w", err)
	}
	c.SyncRootAbs = filepath.Clean(abs)

	return nil
}

func (c Config) ResolveInsideSyncRoot(relativeOrLocalPath string) (string, error) {
	if strings.TrimSpace(c.SyncRootAbs) == "" {
		return "", errors.New("sync_root_abs is not configured")
	}

	input := strings.TrimSpace(relativeOrLocalPath)
	if input == "" {
		return "", errors.New("path must not be empty")
	}

	var candidate string
	if filepath.IsAbs(input) {
		candidate = filepath.Clean(input)
	} else {
		candidate = filepath.Clean(filepath.Join(c.SyncRootAbs, filepath.FromSlash(input)))
	}

	rel, err := filepath.Rel(c.SyncRootAbs, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve path relative to sync_root: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes sync_root: %s", relativeOrLocalPath)
	}

	return candidate, nil
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
