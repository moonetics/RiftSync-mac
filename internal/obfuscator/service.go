package obfuscator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type ExplorerNode struct {
	Name              string          `json:"name"`
	ClassName         string          `json:"class_name"`
	Path              string          `json:"path"`
	Kind              string          `json:"kind"` // service, folder, server_script, client_script, module_script, screengui, frame, etc.
	IsScript          bool            `json:"is_script"`
	Protected         bool            `json:"protected"`
	Enabled           bool            `json:"enabled"`
	Properties        map[string]any  `json:"properties,omitempty"`
	PropertiesRelPath string          `json:"properties_rel_path,omitempty"`
	ScriptRelPath     string          `json:"script_rel_path,omitempty"`
	Children          []*ExplorerNode `json:"children,omitempty"`
}

type VaultEntry struct {
	RelPath      string    `json:"rel_path"`
	OriginalHash string    `json:"original_hash"`
	Timestamp    time.Time `json:"timestamp"`
	Watermark    string    `json:"watermark,omitempty"`
}

type Manifest struct {
	mu      sync.Mutex
	Entries map[string]VaultEntry `json:"entries"`
}

type TreeSummary struct {
	TotalScripts     int           `json:"total_scripts"`
	TotalInstances   int           `json:"total_instances"`
	ProtectedScripts int           `json:"protected_scripts"`
	OriginalScripts  int           `json:"original_scripts"`
	DisabledScripts  int           `json:"disabled_scripts"`
	Tree             *ExplorerNode `json:"tree"`
}

func vaultDir(syncRoot string) string {
	return filepath.Join(syncRoot, ".rblxsync", "vault")
}

func manifestPath(syncRoot string) string {
	return filepath.Join(vaultDir(syncRoot), "manifest.json")
}

func vaultFilePath(syncRoot, relPath string) string {
	return filepath.Join(vaultDir(syncRoot), "files", filepath.FromSlash(relPath))
}

func loadManifest(syncRoot string) (*Manifest, error) {
	p := manifestPath(syncRoot)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Entries: make(map[string]VaultEntry)}, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return &Manifest{Entries: make(map[string]VaultEntry)}, nil
	}
	if m.Entries == nil {
		m.Entries = make(map[string]VaultEntry)
	}
	return &m, nil
}

func saveManifest(syncRoot string, m *Manifest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dir := vaultDir(syncRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath(syncRoot), data, 0644)
}

func parseScriptInfo(filename string) (name string, className string, kind string, isScript bool) {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".server.luau") || strings.HasSuffix(lower, ".server.lua") {
		ext := ".server.luau"
		if strings.HasSuffix(lower, ".server.lua") {
			ext = ".server.lua"
		}
		return filename[:len(filename)-len(ext)], "Script", "server_script", true
	}
	if strings.HasSuffix(lower, ".client.luau") || strings.HasSuffix(lower, ".client.lua") {
		ext := ".client.luau"
		if strings.HasSuffix(lower, ".client.lua") {
			ext = ".client.lua"
		}
		return filename[:len(filename)-len(ext)], "LocalScript", "client_script", true
	}
	if strings.HasSuffix(lower, ".module.luau") || strings.HasSuffix(lower, ".module.lua") {
		ext := ".module.luau"
		if strings.HasSuffix(lower, ".module.lua") {
			ext = ".module.lua"
		}
		return filename[:len(filename)-len(ext)], "ModuleScript", "module_script", true
	}
	if strings.HasSuffix(lower, ".luau") || strings.HasSuffix(lower, ".lua") {
		ext := filepath.Ext(filename)
		return filename[:len(filename)-len(ext)], "ModuleScript", "module_script", true
	}
	return "", "", "", false
}

