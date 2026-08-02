package watcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"riftsync/internal/config"
	"riftsync/internal/records"
	"riftsync/internal/scanner"
	"riftsync/internal/state"
)

const defaultDebounce = 120 * time.Millisecond

type Options struct {
	Debounce time.Duration
	Cache    *scanner.Cache
}

type Service struct {
	cfg      config.Config
	state    *state.AppState
	cache    *scanner.Cache
	debounce time.Duration
	watcher  *fsnotify.Watcher

	mu        sync.Mutex
	processMu sync.Mutex
	watches   map[string]bool
	stopOnce  sync.Once
	wg        sync.WaitGroup
}

type SyncResult struct {
	Added   int
	Removed int
	Watched int
}

func New(cfg config.Config, appState *state.AppState, options Options) (*Service, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	debounce := options.Debounce
	if debounce <= 0 {
		debounce = defaultDebounce
	}
	cache := options.Cache
	if cache == nil {
		cache = scanner.NewCache()
	}
	return &Service{
		cfg:      cfg,
		state:    appState,
		cache:    cache,
		debounce: debounce,
		watcher:  fsWatcher,
		watches:  map[string]bool{},
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	if err := os.MkdirAll(s.cfg.SyncRootAbs, 0o755); err != nil {
		return fmt.Errorf("create sync_root for watcher: %w", err)
	}
	if _, err := s.SyncTree(); err != nil {
		return err
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run(ctx)
	}()
	return nil
}

func (s *Service) Close() error {
	var err error
	s.stopOnce.Do(func() {
		err = s.watcher.Close()
	})
	s.wg.Wait()
	return err
}

func (s *Service) run(ctx context.Context) {
	pending := map[string]fsnotify.Op{}
	var timer *time.Timer
	var timerC <-chan time.Time

	flush := func() {
		if len(pending) == 0 {
			return
		}
		batch := pending
		pending = map[string]fsnotify.Op{}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.processBatch(batch)
		}()
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			abs := filepath.Clean(event.Name)
			if s.isIgnored(abs) {
				continue
			}
			pending[abs] |= event.Op
			if timer == nil {
				timer = time.NewTimer(s.debounce)
				timerC = timer.C
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(s.debounce)
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			s.processWarnings([]string{fmt.Sprintf("watcher error: %v", err)})
		case <-timerC:
			flush()
			timer = nil
			timerC = nil
		}
	}
}

func (s *Service) processBatch(batch map[string]fsnotify.Op) {
	s.processMu.Lock()
	defer s.processMu.Unlock()

	batchStarted := time.Now()
	paths := make([]string, 0, len(batch))
	for path := range batch {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	s.state.LockReconciliation()
	defer s.state.UnlockReconciliation()

	nextRecords := s.state.RecordsCopy()
	warnings := []string{}
	invalidPaths := []string{}
	cacheHits := 0
	cacheMisses := 0
	parseStarted := time.Now()

	for _, absPath := range paths {
		if s.isIgnored(absPath) {
			continue
		}
		relPosix, ok := s.relativePath(absPath)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("watcher skipped outside path: %s", absPath))
			continue
		}

		info, err := os.Stat(absPath)
		if err == nil && info.IsDir() {
			if _, addErr := s.addRecursive(absPath); addErr != nil {
				warnings = append(warnings, fmt.Sprintf("watcher add directory %s: %v", relPosix, addErr))
			}
			s.cache.InvalidateSubtree(relPosix)
			removeUnder(nextRecords, relPosix)
			subtree, scanErr := s.cache.ScanSubtree(s.cfg, relPosix)
			if scanErr != nil {
				warnings = append(warnings, fmt.Sprintf("watcher scan subtree %s: %v", relPosix, scanErr))
				continue
			}
			cacheHits += subtree.CacheHits
			cacheMisses += subtree.CacheMisses
			warnings = append(warnings, subtree.Warnings...)
			invalidPaths = append(invalidPaths, subtree.InvalidPaths...)
			for key, record := range subtree.Records {
				nextRecords[key] = record
			}
			continue
		}

		if errors.Is(err, os.ErrNotExist) {
			s.removeWatchesUnder(absPath)
			s.cache.InvalidateSubtree(relPosix)
			removeUnder(nextRecords, relPosix)
			if sourcePath, ok, pathErr := records.ScriptSourcePathForPropertiesPath(relPosix); pathErr == nil && ok {
				s.cache.Invalidate(sourcePath)
				result := s.cache.ParseRelativeFile(s.cfg, sourcePath)
				if result.CacheHit {
					cacheHits++
				} else {
					cacheMisses++
				}
				if result.Warning != "" {
					warnings = append(warnings, result.Warning)
				}
				if result.InvalidPath != "" {
					invalidPaths = append(invalidPaths, result.InvalidPath)
				}
				delete(nextRecords, sourcePath)
				if result.Record != nil {
					nextRecords[result.Record.LocalPath] = *result.Record
				}
			}
			continue
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("watcher stat %s: %v", relPosix, err))
			continue
		}

		result := s.cache.ParseRelativeFile(s.cfg, relPosix)
		if result.CacheHit {
			cacheHits++
		} else {
			cacheMisses++
		}
		if result.Warning != "" {
			warnings = append(warnings, result.Warning)
		}
		if result.InvalidPath != "" {
			invalidPaths = append(invalidPaths, result.InvalidPath)
			continue
		}
		delete(nextRecords, relPosix)
		if result.Record != nil {
			nextRecords[result.Record.LocalPath] = *result.Record
		}
	}

	parseDuration := time.Since(parseStarted)
	deletedRecords := removedRecords(s.state.RecordsCopy(), nextRecords)
	cleanupWarnings := s.cleanupDeletedRecordPaths(deletedRecords)
	warnings = append(warnings, cleanupWarnings...)
	publishStarted := time.Now()
	snapshot := scanner.NormalizeRecords(nextRecords, warnings, invalidPaths)
	s.state.ApplySnapshot(snapshot.Records, snapshot.Warnings, snapshot.InvalidPaths)
	publishDuration := time.Since(publishStarted)
	s.state.RecordPerformance(len(batch), s.debounce, parseDuration, publishDuration, cacheHits, cacheMisses)
	_ = batchStarted
}

