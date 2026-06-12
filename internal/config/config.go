package config

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
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

const guidebookReadme = `# RiftSync Guidebook

This guidebook documents the local RiftSync project for humans and AI assistants. It is local metadata and is not synced into Roblox Studio.

## What RiftSync Can Do

RiftSync connects a local folder to a Roblox Studio experience through a local Go server and a Studio plugin.

Core capabilities:

- Sync local Luau scripts and supported UI/model metadata into Studio.
- Pull a Studio snapshot back into the local folder.
- Keep a local revision history and optional Git commits in the sync root.
- Run trusted Studio command bar Luau from the local terminal through Remote Exec.
- Preserve local metadata folders: .git, .rblxsync, .guidebook.

## Folder Roots

For a new empty experience, RiftSync creates these service folders automatically:

- ServerScriptService
- ServerStorage
- ReplicatedStorage
- StarterGui
- StarterPack
- StarterPlayer

Common usage:

- ServerScriptService: server-only scripts.
- ServerStorage: server-only assets/modules.
- ReplicatedStorage: shared modules/assets visible to server and client.
- StarterGui: ScreenGui/UI trees.
- StarterPack: tools that spawn in player backpacks.
- StarterPlayer: StarterPlayerScripts and StarterCharacterScripts.

## File Types And Naming

Script files:

- .server.luau for Script/server code.
- .client.luau for LocalScript/client code.
- .luau for ModuleScript/shared modules when placed as a module file.
- properties.init.json may sit next to a folder-style Script/LocalScript/ModuleScript source file to sync safe script properties.

Examples:

- ServerScriptService/MySystem.server.luau
- ReplicatedStorage/Modules/Inventory.luau
- StarterPlayer/StarterPlayerScripts/ClientBoot.client.luau
- ServerScriptService/MySystem.Script/properties.init.json
- ServerScriptService/MySystem.Script/MySystem.server.luau

Script properties example:

~~~json
{
  "properties": {
    "Disabled": true
  }
}
~~~

Notes:

- Disabled is supported for Script and LocalScript.
- Source belongs in the .server.luau/.client.luau/.module.luau file, not in properties.init.json.

UI/model metadata:

- properties.init.json stores className/properties/attributes/tags for folder-per-instance UI trees.
- init.meta.json supports Rojo-style metadata.
- .model.json supports Rojo-style model JSON.

Example UI tree:

- StarterGui/Main.ScreenGui/properties.init.json
- StarterGui/Main.ScreenGui/Loader.LocalScript/source.client.luau
- StarterGui/Main.ScreenGui/TopBar.Frame/properties.init.json

## Normal Sync Workflow

1. Start the RiftSync app or server.
2. Start the RiftSync Studio plugin and connect to the same host/port.
3. Edit files under the sync root.
4. RiftSync detects create/update/delete/rename/move changes and applies them to Studio.
5. Use Resync if Studio needs a fresh local-to-Studio snapshot.
6. Use Pull Studio when Studio should mirror back into the local folder.

Important rules:

- Treat local files as the source of truth during normal sync.
- Do not edit .git, .rblxsync, or .guidebook as Roblox content.
- Do not place generated build artifacts inside service folders unless they should sync to Studio.
- If a folder path does not exist in Studio, RiftSync may create missing parents as Folder instances.
- Pull Studio first shows a preview of files to add, update, delete, and keep unchanged.
- Confirm starts the replace. Cancel leaves local files untouched.
- After Confirm, Pull Studio replace creates a timestamped backup under .rblxsync/backups before deleting replaced local content.

## Remote Exec: Studio Command Bar From Local Terminal

Remote Exec lets local terminal commands run Luau inside Roblox Studio through the RiftSync plugin. Use it only for trusted local automation.

Requirements:

- RiftSync server/app is running.
- sync_config.json has remote_exec_enabled set to true.
- sync_config.json has remote_exec_token set. The desktop app generates a token if one is missing.
- Studio plugin sync is started.
- Exec Token in the plugin matches remote_exec_token. Copy it from the app Exec tab.
- Exec ON is enabled in the Studio widget.
- Studio is in Edit mode, not Play mode.

Inline command:

~~~powershell
.\.guidebook\riftsync-exec.ps1 "print(workspace.Name)"
~~~

Multi-line command:

~~~powershell
@'
local folder = workspace:WaitForChild("MyFolder", 10)
print(folder and folder:GetFullName())
return workspace.Name
'@ | .\.guidebook\riftsync-exec.ps1 --stdin --timeout 15
~~~

Command from file:

~~~powershell
.\.guidebook\riftsync-exec.ps1 .\studio-command.lua --timeout 30
.\.guidebook\riftsync-exec.ps1 --file .\studio-command.luau --timeout 30
.\.guidebook\riftsync-exec.ps1 --last
~~~

The .guidebook/riftsync-exec.ps1 launcher is generated for this machine. It points to the local RiftSync binary and config path so AI assistants can run Remote Exec while working from the sync root.

Remote Exec history:

- CLI and app console commands are stored in .rblxsync/exec-history.json.
- History is local-only and stores full source so rerun works even for inline/stdin/app commands.
- The latest command can be rerun with .\.guidebook\riftsync-exec.ps1 --last.
- The desktop app has an Exec tab for paste/run output, recent reruns, and a readonly token field to copy into the Studio plugin.

Direct fallback:

~~~powershell
.\riftsync-server.exe --config <path-to-sync_config.json> exec "print(workspace.Name)"
.\riftsync-server.exe --config <path-to-sync_config.json> exec --last
~~~

Remote Exec returns these to the local terminal:

- print(...) output.
- warn(...) output.
- return values.
- runtime errors.
- traceback.
- duration_ms.

Remote Exec limitations:

- Commands are rejected during Play mode.
- loadstring must be available in the Studio/plugin context.
- This is not a sandbox. Do not run untrusted code.

## Git And History

RiftSync may create a Git repository in the sync root when git_versioning_enabled is true. It also stores local revision history in .rblxsync/history.json.

Run Git commands from the sync root to inspect game changes:

~~~powershell
git status
git diff
git diff --check
~~~

Notes:

- git status shows modified and untracked files without git add.
- git add only stages changes for commit.
- git diff shows content changes.
- git diff --check only checks whitespace and patch issues.
- If you run git status from the RiftSync tool repo instead of the sync root, you are checking the tool source, not the game files.

## Local Server And Config

Common commands:

~~~powershell
.\riftsync.exe
.\riftsync-server.exe --headless
.\riftsync-server.exe --headless --port 8766
.\riftsync-server.exe validate
.\riftsync-server.exe validate --json
~~~

Important sync_config.json fields:

- sync_root: local game folder.
- host and port: local server address.
- managed_roots: Roblox services managed by sync.
- ignored_rbx_paths: Roblox paths excluded from sync.
- git_versioning_enabled: enable local Git revision commits.
- remote_exec_enabled: enable Remote Exec HTTP API.
- remote_exec_token: bearer token used by CLI and Studio plugin.

Generated machine context:

- .guidebook/status.json is overwritten by RiftSync with the current sync root, config path, server host/port, version, Remote Exec availability, supported file formats, and last known revision.
- validate checks config, metadata JSON, duplicate stable IDs, unsupported paths, and sync root safety without modifying project files.

## Metadata Folders

These folders are local-only:

- .git: Git repository data.
- .rblxsync: RiftSync history/metadata.
- .guidebook: AI/human playbook.

RiftSync preserves and ignores these folders during scan, watch, and bootstrap replace. Do not create Roblox scripts or UI content inside them.

Backups from Pull Studio replace live in .rblxsync/backups/<timestamp> and accumulate until you remove them. Backups are created after the Studio plugin preview is confirmed, before local files are deleted.

## Studio Plugin Health

The Studio widget debug checklist tracks:

- HTTP/server reachable.
- connected to server.
- token valid.
- sync active.
- exec active.
- edit mode.

Keep Studio in Edit mode for Remote Exec. Play mode intentionally blocks command execution.

## Troubleshooting

- Server offline: start riftsync.exe or riftsync-server.exe --headless.
- Port mismatch: use the same host/port in the app/server and Studio plugin.
- HTTP disabled: enable Studio HTTP requests in Game Settings > Security.
- Nothing appears in git status: run git status from sync_root, not the tool repo.
- Remote Exec auth fails: verify remote_exec_token and the plugin Exec Token match.
- Remote Exec times out: make sure Studio sync is running, Exec ON is enabled, and Studio is not stuck.
- Play mode rejection: stop Play mode before running Remote Exec commands.
`

