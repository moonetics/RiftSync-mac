package config

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Config struct {
	Host                        string              `json:"host"`
	Port                        int                 `json:"port"`
	SyncRoot                    string              `json:"sync_root"`
	SyncRootAbs                 string              `json:"-"`
	ConfigPathAbs               string              `json:"-"`
	ScanIntervalSec             float64             `json:"scan_interval_sec"`
	SafetyScanIntervalSec       float64             `json:"safety_scan_interval_sec,omitempty"`
	PollTimeoutSec              int                 `json:"poll_timeout_sec"`
	ChangeRetention             int                 `json:"change_retention"`
	DebugEventRetention         int                 `json:"debug_event_retention"`
	InvalidJSONGraceSec         float64             `json:"invalid_json_grace_sec"`
	GitVersioningEnabled        bool                `json:"git_versioning_enabled"`
	StrictPropertyWhitelist     bool                `json:"strict_property_whitelist"`
	ExtraAllowedProperties      map[string][]string `json:"extra_allowed_properties"`
	ManagedRoots                []string            `json:"managed_roots"`
	IgnoredRbxPaths             []string            `json:"ignored_rbx_paths"`
	RemoteExecEnabled           bool                `json:"remote_exec_enabled"`
	RemoteExecToken             string              `json:"remote_exec_token"`
	RemoteExecMaxSourceBytes    int                 `json:"remote_exec_max_source_bytes"`
	RemoteExecDefaultTimeoutSec int                 `json:"remote_exec_default_timeout_sec"`
	RemoteExecMaxTimeoutSec     int                 `json:"remote_exec_max_timeout_sec"`
}

const MetadataDir = ".rblxsync"
const GuidebookDir = ".guidebook"
const GuidebookExecLauncher = "riftsync-exec.ps1"
const GuidebookStatus = "status.json"
const RemoteExecTokenLength = 32

const remoteExecTokenAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

var BootstrapServiceDirs = []string{
	"ServerScriptService",
	"ServerStorage",
	"ReplicatedStorage",
	"StarterGui",
	"StarterPack",
	"StarterPlayer",
}

type ScaffoldOptions struct {
	ConfigPath        string
	ExecutablePath    string
	Version           string
	LastKnownRevision int
	GeneratedAt       time.Time
	WriteStatusJSON   bool
}

type GuidebookStatusPayload struct {
	SyncRoot             string    `json:"sync_root"`
	ConfigPath           string    `json:"config_path"`
	Host                 string    `json:"host"`
	Port                 int       `json:"port"`
	Version              string    `json:"version"`
	RemoteExecEnabled    bool      `json:"remote_exec_enabled"`
	RemoteExecConfigured bool      `json:"remote_exec_configured"`
	RemoteExecAvailable  bool      `json:"remote_exec_available"`
	SupportedFileFormats []string  `json:"supported_file_formats"`
	LastKnownRevision    int       `json:"last_known_revision"`
	GeneratedAt          time.Time `json:"generated_at"`
}

func SupportedFileFormats() []string {
	return []string{
		".server.luau",
		".client.luau",
		".module.luau",
		".server.lua",
		".client.lua",
		".module.lua",
		"source.server.luau",
		"source.client.luau",
		"source.module.luau",
		"source.server.lua",
		"source.client.lua",
		"source.module.lua",
		"properties.init.json",
		"init.meta.json",
		"*.model.json",
	}
}

func GenerateRemoteExecToken() (string, error) {
	var builder strings.Builder
	builder.Grow(RemoteExecTokenLength)
	limit := big.NewInt(int64(len(remoteExecTokenAlphabet)))
	for builder.Len() < RemoteExecTokenLength {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("generate remote exec token: %w", err)
		}
		builder.WriteByte(remoteExecTokenAlphabet[index.Int64()])
	}
	return builder.String(), nil
}

func EnsureRemoteExecToken(cfg *Config) (bool, error) {
	cfg.RemoteExecToken = strings.TrimSpace(cfg.RemoteExecToken)
	if cfg.RemoteExecToken != "" {
		return false, nil
	}
	token, err := GenerateRemoteExecToken()
	if err != nil {
		return false, err
	}
	cfg.RemoteExecToken = token
	return true, nil
}

const VSCodeDir = ".vscode"

