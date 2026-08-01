package instances

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	RegistryVersion  = 1
	RegistryFilename = "instances.json"
)

type Entry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ConfigPath string `json:"config_path"`
}

type Registry struct {
	Version            int     `json:"version"`
	SelectedInstanceID string  `json:"selected_instance_id"`
	SidebarPinned      bool    `json:"sidebar_pinned"`
	Instances          []Entry `json:"instances"`
}

type Store struct {
	path string
}

func DefaultPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(root, "RiftSync", RegistryFilename), nil
}

func NewStore(path string) *Store {
	return &Store{path: filepath.Clean(path)}
}

func (s *Store) Path() string {
	return s.path
}

// Load returns an empty registry for a missing or empty file. A corrupt registry
// is preserved next to the original before returning a clean registry and warning.
func (s *Store) Load() (Registry, string, error) {
	body, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return emptyRegistry(), "", nil
		}
		return Registry{}, "", fmt.Errorf("read instance registry: %w", err)
	}
	if strings.TrimSpace(string(body)) == "" {
		return emptyRegistry(), "", nil
	}

	var registry Registry
	if err := json.Unmarshal(body, &registry); err != nil {
		backup := s.path + ".corrupt-" + time.Now().UTC().Format("20060102T150405.000000000Z")
		if renameErr := os.Rename(s.path, backup); renameErr != nil {
			return Registry{}, "", fmt.Errorf("parse instance registry: %w (preserve corrupt registry: %v)", err, renameErr)
		}
		return emptyRegistry(), "Corrupt instance registry was preserved at " + backup, nil
	}
	if registry.Version == 0 {
		registry.Version = RegistryVersion
	}
	if registry.Version != RegistryVersion {
		return Registry{}, "", fmt.Errorf("unsupported instance registry version %d", registry.Version)
	}
	registry.Instances = normalizeEntries(registry.Instances)
	if err := Validate(registry); err != nil {
		return Registry{}, "", err
	}
	if !containsID(registry.Instances, registry.SelectedInstanceID) {
		registry.SelectedInstanceID = firstID(registry.Instances)
	}
	return registry, "", nil
}

func (s *Store) Save(registry Registry) error {
	registry.Version = RegistryVersion
	registry.Instances = normalizeEntries(registry.Instances)
	if !containsID(registry.Instances, registry.SelectedInstanceID) {
		registry.SelectedInstanceID = firstID(registry.Instances)
	}
	if err := Validate(registry); err != nil {
		return err
	}
	body, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal instance registry: %w", err)
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create instance registry directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), ".instances-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary instance registry: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(body); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temporary instance registry: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("flush temporary instance registry: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary instance registry: %w", err)
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		// Windows cannot atomically replace an existing destination with Rename.
		backup := s.path + ".previous"
		_ = os.Remove(backup)
		if _, statErr := os.Stat(s.path); statErr == nil {
			if moveErr := os.Rename(s.path, backup); moveErr != nil {
				return fmt.Errorf("prepare instance registry replacement: %w", moveErr)
			}
		}
		if moveErr := os.Rename(tempPath, s.path); moveErr != nil {
			_ = os.Rename(backup, s.path)
			return fmt.Errorf("replace instance registry: %w", moveErr)
		}
		_ = os.Remove(backup)
	}
	return nil
}

func EnsureSeed(registry Registry, configPath string) (Registry, bool, error) {
	if len(registry.Instances) > 0 {
		return registry, false, nil
	}
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		configPath = "sync_config.json"
	}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return Registry{}, false, fmt.Errorf("resolve seed config path: %w", err)
	}
	entry := Entry{
		ID:         NewID(),
		Name:       deriveName(abs),
		ConfigPath: filepath.Clean(abs),
	}
	registry.Version = RegistryVersion
	registry.Instances = []Entry{entry}
	registry.SelectedInstanceID = entry.ID
	return registry, true, nil
}

func NewID() string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err == nil {
		return "instance_" + hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("instance_%d", time.Now().UnixNano())
}

func Validate(registry Registry) error {
	ids := map[string]bool{}
	paths := map[string]bool{}
	for _, entry := range registry.Instances {
		if entry.ID == "" {
			return errors.New("instance id must not be empty")
		}
		if entry.Name == "" {
			return fmt.Errorf("instance %s name must not be empty", entry.ID)
		}
		if entry.ConfigPath == "" {
			return fmt.Errorf("instance %s config path must not be empty", entry.ID)
		}
		if ids[entry.ID] {
			return fmt.Errorf("duplicate instance id %q", entry.ID)
		}
		ids[entry.ID] = true
		key := strings.ToLower(filepath.Clean(entry.ConfigPath))
		if paths[key] {
			return fmt.Errorf("duplicate instance config path %q", entry.ConfigPath)
		}
		paths[key] = true
	}
	return nil
}

func emptyRegistry() Registry {
	return Registry{Version: RegistryVersion, Instances: []Entry{}}
}

func normalizeEntries(entries []Entry) []Entry {
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		entry.ID = strings.TrimSpace(entry.ID)
		entry.Name = strings.TrimSpace(entry.Name)
		entry.ConfigPath = strings.TrimSpace(entry.ConfigPath)
		if entry.ConfigPath != "" {
			if abs, err := filepath.Abs(entry.ConfigPath); err == nil {
				entry.ConfigPath = filepath.Clean(abs)
			}
		}
		result = append(result, entry)
	}
	return result
}

func containsID(entries []Entry, id string) bool {
	id = strings.TrimSpace(id)
	for _, entry := range entries {
		if entry.ID == id {
			return true
		}
	}
	return false
}

func firstID(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	return entries[0].ID
}

func deriveName(configPath string) string {
	body, err := os.ReadFile(configPath)
	if err == nil {
		var raw struct {
			SyncRoot string `json:"sync_root"`
		}
		if json.Unmarshal(body, &raw) == nil && strings.TrimSpace(raw.SyncRoot) != "" {
			name := filepath.Base(filepath.Clean(raw.SyncRoot))
			if name != "." && name != string(filepath.Separator) && name != "" {
				return name
			}
		}
	}
	name := filepath.Base(filepath.Dir(configPath))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "Project"
	}
	return name
}