func classifyKind(className string, isService bool) string {
	if isService {
		return "service"
	}
	switch strings.ToLower(className) {
	case "script":
		return "server_script"
	case "localscript":
		return "client_script"
	case "modulescript":
		return "module_script"
	case "screengui":
		return "screengui"
	case "frame":
		return "frame"
	case "textlabel":
		return "textlabel"
	case "textbutton":
		return "textbutton"
	case "imagelabel":
		return "imagelabel"
	case "imagebutton":
		return "imagebutton"
	case "scrollingframe":
		return "scrollingframe"
	case "canvasgroup":
		return "canvasgroup"
	case "uilistlayout", "uigridlayout", "uipadding", "uicorner", "uistroke", "uiscale", "uiaspectratioconstraint":
		return "uilayout"
	case "model":
		return "model"
	case "part", "meshpart", "wedgepart", "cornerwedgepart", "trusspart":
		return "part"
	case "configuration":
		return "configuration"
	case "intvalue", "stringvalue", "boolvalue", "numbervalue", "objectvalue", "color3value", "cframevalue", "vector3value":
		return "value"
	case "remoteevent", "remotefunction", "bindableevent", "bindablefunction":
		return "event"
	case "folder":
		return "folder"
	default:
		return "folder"
	}
}

func isServiceDir(name string) bool {
	switch strings.ToLower(name) {
	case "workspace", "replicatedstorage", "serverscriptservice", "serverstorage",
		"startergui", "starterplayer", "starterpack", "lighting", "soundservice", "textchatservice":
		return true
	default:
		return false
	}
}