func EnsureSyncRootScaffold(cfg Config, options ...ScaffoldOptions) error {
	if err := os.MkdirAll(cfg.SyncRootAbs, 0o755); err != nil {
		return fmt.Errorf("create sync_root: %w", err)
	}
	for _, dir := range BootstrapServiceDirs {
		if err := os.MkdirAll(filepath.Join(cfg.SyncRootAbs, dir), 0o755); err != nil {
			return fmt.Errorf("create service folder %s: %w", dir, err)
		}
	}
	if err := ensureVSCodeSettings(cfg.SyncRootAbs); err != nil {
		return fmt.Errorf("ensure vscode settings: %w", err)
	}
	return nil
}

func ensureVSCodeSettings(syncRoot string) error {
	target := filepath.Join(syncRoot, VSCodeDir, "settings.json")
	if _, err := os.Stat(target); err == nil {
		data, err := os.ReadFile(target)
		if err != nil {
			return nil
		}
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			return nil
		}
		modified := false
		if _, ok := parsed["luau-lsp.platform.type"]; !ok {
			parsed["luau-lsp.platform.type"] = "roblox"
			modified = true
		}
		if _, ok := parsed["luau-lsp.sourcemap.enabled"]; !ok {
			parsed["luau-lsp.sourcemap.enabled"] = false
			modified = true
		}
		if _, ok := parsed["luau-lsp.sourcemap.autogenerate"]; !ok {
			parsed["luau-lsp.sourcemap.autogenerate"] = false
			modified = true
		}
		if modified {
			if updated, err := json.MarshalIndent(parsed, "", "    "); err == nil {
				_ = os.WriteFile(target, append(updated, '\n'), 0o644)
			}
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	content := `{
    "luau-lsp.platform.type": "roblox",
    "luau-lsp.sourcemap.enabled": false,
    "luau-lsp.sourcemap.autogenerate": false
}
`
	return os.WriteFile(target, []byte(content), 0o644)
}

func WriteGuidebookStatus(cfg Config, options ScaffoldOptions) error {
	return nil
}

func Default() Config {
	return Config{
		Host:                        "127.0.0.1",
		Port:                        8765,
		SyncRoot:                    "src/game",
		ScanIntervalSec:             0.4,
		SafetyScanIntervalSec:       30.0,
		PollTimeoutSec:              25,
		ChangeRetention:             2048,
		DebugEventRetention:         300,
		InvalidJSONGraceSec:         1.2,
		GitVersioningEnabled:        true,
		StrictPropertyWhitelist:     false,
		ExtraAllowedProperties:      map[string][]string{},
		RemoteExecEnabled:           false,
		RemoteExecToken:             "",
		RemoteExecMaxSourceBytes:    262144,
		RemoteExecDefaultTimeoutSec: 10,
		RemoteExecMaxTimeoutSec:     120,
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
	if abs, err := filepath.Abs(path); err == nil {
		cfg.ConfigPathAbs = filepath.Clean(abs)
	}

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
	if c.SafetyScanIntervalSec == 0 {
		c.SafetyScanIntervalSec = 30.0
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
	if c.RemoteExecMaxSourceBytes <= 0 {
		return fmt.Errorf("remote_exec_max_source_bytes must be positive, got %d", c.RemoteExecMaxSourceBytes)
	}
	if c.RemoteExecDefaultTimeoutSec <= 0 {
		return fmt.Errorf("remote_exec_default_timeout_sec must be positive, got %d", c.RemoteExecDefaultTimeoutSec)
	}
	if c.RemoteExecMaxTimeoutSec <= 0 {
		return fmt.Errorf("remote_exec_max_timeout_sec must be positive, got %d", c.RemoteExecMaxTimeoutSec)
	}
	if c.RemoteExecDefaultTimeoutSec > c.RemoteExecMaxTimeoutSec {
		return fmt.Errorf("remote_exec_default_timeout_sec must be <= remote_exec_max_timeout_sec")
	}
	c.RemoteExecToken = strings.TrimSpace(c.RemoteExecToken)

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

// PortabilityWarning returns a non-fatal diagnostic for configurations copied
// between operating systems. The active config is never rewritten here.
func (c Config) PortabilityWarning() string {
	if runtime.GOOS != "windows" && looksLikeWindowsAbsolutePath(c.SyncRoot) {
		return fmt.Sprintf("sync_root uses a Windows path on %s: %s; choose the local folder or use sync_config.example.json", runtime.GOOS, c.SyncRoot)
	}
	if _, err := os.Stat(c.SyncRootAbs); err != nil {
		return fmt.Sprintf("sync_root is not accessible: %s", c.SyncRootAbs)
	}
	return ""
}

func looksLikeWindowsAbsolutePath(value string) bool {
	return len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/')
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