func EnsureSyncRootScaffold(cfg Config, options ...ScaffoldOptions) error {
	if err := os.MkdirAll(cfg.SyncRootAbs, 0o755); err != nil {
		return fmt.Errorf("create sync_root: %w", err)
	}
	for _, dir := range BootstrapServiceDirs {
		if err := os.MkdirAll(filepath.Join(cfg.SyncRootAbs, dir), 0o755); err != nil {
			return fmt.Errorf("create service folder %s: %w", dir, err)
		}
	}

	guidebookPath := filepath.Join(cfg.SyncRootAbs, GuidebookDir, "README.md")
	if _, err := os.Stat(guidebookPath); err == nil {
		// Keep user-edited guidebooks intact.
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat guidebook: %w", err)
	} else {
		if err := os.MkdirAll(filepath.Dir(guidebookPath), 0o755); err != nil {
			return fmt.Errorf("create guidebook dir: %w", err)
		}
		if err := os.WriteFile(guidebookPath, []byte(guidebookReadme), 0o644); err != nil {
			return fmt.Errorf("write guidebook: %w", err)
		}
	}
	if len(options) > 0 {
		if err := writeGuidebookExecLauncher(cfg, options[0]); err != nil {
			return err
		}
		if options[0].WriteStatusJSON {
			if err := WriteGuidebookStatus(cfg, options[0]); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteGuidebookStatus(cfg Config, options ScaffoldOptions) error {
	if strings.TrimSpace(cfg.SyncRootAbs) == "" {
		return errors.New("sync_root_abs is not configured")
	}
	configAbs, err := resolveStatusConfigPath(cfg, options.ConfigPath)
	if err != nil {
		return err
	}
	generatedAt := options.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	} else {
		generatedAt = generatedAt.UTC()
	}
	remoteExecConfigured := strings.TrimSpace(cfg.RemoteExecToken) != ""
	payload := GuidebookStatusPayload{
		SyncRoot:             cfg.SyncRootAbs,
		ConfigPath:           configAbs,
		Host:                 cfg.Host,
		Port:                 cfg.Port,
		Version:              strings.TrimSpace(options.Version),
		RemoteExecEnabled:    cfg.RemoteExecEnabled,
		RemoteExecConfigured: remoteExecConfigured,
		RemoteExecAvailable:  cfg.RemoteExecEnabled && remoteExecConfigured,
		SupportedFileFormats: SupportedFileFormats(),
		LastKnownRevision:    options.LastKnownRevision,
		GeneratedAt:          generatedAt,
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal guidebook status: %w", err)
	}
	body = append(body, '\n')
	target := filepath.Join(cfg.SyncRootAbs, GuidebookDir, GuidebookStatus)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create guidebook status dir: %w", err)
	}
	if err := os.WriteFile(target, body, 0o644); err != nil {
		return fmt.Errorf("write guidebook status: %w", err)
	}
	return nil
}

func resolveStatusConfigPath(cfg Config, configPath string) (string, error) {
	candidate := strings.TrimSpace(configPath)
	if candidate == "" {
		candidate = strings.TrimSpace(cfg.ConfigPathAbs)
	}
	if candidate == "" {
		candidate = "sync_config.json"
	}
	configAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve config path for guidebook status: %w", err)
	}
	return filepath.Clean(configAbs), nil
}

