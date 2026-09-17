package records

import "sort"

func DetectChanges(oldRecords, newRecords map[string]SyncRecord, invalidPaths map[string]bool) []map[string]any {
	deleted := map[string]bool{}
	added := map[string]bool{}
	shared := map[string]bool{}

	for path := range oldRecords {
		if _, ok := newRecords[path]; ok {
			shared[path] = true
		} else {
			deleted[path] = true
		}
	}
	for path := range newRecords {
		if _, ok := oldRecords[path]; !ok {
			added[path] = true
		}
	}

	modified := []string{}
	for path := range shared {
		oldRecord := oldRecords[path]
		newRecord := newRecords[path]
		if oldRecord.ContentHash != newRecord.ContentHash ||
			oldRecord.ClassName != newRecord.ClassName ||
			oldRecord.RbxPath != newRecord.RbxPath ||
			oldRecord.Entity != newRecord.Entity ||
			oldRecord.StableID != newRecord.StableID {
			modified = append(modified, path)
		}
	}

	for path := range deleted {
		if invalidPaths[path] {
			delete(deleted, path)
		}
	}

	renamePairs := [][2]string{}
	for _, pair := range pairRenamesByStableID(oldRecords, newRecords, deleted, added) {
		renamePairs = append(renamePairs, pair)
		delete(deleted, pair[0])
		delete(added, pair[1])
	}
	for _, pair := range pairRenamesByHash(oldRecords, newRecords, deleted, added) {
		renamePairs = append(renamePairs, pair)
		delete(deleted, pair[0])
		delete(added, pair[1])
	}

	changes := []map[string]any{}
	sort.Slice(renamePairs, func(i, j int) bool {
		if renamePairs[i][0] == renamePairs[j][0] {
			return renamePairs[i][1] < renamePairs[j][1]
		}
		return renamePairs[i][0] < renamePairs[j][0]
	})
	for _, pair := range renamePairs {
		changes = append(changes, BuildRenameChange(oldRecords[pair[0]], newRecords[pair[1]]))
	}
	for _, path := range sortedSetKeys(deleted) {
		changes = append(changes, oldRecords[path].ToDelete())
	}
	for _, path := range sortedSetKeys(added) {
		changes = append(changes, newRecords[path].ToUpsert())
	}
	sort.Strings(modified)
	for _, path := range modified {
		changes = append(changes, newRecords[path].ToUpsert())
	}

	return changes
}

func pairRenamesByStableID(oldRecords, newRecords map[string]SyncRecord, deleted, added map[string]bool) [][2]string {
	deletedByKey := map[string][]string{}
	addedByKey := map[string][]string{}

	for path := range deleted {
		record := oldRecords[path]
		if record.StableID != "" {
			deletedByKey[record.Entity+"\x00"+record.StableID] = append(deletedByKey[record.Entity+"\x00"+record.StableID], path)
		}
	}
	for path := range added {
		record := newRecords[path]
		if record.StableID != "" {
			addedByKey[record.Entity+"\x00"+record.StableID] = append(addedByKey[record.Entity+"\x00"+record.StableID], path)
		}
	}

	pairs := [][2]string{}
	keys := make([]string, 0, len(deletedByKey))
	for key := range deletedByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		oldList := deletedByKey[key]
		newList := addedByKey[key]
		if len(oldList) != 1 || len(newList) != 1 {
			continue
		}
		oldPath := oldList[0]
		newPath := newList[0]
		oldRecord := oldRecords[oldPath]
		newRecord := newRecords[newPath]
		if oldRecord.ClassName == newRecord.ClassName || oldRecord.Entity == EntityUIInstance {
			pairs = append(pairs, [2]string{oldPath, newPath})
		}
	}
	return pairs
}

func pairRenamesByHash(oldRecords, newRecords map[string]SyncRecord, deleted, added map[string]bool) [][2]string {
	addedByKey := map[string][]string{}
	for _, path := range sortedSetKeys(added) {
		record := newRecords[path]
		key := record.Entity + "\x00" + record.ClassName + "\x00" + record.ContentHash
		addedByKey[key] = append(addedByKey[key], path)
	}

	pairs := [][2]string{}
	for _, deletedPath := range sortedSetKeys(deleted) {
		oldRecord := oldRecords[deletedPath]
		key := oldRecord.Entity + "\x00" + oldRecord.ClassName + "\x00" + oldRecord.ContentHash
		candidates := addedByKey[key]
		if len(candidates) == 0 {
			continue
		}
		newPath := candidates[0]
		addedByKey[key] = candidates[1:]
		pairs = append(pairs, [2]string{deletedPath, newPath})
	}
	return pairs
}

func sortedSetKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
