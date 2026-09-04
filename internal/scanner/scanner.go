package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"riftsync/internal/config"
	"riftsync/internal/records"
)

type Snapshot struct {
	Records      map[string]records.SyncRecord
	Warnings     []string
	InvalidPaths []string
	ScriptCount  int
	UICount      int
	CacheHits    int
	CacheMisses  int
}

type FileResult struct {
	Record      *records.SyncRecord
	Warning     string
	InvalidPath string
	CacheHit    bool
}

type Progress struct {
	Phase         string
	Current       int
	Total         int
	Indeterminate bool
}

type ProgressFunc func(Progress)

type Cache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	size         int64
	mtimeUnixNS  int64
	dependencies []dependencyStat
	result       FileResult
}

type dependencyStat struct {
	path        string
	exists      bool
	size        int64
	mtimeUnixNS int64
}

func NewCache() *Cache {
	return &Cache{entries: map[string]cacheEntry{}}
}

func Scan(cfg config.Config) (Snapshot, error) {
	return NewCache().Scan(cfg)
}

func ScanSubtree(cfg config.Config, relativeDir string) (Snapshot, error) {
	return NewCache().ScanSubtree(cfg, relativeDir)
}

func ParseRelativeFile(cfg config.Config, relativePath string) FileResult {
	return NewCache().ParseRelativeFile(cfg, relativePath)
}

func (c *Cache) Scan(cfg config.Config) (Snapshot, error) {
	return c.ScanWithProgress(cfg, nil)
}

func (c *Cache) ScanWithProgress(cfg config.Config, progress ProgressFunc) (Snapshot, error) {
	return c.scanTree(cfg, cfg.SyncRootAbs, progress)
}

func (c *Cache) ScanSubtree(cfg config.Config, relativeDir string) (Snapshot, error) {
	target, err := cfg.ResolveInsideSyncRoot(relativeDir)
	if err != nil {
		return Snapshot{Records: map[string]records.SyncRecord{}}, err
	}
	return c.scanTree(cfg, target, nil)
}

func (c *Cache) ParseRelativeFile(cfg config.Config, relativePath string) FileResult {
	return c.parseRelativeFile(cfg, relativePath, managedServiceSet(cfg.ManagedRoots))
}

func (c *Cache) parseRelativeFile(cfg config.Config, relativePath string, managedServices map[string]bool) FileResult {
	relPosix := strings.Trim(strings.ReplaceAll(relativePath, "\\", "/"), "/")
	if relPosix == "" || isIgnoredMetadataPath(relPosix) {
		return FileResult{}
	}

	target, err := cfg.ResolveInsideSyncRoot(relPosix)
	if err != nil {
		return FileResult{Warning: fmt.Sprintf("Skip %s: %v", relPosix, err)}
	}

	info, err := os.Stat(target)
	if err != nil {
		return FileResult{Warning: fmt.Sprintf("Skip %s: %v", relPosix, err)}
	}
	if info.IsDir() {
		return FileResult{}
	}
	size := info.Size()
	mtimeUnixNS := info.ModTime().UnixNano()
	dependencies := dependencyStats(cfg, relPosix)

	c.mu.Lock()
	if entry, ok := c.entries[relPosix]; ok && entry.size == size && entry.mtimeUnixNS == mtimeUnixNS && sameDependencies(entry.dependencies, dependencies) {
		result := cloneFileResult(entry.result)
		result.CacheHit = true
		c.mu.Unlock()
		return result
	}
	c.mu.Unlock()

	body, err := os.ReadFile(target)
	if err != nil {
		return FileResult{Warning: fmt.Sprintf("Skip %s: %v", relPosix, err)}
	}

	var result FileResult
	if !utf8.Valid(body) {
		result = FileResult{Warning: fmt.Sprintf("Skip %s: non-utf8 file", relPosix)}
	} else {
		record, err := parseRecord(cfg, relPosix, string(body), managedServices, cfg.IgnoredRbxPaths)
		if err != nil && isJSONFile(relPosix) {
			result = FileResult{
				InvalidPath: relPosix,
				Warning:     fmt.Sprintf("Invalid JSON at %s: %v", relPosix, err),
			}
		} else if err != nil {
			result = FileResult{Warning: fmt.Sprintf("Skip %s: %v", relPosix, err)}
		} else {
			result = FileResult{Record: record}
		}
	}

	c.mu.Lock()
	c.entries[relPosix] = cacheEntry{
		size:         size,
		mtimeUnixNS:  mtimeUnixNS,
		dependencies: dependencies,
		result:       cloneFileResult(result),
	}
	c.mu.Unlock()

	return result
}

