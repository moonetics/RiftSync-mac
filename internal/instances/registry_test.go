package instances

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureSeedPreservesConfigAndImportsIt(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sync_config.json")
	body := []byte(`{"sync_root":"C:\\Games\\Arena","port":8765}`)
	if err := os.WriteFile(configPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	registry, changed, err := EnsureSeed(emptyRegistry(), configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || len(registry.Instances) != 1 || registry.Instances[0].Name != "Arena" {
		t.Fatalf("registry = %#v changed=%v", registry, changed)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(body) {
		t.Fatal("EnsureSeed modified the existing config")
	}
}

func TestStoreRoundTripAndSelectionFallback(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "RiftSync", RegistryFilename))
	registry := Registry{
		SelectedInstanceID: "missing",
		SidebarPinned:      true,
		Instances: []Entry{
			{ID: "one", Name: "One", ConfigPath: filepath.Join(t.TempDir(), "one.json")},
		},
	}
	if err := store.Save(registry); err != nil {
		t.Fatal(err)
	}
	loaded, warning, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if warning != "" || loaded.SelectedInstanceID != "one" || !loaded.SidebarPinned {
		t.Fatalf("loaded = %#v warning=%q", loaded, warning)
	}
}

func TestStorePreservesCorruptRegistry(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, RegistryFilename)
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	registry, warning, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Instances) != 0 || !strings.Contains(warning, "preserved") {
		t.Fatalf("registry=%#v warning=%q", registry, warning)
	}
	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil || len(matches) != 1 {
		t.Fatalf("corrupt backup matches=%v err=%v", matches, err)
	}
}

func TestValidateRejectsDuplicateIDAndPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sync.json")
	cases := []Registry{
		{Instances: []Entry{{ID: "same", Name: "One", ConfigPath: path}, {ID: "same", Name: "Two", ConfigPath: filepath.Join(root, "two.json")}}},
		{Instances: []Entry{{ID: "one", Name: "One", ConfigPath: path}, {ID: "two", Name: "Two", ConfigPath: strings.ToUpper(path)}}},
	}
	for _, registry := range cases {
		if err := Validate(registry); err == nil {
			t.Fatalf("Validate(%#v) returned nil", registry)
		}
	}
}
