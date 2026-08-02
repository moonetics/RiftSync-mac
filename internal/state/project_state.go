package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"riftsync/internal/config"
)

const projectStateVersion = 1

type DeletionTombstone struct {
	StableID  string  `json:"stable_id,omitempty"`
	LocalPath string  `json:"local_path"`
	RbxPath   string  `json:"rbx_path"`
	ClassName string  `json:"class_name"`
	Entity    string  `json:"entity"`
	Revision  int     `json:"revision"`
	DeletedAt float64 `json:"deleted_at"`
}

type persistentProjectState struct {
	Version     int                 `json:"version"`
	Initialized bool                `json:"initialized"`
	Tombstones  []DeletionTombstone `json:"deletion_tombstones"`
}

func (s *AppState) SyncRootInitialized() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.projectInitialized
}

func (s *AppState) SnapshotAuthoritative() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metrics.LastScanInvalidJSON == 0
}

func (s *AppState) SetSyncRootInitialized(initialized bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projectInitialized = initialized
	return s.saveProjectStateLocked()
}

func (s *AppState) DeletionTombstones() []DeletionTombstone {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]DeletionTombstone, 0, len(s.tombstones))
	for _, tombstone := range s.tombstones {
		result = append(result, tombstone)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Revision != result[j].Revision {
			return result[i].Revision < result[j].Revision
		}
		return tombstoneKey(result[i]) < tombstoneKey(result[j])
	})
	return result
}

func (s *AppState) LoadProjectState() error {
	path := s.projectStatePath()
	if path == "" {
		return nil
	}
	body, err := os.ReadFile(path)
	if err == nil {
		var persisted persistentProjectState
		if jsonErr := json.Unmarshal(body, &persisted); jsonErr == nil && persisted.Version == projectStateVersion {
			s.mu.Lock()
			s.projectInitialized = persisted.Initialized || s.inferInitializedLocked()
			s.tombstones = map[string]DeletionTombstone{}
			for _, tombstone := range persisted.Tombstones {
				if key := tombstoneKey(tombstone); key != "" {
					s.tombstones[key] = tombstone
				}
			}
			for _, event := range s.changeLog {
				s.applyChangesToTombstonesLocked(event.Changes, event.Rev)
			}
			saveErr := s.saveProjectStateLocked()
			s.mu.Unlock()
			return saveErr
		}

		backup := path + ".corrupt-" + time.Now().UTC().Format("20060102T150405Z")
		if renameErr := os.Rename(path, backup); renameErr != nil {
			s.events.Add("project-state", "ignored unreadable project state: "+renameErr.Error())
		} else {
			s.events.Add("project-state", "moved unreadable project state to "+filepath.Base(backup))
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	s.mu.Lock()
	s.projectInitialized = s.inferInitializedLocked()
	s.tombstones = map[string]DeletionTombstone{}
	for _, event := range s.changeLog {
		s.applyChangesToTombstonesLocked(event.Changes, event.Rev)
	}
	saveErr := s.saveProjectStateLocked()
	s.mu.Unlock()
	return saveErr
}

func (s *AppState) applyChangesToTombstonesLocked(changes []map[string]any, revision int) {
	if s.tombstones == nil {
		s.tombstones = map[string]DeletionTombstone{}
	}
	for _, change := range changes {
		op := strings.TrimSpace(fmt.Sprint(change["op"]))
		if op == "delete" {
			tombstone := DeletionTombstone{
				StableID:  firstProjectStateString(change["stable_id"]),
				LocalPath: firstProjectStateString(change["local_path"]),
				RbxPath:   firstProjectStateString(change["rbx_path"], change["old_rbx_path"]),
				ClassName: firstProjectStateString(change["class_name"]),
				Entity:    firstProjectStateString(change["entity"]),
				Revision:  revision,
				DeletedAt: nowSeconds(),
			}
			if key := tombstoneKey(tombstone); key != "" {
				if existing, found := s.tombstones[key]; !found || existing.Revision < revision {
					s.tombstones[key] = tombstone
				}
			}
			continue
		}

		active := DeletionTombstone{
			StableID:  firstProjectStateString(change["stable_id"]),
			LocalPath: firstProjectStateString(change["local_path"]),
			RbxPath:   firstProjectStateString(change["new_rbx_path"], change["rbx_path"]),
			ClassName: firstProjectStateString(change["class_name"]),
			Entity:    firstProjectStateString(change["entity"]),
		}
		for key, tombstone := range s.tombstones {
			if revision >= tombstone.Revision && sameTombstoneIdentity(tombstone, active) {
				delete(s.tombstones, key)
			}
		}
	}
}

func sameTombstoneIdentity(left, right DeletionTombstone) bool {
	if left.StableID != "" && right.StableID != "" && left.StableID == right.StableID {
		return true
	}
	if left.Entity != right.Entity || left.ClassName != right.ClassName {
		return false
	}
	return (left.LocalPath != "" && left.LocalPath == right.LocalPath) ||
		(left.RbxPath != "" && left.RbxPath == right.RbxPath)
}

func tombstoneKey(tombstone DeletionTombstone) string {
	if tombstone.StableID != "" {
		return "id:" + tombstone.StableID
	}
	if tombstone.Entity == "" || tombstone.RbxPath == "" || tombstone.ClassName == "" {
		return ""
	}
	return "path:" + tombstone.Entity + "\x00" + tombstone.RbxPath + "\x00" + tombstone.ClassName
}

func firstProjectStateString(values ...any) string {
	for _, value := range values {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func (s *AppState) inferInitializedLocked() bool {
	if s.revision > 0 || len(s.records) > 0 {
		return true
	}
	root := strings.TrimSpace(s.cfg.SyncRootAbs)
	if root == "" {
		return false
	}
	found := false
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || found {
			return nil
		}
		if path == root {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		first := strings.Split(filepath.ToSlash(relative), "/")[0]
		if first == ".git" || first == config.MetadataDir || first == config.GuidebookDir {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			found = true
		}
		return nil
	})
	return found
}

func (s *AppState) saveProjectStateLocked() error {
	path := s.projectStatePath()
	if path == "" {
		return nil
	}
	tombstones := make([]DeletionTombstone, 0, len(s.tombstones))
	for _, tombstone := range s.tombstones {
		tombstones = append(tombstones, tombstone)
	}
	sort.Slice(tombstones, func(i, j int) bool { return tombstoneKey(tombstones[i]) < tombstoneKey(tombstones[j]) })
	body, err := json.MarshalIndent(persistentProjectState{
		Version:     projectStateVersion,
		Initialized: s.projectInitialized,
		Tombstones:  tombstones,
	}, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return writeProjectStateAtomic(path, body)
}

func (s *AppState) projectStatePath() string {
	if strings.TrimSpace(s.cfg.SyncRootAbs) == "" {
		return ""
	}
	return filepath.Join(s.cfg.SyncRootAbs, config.MetadataDir, "project-state.json")
}

func writeProjectStateAtomic(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".project-state-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(body); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err == nil {
		return nil
	}
	backup := path + ".bak"
	_ = os.Remove(backup)
	if _, statErr := os.Stat(path); statErr == nil {
		if err := os.Rename(path, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Rename(backup, path)
		return err
	}
	_ = os.Remove(backup)
	return nil
}
