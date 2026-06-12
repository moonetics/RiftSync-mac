package records

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

const (
	EntityScript     = "script"
	EntityUIInstance = "ui_instance"

	UIPropertiesFilename = "properties.init.json"
	UIInitMetaFilename   = "init.meta.json"
	UIModelJSONSuffix    = ".model.json"
)

var scriptSuffixToClass = map[string]string{
	".server.luau": "Script",
	".client.luau": "LocalScript",
	".module.luau": "ModuleScript",
	".server.lua":  "Script",
	".client.lua":  "LocalScript",
	".module.lua":  "ModuleScript",
}

var canonicalSourceFilenameToClass = map[string]string{
	"source.server.luau": "Script",
	"source.server.lua":  "Script",
	"source.client.luau": "LocalScript",
	"source.client.lua":  "LocalScript",
	"source.module.luau": "ModuleScript",
	"source.module.lua":  "ModuleScript",
}

var scriptClassToSourceSuffix = map[string]string{
	"Script":       ".server.luau",
	"LocalScript":  ".client.luau",
	"ModuleScript": ".module.luau",
}

var scriptClasses = map[string]bool{
	"Script":       true,
	"LocalScript":  true,
	"ModuleScript": true,
}

var rootServiceClass = map[string]string{
	"StarterGui":          "StarterGui",
	"Workspace":           "Workspace",
	"Lighting":            "Lighting",
	"ReplicatedStorage":   "ReplicatedStorage",
	"ReplicatedFirst":     "ReplicatedFirst",
	"StarterPlayer":       "StarterPlayer",
	"StarterPack":         "StarterPack",
	"ServerScriptService": "ServerScriptService",
	"ServerStorage":       "ServerStorage",
}

type SyncRecord struct {
	Entity      string
	LocalPath   string
	LocalDir    string
	RbxPath     string
	ClassName   string
	Source      string
	Payload     map[string]any
	StableID    string
	ContentHash string
}

func (r SyncRecord) ToUpsert() map[string]any {
	data := map[string]any{
		"op":           "upsert",
		"entity":       r.Entity,
		"local_path":   r.LocalPath,
		"local_dir":    r.LocalDir,
		"rbx_path":     r.RbxPath,
		"class_name":   r.ClassName,
		"stable_id":    r.StableID,
		"content_hash": r.ContentHash,
	}
	if r.Entity == EntityScript {
		data["source"] = r.Source
		if r.Payload != nil {
			data["payload"] = r.Payload
		}
	} else if r.Payload != nil {
		data["payload"] = r.Payload
	}
	return data
}

func CompactChange(change map[string]any) map[string]any {
	result := map[string]any{}
	if op, ok := change["op"].(string); ok {
		switch op {
		case "upsert":
			result["o"] = "u"
		case "delete":
			result["o"] = "d"
		case "rename":
			result["o"] = "r"
		default:
			result["o"] = op
		}
	}
	if entity, ok := change["entity"].(string); ok {
		switch entity {
		case EntityScript:
			result["e"] = "s"
		case EntityUIInstance:
			result["e"] = "ui"
		default:
			result["e"] = entity
		}
	}
	copyCompactKey(result, change, "l", "local_path")
	copyCompactKey(result, change, "ld", "local_dir")
	copyCompactKey(result, change, "rp", "rbx_path")
	copyCompactKey(result, change, "c", "class_name")
	copyCompactKey(result, change, "sid", "stable_id")
	copyCompactKey(result, change, "h", "content_hash")
	copyCompactKey(result, change, "src", "source")
	copyCompactKey(result, change, "p", "payload")
	copyCompactKey(result, change, "ol", "old_local_path")
	copyCompactKey(result, change, "or", "old_rbx_path")
	copyCompactKey(result, change, "nr", "new_rbx_path")
	return result
}