func (c *Cache) Invalidate(relativePath string) {
	relPosix := strings.Trim(strings.ReplaceAll(relativePath, "\\", "/"), "/")
	c.mu.Lock()
	delete(c.entries, relPosix)
	c.mu.Unlock()
}

func (c *Cache) InvalidateSubtree(relativeDir string) {
	relPosix := strings.Trim(strings.ReplaceAll(relativeDir, "\\", "/"), "/")
	c.mu.Lock()
	defer c.mu.Unlock()
	if relPosix == "" || relPosix == "." {
		c.entries = map[string]cacheEntry{}
		return
	}
	for key := range c.entries {
		if key == relPosix || strings.HasPrefix(key, relPosix+"/") {
			delete(c.entries, key)
		}
	}
}

func NormalizeRecords(input map[string]records.SyncRecord, warnings []string, invalidPaths []string) Snapshot {
	snapshot := Snapshot{
		Records:      map[string]records.SyncRecord{},
		Warnings:     append([]string(nil), warnings...),
		InvalidPaths: append([]string(nil), invalidPaths...),
	}
	identityToLocalPath := map[string]string{}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		addRecord(&snapshot, identityToLocalPath, input[key])
	}
	countRecords(&snapshot)
	return snapshot
}

func (c *Cache) scanTree(cfg config.Config, walkRoot string, progress ProgressFunc) (Snapshot, error) {
	snapshot := Snapshot{
		Records: map[string]records.SyncRecord{},
	}
	report := func(value Progress) {
		if progress != nil {
			progress(value)
		}
	}

	if _, err := os.Stat(walkRoot); err != nil {
		if os.IsNotExist(err) {
			report(Progress{Phase: "scanning", Total: 0})
			return snapshot, nil
		}
		return snapshot, fmt.Errorf("stat scan root: %w", err)
	}

	report(Progress{Phase: "enumerating", Indeterminate: true})
	files := make([]string, 0, 256)
	enumerated := 0
	walkWarnings := []string{}
	err := filepath.WalkDir(walkRoot, func(absPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			walkWarnings = append(walkWarnings, fmt.Sprintf("scan walk interrupted at %s: %v", absPath, walkErr))
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == config.MetadataDir || entry.Name() == config.GuidebookDir {
				return filepath.SkipDir
			}
			return nil
		}

		relative, relErr := filepath.Rel(cfg.SyncRootAbs, absPath)
		if relErr != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(relative))
		enumerated++
		if enumerated == 1 || enumerated%256 == 0 {
			report(Progress{Phase: "enumerating", Current: enumerated, Indeterminate: true})
		}
		return nil
	})
	if err != nil {
		walkWarnings = append(walkWarnings, fmt.Sprintf("scan walk interrupted: %v", err))
	}
	sort.Strings(files)
	report(Progress{Phase: "scanning", Current: 0, Total: len(files)})

	results := make([]FileResult, len(files))
	if len(files) > 0 {
		workerCount := runtime.GOMAXPROCS(0)
		if workerCount > 1 {
			workerCount-- // Leave CPU headroom for the dashboard and filesystem watcher.
		}
		if workerCount > 8 {
			workerCount = 8
		}
		if workerCount < 1 {
			workerCount = 1
		}
		if workerCount > len(files) {
			workerCount = len(files)
		}

		type scanJob struct {
			index int
			path  string
		}
		type scanResult struct {
			index  int
			result FileResult
		}
		jobs := make(chan scanJob)
		completed := make(chan scanResult, workerCount)
		managedServices := managedServiceSet(cfg.ManagedRoots)
		var workers sync.WaitGroup
		for workerIndex := 0; workerIndex < workerCount; workerIndex++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				for job := range jobs {
					completed <- scanResult{
						index:  job.index,
						result: c.parseRelativeFile(cfg, job.path, managedServices),
					}
				}
			}()
		}
		go func() {
			for index, relPosix := range files {
				jobs <- scanJob{index: index, path: relPosix}
			}
			close(jobs)
			workers.Wait()
			close(completed)
		}()

		reportEvery := len(files) / 100
		if reportEvery < 1 {
			reportEvery = 1
		}
		completedCount := 0
		for item := range completed {
			results[item.index] = item.result
			completedCount++
			if completedCount == len(files) || completedCount%reportEvery == 0 {
				report(Progress{Phase: "scanning", Current: completedCount, Total: len(files)})
			}
		}
	}

	snapshot.Warnings = append(snapshot.Warnings, walkWarnings...)
	identityToLocalPath := map[string]string{}
	skippedNonUTF8 := 0
	skippedUnreadable := 0
	unreadableSamples := []string{}
	for index, relPosix := range files {
		result := results[index]
		if result.CacheHit {
			snapshot.CacheHits++
		} else {
			snapshot.CacheMisses++
		}
		if strings.Contains(result.Warning, "non-utf8 file") {
			skippedNonUTF8++
			continue
		}
		if result.Warning != "" {
			if strings.Contains(result.Warning, "read") || strings.Contains(result.Warning, "access") {
				skippedUnreadable++
				if len(unreadableSamples) < 3 {
					unreadableSamples = append(unreadableSamples, fmt.Sprintf("%s (%s)", relPosix, result.Warning))
				}
			}
			snapshot.Warnings = append(snapshot.Warnings, result.Warning)
		}
		if result.InvalidPath != "" {
			snapshot.InvalidPaths = append(snapshot.InvalidPaths, result.InvalidPath)
			continue
		}
		if result.Record == nil {
			continue
		}
		addRecord(&snapshot, identityToLocalPath, *result.Record)
	}

	if skippedNonUTF8 > 0 {
		snapshot.Warnings = append(snapshot.Warnings, fmt.Sprintf("skipped non-utf8 files: %d", skippedNonUTF8))
	}
	if skippedUnreadable > 0 {
		snapshot.Warnings = append(snapshot.Warnings, fmt.Sprintf("skipped unreadable files: %d (sample: %s)", skippedUnreadable, strings.Join(unreadableSamples, " | ")))
	}

	seenStableIDs := map[string]string{}
	for _, record := range snapshot.Records {
		if record.Entity != records.EntityUIInstance || record.StableID == "" {
			continue
		}
		if previous, ok := seenStableIDs[record.StableID]; ok {
			snapshot.Warnings = append(snapshot.Warnings, fmt.Sprintf("Duplicate stable id %s at %s and %s", record.StableID, record.LocalPath, previous))
		} else {
			seenStableIDs[record.StableID] = record.LocalPath
		}
	}

	countRecords(&snapshot)

	return snapshot, nil
}