func BuildExplorer(syncRoot string) (*TreeSummary, error) {
	manifest, err := loadManifest(syncRoot)
	if err != nil {
		return nil, err
	}

	rootNode := &ExplorerNode{
		Name:      filepath.Base(syncRoot),
		ClassName: "DataModel",
		Path:      "",
		Kind:      "folder",
		Children:  []*ExplorerNode{},
	}

	totalScripts := 0
	protectedScripts := 0
	disabledScripts := 0
	totalInstances := 0

	nodeMap := make(map[string]*ExplorerNode)
	nodeMap[""] = rootNode

	// Walk all files and directories using WalkDir (faster, no lstat)
	var allDirs []string
	var allFiles []string
	dirFilesMap := make(map[string][]string)

	err = filepath.WalkDir(syncRoot, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, rErr := filepath.Rel(syncRoot, p)
		if rErr != nil || rel == "." {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" || name == "build" || name == "target" || name == "vendor" {
				return filepath.SkipDir
			}
			allDirs = append(allDirs, rel)
		} else {
			slashParent := filepath.ToSlash(filepath.Dir(rel))
			if slashParent == "." {
				slashParent = ""
			}
			dirFilesMap[slashParent] = append(dirFilesMap[slashParent], name)
			allFiles = append(allFiles, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(allDirs)
	sort.Strings(allFiles)

	consumedFiles := make(map[string]bool)

	// Step 1: Process directories
	for _, rel := range allDirs {
		slashRel := filepath.ToSlash(rel)
		parts := strings.Split(slashRel, "/")
		fullDir := filepath.Join(syncRoot, filepath.FromSlash(slashRel))
		dirName := filepath.Base(fullDir)

		isService := len(parts) == 1 && isServiceDir(dirName)
		var instanceName, className string

		if isService {
			instanceName = dirName
			className = dirName
		} else if split := strings.LastIndex(dirName, "."); split > 0 {
			instanceName = dirName[:split]
			className = dirName[split+1:]
		} else {
			instanceName = dirName
			className = "Folder"
		}

		kind := classifyKind(className, isService)

		node := &ExplorerNode{
			Name:       instanceName,
			ClassName:  className,
			Path:       slashRel,
			Kind:       kind,
			Properties: make(map[string]any),
			Children:   []*ExplorerNode{},
		}

		fileNames := dirFilesMap[slashRel]
		hasProps := false
		for _, fn := range fileNames {
			if fn == "properties.init.json" {
				hasProps = true
				break
			}
		}

		// Check for properties.init.json inside this directory
		if hasProps {
			propsPath := filepath.Join(fullDir, "properties.init.json")
			propsRel := filepath.ToSlash(filepath.Join(slashRel, "properties.init.json"))
			if pData, pErr := os.ReadFile(propsPath); pErr == nil {
				var pObj struct {
					Name       string         `json:"name,omitempty"`
					ClassName  string         `json:"className,omitempty"`
					Properties map[string]any `json:"properties,omitempty"`
				}
				if json.Unmarshal(pData, &pObj) == nil {
					if pObj.Name != "" {
						node.Name = pObj.Name
					}
					if pObj.ClassName != "" {
						node.ClassName = pObj.ClassName
						node.Kind = classifyKind(node.ClassName, isService)
					}
					if pObj.Properties != nil {
						node.Properties = pObj.Properties
					}
				}
				node.PropertiesRelPath = propsRel
				consumedFiles[propsRel] = true
			}
		}

		// Check if this directory represents a script instance with an inner script file
		for _, fn := range fileNames {
			sName, sClass, sKind, isScript := parseScriptInfo(fn)
			if isScript {
				// Matches instance name or standard init script
				eLower := strings.ToLower(fn)
				isInit := strings.HasPrefix(eLower, "init.")
				matchesName := strings.EqualFold(sName, instanceName)
				if isInit || matchesName || strings.EqualFold(sClass, className) {
					innerScriptRel := filepath.ToSlash(filepath.Join(slashRel, fn))
					node.IsScript = true
					node.ScriptRelPath = innerScriptRel
					node.Kind = sKind
					consumedFiles[innerScriptRel] = true
					break
				}
			}
		}

		nodeMap[slashRel] = node
	}

	// Step 2: Process standalone files not consumed by instance directories
	for _, rel := range allFiles {
		slashRel := filepath.ToSlash(rel)
		if consumedFiles[slashRel] {
			continue
		}
		name, className, kind, isScript := parseScriptInfo(filepath.Base(slashRel))
		if !isScript {
			// Skip other non-script assets from being tree nodes
			continue
		}

		node := &ExplorerNode{
			Name:          name,
			ClassName:     className,
			Path:          slashRel,
			Kind:          kind,
			IsScript:      true,
			ScriptRelPath: slashRel,
			Properties:    make(map[string]any),
			Children:      []*ExplorerNode{},
		}

		parentRel := filepath.ToSlash(filepath.Dir(slashRel))
		if parentRel == "." {
			parentRel = ""
		}
		parentFiles := dirFilesMap[parentRel]
		hasProps := false
		for _, fn := range parentFiles {
			if fn == "properties.init.json" {
				hasProps = true
				break
			}
		}

		if hasProps {
			fullPath := filepath.Join(syncRoot, filepath.FromSlash(slashRel))
			parentDir := filepath.Dir(fullPath)
			propsFile := filepath.Join(parentDir, "properties.init.json")
			propsRel := filepath.ToSlash(filepath.Join(parentRel, "properties.init.json"))
			if pData, pErr := os.ReadFile(propsFile); pErr == nil {
				var pObj struct {
					Properties map[string]any `json:"properties,omitempty"`
				}
				if json.Unmarshal(pData, &pObj) == nil && pObj.Properties != nil {
					node.Properties = pObj.Properties
					node.PropertiesRelPath = propsRel
				}
			}
		}

		nodeMap[slashRel] = node
	}

	// Step 3: Link tree hierarchy
	for slashRel, node := range nodeMap {
		if slashRel == "" {
			continue
		}
		totalInstances++

		// Script status (enabled / protected)
		if node.IsScript {
			totalScripts++
			isEnabled := true
			if node.Properties != nil {
				if enVal, has := node.Properties["Enabled"]; has {
					if bVal, ok := enVal.(bool); ok {
						isEnabled = bVal
					}
				} else if disVal, hasDis := node.Properties["Disabled"]; hasDis {
					if bDis, ok := disVal.(bool); ok {
						isEnabled = !bDis
					}
				}
			}
			node.Enabled = isEnabled
			if !isEnabled {
				disabledScripts++
			}

			isProtected := false
			if node.ScriptRelPath != "" {
				if _, exists := manifest.Entries[node.ScriptRelPath]; exists {
					isProtected = true
				}
			}
			if !isProtected && node.Path != "" {
				if _, exists := manifest.Entries[node.Path]; exists {
					isProtected = true
				}
			}
			if !isProtected && node.ScriptRelPath != "" {
				vPath := vaultFilePath(syncRoot, node.ScriptRelPath)
				if _, err := os.Stat(vPath); err == nil {
					isProtected = true
					manifest.Entries[node.ScriptRelPath] = VaultEntry{
						RelPath:   node.ScriptRelPath,
						Timestamp: time.Now(),
					}
				}
			}
			node.Protected = isProtected
			if isProtected {
				protectedScripts++
			}
		}

		// Find parent node
		parts := strings.Split(slashRel, "/")
		parentSlashRel := ""
		if len(parts) > 1 {
			parentSlashRel = strings.Join(parts[:len(parts)-1], "/")
		}

		parentNode, parentExists := nodeMap[parentSlashRel]
		if parentExists {
			parentNode.Children = append(parentNode.Children, node)
		} else {
			rootNode.Children = append(rootNode.Children, node)
		}
	}

	// Prune empty generic folders that have 0 children
	pruneEmptyGenericFolders(rootNode)

	// Sort children by name
	sortNodeChildren(rootNode)

	return &TreeSummary{
		TotalScripts:     totalScripts,
		TotalInstances:   totalInstances,
		ProtectedScripts: protectedScripts,
		OriginalScripts:  totalScripts - protectedScripts,
		DisabledScripts:  disabledScripts,
		Tree:             rootNode,
	}, nil
}

func pruneEmptyGenericFolders(node *ExplorerNode) bool {
	var kept []*ExplorerNode
	for _, child := range node.Children {
		if pruneEmptyGenericFolders(child) {
			kept = append(kept, child)
		}
	}
	node.Children = kept

	// Keep if it's root, service, script, has properties, has a canonical class name, or has children
	if node.Path == "" || node.Kind == "service" || node.IsScript || len(node.Properties) > 0 || node.ClassName != "Folder" || len(node.Children) > 0 {
		return true
	}
	return false
}

func sortNodeChildren(node *ExplorerNode) {
	sort.SliceStable(node.Children, func(i, j int) bool {
		// Services on top, then folders, then instances
		iRank := 2
		jRank := 2
		if node.Children[i].Kind == "service" {
			iRank = 0
		} else if !node.Children[i].IsScript {
			iRank = 1
		}
		if node.Children[j].Kind == "service" {
			jRank = 0
		} else if !node.Children[j].IsScript {
			jRank = 1
		}
		if iRank != jRank {
			return iRank < jRank
		}
		return strings.ToLower(node.Children[i].Name) < strings.ToLower(node.Children[j].Name)
	})
	for _, c := range node.Children {
		sortNodeChildren(c)
	}
}

func collectScriptFiles(syncRoot string, relPaths []string) []string {
	var collected []string
	seen := make(map[string]bool)

	for _, rel := range relPaths {
		cleanRel := filepath.ToSlash(filepath.Clean(rel))
		if cleanRel == "." || cleanRel == "" {
			continue
		}
		fullPath := filepath.Join(syncRoot, filepath.FromSlash(cleanRel))
		fi, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		if fi.IsDir() {
			_ = filepath.Walk(fullPath, func(subPath string, subFi os.FileInfo, walkErr error) error {
				if walkErr != nil || subFi.IsDir() {
					return nil
				}
				subRel, rErr := filepath.Rel(syncRoot, subPath)
				if rErr != nil {
					return nil
				}
				_, _, _, isScript := parseScriptInfo(subFi.Name())
				if isScript {
					sRel := filepath.ToSlash(subRel)
					if !seen[sRel] {
						seen[sRel] = true
						collected = append(collected, sRel)
					}
				}
				return nil
			})
		} else {
			_, _, _, isScript := parseScriptInfo(filepath.Base(cleanRel))
			if isScript && !seen[cleanRel] {
				seen[cleanRel] = true
				collected = append(collected, cleanRel)
			}
		}
	}
	return collected
}

func ProtectScripts(syncRoot string, relPaths []string, customWatermark string) ([]string, error) {
	targets := collectScriptFiles(syncRoot, relPaths)
	if len(targets) == 0 {
		return nil, nil
	}

	manifest, err := loadManifest(syncRoot)
	if err != nil {
		return nil, err
	}

	var processed []string
	for _, cleanRel := range targets {
		fullPath := filepath.Join(syncRoot, filepath.FromSlash(cleanRel))

		// Read source
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return processed, fmt.Errorf("read %s: %w", cleanRel, err)
		}

		// Save original to vault if not already protected
		vaultPath := vaultFilePath(syncRoot, cleanRel)
		if _, exists := manifest.Entries[cleanRel]; !exists {
			if _, vErr := os.Stat(vaultPath); os.IsNotExist(vErr) {
				if err := os.MkdirAll(filepath.Dir(vaultPath), 0755); err != nil {
					return processed, fmt.Errorf("create vault dir: %w", err)
				}
				if err := os.WriteFile(vaultPath, content, 0644); err != nil {
					return processed, fmt.Errorf("backup to vault: %w", err)
				}
			}
			hasher := sha256.New()
			hasher.Write(content)
			hashStr := hex.EncodeToString(hasher.Sum(nil))

			manifest.Entries[cleanRel] = VaultEntry{
				RelPath:      cleanRel,
				OriginalHash: hashStr,
				Timestamp:    time.Now(),
				Watermark:    customWatermark,
			}
		}

		// Obfuscate
		obfuscated, err := Obfuscate(string(content))
		if err != nil {
			return processed, fmt.Errorf("obfuscate %s: %w", cleanRel, err)
		}

		if strings.TrimSpace(customWatermark) != "" {
			obfuscated = fmt.Sprintf("--[[ %s ]]\n%s", strings.TrimSpace(customWatermark), obfuscated)
		}

		// Overwrite active file
		if err := os.WriteFile(fullPath, []byte(obfuscated), 0644); err != nil {
			return processed, fmt.Errorf("write obfuscated %s: %w", cleanRel, err)
		}

		processed = append(processed, cleanRel)
	}

	if err := saveManifest(syncRoot, manifest); err != nil {
		return processed, err
	}

	return processed, nil
}

func RestoreScripts(syncRoot string, relPaths []string) ([]string, error) {
	manifest, err := loadManifest(syncRoot)
	if err != nil {
		return nil, err
	}

	// Auto-discover any orphaned vault files in .rblxsync/vault/files/
	vaultRoot := filepath.Join(vaultDir(syncRoot), "files")
	if vFi, vErr := os.Stat(vaultRoot); vErr == nil && vFi.IsDir() {
		_ = filepath.Walk(vaultRoot, func(p string, fi os.FileInfo, wErr error) error {
			if wErr != nil || fi.IsDir() {
				return nil
			}
			vRel, rErr := filepath.Rel(vaultRoot, p)
			if rErr == nil {
				sRel := filepath.ToSlash(vRel)
				if _, has := manifest.Entries[sRel]; !has {
					manifest.Entries[sRel] = VaultEntry{
						RelPath:   sRel,
						Timestamp: fi.ModTime(),
					}
				}
			}
			return nil
		})
	}

	targetSet := make(map[string]bool)
	for _, t := range collectScriptFiles(syncRoot, relPaths) {
		targetSet[t] = true
	}
	for _, rel := range relPaths {
		cleanRel := filepath.ToSlash(filepath.Clean(rel))
		if cleanRel == "." || cleanRel == "" {
			continue
		}
		for mRel := range manifest.Entries {
			if mRel == cleanRel || strings.HasPrefix(mRel, cleanRel+"/") {
				targetSet[mRel] = true
			}
		}
	}

	var restored []string
	for cleanRel := range targetSet {
		entry, exists := manifest.Entries[cleanRel]
		if !exists {
			vPath := vaultFilePath(syncRoot, cleanRel)
			if _, vErr := os.Stat(vPath); vErr == nil {
				entry = VaultEntry{RelPath: cleanRel}
				exists = true
			}
		}
		if !exists {
			continue
		}

		vaultPath := vaultFilePath(syncRoot, entry.RelPath)
		origContent, err := os.ReadFile(vaultPath)
		if err != nil {
			continue
		}

		fullPath := filepath.Join(syncRoot, filepath.FromSlash(cleanRel))
		if err := os.WriteFile(fullPath, origContent, 0644); err != nil {
			return restored, fmt.Errorf("restore %s: %w", cleanRel, err)
		}

		// Remove from vault
		_ = os.Remove(vaultPath)
		delete(manifest.Entries, cleanRel)
		restored = append(restored, cleanRel)
	}

	if err := saveManifest(syncRoot, manifest); err != nil {
		return restored, err
	}

	return restored, nil
}

func SetInstanceProperties(syncRoot string, relPath string, newProps map[string]any) error {
	cleanRel := filepath.ToSlash(filepath.Clean(relPath))
	fullPath := filepath.Join(syncRoot, filepath.FromSlash(cleanRel))

	fi, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("stat path %s: %w", cleanRel, err)
	}

	targetDir := fullPath
	if !fi.IsDir() {
		parentDir := filepath.Dir(fullPath)
		parentBase := filepath.Base(parentDir)
		isParentInstance := strings.Contains(parentBase, ".")

		if isParentInstance {
			targetDir = parentDir
		} else {
			name, className, _, isScript := parseScriptInfo(filepath.Base(cleanRel))
			if !isScript {
				className = "Folder"
				name = filepath.Base(cleanRel)
			}
			folderName := fmt.Sprintf("%s.%s", name, className)
			targetDir = filepath.Join(parentDir, folderName)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				return fmt.Errorf("create instance dir %s: %w", targetDir, err)
			}
			newFilePath := filepath.Join(targetDir, filepath.Base(cleanRel))
			if err := os.Rename(fullPath, newFilePath); err != nil {
				return fmt.Errorf("move file to %s: %w", newFilePath, err)
			}
		}
	}

	propsFile := filepath.Join(targetDir, "properties.init.json")
	var pObj map[string]any
	if pData, pErr := os.ReadFile(propsFile); pErr == nil {
		_ = json.Unmarshal(pData, &pObj)
	}
	if pObj == nil {
		pObj = make(map[string]any)
		baseName := filepath.Base(targetDir)
		if split := strings.LastIndex(baseName, "."); split > 0 {
			pObj["name"] = baseName[:split]
			pObj["className"] = baseName[split+1:]
		} else {
			pObj["name"] = baseName
			pObj["className"] = "Folder"
		}
	}

	propsMap, ok := pObj["properties"].(map[string]any)
	if !ok || propsMap == nil {
		propsMap = make(map[string]any)
	}

	for k, v := range newProps {
		propsMap[k] = v
		if k == "Enabled" {
			delete(propsMap, "Disabled")
		}
	}
	pObj["properties"] = propsMap

	outData, err := json.MarshalIndent(pObj, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal properties: %w", err)
	}
	if err := os.WriteFile(propsFile, outData, 0644); err != nil {
		return fmt.Errorf("write properties: %w", err)
	}
	return nil
}

