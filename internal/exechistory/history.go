package exechistory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"riftsync/internal/config"
)

const (
	Version      = 1
	DefaultLimit = 100
	Filename     = "exec-history.json"
)

type File struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

type Entry struct {
	ID            string        `json:"id"`
	Timestamp     time.Time     `json:"timestamp"`
	SubmittedBy   string        `json:"submitted_by"`
	SourceKind    string        `json:"source_kind"`
	Source        string        `json:"source"`
	SourceSummary SourceSummary `json:"source_summary"`
	FilePath      string        `json:"file_path,omitempty"`
	TimeoutSec    int           `json:"timeout_sec"`
	ResultState   string        `json:"result_state"`
	OK            bool          `json:"ok"`
	Error         string        `json:"error,omitempty"`
	Output        string        `json:"output,omitempty"`
	Traceback     string        `json:"traceback,omitempty"`
	DurationMS    float64       `json:"duration_ms,omitempty"`
	CommandID     string        `json:"command_id,omitempty"`
	RerunOfID     string        `json:"rerun_of_id,omitempty"`
}

type SourceSummary struct {
	LineCount int    `json:"line_count"`
	CharCount int    `json:"char_count"`
	ByteCount int    `json:"byte_count"`
	FirstLine string `json:"first_line,omitempty"`
}

func Path(syncRoot string) string {
	return filepath.Join(syncRoot, config.MetadataDir, Filename)
}

func Load(syncRoot string) (File, string, error) {
	path := Path(syncRoot)
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{Version: Version, Entries: []Entry{}}, "", nil
		}
		return File{}, "", err
	}
	if strings.TrimSpace(string(body)) == "" {
		return File{Version: Version, Entries: []Entry{}}, "", nil
	}
	var file File
	if err := json.Unmarshal(body, &file); err != nil {
		return File{Version: Version, Entries: []Entry{}}, fmt.Sprintf("ignored corrupt exec history: %v", err), nil
	}
	if file.Version == 0 {
		file.Version = Version
	}
	if file.Entries == nil {
		file.Entries = []Entry{}
	}
	return file, "", nil
}

func Append(syncRoot string, entry Entry, limit int) (Entry, string, error) {
	file, warning, err := Load(syncRoot)
	if err != nil {
		return Entry{}, warning, err
	}
	entry = NormalizeEntry(entry)
	if limit <= 0 {
		limit = DefaultLimit
	}
	entries := append([]Entry{entry}, file.Entries...)
	if len(entries) > limit {
		entries = entries[:limit]
	}
	file = File{Version: Version, Entries: entries}
	if err := os.MkdirAll(filepath.Dir(Path(syncRoot)), 0o755); err != nil {
		return Entry{}, warning, err
	}
	body, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return Entry{}, warning, err
	}
	body = append(body, '\n')
	if err := os.WriteFile(Path(syncRoot), body, 0o644); err != nil {
		return Entry{}, warning, err
	}
	return entry, warning, nil
}

func LatestUsable(file File) (Entry, bool) {
	for _, entry := range file.Entries {
		if strings.TrimSpace(entry.Source) != "" {
			return entry, true
		}
	}
	return Entry{}, false
}

func NormalizeEntry(entry Entry) Entry {
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = fmt.Sprintf("exec_%d", time.Now().UnixNano())
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	} else {
		entry.Timestamp = entry.Timestamp.UTC()
	}
	entry.SubmittedBy = strings.TrimSpace(entry.SubmittedBy)
	entry.SourceKind = strings.TrimSpace(entry.SourceKind)
	entry.FilePath = strings.TrimSpace(entry.FilePath)
	entry.ResultState = strings.TrimSpace(entry.ResultState)
	entry.Error = strings.TrimSpace(entry.Error)
	entry.Output = strings.TrimSpace(entry.Output)
	entry.Traceback = strings.TrimSpace(entry.Traceback)
	entry.CommandID = strings.TrimSpace(entry.CommandID)
	entry.RerunOfID = strings.TrimSpace(entry.RerunOfID)
	if entry.SourceSummary == (SourceSummary{}) {
		entry.SourceSummary = SummarizeSource(entry.Source)
	}
	return entry
}

func SummarizeSource(source string) SourceSummary {
	normalized := strings.ReplaceAll(strings.ReplaceAll(source, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(normalized, "\n")
	lineCount := len(lines)
	if normalized == "" {
		lineCount = 0
	}
	firstLine := ""
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			firstLine = strings.TrimSpace(line)
			break
		}
	}
	if len(firstLine) > 120 {
		firstLine = firstLine[:120]
	}
	return SourceSummary{
		LineCount: lineCount,
		CharCount: len([]rune(source)),
		ByteCount: len([]byte(source)),
		FirstLine: firstLine,
	}
}
