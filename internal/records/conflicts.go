package records

import (
	"fmt"
	"sort"
	"strings"
)

// ConflictKind identifies a validation or apply conflict without requiring
// callers to parse a human-readable message.
type ConflictKind string

const (
	ConflictDuplicateStableID  ConflictKind = "duplicate_stable_id"
	ConflictDuplicateLocalPath ConflictKind = "duplicate_local_path"
	ConflictAmbiguousTarget    ConflictKind = "ambiguous_target"
	ConflictMissingStableID    ConflictKind = "missing_stable_id"
	ConflictInvalidMetadata    ConflictKind = "invalid_metadata"
)

// Conflict is intentionally JSON-friendly because it is shared by the
// validator, debug endpoints, and the desktop UI.
type Conflict struct {
	Kind            ConflictKind `json:"kind"`
	Severity        string       `json:"severity"`
	Operation       string       `json:"operation,omitempty"`
	Entity          string       `json:"entity,omitempty"`
	LocalPath       string       `json:"local_path,omitempty"`
	RbxPath         string       `json:"rbx_path,omitempty"`
	StableID        string       `json:"stable_id,omitempty"`
	ConflictingPath string       `json:"conflicting_path,omitempty"`
	ConflictingID   string       `json:"conflicting_id,omitempty"`
	Message         string       `json:"message"`
	Hint            string       `json:"hint,omitempty"`
}

func (c Conflict) Error() bool {
	return strings.EqualFold(c.Severity, "error")
}

// ValidateRecords checks invariants that must hold before a local snapshot
// can be sent to Studio. Explicit IDs are authoritative; path:* values are
// fallbacks and cannot disambiguate duplicate Roblox targets.
func ValidateRecords(snapshot map[string]SyncRecord) []Conflict {
	conflicts := []Conflict{}
	byStableID := map[string]SyncRecord{}
	byTarget := map[string][]SyncRecord{}
	paths := make([]string, 0, len(snapshot))
	for path := range snapshot {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, localPath := range paths {
		record := snapshot[localPath]
		if explicitStableID(record.StableID) {
			if previous, ok := byStableID[record.StableID]; ok {
				conflicts = append(conflicts, Conflict{
					Kind:            ConflictDuplicateStableID,
					Severity:        "error",
					Entity:          record.Entity,
					LocalPath:       record.LocalPath,
					RbxPath:         record.RbxPath,
					StableID:        record.StableID,
					ConflictingPath: previous.LocalPath,
					ConflictingID:   previous.StableID,
					Message:         fmt.Sprintf("stable ID %s is used by %s and %s", record.StableID, previous.LocalPath, record.LocalPath),
					Hint:            "Give each instance a unique StableId; valid IDs are never rewritten automatically.",
				})
			} else {
				byStableID[record.StableID] = record
			}
		}
		if record.RbxPath != "" {
			key := record.Entity + "\x00" + record.RbxPath
			byTarget[key] = append(byTarget[key], record)
		}
	}

	targetKeys := make([]string, 0, len(byTarget))
	for key := range byTarget {
		targetKeys = append(targetKeys, key)
	}
	sort.Strings(targetKeys)
	for _, key := range targetKeys {
		items := byTarget[key]
		if len(items) < 2 {
			continue
		}
		for index := 0; index < len(items); index++ {
			for other := index + 1; other < len(items); other++ {
				left, right := items[index], items[other]
				if explicitStableID(left.StableID) && explicitStableID(right.StableID) && left.StableID != right.StableID {
					continue
				}
				stableID := left.StableID
				if stableID == "" || strings.HasPrefix(stableID, "path:") {
					stableID = right.StableID
				}
				conflicts = append(conflicts, Conflict{
					Kind:            ConflictAmbiguousTarget,
					Severity:        "error",
					Entity:          left.Entity,
					LocalPath:       left.LocalPath,
					RbxPath:         left.RbxPath,
					StableID:        stableID,
					ConflictingPath: right.LocalPath,
					ConflictingID:   right.StableID,
					Message:         fmt.Sprintf("multiple records target %s without two distinct explicit StableIds", left.RbxPath),
					Hint:            "Use a unique StableId and the ~rid_<StableId> local path suffix for duplicate siblings.",
				})
				for _, item := range []SyncRecord{left, right} {
					if !explicitStableID(item.StableID) {
						conflicts = append(conflicts, Conflict{
							Kind:      ConflictMissingStableID,
							Severity:  "warning",
							Entity:    item.Entity,
							LocalPath: item.LocalPath,
							RbxPath:   item.RbxPath,
							StableID:  item.StableID,
							Message:   fmt.Sprintf("record %s has no explicit StableId", item.LocalPath),
							Hint:      "Add an explicit StableId before using duplicate sibling names.",
						})
					}
				}
			}
		}
	}

	return dedupeConflicts(conflicts)
}

func explicitStableID(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.HasPrefix(value, "path:")
}

func dedupeConflicts(input []Conflict) []Conflict {
	seen := map[string]bool{}
	result := make([]Conflict, 0, len(input))
	for _, conflict := range input {
		key := string(conflict.Kind) + "\x00" + conflict.LocalPath + "\x00" + conflict.ConflictingPath + "\x00" + conflict.RbxPath
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, conflict)
	}
	return result
}