func parseRecord(cfg config.Config, relPosix, source string, managedServices map[string]bool, ignored []string) (*records.SyncRecord, error) {
	filename := pathBase(relPosix)
	if filename == records.UIPropertiesFilename {
		if record, ok, err := parseScriptPropertiesRecord(cfg, relPosix, source, managedServices, ignored); ok || err != nil {
			return record, err
		}
		return records.NewUIRecord(relPosix, source, managedServices, ignored)
	}
	if filename == records.UIInitMetaFilename {
		return records.NewRojoInitMetaRecord(relPosix, source, managedServices, ignored)
	}
	if strings.HasSuffix(filename, records.UIModelJSONSuffix) {
		return records.NewRojoModelRecord(relPosix, source, managedServices, ignored)
	}
	record, err := records.NewScriptRecord(relPosix, source, managedServices, ignored)
	if err != nil || record == nil {
		return record, err
	}
	if err := attachScriptProperties(cfg, record, managedServices, ignored); err != nil {
		return nil, err
	}
	return record, nil
}

func addRecord(snapshot *Snapshot, identityToLocalPath map[string]string, record records.SyncRecord) {
	if existing, exists := snapshot.Records[record.LocalPath]; exists {
		shouldReplace, reason := records.ChoosePreferredRecord(existing, record)
		if shouldReplace {
			snapshot.Records[record.LocalPath] = record
			snapshot.Warnings = append(snapshot.Warnings, fmt.Sprintf("Duplicate local_path replaced: %s (%s)", record.LocalPath, reason))
		} else {
			snapshot.Warnings = append(snapshot.Warnings, fmt.Sprintf("Duplicate local_path ignored: %s (%s)", record.LocalPath, reason))
		}
		return
	}

	identityKey := records.IdentityKey(record)
	existingPath, exists := identityToLocalPath[identityKey]
	if !exists {
		snapshot.Records[record.LocalPath] = record
		identityToLocalPath[identityKey] = record.LocalPath
		return
	}

	existingRecord, ok := snapshot.Records[existingPath]
	if !ok {
		snapshot.Records[record.LocalPath] = record
		identityToLocalPath[identityKey] = record.LocalPath
		return
	}

	shouldReplace, reason := records.ChoosePreferredRecord(existingRecord, record)
	if shouldReplace {
		delete(snapshot.Records, existingPath)
		snapshot.Records[record.LocalPath] = record
		identityToLocalPath[identityKey] = record.LocalPath
		snapshot.Warnings = append(snapshot.Warnings, fmt.Sprintf("Merged duplicate identity %s: replaced %s -> %s (%s)", identityKey, existingPath, record.LocalPath, reason))
		return
	}

	snapshot.Warnings = append(snapshot.Warnings, fmt.Sprintf("Merged duplicate identity %s: skipped %s, kept %s (%s)", identityKey, record.LocalPath, existingPath, reason))
}