func writeGuidebookExecLauncher(cfg Config, options ScaffoldOptions) error {
	configPath := options.ConfigPath
	if strings.TrimSpace(configPath) == "" {
		configPath = "sync_config.json"
	}
	configAbs, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve config path for guidebook launcher: %w", err)
	}
	executablePath := strings.TrimSpace(options.ExecutablePath)
	if executablePath == "" {
		executablePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable path for guidebook launcher: %w", err)
		}
	}
	serverPath, exists := resolveRiftSyncServerPath(executablePath)
	launcher := buildGuidebookExecLauncher(serverPath, configAbs, exists)
	target := filepath.Join(cfg.SyncRootAbs, GuidebookDir, GuidebookExecLauncher)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create guidebook launcher dir: %w", err)
	}
	if err := os.WriteFile(target, []byte(launcher), 0o644); err != nil {
		return fmt.Errorf("write guidebook launcher: %w", err)
	}
	return nil
}

func resolveRiftSyncServerPath(executablePath string) (string, bool) {
	executablePath = filepath.Clean(executablePath)
	base := strings.ToLower(filepath.Base(executablePath))
	if base == "riftsync-server.exe" {
		_, err := os.Stat(executablePath)
		return executablePath, err == nil
	}
	candidate := filepath.Join(filepath.Dir(executablePath), "riftsync-server.exe")
	_, err := os.Stat(candidate)
	return candidate, err == nil
}