func (s *Service) cleanupDeletedRecordPaths(deleted []records.SyncRecord) []string {
	warnings := []string{}
	directories := map[string]bool{}
	nonEmptyCount := 0
	nonEmptySamples := []string{}

	for _, record := range deleted {
		if record.Entity == records.EntityScript {
			propertiesPath, ok, pathErr := records.ScriptPropertiesPathForSourcePath(record.LocalPath)
			if pathErr != nil {
				warnings = append(warnings, "cleanup script metadata path: "+pathErr.Error())
			} else if ok {
				sourceAbs, sourceErr := s.cfg.ResolveInsideSyncRoot(record.LocalPath)
				propertiesAbs, propertiesErr := s.cfg.ResolveInsideSyncRoot(propertiesPath)
				if sourceErr == nil && propertiesErr == nil {
					if _, statErr := os.Stat(sourceAbs); errors.Is(statErr, os.ErrNotExist) {
						if removeErr := os.Remove(propertiesAbs); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
							warnings = append(warnings, fmt.Sprintf("cleanup orphan metadata %s: %v", propertiesPath, removeErr))
						} else {
							s.cache.Invalidate(propertiesPath)
						}
					}
				}
			}
		}

		localDir := strings.Trim(strings.ReplaceAll(record.LocalDir, "\\", "/"), "/")
		for localDir != "" && strings.Contains(localDir, "/") {
			if !looksLikeTypedDirectory(filepath.Base(filepath.FromSlash(localDir))) {
				break
			}
			directories[localDir] = true
			localDir = strings.Trim(strings.ReplaceAll(filepath.ToSlash(filepath.Dir(filepath.FromSlash(localDir))), "\\", "/"), "/")
		}
	}

	ordered := make([]string, 0, len(directories))
	for localDir := range directories {
		ordered = append(ordered, localDir)
	}
	sort.Slice(ordered, func(i, j int) bool {
		leftDepth := strings.Count(ordered[i], "/")
		rightDepth := strings.Count(ordered[j], "/")
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return ordered[i] > ordered[j]
	})

	for _, localDir := range ordered {
		absDir, resolveErr := s.cfg.ResolveInsideSyncRoot(localDir)
		if resolveErr != nil {
			warnings = append(warnings, fmt.Sprintf("cleanup typed folder %s: %v", localDir, resolveErr))
			continue
		}
		removeErr := os.Remove(absDir)
		switch {
		case removeErr == nil:
			s.removeWatchesUnder(absDir)
			s.cache.InvalidateSubtree(localDir)
		case errors.Is(removeErr, os.ErrNotExist):
			continue
		default:
			entries, readErr := os.ReadDir(absDir)
			if readErr == nil && len(entries) > 0 {
				nonEmptyCount++
				if len(nonEmptySamples) < 10 {
					nonEmptySamples = append(nonEmptySamples, localDir)
				}
				continue
			}
			warnings = append(warnings, fmt.Sprintf("cleanup typed folder %s: %v", localDir, removeErr))
		}
	}
	if nonEmptyCount > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"cleanup preserved %d non-empty typed folders (examples: %s)",
			nonEmptyCount,
			strings.Join(nonEmptySamples, ", "),
		))
	}
	return warnings
}

func removedRecords(previous, next map[string]records.SyncRecord) []records.SyncRecord {
	deleted := []records.SyncRecord{}
	for localPath, record := range previous {
		if _, exists := next[localPath]; !exists {
			deleted = append(deleted, record)
		}
	}
	return deleted
}