func parseScriptPropertiesRecord(cfg config.Config, relPosix, source string, managedServices map[string]bool, ignored []string) (*records.SyncRecord, bool, error) {
	sourcePath, ok, err := records.ScriptSourcePathForPropertiesPath(relPosix)
	if err != nil || !ok {
		return nil, ok, err
	}
	sourceAbs, err := cfg.ResolveInsideSyncRoot(sourcePath)
	if err != nil {
		return nil, true, err
	}
	sourceBody, err := os.ReadFile(sourceAbs)
	if os.IsNotExist(err) {
		return nil, true, nil
	}
	if err != nil {
		return nil, true, err
	}
	if !utf8.Valid(sourceBody) {
		return nil, true, fmt.Errorf("script source %s is non-utf8", sourcePath)
	}

	scriptRecord, err := records.NewScriptRecord(sourcePath, string(sourceBody), managedServices, ignored)
	if err != nil || scriptRecord == nil {
		return scriptRecord, true, err
	}
	propertiesRecord, err := records.NewUIRecord(relPosix, source, managedServices, ignored)
	if err != nil || propertiesRecord == nil {
		return scriptRecord, true, err
	}
	if propertiesRecord.RbxPath != scriptRecord.RbxPath || propertiesRecord.ClassName != scriptRecord.ClassName {
		return nil, true, fmt.Errorf("script properties %s target mismatch with %s", relPosix, sourcePath)
	}
	if err := scriptRecord.ApplyScriptPayload(propertiesRecord.Payload); err != nil {
		return nil, true, err
	}
	return scriptRecord, true, nil
}