func ExpandCompactChange(change map[string]any) map[string]any {
	result := map[string]any{}
	if op, ok := change["o"].(string); ok {
		switch op {
		case "u":
			result["op"] = "upsert"
		case "d":
			result["op"] = "delete"
		case "r":
			result["op"] = "rename"
		default:
			result["op"] = op
		}
	}
	if entity, ok := change["e"].(string); ok {
		switch entity {
		case "s":
			result["entity"] = EntityScript
		case "ui":
			result["entity"] = EntityUIInstance
		default:
			result["entity"] = entity
		}
	}
	copyCompactKey(result, change, "local_path", "l")
	copyCompactKey(result, change, "local_dir", "ld")
	copyCompactKey(result, change, "rbx_path", "rp")
	copyCompactKey(result, change, "class_name", "c")
	copyCompactKey(result, change, "stable_id", "sid")
	copyCompactKey(result, change, "content_hash", "h")
	copyCompactKey(result, change, "source", "src")
	copyCompactKey(result, change, "payload", "p")
	copyCompactKey(result, change, "old_local_path", "ol")
	copyCompactKey(result, change, "old_rbx_path", "or")
	copyCompactKey(result, change, "new_rbx_path", "nr")
	return result
}

func copyCompactKey(dst, src map[string]any, dstKey, srcKey string) {
	if value, ok := src[srcKey]; ok {
		dst[dstKey] = value
	}
}

func (r SyncRecord) ToDelete() map[string]any {
	return map[string]any{
		"op":         "delete",
		"entity":     r.Entity,
		"local_path": r.LocalPath,
		"local_dir":  r.LocalDir,
		"rbx_path":   r.RbxPath,
		"class_name": r.ClassName,
		"stable_id":  r.StableID,
	}
}

func BuildRenameChange(oldRecord, newRecord SyncRecord) map[string]any {
	payload := map[string]any{
		"op":             "rename",
		"entity":         newRecord.Entity,
		"local_path":     newRecord.LocalPath,
		"local_dir":      newRecord.LocalDir,
		"old_local_path": oldRecord.LocalPath,
		"old_rbx_path":   oldRecord.RbxPath,
		"new_rbx_path":   newRecord.RbxPath,
		"rbx_path":       newRecord.RbxPath,
		"class_name":     newRecord.ClassName,
		"stable_id":      firstNonEmpty(newRecord.StableID, oldRecord.StableID),
		"content_hash":   newRecord.ContentHash,
	}
	if newRecord.Entity == EntityScript {
		payload["source"] = newRecord.Source
		if newRecord.Payload != nil {
			payload["payload"] = newRecord.Payload
		}
	} else if newRecord.Payload != nil {
		payload["payload"] = newRecord.Payload
	}
	return payload
}

func IdentityKey(record SyncRecord) string {
	if record.Entity == EntityScript {
		return record.Entity + "\x00" + record.RbxPath
	}
	if record.Entity == EntityUIInstance {
		return record.Entity + "\x00rbx_path\x00" + record.RbxPath
	}
	return record.Entity + "\x00" + record.RbxPath + "\x00" + record.LocalPath
}

func PreferenceScore(record SyncRecord) int {
	score := 0
	switch record.Entity {
	case EntityScript:
		if IsNamedScriptSourceLocalPath(record.LocalPath) {
			score += 120
		} else if IsCanonicalScriptLocalPath(record.LocalPath) {
			score += 100
		}
		if record.Payload != nil {
			score += 10
		}
		if strings.HasSuffix(strings.ToLower(record.LocalPath), ".luau") {
			score += 3
		}
	case EntityUIInstance:
		if IsRojoUILocalPath(record.LocalPath) {
			score += 110
		}
		if IsCanonicalUILocalPath(record.LocalPath) {
			score += 100
		}
		if record.StableID != "" {
			score += 20
		}
		if record.Payload != nil {
			if stringValue(record.Payload["className"]) != "" {
				score += 3
			}
			if stringValue(record.Payload["name"]) != "" {
				score += 2
			}
		}
	}
	return score
}

func ChoosePreferredRecord(existing, candidate SyncRecord) (bool, string) {
	existingScore := PreferenceScore(existing)
	candidateScore := PreferenceScore(candidate)
	if candidateScore > existingScore {
		return true, fmt.Sprintf("higher score %d>%d", candidateScore, existingScore)
	}
	if candidateScore < existingScore {
		return false, fmt.Sprintf("lower score %d<%d", candidateScore, existingScore)
	}
	if len(candidate.LocalPath) < len(existing.LocalPath) {
		return true, "same score, shorter local_path"
	}
	if len(candidate.LocalPath) > len(existing.LocalPath) {
		return false, "same score, longer local_path"
	}
	if candidate.LocalPath < existing.LocalPath {
		return true, "same score and length, lexical preference"
	}
	return false, "same score and length, lexical keep-existing"
}