func looksLikeTypedDirectory(name string) bool {
	dot := strings.LastIndex(name, ".")
	if dot <= 0 || dot == len(name)-1 {
		return false
	}
	className := name[dot+1:]
	for _, char := range className {
		if (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	return true
}

func (s *Service) processWarnings(warnings []string) {
	s.state.LockReconciliation()
	defer s.state.UnlockReconciliation()

	current := s.state.RecordsCopy()
	snapshot := scanner.NormalizeRecords(current, warnings, nil)
	s.state.ApplySnapshot(snapshot.Records, snapshot.Warnings, snapshot.InvalidPaths)
}

func (s *Service) SyncTree() (SyncResult, error) {
	result := SyncResult{}
	desired := map[string]bool{}
	root := filepath.Clean(s.cfg.SyncRootAbs)
	err := filepath.WalkDir(root, func(absPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if entry.Name() == ".git" || entry.Name() == config.MetadataDir || entry.Name() == config.GuidebookDir {
			return filepath.SkipDir
		}
		if s.isIgnored(absPath) {
			return filepath.SkipDir
		}
		desired[filepath.Clean(absPath)] = true
		return nil
	})
	if err != nil {
		s.state.RecordWatcher("watch tree refresh error: " + err.Error())
		return result, err
	}

	s.mu.Lock()
	current := make([]string, 0, len(s.watches))
	for watched := range s.watches {
		current = append(current, watched)
	}
	s.mu.Unlock()

	for _, watched := range current {
		if desired[watched] {
			continue
		}
		_ = s.removeWatch(watched)
		result.Removed++
	}
	for absPath := range desired {
		added, addErr := s.addWatch(absPath)
		if addErr != nil {
			s.state.RecordWatcher("watch add error: " + addErr.Error())
			return result, addErr
		}
		if added {
			result.Added++
		}
	}

	result.Watched = s.WatchCount()
	if result.Added > 0 || result.Removed > 0 {
		s.state.RecordWatcher(fmt.Sprintf("watch tree refreshed added=%d removed=%d watched=%d", result.Added, result.Removed, result.Watched))
	}
	return result, nil
}

func (s *Service) WatchCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.watches)
}

func (s *Service) addRecursive(root string) (int, error) {
	root = filepath.Clean(root)
	addedCount := 0
	err := filepath.WalkDir(root, func(absPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if entry.Name() == ".git" || entry.Name() == config.MetadataDir || entry.Name() == config.GuidebookDir {
			return filepath.SkipDir
		}
		if s.isIgnored(absPath) {
			return filepath.SkipDir
		}
		added, err := s.addWatch(absPath)
		if added {
			addedCount++
		}
		return err
	})
	return addedCount, err
}

func (s *Service) addWatch(absPath string) (bool, error) {
	absPath = filepath.Clean(absPath)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.watches[absPath] {
		return false, nil
	}

	if err := s.watcher.Add(absPath); err != nil {
		return false, err
	}

	s.watches[absPath] = true
	return true, nil
}

func (s *Service) removeWatch(absPath string) error {
	absPath = filepath.Clean(absPath)
	s.mu.Lock()
	if !s.watches[absPath] {
		s.mu.Unlock()
		return nil
	}
	delete(s.watches, absPath)
	s.mu.Unlock()
	return s.watcher.Remove(absPath)
}

func (s *Service) removeWatchesUnder(absPath string) {
	absPath = filepath.Clean(absPath)
	s.mu.Lock()
	targets := []string{}
	for watched := range s.watches {
		if watched == absPath || isChildPath(absPath, watched) {
			targets = append(targets, watched)
			delete(s.watches, watched)
		}
	}
	s.mu.Unlock()

	for _, target := range targets {
		_ = s.watcher.Remove(target)
	}
}

func (s *Service) relativePath(absPath string) (string, bool) {
	rel, err := filepath.Rel(s.cfg.SyncRootAbs, filepath.Clean(absPath))
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func (s *Service) isIgnored(absPath string) bool {
	rel, err := filepath.Rel(s.cfg.SyncRootAbs, filepath.Clean(absPath))
	if err != nil {
		return true
	}
	if rel == ".git" || strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) ||
		rel == config.MetadataDir || strings.HasPrefix(rel, config.MetadataDir+string(os.PathSeparator)) ||
		rel == config.GuidebookDir || strings.HasPrefix(rel, config.GuidebookDir+string(os.PathSeparator)) {
		return true
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	for _, part := range parts {
		if part == ".git" || part == config.MetadataDir || part == config.GuidebookDir {
			return true
		}
	}
	return false
}

func removeUnder(recordMap map[string]records.SyncRecord, relPosix string) {
	relPosix = strings.Trim(strings.ReplaceAll(relPosix, "\\", "/"), "/")
	if relPosix == "" {
		return
	}
	for key := range recordMap {
		if key == relPosix || strings.HasPrefix(key, relPosix+"/") {
			delete(recordMap, key)
		}
	}
}

func isChildPath(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}