func buildGuidebookExecLauncher(serverPath, configPath string, serverExists bool) string {
	warning := ""
	if !serverExists {
		warning = "# WARNING: riftsync-server.exe was not found at generation time. Ensure this path exists or restart RiftSync from the installed folder.\n"
	}
	return "# AUTO-GENERATED BY RIFTSYNC. DO NOT EDIT.\n" +
		"# Re-generated whenever RiftSync starts for this sync root.\n" +
		warning +
		"$RiftSyncServer = " + powershellSingleQuoted(serverPath) + "\n" +
		"$RiftSyncConfig = " + powershellSingleQuoted(configPath) + "\n\n" +
		"$execArgs = @($args)\n" +
		"if (-not $MyInvocation.ExpectingInput -and $execArgs.Count -gt 0) {\n" +
		"    $hasExplicitSource = $false\n" +
		"    foreach ($arg in $execArgs) {\n" +
		"        if ($arg -eq '--stdin' -or $arg -eq '--file') { $hasExplicitSource = $true }\n" +
		"    }\n" +
		"    $firstArg = [string]$execArgs[0]\n" +
		"    if (-not $hasExplicitSource -and -not $firstArg.StartsWith('-') -and ($firstArg -match '\\.(lua|luau)$')) {\n" +
		"        $remainingArgs = @()\n" +
		"        if ($execArgs.Count -gt 1) { $remainingArgs = @($execArgs[1..($execArgs.Count - 1)]) }\n" +
		"        $execArgs = @('--file', $firstArg) + $remainingArgs\n" +
		"    }\n" +
		"}\n\n" +
		"if ($MyInvocation.ExpectingInput) {\n" +
		"    $input | & $RiftSyncServer --config $RiftSyncConfig exec @execArgs\n" +
		"} else {\n" +
		"    & $RiftSyncServer --config $RiftSyncConfig exec @execArgs\n" +
		"}\n" +
		"exit $LASTEXITCODE\n"
}

func powershellSingleQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func Default() Config {
	return Config{
		Host:                        "127.0.0.1",
		Port:                        8765,
		SyncRoot:                    "src/game",
		ScanIntervalSec:             0.4,
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