func SetScriptsEnabled(syncRoot string, relPaths []string, enabled bool) ([]string, error) {
	var updated []string
	for _, rel := range relPaths {
		err := SetInstanceProperties(syncRoot, rel, map[string]any{"Enabled": enabled})
		if err != nil {
			return updated, err
		}
		updated = append(updated, filepath.ToSlash(filepath.Clean(rel)))
	}
	return updated, nil
}

func RevealPath(syncRoot, relPath string) error {
	fullPath := filepath.Join(syncRoot, filepath.FromSlash(filepath.Clean(relPath)))
	return exec.Command("open", "-R", fullPath).Run()
}

func RenameInstance(syncRoot, relPath, newName string) (string, error) {
	cleanRel := filepath.ToSlash(filepath.Clean(relPath))
	if cleanRel == "" || cleanRel == "." {
		return "", fmt.Errorf("cannot rename root folder")
	}
	parts := strings.Split(cleanRel, "/")
	if len(parts) == 1 && isServiceDir(parts[0]) {
		return "", fmt.Errorf("cannot rename root service '%s'", parts[0])
	}
	trimmedNewName := strings.TrimSpace(newName)
	if trimmedNewName == "" {
		return "", fmt.Errorf("new name cannot be empty")
	}
	if strings.ContainsAny(trimmedNewName, `/\:;*?"<>|`) {
		return "", fmt.Errorf("new name contains invalid characters")
	}

	fullPath := filepath.Join(syncRoot, filepath.FromSlash(cleanRel))
	fi, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("stat path %s: %w", cleanRel, err)
	}

	parentDir := filepath.Dir(fullPath)
	parentRel := filepath.ToSlash(filepath.Dir(cleanRel))
	if parentRel == "." {
		parentRel = ""
	}

	if fi.IsDir() {
		baseName := filepath.Base(fullPath)
		var newFolderName string
		var oldInstanceName string
		if split := strings.LastIndex(baseName, "."); split > 0 {
			oldInstanceName = baseName[:split]
			className := baseName[split+1:]
			newFolderName = fmt.Sprintf("%s.%s", trimmedNewName, className)
		} else {
			oldInstanceName = baseName
			newFolderName = trimmedNewName
		}

		newFullPath := filepath.Join(parentDir, newFolderName)
		if strings.EqualFold(fullPath, newFullPath) && fullPath != newFullPath {
			// Case-only rename on case-insensitive filesystems (macOS default APFS)
			tempPath := filepath.Join(parentDir, fmt.Sprintf("%s.tmp_rename_%d", newFolderName, time.Now().UnixNano()))
			if err := os.Rename(fullPath, tempPath); err != nil {
				return "", fmt.Errorf("temp rename folder: %w", err)
			}
			if err := os.Rename(tempPath, newFullPath); err != nil {
				return "", fmt.Errorf("rename folder to %s: %w", newFolderName, err)
			}
		} else {
			if err := os.Rename(fullPath, newFullPath); err != nil {
				return "", fmt.Errorf("rename folder to %s: %w", newFolderName, err)
			}
		}

		// Update properties.init.json if present
		propsFile := filepath.Join(newFullPath, "properties.init.json")
		if pData, pErr := os.ReadFile(propsFile); pErr == nil {
			var pObj map[string]any
			if json.Unmarshal(pData, &pObj) == nil && pObj != nil {
				pObj["name"] = trimmedNewName
				if outData, err := json.MarshalIndent(pObj, "", "  "); err == nil {
					_ = os.WriteFile(propsFile, outData, 0644)
				}
			}
		}

		// Rename matching inner script if exists
		entries, _ := os.ReadDir(newFullPath)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			fn := e.Name()
			sName, _, _, isScript := parseScriptInfo(fn)
			if isScript && strings.EqualFold(sName, oldInstanceName) {
				ext := ""
				if strings.HasSuffix(fn, ".server.luau") {
					ext = ".server.luau"
				} else if strings.HasSuffix(fn, ".client.luau") {
					ext = ".client.luau"
				} else if strings.HasSuffix(fn, ".module.luau") {
					ext = ".module.luau"
				} else {
					ext = filepath.Ext(fn)
				}
				newScriptFile := filepath.Join(newFullPath, trimmedNewName+ext)
				_ = os.Rename(filepath.Join(newFullPath, fn), newScriptFile)
				break
			}
		}

		newRel := filepath.ToSlash(filepath.Join(parentRel, newFolderName))
		return newRel, nil
	}

	// Standalone file
	baseName := filepath.Base(fullPath)
	ext := ""
	if strings.HasSuffix(baseName, ".server.luau") {
		ext = ".server.luau"
	} else if strings.HasSuffix(baseName, ".client.luau") {
		ext = ".client.luau"
	} else if strings.HasSuffix(baseName, ".module.luau") {
		ext = ".module.luau"
	} else {
		ext = filepath.Ext(baseName)
	}
	newFileName := trimmedNewName + ext
	newFullPath := filepath.Join(parentDir, newFileName)

	if strings.EqualFold(fullPath, newFullPath) && fullPath != newFullPath {
		tempPath := filepath.Join(parentDir, fmt.Sprintf("%s.tmp_rename_%d", newFileName, time.Now().UnixNano()))
		if err := os.Rename(fullPath, tempPath); err != nil {
			return "", fmt.Errorf("temp rename file: %w", err)
		}
		if err := os.Rename(tempPath, newFullPath); err != nil {
			return "", fmt.Errorf("rename file to %s: %w", newFileName, err)
		}
	} else {
		if err := os.Rename(fullPath, newFullPath); err != nil {
			return "", fmt.Errorf("rename file to %s: %w", newFileName, err)
		}
	}

	newRel := filepath.ToSlash(filepath.Join(parentRel, newFileName))
	return newRel, nil
}

func DeleteInstance(syncRoot, relPath string) error {
	cleanRel := filepath.ToSlash(filepath.Clean(relPath))
	if cleanRel == "" || cleanRel == "." {
		return fmt.Errorf("cannot delete root folder")
	}
	parts := strings.Split(cleanRel, "/")
	if len(parts) == 1 && isServiceDir(parts[0]) {
		return fmt.Errorf("cannot delete root service '%s'", parts[0])
	}

	fullPath := filepath.Join(syncRoot, filepath.FromSlash(cleanRel))
	if _, err := os.Stat(fullPath); err != nil {
		return fmt.Errorf("stat path %s: %w", cleanRel, err)
	}

	return os.RemoveAll(fullPath)
}