func attachScriptProperties(cfg config.Config, record *records.SyncRecord, managedServices map[string]bool, ignored []string) error {
	propertiesPath, ok, err := records.ScriptPropertiesPathForSourcePath(record.LocalPath)
	if err != nil || !ok {
		return err
	}
	propertiesAbs, err := cfg.ResolveInsideSyncRoot(propertiesPath)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(propertiesAbs)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !utf8.Valid(body) {
		return fmt.Errorf("script properties %s is non-utf8", propertiesPath)
	}
	propertiesRecord, err := records.NewUIRecord(propertiesPath, string(body), managedServices, ignored)
	if err != nil || propertiesRecord == nil {
		return err
	}
	if propertiesRecord.RbxPath != record.RbxPath || propertiesRecord.ClassName != record.ClassName {
		return fmt.Errorf("script properties %s target mismatch with %s", propertiesPath, record.LocalPath)
	}
	return record.ApplyScriptPayload(propertiesRecord.Payload)
}

func countRecords(snapshot *Snapshot) {
	snapshot.ScriptCount = 0
	snapshot.UICount = 0
	for _, record := range snapshot.Records {
		countRecord(snapshot, record)
	}
}

func countRecord(snapshot *Snapshot, record records.SyncRecord) {
	switch record.Entity {
	case records.EntityScript:
		snapshot.ScriptCount++
	case records.EntityUIInstance:
		snapshot.UICount++
	}
}

func managedServiceSet(managedRoots []string) map[string]bool {
	result := map[string]bool{}
	for _, root := range managedRoots {
		parts := strings.Split(root, ".")
		if len(parts) >= 2 && parts[0] == "game" && parts[1] != "" {
			result[parts[1]] = true
		}
	}
	return result
}

func dependencyStats(cfg config.Config, relPosix string) []dependencyStat {
	dependencyPaths := []string{}
	if propertiesPath, ok, err := records.ScriptPropertiesPathForSourcePath(relPosix); err == nil && ok {
		dependencyPaths = append(dependencyPaths, propertiesPath)
	}
	if sourcePath, ok, err := records.ScriptSourcePathForPropertiesPath(relPosix); err == nil && ok {
		dependencyPaths = append(dependencyPaths, sourcePath)
	}
	if len(dependencyPaths) == 0 {
		return nil
	}

	result := make([]dependencyStat, 0, len(dependencyPaths))
	for _, dependencyPath := range dependencyPaths {
		stat := dependencyStat{path: dependencyPath}
		target, err := cfg.ResolveInsideSyncRoot(dependencyPath)
		if err == nil {
			if info, statErr := os.Stat(target); statErr == nil && !info.IsDir() {
				stat.exists = true
				stat.size = info.Size()
				stat.mtimeUnixNS = info.ModTime().UnixNano()
			}
		}
		result = append(result, stat)
	}
	return result
}

func sameDependencies(left, right []dependencyStat) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func isJSONFile(relPosix string) bool {
	filename := pathBase(relPosix)
	return filename == records.UIPropertiesFilename || filename == records.UIInitMetaFilename || strings.HasSuffix(filename, records.UIModelJSONSuffix)
}

func pathBase(relPosix string) string {
	index := strings.LastIndex(relPosix, "/")
	if index < 0 {
		return relPosix
	}
	return relPosix[index+1:]
}

func isIgnoredMetadataPath(relPosix string) bool {
	relPosix = strings.Trim(strings.ReplaceAll(relPosix, "\\", "/"), "/")
	return relPosix == ".git" ||
		strings.HasPrefix(relPosix, ".git/") ||
		relPosix == config.MetadataDir ||
		strings.HasPrefix(relPosix, config.MetadataDir+"/") ||
		relPosix == config.GuidebookDir ||
		strings.HasPrefix(relPosix, config.GuidebookDir+"/")
}

func cloneFileResult(source FileResult) FileResult {
	result := source
	if source.Record != nil {
		record := *source.Record
		if source.Record.Payload != nil {
			record.Payload = cloneMap(source.Record.Payload)
		}
		result.Record = &record
	}
	return result
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