func IsNamedScriptSourceLocalPath(localPath string) bool {
	parts := splitPath(localPath)
	if len(parts) < 3 {
		return false
	}
	filename := parts[len(parts)-1]
	if _, ok := canonicalSourceFilenameToClass[strings.ToLower(filename)]; ok {
		return false
	}
	filenameStem, className, _, ok := splitScriptSourceFilename(filename)
	if !ok {
		return false
	}
	decodedNames, classStack, err := decodeInstanceFolderParts(parts[1 : len(parts)-1])
	if err != nil || len(decodedNames) == 0 {
		return false
	}
	leafClass := classStack[len(classStack)-1]
	return scriptClasses[leafClass] && leafClass == className && decodeLocalSegment(filenameStem) == decodedNames[len(decodedNames)-1]
}

func IsCanonicalScriptLocalPath(localPath string) bool {
	parts := splitPath(localPath)
	if len(parts) < 3 {
		return false
	}
	_, canonical := canonicalSourceFilenameToClass[strings.ToLower(parts[len(parts)-1])]
	return canonical || IsNamedScriptSourceLocalPath(localPath)
}

func ScriptSourcePathForPropertiesPath(localPath string) (string, bool, error) {
	parts := splitPath(localPath)
	if len(parts) < 3 || parts[len(parts)-1] != UIPropertiesFilename {
		return "", false, nil
	}
	folderParts := parts[1 : len(parts)-1]
	if len(folderParts) == 0 {
		return "", false, nil
	}
	lastFolder := folderParts[len(folderParts)-1]
	splitIndex := strings.LastIndex(lastFolder, ".")
	if splitIndex <= 0 || splitIndex >= len(lastFolder)-1 {
		return "", false, nil
	}
	className := lastFolder[splitIndex+1:]
	suffix := scriptClassToSourceSuffix[className]
	if suffix == "" {
		return "", false, nil
	}
	decodedNames, classStack, err := decodeInstanceFolderParts(folderParts)
	if err != nil {
		return "", false, err
	}
	if len(decodedNames) == 0 || classStack[len(classStack)-1] != className {
		return "", false, nil
	}
	sourceFilename := lastFolder[:splitIndex] + suffix
	return path.Join(path.Dir(normalizeSlash(localPath)), sourceFilename), true, nil
}

func ScriptPropertiesPathForSourcePath(localPath string) (string, bool, error) {
	parts := splitPath(localPath)
	if len(parts) < 3 {
		return "", false, nil
	}
	filename := parts[len(parts)-1]
	stem, className, _, ok := splitScriptSourceFilename(filename)
	if !ok {
		return "", false, nil
	}
	folderParts := parts[1 : len(parts)-1]
	if len(folderParts) == 0 {
		return "", false, nil
	}
	decodedNames, classStack, err := decodeInstanceFolderParts(folderParts)
	if err != nil {
		return "", false, err
	}
	if len(decodedNames) == 0 || classStack[len(classStack)-1] != className {
		return "", false, nil
	}
	if decodeLocalSegment(stem) != decodedNames[len(decodedNames)-1] {
		return "", false, nil
	}
	return path.Join(path.Dir(normalizeSlash(localPath)), UIPropertiesFilename), true, nil
}

func IsCanonicalUILocalPath(localPath string) bool {
	parts := splitPath(localPath)
	if len(parts) < 2 || parts[len(parts)-1] != UIPropertiesFilename {
		return false
	}
	folderParts := parts[1 : len(parts)-1]
	if len(folderParts) == 0 {
		return true
	}
	for _, segment := range folderParts {
		if _, _, ok := parseUISegment(segment); !ok {
			return false
		}
	}
	return true
}

func IsRojoUILocalPath(localPath string) bool {
	parts := splitPath(localPath)
	if len(parts) < 2 {
		return false
	}
	filename := parts[len(parts)-1]
	return filename == UIInitMetaFilename || strings.HasSuffix(filename, UIModelJSONSuffix)
}

