package exechistory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingEmptyAndCorruptHistory(t *testing.T) {
	root := t.TempDir()
	file, warning, err := Load(root)
	if err != nil || warning != "" || len(file.Entries) != 0 {
		t.Fatalf("missing load file=%#v warning=%q err=%v", file, warning, err)
	}

	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(Path(root), []byte("   "), 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	file, warning, err = Load(root)
	if err != nil || warning != "" || len(file.Entries) != 0 {
		t.Fatalf("empty load file=%#v warning=%q err=%v", file, warning, err)
	}

	if err := os.WriteFile(Path(root), []byte("{bad"), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	file, warning, err = Load(root)
	if err != nil || warning == "" || len(file.Entries) != 0 {
		t.Fatalf("corrupt load file=%#v warning=%q err=%v", file, warning, err)
	}
}

func TestAppendWritesRequiredFieldsAndTrims(t *testing.T) {
	root := t.TempDir()
	for index := 0; index < 105; index++ {
		_, _, err := Append(root, Entry{
			ID:          string(rune('a' + index%26)),
			SubmittedBy: "cli",
			SourceKind:  "inline",
			Source:      "print(" + string(rune('0'+index%10)) + ")",
			TimeoutSec:  10,
			ResultState: "done",
			OK:          true,
		}, DefaultLimit)
		if err != nil {
			t.Fatalf("append %d: %v", index, err)
		}
	}

	body, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	var file File
	if err := json.Unmarshal(body, &file); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if file.Version != Version || len(file.Entries) != DefaultLimit {
		t.Fatalf("file version=%d entries=%d", file.Version, len(file.Entries))
	}
	latest := file.Entries[0]
	if latest.Source == "" || latest.SourceSummary.ByteCount == 0 || latest.Timestamp.IsZero() || latest.SubmittedBy != "cli" {
		t.Fatalf("latest entry missing required fields: %#v", latest)
	}
}

func TestLatestUsableSkipsEntriesWithoutSource(t *testing.T) {
	entry, ok := LatestUsable(File{Entries: []Entry{
		{ID: "empty"},
		{ID: "usable", Source: "print(1)"},
	}})
	if !ok || entry.ID != "usable" {
		t.Fatalf("LatestUsable = %#v %t, want usable", entry, ok)
	}
}