func NewScriptRecord(relativePath, source string, managedServices map[string]bool, ignored []string) (*SyncRecord, error) {
	relPosix := normalizeSlash(relativePath)
	parts := splitPath(relPosix)
	if len(parts) == 0 {
		return nil, nil
	}
	filename := parts[len(parts)-1]
	lowerFilename := strings.ToLower(filename)

	if canonicalClass, ok := canonicalSourceFilenameToClass[lowerFilename]; ok {
		return newCanonicalScriptRecord(relPosix, parts, source, canonicalClass, managedServices, ignored)
	}

	stem, className, suffix, ok := splitScriptSourceFilename(filename)
	if !ok {
		return nil, nil
	}
	return newNamedScriptRecord(relPosix, parts, stem, className, suffix, source, managedServices, ignored)
}

func NewUIRecord(relativePath, source string, managedServices map[string]bool, ignored []string) (*SyncRecord, error) {
	relPosix := normalizeSlash(relativePath)
	parts := splitPath(relPosix)
	if len(parts) < 2 || parts[len(parts)-1] != UIPropertiesFilename {
		return nil, nil
	}

	serviceName := decodeLocalSegment(parts[0])
	if !managedServices[serviceName] {
		return nil, nil
	}

	folderParts := parts[1 : len(parts)-1]
	decodedNames := []string{}
	classStack := []string{}
	useRootService := len(folderParts) == 0
	if !useRootService {
		var err error
		decodedNames, classStack, err = decodeInstanceFolderParts(folderParts)
		if err != nil {
			return nil, err
		}

		if len(decodedNames) == 1 {
			rootClass := serviceClass(serviceName)
			if decodedNames[0] == serviceName && classStack[0] == rootClass {
				useRootService = true
			}
		}
	}

	rbxPath := "game." + serviceName
	if !useRootService {
		rbxPath = "game." + strings.Join(append([]string{serviceName}, decodedNames...), ".")
	}
	if IsIgnoredRbxPath(rbxPath, ignored) {
		return nil, nil
	}

	payload, err := decodeJSONObject(source)
	if err != nil {
		return nil, err
	}

	folderName := serviceName
	folderClass := serviceClass(serviceName)
	if !useRootService {
		folderName = decodedNames[len(decodedNames)-1]
		folderClass = classStack[len(classStack)-1]
	}

	if strings.TrimSpace(stringValue(payload["name"])) == "" {
		payload["name"] = folderName
	}
	if strings.TrimSpace(stringValue(payload["className"])) == "" {
		payload["className"] = folderClass
	}

	className := strings.TrimSpace(stringValue(payload["className"]))
	if className == "" {
		return nil, fmt.Errorf("missing className in %s", relPosix)
	}

	canonical, err := canonicalJSON(payload)
	if err != nil {
		return nil, err
	}
	return &SyncRecord{
		Entity:      EntityUIInstance,
		LocalPath:   relPosix,
		LocalDir:    path.Dir(relPosix),
		RbxPath:     rbxPath,
		ClassName:   className,
		Source:      canonical,
		ContentHash: ContentDigest(canonical),
		StableID:    strings.TrimSpace(stringValue(payload["id"])),
		Payload:     payload,
	}, nil
}

func NewRojoInitMetaRecord(relativePath, source string, managedServices map[string]bool, ignored []string) (*SyncRecord, error) {
	relPosix := normalizeSlash(relativePath)
	parts := splitPath(relPosix)
	if len(parts) < 2 || parts[len(parts)-1] != UIInitMetaFilename {
		return nil, nil
	}

	serviceName := decodeLocalSegment(parts[0])
	if !managedServices[serviceName] {
		return nil, nil
	}

	folderParts := parts[1 : len(parts)-1]
	useRootService := len(folderParts) == 0
	decodedNames, err := decodeUIPathNames(folderParts)
	if err != nil {
		return nil, err
	}

	rbxPath := "game." + serviceName
	defaultName := serviceName
	defaultClassName := serviceClass(serviceName)
	if !useRootService {
		rbxPath = "game." + strings.Join(append([]string{serviceName}, decodedNames...), ".")
		defaultName = decodedNames[len(decodedNames)-1]
		_, parsedClass, ok := parseUISegment(folderParts[len(folderParts)-1])
		if ok {
			defaultClassName = parsedClass
		} else {
			defaultClassName = ""
		}
	}
	if IsIgnoredRbxPath(rbxPath, ignored) {
		return nil, nil
	}

	rawPayload, err := decodeJSONObject(source)
	if err != nil {
		return nil, err
	}
	payload, stableID, err := normalizeInitMetaPayload(rawPayload, relPosix, defaultName, defaultClassName)
	if err != nil {
		return nil, err
	}
	canonical, err := canonicalJSON(payload)
	if err != nil {
		return nil, err
	}
	return &SyncRecord{
		Entity:      EntityUIInstance,
		LocalPath:   relPosix,
		LocalDir:    path.Dir(relPosix),
		RbxPath:     rbxPath,
		ClassName:   stringValue(payload["className"]),
		Source:      canonical,
		ContentHash: ContentDigest(canonical),
		StableID:    stableID,
		Payload:     payload,
	}, nil
}

func NewRojoModelRecord(relativePath, source string, managedServices map[string]bool, ignored []string) (*SyncRecord, error) {
	relPosix := normalizeSlash(relativePath)
	parts := splitPath(relPosix)
	if len(parts) < 2 || !strings.HasSuffix(parts[len(parts)-1], UIModelJSONSuffix) {
		return nil, nil
	}

	serviceName := decodeLocalSegment(parts[0])
	if !managedServices[serviceName] {
		return nil, nil
	}

	filename := parts[len(parts)-1]
	defaultName := decodeLocalSegment(strings.TrimSuffix(filename, UIModelJSONSuffix))
	if defaultName == "" {
		return nil, fmt.Errorf("invalid model filename in %s", relPosix)
	}

	parentNames, err := decodeUIPathNames(parts[1 : len(parts)-1])
	if err != nil {
		return nil, err
	}

	rawPayload, err := decodeJSONObject(source)
	if err != nil {
		return nil, err
	}
	payload, stableID, err := normalizeModelPayload(rawPayload, relPosix, defaultName)
	if err != nil {
		return nil, err
	}

	targetName := stringValue(payload["name"])
	rbxPath := "game." + strings.Join(append([]string{serviceName}, append(parentNames, targetName)...), ".")
	if IsIgnoredRbxPath(rbxPath, ignored) {
		return nil, nil
	}

	canonical, err := canonicalJSON(payload)
	if err != nil {
		return nil, err
	}
	return &SyncRecord{
		Entity:      EntityUIInstance,
		LocalPath:   relPosix,
		LocalDir:    path.Dir(relPosix),
		RbxPath:     rbxPath,
		ClassName:   stringValue(payload["className"]),
		Source:      canonical,
		ContentHash: ContentDigest(canonical),
		StableID:    stableID,
		Payload:     payload,
	}, nil
}

func StableID(payload map[string]any) string {
	for _, key := range []string{"id", "$id", "syncId"} {
		if raw, ok := payload[key]; ok {
			if value := strings.TrimSpace(fmt.Sprint(raw)); value != "" {
				return value
			}
		}
	}
	return ""
}

func StableIDWithFallback(payload map[string]any, relPosix string) string {
	if stableID := StableID(payload); stableID != "" {
		return stableID
	}
	return "path:" + relPosix
}

func ContentDigest(source string) string {
	sum := sha256.Sum256([]byte(source))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newCanonicalScriptRecord(relPosix string, parts []string, source, className string, managedServices map[string]bool, ignored []string) (*SyncRecord, error) {
	if len(parts) < 3 {
		return nil, nil
	}
	serviceName := decodeLocalSegment(parts[0])
	if !managedServices[serviceName] {
		return nil, nil
	}

	decodedNames, classStack, err := decodeInstanceFolderParts(parts[1 : len(parts)-1])
	if err != nil {
		return nil, err
	}
	if len(decodedNames) == 0 {
		return nil, nil
	}

	leafClass := classStack[len(classStack)-1]
	if leafClass != "" {
		if scriptClasses[leafClass] {
			if leafClass != className {
				return nil, fmt.Errorf("script source %s class mismatch: folder=%s filename=%s", relPosix, leafClass, className)
			}
			className = leafClass
		} else {
			return nil, fmt.Errorf("script source %s points to non-script class folder %s", relPosix, leafClass)
		}
	}

	rbxPath := "game." + strings.Join(append([]string{serviceName}, decodedNames...), ".")
	if IsIgnoredRbxPath(rbxPath, ignored) {
		return nil, nil
	}
	return buildScriptRecord(relPosix, rbxPath, className, source), nil
}

func newNamedScriptRecord(relPosix string, parts []string, filenameStem, className, suffix, source string, managedServices map[string]bool, ignored []string) (*SyncRecord, error) {
	serviceName := decodeLocalSegment(parts[0])
	if !managedServices[serviceName] {
		return nil, nil
	}

	if len(parts) >= 3 {
		decodedNames, classStack, err := decodeInstanceFolderParts(parts[1 : len(parts)-1])
		if err != nil {
			return nil, err
		}
		if len(decodedNames) > 0 && scriptClasses[classStack[len(classStack)-1]] {
			if classStack[len(classStack)-1] != className {
				return nil, fmt.Errorf("script source %s class mismatch: folder=%s filename=%s", relPosix, classStack[len(classStack)-1], className)
			}
			if decodeLocalSegment(filenameStem) == decodedNames[len(decodedNames)-1] {
				rbxPath := "game." + strings.Join(append([]string{serviceName}, decodedNames...), ".")
				if IsIgnoredRbxPath(rbxPath, ignored) {
					return nil, nil
				}
				return buildScriptRecord(relPosix, rbxPath, className, source), nil
			}
		}
	}

	stem := strings.TrimSuffix(relPosix, suffix)
	stemParts := splitPath(stem)
	if len(stemParts) == 0 {
		return nil, nil
	}
	resolvedParts := stemParts
	leafName := stemParts[len(stemParts)-1]
	if len(stemParts) >= 2 && leafName == "init" {
		resolvedParts = stemParts[:len(stemParts)-1]
	} else if len(stemParts) >= 2 && strings.HasPrefix(leafName, "init.") {
		expectedName := strings.TrimPrefix(leafName, "init.")
		parentName := stemParts[len(stemParts)-2]
		if expectedName == parentName {
			resolvedParts = stemParts[:len(stemParts)-1]
		}
	}
	if len(resolvedParts) == 0 {
		return nil, nil
	}

	decodedParts := make([]string, 0, len(resolvedParts))
	for _, part := range resolvedParts {
		decodedParts = append(decodedParts, decodeLocalSegment(part))
	}
	rbxPath := "game." + strings.Join(decodedParts, ".")
	if IsIgnoredRbxPath(rbxPath, ignored) {
		return nil, nil
	}
	return buildScriptRecord(relPosix, rbxPath, className, source), nil
}

func buildScriptRecord(localPath, rbxPath, className, source string) *SyncRecord {
	return &SyncRecord{
		Entity:      EntityScript,
		LocalPath:   localPath,
		LocalDir:    path.Dir(localPath),
		RbxPath:     rbxPath,
		ClassName:   className,
		Source:      source,
		ContentHash: ContentDigest(source),
	}
}

func (r *SyncRecord) ApplyScriptPayload(payload map[string]any) error {
	if r == nil || r.Entity != EntityScript || payload == nil {
		return nil
	}
	canonical, err := canonicalJSON(payload)
	if err != nil {
		return err
	}
	r.Payload = payload
	r.ContentHash = ContentDigest(r.Source + "\x00" + canonical)
	return nil
}

func IsIgnoredRbxPath(rbxPath string, ignored []string) bool {
	if rbxPath == "" {
		return false
	}
	for _, item := range ignored {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if rbxPath == item || strings.HasPrefix(rbxPath, item+".") {
			return true
		}
	}
	return false
}

func splitScriptSourceFilename(filename string) (string, string, string, bool) {
	lowered := strings.ToLower(filename)
	for suffix, className := range scriptSuffixToClass {
		if strings.HasSuffix(lowered, suffix) {
			stem := filename[:len(filename)-len(suffix)]
			if stem == "" {
				return "", "", "", false
			}
			return stem, className, suffix, true
		}
	}
	return "", "", "", false
}

func decodeInstanceFolderParts(folderParts []string) ([]string, []string, error) {
	decodedNames := make([]string, 0, len(folderParts))
	classStack := make([]string, 0, len(folderParts))
	for _, segment := range folderParts {
		instanceName, className, ok := parseUISegment(segment)
		if !ok {
			decoded := decodeLocalSegment(segment)
			if decoded == "" {
				return nil, nil, fmt.Errorf("invalid instance folder segment: %s", segment)
			}
			decodedNames = append(decodedNames, decoded)
			classStack = append(classStack, "")
			continue
		}
		decodedNames = append(decodedNames, instanceName)
		classStack = append(classStack, className)
	}
	return decodedNames, classStack, nil
}

func decodeUIPathNames(folderParts []string) ([]string, error) {
	decodedNames := make([]string, 0, len(folderParts))
	for _, segment := range folderParts {
		instanceName, _, ok := parseUISegment(segment)
		if !ok {
			instanceName = decodeLocalSegment(segment)
		}
		if instanceName == "" {
			return nil, fmt.Errorf("invalid instance folder segment: %s", segment)
		}
		decodedNames = append(decodedNames, instanceName)
	}
	return decodedNames, nil
}

func parseUISegment(segment string) (string, string, bool) {
	splitIndex := strings.LastIndex(segment, ".")
	if splitIndex <= 0 || splitIndex >= len(segment)-1 {
		return "", "", false
	}
	return decodeLocalSegment(segment[:splitIndex]), segment[splitIndex+1:], true
}

func normalizeInitMetaPayload(payload map[string]any, relPosix, defaultName, defaultClassName string) (map[string]any, string, error) {
	className := firstNonEmpty(payloadClassName(payload), defaultClassName)
	if className == "" {
		return nil, "", fmt.Errorf("missing className in %s", relPosix)
	}
	name := firstNonEmpty(payloadName(payload), defaultName)
	stableID := StableIDWithFallback(payload, relPosix)
	normalized := map[string]any{
		"id":         stableID,
		"className":  className,
		"name":       name,
		"properties": objectOrEmpty(payload["properties"]),
		"attributes": objectOrEmpty(payload["attributes"]),
		"tags":       arrayOrEmpty(payload["tags"]),
	}
	return normalized, stableID, nil
}

func normalizeModelPayload(payload map[string]any, relPosix, defaultName string) (map[string]any, string, error) {
	className := payloadClassName(payload)
	if className == "" {
		return nil, "", fmt.Errorf("missing $className/className in %s", relPosix)
	}
	name := firstNonEmpty(payloadName(payload), defaultName)
	properties := map[string]any{}
	if explicit, ok := payload["properties"].(map[string]any); ok {
		for key, value := range explicit {
			properties[key] = value
		}
	}

	metadataKeys := map[string]bool{
		"$className": true,
		"className":  true,
		"Name":       true,
		"name":       true,
		"id":         true,
		"$id":        true,
		"syncId":     true,
		"properties": true,
		"attributes": true,
		"tags":       true,
	}
	for key, value := range payload {
		if !metadataKeys[key] {
			properties[key] = value
		}
	}

	stableID := StableIDWithFallback(payload, relPosix)
	normalized := map[string]any{
		"id":         stableID,
		"className":  className,
		"name":       name,
		"properties": properties,
		"attributes": objectOrEmpty(payload["attributes"]),
		"tags":       arrayOrEmpty(payload["tags"]),
	}
	return normalized, stableID, nil
}

func decodeJSONObject(source string) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(source), &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, fmt.Errorf("payload must be object")
	}
	return payload, nil
}

func canonicalJSON(payload map[string]any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func payloadClassName(payload map[string]any) string {
	return strings.TrimSpace(firstNonEmpty(stringValue(payload["className"]), stringValue(payload["$className"])))
}

func payloadName(payload map[string]any) string {
	return strings.TrimSpace(firstNonEmpty(stringValue(payload["name"]), stringValue(payload["Name"])))
}

func objectOrEmpty(value any) map[string]any {
	if object, ok := value.(map[string]any); ok {
		return object
	}
	return map[string]any{}
}

func arrayOrEmpty(value any) []any {
	if array, ok := value.([]any); ok {
		return array
	}
	return []any{}
}

func serviceClass(serviceName string) string {
	if className, ok := rootServiceClass[serviceName]; ok {
		return className
	}
	return serviceName
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func decodeLocalSegment(segment string) string {
	if strings.HasPrefix(segment, "~x_") {
		payload := segment[3:]
		if len(payload)%2 == 0 && isHex(payload) {
			if bytes, err := hex.DecodeString(payload); err == nil && utf8.Valid(bytes) {
				return string(bytes)
			}
		}
	}
	return segment
}

func splitPath(value string) []string {
	trimmed := strings.Trim(normalizeSlash(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func normalizeSlash(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}

func isHex(value string) bool {
	for _, ch := range value {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
