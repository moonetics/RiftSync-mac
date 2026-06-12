local SyncAPI = {}
SyncAPI.__index = SyncAPI

local TypeList = require(script:WaitForChild("TypeList"))

local httpService = game:GetService("HttpService")
local scriptEditorService = game:GetService("ScriptEditorService")
local runService = game:GetService("RunService")

local DEBUG_EVENT_LIMIT = 28
local SERVER_DEBUG_PULL_INTERVAL = 2.0

local function isManagedAttributeName(attributeName : any)
	if typeof(attributeName) ~= "string" then
		return false
	end
	if attributeName == "ManagedByLocalSync" or attributeName == "SyncId" or attributeName == "SourceId" then
		return true
	end

	for _, managedName in pairs(TypeList.MANAGED_ATTRIBUTES) do
		if attributeName == managedName then
			return true
		end
	end

	return string.sub(string.lower(attributeName), 1, 10) == "_rbxlsync_"
end

local function isIgnoredPath(pathValue : string, ignoredPaths : {string})
	if TypeList.isIgnoredRbxPath then
		return TypeList.isIgnoredRbxPath(pathValue, ignoredPaths)
	end

	if typeof(pathValue) ~= "string" then
		return false
	end

	for _, ignored in ipairs(ignoredPaths) do
		if typeof(ignored) == "string" and ignored ~= "" then
			if pathValue == ignored then
				return true
			end
			if string.sub(pathValue, 1, #ignored + 1) == ignored .. "." then
				return true
			end
		end
	end

	return false
end

local function splitByDot(value : string)
	local segments = {}
	for segment in string.gmatch(value, "[^%.]+") do
		table.insert(segments, segment)
	end
	return segments
end

local function encodeLocalSegment(value : string)
	local trailingTrimmed = string.gsub(value, "[%.%s]+$", "")
	local upperTrimmed = string.upper(trailingTrimmed)
	local isReserved = upperTrimmed == "CON"
		or upperTrimmed == "PRN"
		or upperTrimmed == "AUX"
		or upperTrimmed == "NUL"
		or string.match(upperTrimmed, "^COM[1-9]$") ~= nil
		or string.match(upperTrimmed, "^LPT[1-9]$") ~= nil

	local hasInvalidWindowsChar = false
	for index = 1, #value do
		local byteValue = string.byte(value, index)
		if byteValue and byteValue < 32 then
			hasInvalidWindowsChar = true
			break
		end

		local character = string.sub(value, index, index)
		if character == "<"
			or character == ">"
			or character == ":"
			or character == "\""
			or character == "/"
			or character == "\\"
			or character == "|"
			or character == "?"
			or character == "*"
		then
			hasInvalidWindowsChar = true
			break
		end
	end

	local endsWithDotOrSpace = string.match(value, "[%.%s]$") ~= nil
	local looksLikeEncodedPrefix = string.match(value, "^~x_[%x]*$") ~= nil
	local shouldEncode = value == ""
		or hasInvalidWindowsChar
		or endsWithDotOrSpace
		or isReserved
		or looksLikeEncodedPrefix

	if not shouldEncode then
		return value
	end

	local encoded = table.create(#value + 1)
	encoded[1] = "~x_"
	for index = 1, #value do
		encoded[index + 1] = string.format("%02X", string.byte(value, index))
	end
	return table.concat(encoded)
end

local function getServiceByName(serviceName : string)
	local success, result = pcall(function()
		return game:GetService(serviceName)
	end)

	if success then
		return result
	end

	return game:FindFirstChild(serviceName)
end

local function buildGamePath(instance : Instance)
	local segments = {}
	local current = instance

	while current and current ~= game do
		table.insert(segments, 1, current.Name)
		current = current.Parent
	end

	return "game." .. table.concat(segments, ".")
end

local function buildLocalPathFromInstance(instance : Instance, suffix : string, useInitFormat : boolean)
	local segments = {}
	local current = instance
	while current and current ~= game do
		table.insert(segments, 1, encodeLocalSegment(current.Name))
		current = current.Parent
	end

	if #segments == 0 then
		return nil
	end

	if useInitFormat then
		local scriptName = segments[#segments]
		table.insert(segments, "init." .. scriptName .. suffix)
	else
		segments[#segments] ..= suffix
	end
	return table.concat(segments, "/")
end

local function readStudioScriptSource(scriptInstance : LuaSourceContainer)
	local sourceSuccess, sourceResult = pcall(function()
		return scriptEditorService:GetEditorSource(scriptInstance)
	end)

	if sourceSuccess and typeof(sourceResult) == "string" then
		return sourceResult
	end

	local fallbackSuccess, fallbackSource = pcall(function()
		return scriptInstance.Source
	end)

	if fallbackSuccess and typeof(fallbackSource) == "string" then
		return fallbackSource
	end

	return nil, sourceResult
end

local function decodeJson(body : string)
	if body == "" then
		return {}
	end

	local ok, decoded = pcall(function()
		return httpService:JSONDecode(body)
	end)

	if ok then
		return decoded
	end

	return nil, decoded
end

local function setScriptSource(target : LuaSourceContainer, source : string)
	local updated = false

	local updateSuccess = pcall(function()
		scriptEditorService:UpdateSourceAsync(target, function()
			return source
		end)
	end)

	if updateSuccess then
		updated = true
	end

	if not updated then
		local fallbackSuccess = pcall(function()
			target.Source = source
		end)
		if fallbackSuccess then
			updated = true
		end
	end

	return updated
end

local function resolvePath(path : string)
	local segments = splitByDot(path)
	if #segments < 2 or segments[1] ~= "game" then
		return nil, "Invalid Roblox path: " .. path
	end

	local current = getServiceByName(segments[2])
	if not current then
		return nil, "Service not found: " .. segments[2]
	end

	for index = 3, #segments do
		local child = current:FindFirstChild(segments[index])
		if not child then
			return nil
		end
		current = child
	end

	return current
end

local function ensureParentPath(path : string)
	local segments = splitByDot(path)
	if #segments < 3 or segments[1] ~= "game" then
		return nil, nil, "Invalid Roblox path: " .. path
	end

	local root = getServiceByName(segments[2])
	if not root then
		return nil, nil, "Service not found: " .. segments[2]
	end

	local current = root
	for index = 3, #segments - 1 do
		local childName = segments[index]
		local existing = current:FindFirstChild(childName)

		if not existing then
			existing = Instance.new("Folder")
			existing.Name = childName
			existing.Parent = current
		end

		current = existing
	end

	return current, segments[#segments], nil
end

local function replaceManagedConflict(parent : Instance, targetName : string, className : string)
	local function isProtectedByPath(instance : Instance)
		if instance:IsA("Terrain") then
			return true
		end
		if instance.Parent ~= game then
			return false
		end
		local rootServiceClass = TypeList.ROOT_SERVICE_CLASS
		if typeof(rootServiceClass) ~= "table" then
			return false
		end
		return rootServiceClass[instance.Name] ~= nil
	end

	local function findBackupName(baseName : string)
		local prefix = baseName .. "__rbxlsync_unmanaged_backup"
		local candidate = prefix
		local suffix = 1
		while parent:FindFirstChild(candidate) ~= nil do
			candidate = prefix .. "_" .. tostring(suffix)
			suffix += 1
			if suffix > 200 then
				candidate = prefix .. "_" .. string.sub(httpService:GenerateGUID(false), 1, 8)
				if parent:FindFirstChild(candidate) == nil then
					break
				end
			end
		end
		return candidate
	end

	local function moveUnmanagedAside(existing : Instance)
		if isProtectedByPath(existing) then
			return false, "Refused replacing protected instance: " .. existing:GetFullName()
		end
		local fromPath = existing:GetFullName()
		local backupName = findBackupName(targetName)
		local moved, moveError = pcall(function()
			existing.Name = backupName
		end)
		if not moved then
			return false, "Failed moving unmanaged conflict at " .. fromPath .. " : " .. tostring(moveError)
		end
		return true
	end

	local existing = parent:FindFirstChild(targetName)
	if not existing then
		return nil
	end

	if existing:IsA(className) then
		return existing
	end

	if existing:IsA("Folder") and (className == "Script" or className == "LocalScript" or className == "ModuleScript") then
		local hasManagedDescendant = false
		for _, descendant in ipairs(existing:GetDescendants()) do
			if descendant:GetAttribute(TypeList.MANAGED_ATTRIBUTES.IsManaged) then
				hasManagedDescendant = true
				break
			end
		end

		if hasManagedDescendant or existing:GetAttribute(TypeList.MANAGED_ATTRIBUTES.IsManaged) then
			local upgraded = Instance.new(className)
			upgraded.Name = targetName
			upgraded.Parent = parent
			for _, child in ipairs(existing:GetChildren()) do
				child.Parent = upgraded
			end
			existing:Destroy()
			return upgraded
		end
	end

	if existing:GetAttribute(TypeList.MANAGED_ATTRIBUTES.IsManaged) then
		existing:Destroy()
		return nil
	end

	local moved, moveError = moveUnmanagedAside(existing)
	if not moved then
		return nil, moveError
	end
	return nil
end

local function formatHttpError(response : {[string]: any})
	local statusCode = tostring(response.StatusCode or "?")
	local statusMessage = tostring(response.StatusMessage or "")
	local body = tostring(response.Body or "")
	return "HTTP " .. statusCode .. " " .. statusMessage .. " " .. body
end

local function buildQuery(params : {[string]: string | number})
	local fields = {}
	for key, value in pairs(params) do
		table.insert(fields, key .. "=" .. httpService:UrlEncode(tostring(value)))
	end
	table.sort(fields)
	return table.concat(fields, "&")
end

local function createBody(payload : {[string]: any}?)
	if not payload then
		return nil
	end

	local success, encoded = pcall(function()
		return httpService:JSONEncode(payload)
	end)

	if success then
		return encoded
	end

	return nil
end

local function isScriptClass(className : string)
	return className == "Script" or className == "LocalScript" or className == "ModuleScript"
end

local function getPreferredSuffixForClass(className : string)
	return TypeList.CLASS_TO_PREFERRED_SUFFIX[className]
end

local function classifySyncError(message : any) : {[string]: string}?
	if typeof(message) ~= "string" or message == "" then
		return nil
	end

	local lowered = string.lower(message)
	if string.find(lowered, "invalid json", 1, true)
		or string.find(lowered, "jsondecodeerror", 1, true)
		or string.find(lowered, "payload must be object", 1, true)
	then
		return {
			category = "invalid_json",
			title = "Invalid JSON",
			hint = "Cek syntax JSON, koma, bracket, dan value typed.",
		}
	end

	if string.find(lowered, "duplicate stable id", 1, true) then
		return {
			category = "duplicate_stable_id",
			title = "Duplicate stable id",
			hint = "Ganti atau hapus duplicate id di properties.init.json.",
		}
	end

	if string.find(lowered, "unmanaged", 1, true)
		or string.find(lowered, "occupied by unmanaged", 1, true)
		or string.find(lowered, "refused replacing", 1, true)
	then
		return {
			category = "unmanaged_conflict",
			title = "Unmanaged conflict",
			hint = "Rename/remove instance di Studio atau jadikan target managed.",
		}
	end

	if string.find(lowered, "unsupported class", 1, true)
		or string.find(lowered, "unsupported script class", 1, true)
		or string.find(lowered, "unsupported terrain", 1, true)
	then
		return {
			category = "unsupported_class",
			title = "Unsupported class",
			hint = "Cek className dan managed_roots untuk target ini.",
		}
	end

	if string.find(lowered, "unsupported or invalid property type", 1, true)
		or string.find(lowered, "skip non-whitelist property", 1, true)
		or string.find(lowered, "property gagal", 1, true)
		or string.find(lowered, "protected property", 1, true)
	then
		return {
			category = "unsupported_property",
			title = "Unsupported property",
			hint = "Cek nama/type property atau tambahkan ke extra_allowed_properties.",
		}
	end

	if string.find(lowered, "request failed", 1, true)
		or string.find(lowered, "gagal request", 1, true)
		or string.find(lowered, "http 0", 1, true)
		or string.find(lowered, "http 403", 1, true)
		or string.find(lowered, "http requests are not enabled", 1, true)
	then
		return {
			category = "server_offline",
			title = "Server offline / HTTP disabled",
			hint = "Jalankan riftsync-server.exe, cek host/port, dan aktifkan HTTP Requests.",
		}
	end

	return nil
end

local function formatGuidedError(message : any) : string
	local raw = tostring(message or "")
	local guidance = classifySyncError(raw)
	if not guidance then
		return raw
	end
	if string.find(raw, guidance.title, 1, true) and string.find(raw, guidance.hint, 1, true) then
		return raw
	end
	return guidance.title .. " | " .. guidance.hint .. " | " .. raw
end

local function buildErrorSummary(prefix : string, errors : {string}, maxItems : number)
	if #errors == 0 then
		return prefix
	end

	local parts = {}
	local limit = math.min(#errors, maxItems)
	for index = 1, limit do
		table.insert(parts, formatGuidedError(errors[index]))
	end

	local summary = table.concat(parts, " | ")
	if #errors > maxItems then
		summary ..= " | +" .. tostring(#errors - maxItems) .. " error lagi"
	end

	return prefix .. ": " .. summary
end

local function warnErrorList(context : string, errors : {string})
	for index, message in ipairs(errors) do
		warn("[RiftSync] " .. context .. " [" .. tostring(index) .. "] " .. message)
	end
end

local function getPathLeaf(pathValue : string) : string
	local leaf = string.match(pathValue, "[^/\\]+$") or pathValue
	if leaf ~= "" then
		return leaf
	end
	return pathValue
end

local function getChangeRbxPath(change : {[string]: any}) : string
	local candidates = {
		change.new_rbx_path,
		change.rbx_path,
		change.old_rbx_path,
	}
	for _, value in ipairs(candidates) do
		if typeof(value) == "string" and value ~= "" then
			return value
		end
	end
	return "unknown"
end

local function getChangeLocalPath(change : {[string]: any}) : string
	local candidates = {
		change.local_path,
		change.new_local_path,
	}
	for _, value in ipairs(candidates) do
		if typeof(value) == "string" and value ~= "" then
			return value
		end
	end
	return ""
end

local function getHistoryEntityLabel(change : {[string]: any}) : string
	local entity = change.entity
	if typeof(entity) == "string" and entity ~= "" then
		if TypeList.normaliseEntityKind then
			entity = TypeList.normaliseEntityKind(entity)
		end
		return tostring(entity)
	end
	if typeof(change.payload) == "table" then
		return "ui_instance"
	end
	return "script"
end

local function expandCompactChange(change : any) : any
	if typeof(change) ~= "table" or typeof(change.o) ~= "string" then
		return change
	end

	local expanded = {}
	local opMap = {
		u = "upsert",
		d = "delete",
		r = "rename",
	}
	local entityMap = {
		s = "script",
		ui = "ui_instance",
	}

	expanded.op = opMap[tostring(change.o)] or tostring(change.o)
	if typeof(change.e) == "string" then
		expanded.entity = entityMap[tostring(change.e)] or tostring(change.e)
	end
	if typeof(change.l) == "string" then expanded.local_path = change.l end
	if typeof(change.ld) == "string" then expanded.local_dir = change.ld end
	if typeof(change.rp) == "string" then expanded.rbx_path = change.rp end
	if typeof(change.c) == "string" then expanded.class_name = change.c end
	if typeof(change.sid) == "string" then expanded.stable_id = change.sid end
	if typeof(change.h) == "string" then expanded.content_hash = change.h end
	if typeof(change.src) == "string" then expanded.source = change.src end
	if typeof(change.p) == "table" then expanded.payload = change.p end
	if typeof(change.ol) == "string" then expanded.old_local_path = change.ol end
	if typeof(change["or"]) == "string" then expanded.old_rbx_path = change["or"] end
	if typeof(change.nr) == "string" then expanded.new_rbx_path = change.nr end
	return expanded
end

local function expandCompactChanges(changes : any, encoding : any) : {any}
	if typeof(changes) ~= "table" then
		return {}
	end
	if tostring(encoding) ~= "compact-json-v1" then
		return changes
	end
	local expanded = {}
	for _, change in ipairs(changes) do
		table.insert(expanded, expandCompactChange(change))
	end
	return expanded
end

local function shouldMarkSkipped(operation : string, resultOrNotice : any) : boolean
	if operation ~= "delete" or typeof(resultOrNotice) ~= "string" then
		return false
	end
	local lowered = string.lower(resultOrNotice)
	return string.sub(lowered, 1, 4) == "skip"
		or string.sub(lowered, 1, 7) == "skipped"
		or string.find(lowered, "not found", 1, true) ~= nil
end

local function formatSyncHistoryEvent(change : {[string]: any}, resultOrNotice : any) : {[string]: string}
	local operation = tostring(change.op or "upsert")
	local operationLabel = string.upper(operation)
	if operationLabel == "UPSERT" then
		operationLabel = "SYNC"
	end

	local entityLabel = getHistoryEntityLabel(change)
	local className = typeof(change.class_name) == "string" and change.class_name or ""
	local classLabel = className ~= "" and (" " .. className) or ""

	if operation == "rename" then
		local oldPath = typeof(change.old_rbx_path) == "string" and change.old_rbx_path or getChangeRbxPath(change)
		local newPath = typeof(change.new_rbx_path) == "string" and change.new_rbx_path or getChangeRbxPath(change)
		local localPath = getChangeLocalPath(change)
		local localLabel = localPath ~= "" and (" <- " .. localPath) or ""
		local detail = typeof(resultOrNotice) == "string" and resultOrNotice ~= "" and (" | " .. resultOrNotice) or ""
		return {
			status = "renamed",
			message = operationLabel
				.. " "
				.. entityLabel
				.. classLabel
				.. " @ "
				.. oldPath
				.. " -> "
				.. newPath
				.. localLabel
				.. detail,
			path = newPath,
		}
	end

	local rbxPath = getChangeRbxPath(change)
	local localPath = getChangeLocalPath(change)
	local localLabel = localPath ~= "" and (" <- " .. getPathLeaf(localPath)) or ""
	local status = shouldMarkSkipped(operation, resultOrNotice) and "skipped"
		or (operation == "delete" and "deleted" or "synced")
	local detail = ""
	if typeof(resultOrNotice) == "string" and resultOrNotice ~= "" then
		detail = " | " .. resultOrNotice
	elseif operation == "delete" then
		detail = " | deleted"
	end
	return {
		status = status,
		message = operationLabel .. " " .. entityLabel .. classLabel .. " @ " .. rbxPath .. localLabel .. detail,
		path = rbxPath,
	}
end

local function isConflictMessage(message : any) : boolean
	if typeof(message) ~= "string" then
		return false
	end
	local lowered = string.lower(message)
	return string.find(lowered, "conflict", 1, true) ~= nil
		or string.find(lowered, "unmanaged instance", 1, true) ~= nil
		or string.find(lowered, "occupied by unmanaged", 1, true) ~= nil
end

local function formatConflictHistoryEvent(message : string) : {[string]: string}
	return {
		status = "conflict",
		message = "Conflict warning: " .. tostring(message),
		path = tostring(message),
	}
end

local function formatSummaryHistoryEvent(total : number, counts : {[string]: number}) : {[string]: string}
	local parts = {}
	if (counts.synced or 0) > 0 then
		table.insert(parts, tostring(counts.synced) .. " synced")
	end
	if (counts.renamed or 0) > 0 then
		table.insert(parts, tostring(counts.renamed) .. " renamed")
	end
	if (counts.deleted or 0) > 0 then
		table.insert(parts, tostring(counts.deleted) .. " deleted")
	end
	if (counts.skipped or 0) > 0 then
		table.insert(parts, tostring(counts.skipped) .. " skipped")
	end
	if (counts.conflict or 0) > 0 then
		table.insert(parts, tostring(counts.conflict) .. " warnings")
	end
	if (counts.error or 0) > 0 then
		table.insert(parts, tostring(counts.error) .. " errors")
	end

	local detail = table.concat(parts, ", ")
	if detail == "" then
		detail = tostring(total) .. " changes"
	end

	local hasIssue = (counts.conflict or 0) > 0 or (counts.error or 0) > 0
	return {
		status = hasIssue and "conflict" or "summary",
		message = "Activity batch: " .. detail,
		path = detail,
	}
end

local function summariseSyncHistoryEvents(events : {any}) : {any}
	local threshold = tonumber(TypeList.ACTIVITY_SUMMARY_THRESHOLD) or 8
	if #events <= threshold then
		return events
	end

	local counts = {}
	for _, event in ipairs(events) do
		local status = typeof(event) == "table" and tostring(event.status or "synced") or "synced"
		counts[status] = (counts[status] or 0) + 1
	end

	local highlighted = {}
	for _, event in ipairs(events) do
		local status = typeof(event) == "table" and tostring(event.status or "synced") or "synced"
		if status == "deleted" or status == "renamed" or status == "conflict" or status == "error" or status == "skipped" then
			table.insert(highlighted, event)
		end
		if #highlighted >= 6 then
			break
		end
	end

	if #highlighted == 0 then
		local keep = math.min(3, #events)
		for index = 1, keep do
			table.insert(highlighted, events[index])
		end
	end

	local compact = {
		formatSummaryHistoryEvent(#events, counts),
	}
	for _, event in ipairs(highlighted) do
		table.insert(compact, event)
	end
	return compact
end

local function shouldRecordSyncHistory(self, change : {[string]: any}, resultOrNotice : any) : boolean
	local targetPath = change.new_rbx_path or change.rbx_path or change.old_rbx_path
	if typeof(targetPath) == "string" and isIgnoredPath(targetPath, self.ignoredRbxPaths) then
		return false
	end

	local operation = tostring(change.op or "upsert")
	if typeof(resultOrNotice) == "string" then
		local lowered = string.lower(resultOrNotice)
		if string.sub(lowered, 1, 4) == "skip"
			or string.sub(lowered, 1, 7) == "skipped"
			or string.find(lowered, "not found", 1, true)
		then
			return operation == "delete"
		end
	end

	return true
end

local SETTING_KEYS = {
	Host = "riftsync_host",
	Port = "riftsync_port",
	StartMode = "riftsync_start_mode",
	ClientId = "riftsync_client_id",
	LastRevision = "riftsync_last_revision",
	DebugEnabled = "riftsync_debug_enabled",
	RemoteExecToken = "riftsync_remote_exec_token",
}

local LEGACY_SETTING_KEYS = {
	Host = "rbxlsync_host",
	Port = "rbxlsync_port",
	StartMode = "rbxlsync_start_mode",
	ClientId = "rbxlsync_client_id",
	LastRevision = "rbxlsync_last_revision",
	DebugEnabled = "rbxlsync_debug_enabled",
	RemoteExecToken = "rbxlsync_remote_exec_token",
}

local function getPluginSetting(pluginInstance, keyName)
	local value = pluginInstance:GetSetting(SETTING_KEYS[keyName])
	if value ~= nil then
		return value
	end
	return pluginInstance:GetSetting(LEGACY_SETTING_KEYS[keyName])
end

function SyncAPI.new(pluginInstance : Plugin, updateStatusCallback : (string, boolean?, any?) -> (), updateDebugCallback)
	local self = setmetatable({}, SyncAPI)

	local configuredHost = getPluginSetting(pluginInstance, "Host")
	local configuredPort = getPluginSetting(pluginInstance, "Port")
	local configuredStartMode = getPluginSetting(pluginInstance, "StartMode")

	self.plugin = pluginInstance
	local rawUpdateStatus = updateStatusCallback or function()
		return
	end
	self.updateStatus = function(text, isError, meta)
		rawUpdateStatus(text, isError, meta)
		self:reportActivity(text, isError, meta)
	end
	self.updateDebug = updateDebugCallback or function()
		return
	end
	self.host = TypeList.normaliseHost(tostring(configuredHost or TypeList.DEFAULT_HOST))
	self.port = TypeList.normalisePort(configuredPort or TypeList.DEFAULT_PORT)
	self.baseUrl = TypeList.buildBaseUrl(self.host, self.port)
	self.clientId = getPluginSetting(pluginInstance, "ClientId") or httpService:GenerateGUID(false)
	self.lastAppliedRevision = tonumber(getPluginSetting(pluginInstance, "LastRevision")) or 0
	self.running = false
	self.sessionId = ""
	self.runToken = 0
	self.initialSyncInProgress = false
	self.managedRoots = TypeList.DEFAULT_MANAGED_ROOTS
	self.ignoredRbxPaths = TypeList.DEFAULT_IGNORED_RBX_PATHS or {}
	self.strictPropertyWhitelist = TypeList.DEFAULT_STRICT_PROPERTY_WHITELIST == true
	self.extraAllowedProperties = TypeList.normaliseExtraAllowedProperties
			and TypeList.normaliseExtraAllowedProperties(TypeList.DEFAULT_EXTRA_ALLOWED_PROPERTIES or {})
		or {}
	self.startMode = TypeList.normaliseStartMode(tostring(configuredStartMode or ""))
self.serverIndexedCount = 0
	self.changeEncoding = TypeList.CHANGE_ENCODINGS and TypeList.CHANGE_ENCODINGS.Compact or "compact-json-v1"
	self.debugEnabled = getPluginSetting(pluginInstance, "DebugEnabled") == true
	self.remoteExecEnabled = false
	self.remoteExecToken = tostring(getPluginSetting(pluginInstance, "RemoteExecToken") or "")
	self.remoteExecBusy = false
	self.remoteExecLoopRunning = false
	self.lastExecStatus = "Exec disabled"
	self.debugEvents = {}
	self.serverDebugState = {}
	self.debugState = {
		handshake = false,
		snapshot = false,
		poll = false,
		delta = false,
		ackSent = false,
		ackOk = false,
		lastError = "",
		lastPollChangeCount = 0,
		lastPollTargetRev = self.lastAppliedRevision,
		lastServerRev = self.lastAppliedRevision,
		pollErrorCount = 0,
	}
	self.lastServerDebugPullAt = 0

	pluginInstance:SetSetting(SETTING_KEYS.ClientId, self.clientId)
	pluginInstance:SetSetting(SETTING_KEYS.StartMode, self.startMode)

	self:pushDebugUpdate()
	return self
end

function SyncAPI:setConnection(host : string, port : string | number)
	self.host = TypeList.normaliseHost(host)
	self.port = TypeList.normalisePort(port)
	self.baseUrl = TypeList.buildBaseUrl(self.host, self.port)
	self.plugin:SetSetting(SETTING_KEYS.Host, self.host)
	self.plugin:SetSetting(SETTING_KEYS.Port, self.port)
end

function SyncAPI:getConnection()
	return self.host, self.port
end

function SyncAPI:getStartMode()
	return self.startMode
end

function SyncAPI:setStartMode(mode : string)
	self.startMode = TypeList.normaliseStartMode(mode)
	self.plugin:SetSetting(SETTING_KEYS.StartMode, self.startMode)
end

function SyncAPI:isRunning()
	return self.running
end

function SyncAPI:isDebugEnabled()
	return self.debugEnabled == true
end

function SyncAPI:setDebugEnabled(enabled : boolean)
	self.debugEnabled = enabled == true
	self.plugin:SetSetting(SETTING_KEYS.DebugEnabled, self.debugEnabled)
	if self.debugEnabled then
		self:appendDebugEvent("Debug mode aktif", false)
	else
		self.debugEvents = {}
	end
	self:pushDebugUpdate()
end

function SyncAPI:isRemoteExecEnabled()
	return self.remoteExecEnabled == true
end

function SyncAPI:setRemoteExecEnabled(enabled : boolean)
	self.remoteExecEnabled = enabled == true
	if self.remoteExecEnabled then
		if tostring(self.remoteExecToken or "") == "" then
			self.lastExecStatus = "Exec error: token required"
		elseif self.running ~= true then
			self.lastExecStatus = "Exec disabled"
		else
			self.lastExecStatus = "Exec ready"
		end
		self:appendDebugEvent("Remote Exec aktif", false)
	else
		self.lastExecStatus = "Exec disabled"
		self.remoteExecBusy = false
		self:appendDebugEvent("Remote Exec nonaktif", false)
	end
	self:pushDebugUpdate()
end

function SyncAPI:getRemoteExecToken()
	return self.remoteExecToken or ""
end

function SyncAPI:setRemoteExecToken(token : string)
	self.remoteExecToken = tostring(token or "")
	self.plugin:SetSetting(SETTING_KEYS.RemoteExecToken, self.remoteExecToken)
	if self.remoteExecEnabled and self.remoteExecToken == "" then
		self.lastExecStatus = "Exec error: token required"
	elseif self.remoteExecEnabled and self.running == true then
		self.lastExecStatus = "Exec ready"
	elseif self.remoteExecEnabled then
		self.lastExecStatus = "Exec disabled"
	end
	self:pushDebugUpdate()
end

function SyncAPI:getRemoteExecStatus()
	return self.lastExecStatus or "Exec disabled"
end

function SyncAPI:resetDebugState()
	self.debugState.handshake = false
	self.debugState.snapshot = false
	self.debugState.poll = false
	self.debugState.delta = false
	self.debugState.ackSent = false
	self.debugState.ackOk = false
	self.debugState.lastError = ""
	self.debugState.lastPollChangeCount = 0
	self.debugState.lastPollTargetRev = self.lastAppliedRevision
	self.debugState.lastServerRev = self.lastAppliedRevision
	self.debugState.pollErrorCount = 0
	self:pushDebugUpdate()
end

function SyncAPI:buildDebugPayload()
	local events = {}
	for _, line in ipairs(self.debugEvents) do
		table.insert(events, line)
	end

	local serverState = {}
	for key, value in pairs(self.serverDebugState) do
		serverState[key] = value
	end

	return {
		enabled = self.debugEnabled == true,
		checks = {
			handshake = self.debugState.handshake,
			snapshot = self.debugState.snapshot,
			poll = self.debugState.poll,
			delta = self.debugState.delta,
			ack_sent = self.debugState.ackSent,
			ack_ok = self.debugState.ackOk,
		},
		stats = {
			last_applied_rev = self.lastAppliedRevision,
			last_target_rev = self.debugState.lastPollTargetRev,
			last_server_rev = self.debugState.lastServerRev,
			last_change_count = self.debugState.lastPollChangeCount,
			poll_error_count = self.debugState.pollErrorCount,
			last_error = self.debugState.lastError,
			remote_exec_enabled = self.remoteExecEnabled == true,
			remote_exec_busy = self.remoteExecBusy == true,
			remote_exec_status = self.lastExecStatus or "Exec disabled",
		},
		server = serverState,
		events = events,
	}
end

function SyncAPI:pushDebugUpdate()
	local payload = self:buildDebugPayload()
	pcall(function()
		self.updateDebug(payload)
	end)
end

function SyncAPI:publishSyncHistory(historyEvents : {any}?)
	if typeof(historyEvents) ~= "table" or #historyEvents == 0 then
		return
	end

	pcall(function()
		self.updateStatus("Idle - synced rev " .. tostring(self.lastAppliedRevision), false, {
			kind = "history",
			events = historyEvents,
			revision = self.lastAppliedRevision,
		})
	end)
end

function SyncAPI:appendDebugEvent(message : string, isError : boolean?)
	if not self.debugEnabled then
		return
	end

	local stamp = os.date("!%H:%M:%S")
	local prefix = isError and "ERR" or "OK"
	local line = "[" .. stamp .. "] [" .. prefix .. "] " .. tostring(message)
	table.insert(self.debugEvents, line)
	if #self.debugEvents > DEBUG_EVENT_LIMIT then
		table.remove(self.debugEvents, 1)
	end

	if isError then
		warn("[RiftSync][debug] " .. tostring(message))
	else
		print("[RiftSync][debug] " .. tostring(message))
	end
	self:pushDebugUpdate()
end

function SyncAPI:setDebugError(message : string)
	self.debugState.lastError = formatGuidedError(message)
	self.debugState.pollErrorCount += 1
	self:appendDebugEvent(self.debugState.lastError, true)
	self:pushDebugUpdate()
end

function SyncAPI:refreshServerDebugState(force : boolean?)
	local nowClock = os.clock()
	if not force and (nowClock - self.lastServerDebugPullAt) < SERVER_DEBUG_PULL_INTERVAL then
		return
	end
	self.lastServerDebugPullAt = nowClock

	local response, requestError = self:requestJson("GET", TypeList.ENDPOINTS.DebugState, nil, {
		limit = 10,
	})
	if not response then
		self:appendDebugEvent("Fetch /debug/state gagal: " .. tostring(requestError), true)
		return
	end
	if response.status ~= "ok" then
		self:appendDebugEvent("Fetch /debug/state status: " .. tostring(response.status), true)
		return
	end

	local metrics = response.metrics
	local lastAck = {}
	local lastChanges = {}
	if typeof(metrics) == "table" then
		if typeof(metrics.last_ack) == "table" then
			lastAck = metrics.last_ack
		end
		if typeof(metrics.last_changes) == "table" then
			lastChanges = metrics.last_changes
		end
	end

	self.serverDebugState = {
		server_rev = tonumber(response.server_rev) or 0,
		indexed_script_count = tonumber(response.indexed_script_count) or 0,
		sessions_active = tonumber(response.sessions_active) or 0,
		ack_count = metrics and (tonumber(metrics.ack_count) or 0) or 0,
		ack_error_count = metrics and (tonumber(metrics.ack_error_count) or 0) or 0,
		changes_requests = metrics and (tonumber(metrics.changes_requests) or 0) or 0,
		changes_timeouts = metrics and (tonumber(metrics.changes_timeouts) or 0) or 0,
		last_ack_status = tostring(lastAck.status or "-"),
		last_ack_rev = tostring(lastAck.applied_rev or "-"),
		last_changes_count = tonumber(lastChanges.change_count) or 0,
		last_changes_target_rev = tonumber(lastChanges.target_rev) or 0,
	}
	self:pushDebugUpdate()
end

function SyncAPI:reportActivity(text : any, isError : boolean?, meta : any?)
	local textValue = tostring(text or "")
	if textValue == "" then
		return
	end
	local progress = tonumber(string.match(textValue, "%((%d+)%%%)")) or 0
	local loweredForThrottle = string.lower(textValue)
	local isTerminalActivity = progress >= 100
		or string.find(loweredForThrottle, "idle", 1, true) ~= nil
		or string.find(loweredForThrottle, "stopped", 1, true) ~= nil
		or string.find(loweredForThrottle, "synced", 1, true) ~= nil
		or string.find(loweredForThrottle, "resynced", 1, true) ~= nil
		or string.find(loweredForThrottle, "pulled", 1, true) ~= nil
	local nowClock = os.clock()
	if isError ~= true and isTerminalActivity ~= true and self.lastActivityReportAt ~= nil then
		local elapsed = nowClock - self.lastActivityReportAt
		if elapsed < 0.15 then
			return
		end
		if elapsed < 0.35 and textValue == self.lastActivityText then
			return
		end
	end

	local operation = ""
	if typeof(meta) == "table" and typeof(meta.operation) == "string" then
		operation = meta.operation
	else
		local lowered = string.lower(textValue)
		if string.find(lowered, "pull", 1, true) then
			operation = "pull_studio"
		elseif string.find(lowered, "snapshot", 1, true) or string.find(lowered, "resync", 1, true) then
			operation = "resync"
		elseif string.find(lowered, "connect", 1, true) then
			operation = "connect"
		elseif string.find(lowered, "sync", 1, true) then
			operation = "sync"
		elseif string.find(lowered, "stopped", 1, true) then
			operation = "stop"
		elseif string.find(lowered, "idle", 1, true) then
			operation = "idle"
		end
	end

	local payload = {
		client_id = self.clientId,
		text = textValue,
		error = isError == true,
		operation = operation,
		progress = progress,
		revision = tonumber(self.lastAppliedRevision) or 0,
	}

	local body
	local encodeOk = pcall(function()
		body = httpService:JSONEncode(payload)
	end)
	if not encodeOk then
		return
	end

	pcall(function()
		httpService:RequestAsync({
			Url = self.baseUrl .. (TypeList.ENDPOINTS.Activity or "/activity"),
			Method = "POST",
			Headers = {
				["Content-Type"] = "application/json",
			},
			Body = body,
		})
	end)
	self.lastActivityReportAt = nowClock
	self.lastActivityText = textValue
end

function SyncAPI:requestJson(method : string, endpoint : string, payload : {[string]: any}?, queryParams : {[string]: string | number}?)
	return self:requestJsonWithHeaders(method, endpoint, payload, queryParams, nil)
end

function SyncAPI:requestJsonWithHeaders(method : string, endpoint : string, payload : {[string]: any}?, queryParams : {[string]: string | number}?, extraHeaders : {[string]: string}?)
	local url = self.baseUrl .. endpoint
	if queryParams then
		url ..= "?" .. buildQuery(queryParams)
	end

	local headers = {
		["Content-Type"] = "application/json"
	}
	if typeof(extraHeaders) == "table" then
		for key, value in pairs(extraHeaders) do
			headers[tostring(key)] = tostring(value)
		end
	end

	local requestOptions = {
		Url = url,
		Method = method,
		Headers = headers
	}

	local body = createBody(payload)
	if body then
		requestOptions.Body = body
	end

	local requestStarted = os.clock()
	self:appendDebugEvent("HTTP " .. method .. " " .. endpoint, false)

	local response
	local success, requestError = pcall(function()
		response = httpService:RequestAsync(requestOptions)
	end)

	if not success then
		local rawError = "Request failed: " .. tostring(requestError)
		self:appendDebugEvent("HTTP " .. method .. " " .. endpoint .. " gagal request: " .. tostring(requestError), true)
		return nil, formatGuidedError(rawError)
	end

	if not response.Success then
		local rawError = formatHttpError(response)
		self:appendDebugEvent("HTTP " .. method .. " " .. endpoint .. " gagal: " .. rawError, true)
		return nil, formatGuidedError(rawError)
	end

	local decoded, decodeError = decodeJson(response.Body)
	if decoded == nil then
		local rawError = "Invalid JSON response: " .. tostring(decodeError)
		self:appendDebugEvent("HTTP " .. method .. " " .. endpoint .. " invalid JSON: " .. tostring(decodeError), true)
		return nil, formatGuidedError(rawError)
	end

	local elapsedMs = math.floor((os.clock() - requestStarted) * 1000 + 0.5)
	self:appendDebugEvent("HTTP " .. method .. " " .. endpoint .. " ok (" .. tostring(elapsedMs) .. "ms)", false)

	return decoded
end

function SyncAPI:fetchRevisionHistory(limit : number?)
	local endpoint = TypeList.ENDPOINTS.History or "/history"
	local response, requestError = self:requestJson("GET", endpoint, nil, {
		limit = tonumber(limit) or 15,
	})
	if not response then
		return nil, requestError
	end
	if response.status ~= "ok" then
		return nil, "History rejected: " .. tostring(response.status)
	end
	return response
end

function SyncAPI:fetchRevisionDetail(revision : number)
	local endpoint = TypeList.ENDPOINTS.History or "/history"
	local numericRevision = tonumber(revision)
	if not numericRevision then
		return nil, "Revision tidak valid"
	end

	local response, requestError = self:requestJson("GET", endpoint, nil, {
		rev = numericRevision,
	})
	if not response then
		return nil, requestError
	end
	if response.status ~= "ok" then
		return nil, "History detail rejected: " .. tostring(response.status)
	end
	return response
end

function SyncAPI:sendAck(status : string, errors : {string}?)
	local payload = {
		client_id = self.clientId,
		session_id = self.sessionId,
		status = status,
		applied_rev = self.lastAppliedRevision,
		errors = errors or {}
	}

	self.debugState.ackSent = true
	self.debugState.ackOk = false
	self:appendDebugEvent(
		"ACK kirim status=" .. tostring(status) .. " rev=" .. tostring(self.lastAppliedRevision),
		status == "error"
	)

	local response, requestError = self:requestJson("POST", TypeList.ENDPOINTS.Ack, payload)
	if not response then
		self:setDebugError("ACK request gagal: " .. tostring(requestError))
		return false, requestError
	end

	if response.status ~= "ok" then
		local message = "ACK ditolak server: " .. tostring(response.status)
		self:setDebugError(message)
		return false, message
	end

	self.debugState.ackOk = true
	self:appendDebugEvent("ACK diterima server", false)
	self:refreshServerDebugState(false)
	self:pushDebugUpdate()
	return true
end

function SyncAPI:collectStudioSnapshot()
	local scripts = {}
	local skippedCount = 0
	local seenByPath = {}
	local sourceReadFailedCount = 0
	local sourceReadFailedSamples = {}
	local candidateScripts = {}
	local candidateLookup = {}
	local hasNestedScriptDescendant = {}

	for _, rootPath in ipairs(self.managedRoots) do
		local rootInstance = resolvePath(rootPath)
		if rootInstance then
			local candidates = rootInstance:GetDescendants()
			if isScriptClass(rootInstance.ClassName) then
				table.insert(candidates, rootInstance)
			end

			for _, candidate in ipairs(candidates) do
				if not isScriptClass(candidate.ClassName) then
					continue
				end

				if not candidateLookup[candidate] then
					candidateLookup[candidate] = true
					table.insert(candidateScripts, candidate)
				end
			end
		end
	end

	for _, candidate in ipairs(candidateScripts) do
		local parent = candidate.Parent
		while parent and parent ~= game do
			if isScriptClass(parent.ClassName) then
				hasNestedScriptDescendant[parent] = true
			end
			parent = parent.Parent
		end
	end

	for _, candidate in ipairs(candidateScripts) do
		local className = candidate.ClassName
		local suffix = getPreferredSuffixForClass(className)
		if not suffix then
			skippedCount += 1
			continue
		end

		local gamePath = buildGamePath(candidate)
		if isIgnoredPath(gamePath, self.ignoredRbxPaths) then
			skippedCount += 1
			continue
		end

		local useInitFormat = hasNestedScriptDescendant[candidate] == true
		local localPath = buildLocalPathFromInstance(candidate, suffix, useInitFormat)
		if not localPath then
			skippedCount += 1
			continue
		end

		if seenByPath[localPath] then
			skippedCount += 1
			continue
		end

		local source, sourceError = readStudioScriptSource(candidate)
		if typeof(source) ~= "string" then
			skippedCount += 1
			sourceReadFailedCount += 1
			if #sourceReadFailedSamples < 5 then
				table.insert(sourceReadFailedSamples, candidate:GetFullName() .. " " .. tostring(sourceError))
			end
			continue
		end

		seenByPath[localPath] = true
		table.insert(scripts, {
			local_path = localPath,
			rbx_path = gamePath,
			class_name = className,
			source = source,
		})
	end

	if sourceReadFailedCount > 0 then
		warn(
			"[RiftSync] Failed reading source: "
				.. tostring(sourceReadFailedCount)
				.. " (sample: "
				.. table.concat(sourceReadFailedSamples, " | ")
				.. ")"
		)
	end

	return scripts, skippedCount, sourceReadFailedCount
end

function SyncAPI:pushStudioSnapshot()
	local files, skippedCount, sourceReadFailedCount = self:collectStudioSnapshot()

	local payload = {
		client_id = self.clientId,
		session_id = self.sessionId,
		mode = "replace",
		files = files,
	}

	local response, requestError = self:requestJson("POST", TypeList.ENDPOINTS.Bootstrap, payload, nil)
	if not response then
		self:setDebugError("Bootstrap request gagal: " .. tostring(requestError))
		return false, requestError
	end

	if response.status ~= "ok" then
		self:setDebugError("Bootstrap ditolak: " .. tostring(response.status))
		return false, "Bootstrap rejected"
	end

	local serverErrors = response.errors
	if typeof(serverErrors) == "table" and #serverErrors > 0 then
		warn("Bootstrap server errors: " .. table.concat(serverErrors, " | "))
	end

	local pushedCount = tonumber(response.written_count) or #files
	self.updateStatus(
		"Bootstrap: "
			.. tostring(pushedCount)
			.. " ditulis, "
			.. tostring(skippedCount)
			.. " dilewati (readFail="
			.. tostring(sourceReadFailedCount)
			.. ")",
		false
	)
	self:appendDebugEvent(
		"Bootstrap sukses, written="
			.. tostring(tonumber(response.written_count) or #files)
			.. ", skipped="
			.. tostring(skippedCount),
		false
	)
	self:pushDebugUpdate()

	return true
end

function SyncAPI:applyUpsert(change : {[string]: any}, revision : number)
	local rbxPath = change.rbx_path
	local className = change.class_name
	local source = change.source
	local localPath = change.local_path

	if typeof(rbxPath) ~= "string" or typeof(className) ~= "string" or typeof(source) ~= "string" then
		return false, "Invalid upsert payload"
	end

	if isIgnoredPath(rbxPath, self.ignoredRbxPaths) then
		return true
	end

	if not isScriptClass(className) then
		return false, "Unsupported class in upsert: " .. className
	end

	local parent, targetName, parentError = ensureParentPath(rbxPath)
	if not parent then
		return false, parentError
	end

	local scriptInstance, conflictError = replaceManagedConflict(parent, targetName, className)
	if conflictError then
		return false, conflictError
	end

	if not scriptInstance then
		scriptInstance = Instance.new(className)
		scriptInstance.Name = targetName
		scriptInstance.Parent = parent
	end

	local didUpdate = setScriptSource(scriptInstance, source)
	if not didUpdate then
		return false, "Failed to update source for " .. scriptInstance:GetFullName()
	end

	scriptInstance:SetAttribute(TypeList.MANAGED_ATTRIBUTES.IsManaged, true)
	scriptInstance:SetAttribute(TypeList.MANAGED_ATTRIBUTES.LocalPath, localPath)
	scriptInstance:SetAttribute(TypeList.MANAGED_ATTRIBUTES.LastRevision, revision)

	return true
end

function SyncAPI:applyDelete(change : {[string]: any})
	local rbxPath = change.rbx_path
	if typeof(rbxPath) ~= "string" then
		return false, "Invalid delete payload"
	end

	if isIgnoredPath(rbxPath, self.ignoredRbxPaths) then
		return true
	end

	local target, resolveError = resolvePath(rbxPath)
	if resolveError then
		return false, resolveError
	end

	if not target then
		return true
	end

	if target:GetAttribute(TypeList.MANAGED_ATTRIBUTES.IsManaged) then
		target:Destroy()
		return true
	end

	return true, "Skipped unmanaged delete at " .. rbxPath
end

function SyncAPI:applyRename(change : {[string]: any}, revision : number)
	local oldPath = change.old_rbx_path
	local newPath = change.new_rbx_path
	if typeof(oldPath) ~= "string" or typeof(newPath) ~= "string" then
		return false, "Invalid rename payload"
	end

	if isIgnoredPath(oldPath, self.ignoredRbxPaths) or isIgnoredPath(newPath, self.ignoredRbxPaths) then
		return true
	end

	local existing = resolvePath(oldPath)
	if not existing then
		if typeof(change.source) == "string" then
			return self:applyUpsert(change, revision)
		end
		return false, "Rename source not found: " .. oldPath
	end

	if not existing:GetAttribute(TypeList.MANAGED_ATTRIBUTES.IsManaged) then
		return false, "Refused to rename unmanaged instance at " .. oldPath
	end

	local newParent, newName, parentError = ensureParentPath(newPath)
	if not newParent then
		return false, parentError
	end

	local occupied = newParent:FindFirstChild(newName)
	if occupied and occupied ~= existing then
		if occupied:GetAttribute(TypeList.MANAGED_ATTRIBUTES.IsManaged) then
			occupied:Destroy()
		else
			return false, "Rename target occupied by unmanaged instance: " .. newPath
		end
	end

	existing.Name = newName
	existing.Parent = newParent
	existing:SetAttribute(TypeList.MANAGED_ATTRIBUTES.LocalPath, change.local_path)
	existing:SetAttribute(TypeList.MANAGED_ATTRIBUTES.LastRevision, revision)
	return true
end

function SyncAPI:applyChanges(changes : {any}, targetRevision : number)
	if #changes == 0 then
		self.lastAppliedRevision = targetRevision
		self.plugin:SetSetting(SETTING_KEYS.LastRevision, self.lastAppliedRevision)
		return true, {}, {}, {
			total = 0,
			applied = 0,
		}
	end

	local orderedChanges = {}
	for _, change in ipairs(changes) do
		table.insert(orderedChanges, change)
	end

	local function pathDepth(pathValue : any)
		if typeof(pathValue) ~= "string" then
			return 9999
		end

		local _, dotCount = string.gsub(pathValue, "%.", "")
		return dotCount + 1
	end

	table.sort(orderedChanges, function(a, b)
		local aOp = tostring(a.op)
		local bOp = tostring(b.op)
		local aDelete = aOp == "delete"
		local bDelete = bOp == "delete"

		if aDelete ~= bDelete then
			-- Apply upsert/rename first so parent scripts exist before child scripts.
			return not aDelete
		end

		if aDelete and bDelete then
			local aDepth = pathDepth(a.rbx_path)
			local bDepth = pathDepth(b.rbx_path)
			if aDepth ~= bDepth then
				return aDepth > bDepth
			end
		else
			local aPath = a.new_rbx_path or a.rbx_path
			local bPath = b.new_rbx_path or b.rbx_path
			local aDepth = pathDepth(aPath)
			local bDepth = pathDepth(bPath)
			if aDepth ~= bDepth then
				return aDepth < bDepth
			end
		end

		local aKey = tostring(a.new_rbx_path or a.rbx_path or a.local_path or "")
		local bKey = tostring(b.new_rbx_path or b.rbx_path or b.local_path or "")
		return aKey < bKey
	end)

	local errors = {}
	local errorSeen = {}
	local notices = {}
	local noticeSeen = {}
	local appliedCount = 0

	for _, change in ipairs(orderedChanges) do
		local operation = change.op
		local ok, errorMessage

		if operation == "upsert" then
			ok, errorMessage = self:applyUpsert(change, targetRevision)
		elseif operation == "delete" then
			ok, errorMessage = self:applyDelete(change)
		elseif operation == "rename" then
			ok, errorMessage = self:applyRename(change, targetRevision)
		else
			ok = false
			errorMessage = "Unsupported operation: " .. tostring(operation)
		end

		if ok then
			appliedCount += 1
			if typeof(errorMessage) == "string" and errorMessage ~= "" then
				local notice = tostring(errorMessage)
				if not noticeSeen[notice] then
					noticeSeen[notice] = true
					table.insert(notices, notice)
				end
			end
		else
			local message = tostring(errorMessage)
			if not errorSeen[message] then
				errorSeen[message] = true
				table.insert(errors, message)
			end
		end
	end

	local wasSuccessful = #errors == 0
	if wasSuccessful then
		self.lastAppliedRevision = targetRevision
		self.plugin:SetSetting(SETTING_KEYS.LastRevision, self.lastAppliedRevision)
	end

	return wasSuccessful, errors, notices, {
		total = #orderedChanges,
		applied = appliedCount,
	}
end

function SyncAPI:fetchSnapshot()
	local response, snapshotError = self:requestJson("GET", TypeList.ENDPOINTS.Snapshot, nil, {
		encoding = self.changeEncoding or "compact-json-v1",
	})
	if not response then
		self.debugState.snapshot = false
		self:setDebugError("Fetch snapshot gagal: " .. tostring(snapshotError))
		return false, snapshotError
	end

	local changes = expandCompactChanges(response.changes or {}, response.change_encoding)
	local serverRevision = tonumber(response.server_rev) or self.lastAppliedRevision
	self.debugState.lastServerRev = serverRevision
	local ok, errors, notices, applyStats = self:applyChanges(changes, serverRevision)
	if not ok then
		if typeof(applyStats) == "table" then
			self:publishSyncHistory(applyStats.history)
		end
		self.debugState.snapshot = false
		self:setDebugError(buildErrorSummary("Snapshot apply gagal", errors, 3))
		warnErrorList("Snapshot apply", errors)
		return false, buildErrorSummary("Snapshot apply gagal", errors, 3)
	end

	self.debugState.snapshot = true
	if #notices > 0 then
		local summary = buildErrorSummary("Snapshot notice", notices, 3)
		self:appendDebugEvent(summary, false)
	end
	self:appendDebugEvent("Snapshot apply sukses, rev " .. tostring(self.lastAppliedRevision), false)
	if typeof(applyStats) == "table" then
		self:publishSyncHistory(applyStats.history)
	end
	self:pushDebugUpdate()
	return true
end

function SyncAPI:performHandshake()
	local payload = {
		protocol = TypeList.PROTOCOL,
		plugin_version = TypeList.VERSION,
		accept_encodings = {
			TypeList.CHANGE_ENCODINGS and TypeList.CHANGE_ENCODINGS.Compact or "compact-json-v1",
			TypeList.CHANGE_ENCODINGS and TypeList.CHANGE_ENCODINGS.Verbose or "verbose-json-v1",
		},
		client_id = self.clientId,
		last_applied_rev = self.lastAppliedRevision
	}

	local response, requestError = self:requestJson("POST", TypeList.ENDPOINTS.Handshake, payload, nil)
	if not response then
		self.debugState.handshake = false
		self:setDebugError("Handshake gagal: " .. tostring(requestError))
		return false, requestError
	end

	if response.status ~= "ok" then
		self.debugState.handshake = false
		self:setDebugError("Handshake ditolak: " .. tostring(response.status))
		return false, "Handshake rejected"
	end

	self.sessionId = tostring(response.session_id or "")
	if typeof(response.managed_roots) == "table" and #response.managed_roots > 0 then
		self.managedRoots = response.managed_roots
	end
	if typeof(response.ignored_rbx_paths) == "table" then
		self.ignoredRbxPaths = response.ignored_rbx_paths
	end
	self.serverIndexedCount = tonumber(response.indexed_script_count) or 0
	self.debugState.handshake = true
	self.debugState.lastServerRev = tonumber(response.server_rev) or self.debugState.lastServerRev
	self.debugState.lastError = ""
	self:appendDebugEvent(
		"Handshake ok, session=" .. self.sessionId .. ", serverRev=" .. tostring(self.debugState.lastServerRev),
		false
	)
	self:refreshServerDebugState(true)
	self:pushDebugUpdate()
	return true
end

function SyncAPI:pollChanges()
	local query = {
		since_rev = self.lastAppliedRevision,
		timeout = TypeList.POLL_TIMEOUT_SECONDS,
		encoding = self.changeEncoding or "compact-json-v1",
	}

	local response, requestError = self:requestJson("GET", TypeList.ENDPOINTS.Changes, nil, query)
	if not response then
		self.debugState.poll = false
		self:setDebugError("Poll changes gagal: " .. tostring(requestError))
		return false, requestError, "connection"
	end

	self.debugState.poll = true
	self.debugState.lastServerRev = tonumber(response.server_rev) or self.debugState.lastServerRev
	self.debugState.lastPollTargetRev = tonumber(response.target_rev) or self.lastAppliedRevision
	local incomingChanges = expandCompactChanges(response.changes or {}, response.change_encoding)
	self.debugState.lastPollChangeCount = #incomingChanges
	self.debugState.delta = #incomingChanges > 0
	self:appendDebugEvent(
		"Poll rev " .. tostring(self.lastAppliedRevision) .. " -> " .. tostring(self.debugState.lastPollTargetRev)
			.. " (changes="
			.. tostring(self.debugState.lastPollChangeCount)
			.. ", snapshot="
			.. tostring(response.needs_snapshot == true)
			.. ")",
		false
	)

	if response.needs_snapshot then
		local didSnapshot, snapshotError = self:fetchSnapshot()
		if not didSnapshot then
			local category = string.find(string.lower(tostring(snapshotError)), "apply gagal", 1, true) and "apply"
				or "connection"
			return false, "Snapshot sync failed: " .. tostring(snapshotError), category
		end
		local acked, ackError = self:sendAck("ok")
		if not acked then
			return false, "Ack gagal: " .. tostring(ackError), "connection"
		end
		return true, true
	end

	local targetRevision = tonumber(response.target_rev) or self.lastAppliedRevision
	local changes = incomingChanges
	local hadChanges = #changes > 0
	local applied, errors, notices, applyStats = self:applyChanges(changes, targetRevision)

	if #notices > 0 then
		self:appendDebugEvent(buildErrorSummary("Delta notice", notices, 3), false)
	end

	if applied then
		local acked, ackError = self:sendAck("ok")
		if not acked then
			return false, "Ack gagal: " .. tostring(ackError), "connection"
		end
		self.debugState.lastError = ""
		if typeof(applyStats) == "table" then
			self:appendDebugEvent(
				"Delta apply sukses " .. tostring(applyStats.applied or 0) .. "/" .. tostring(applyStats.total or 0),
				false
			)
			self:publishSyncHistory(applyStats.history)
		end
		self:pushDebugUpdate()
		return true, hadChanges
	end

	local acked = self:sendAck("error", errors)
	if not acked then
		self:appendDebugEvent("ACK error payload gagal terkirim", true)
	end
	if typeof(applyStats) == "table" then
		self:publishSyncHistory(applyStats.history)
	end
	self:setDebugError(buildErrorSummary("Delta apply gagal", errors, 3))
	warnErrorList("Delta apply", errors)
	return false, buildErrorSummary("Delta apply gagal", errors, 3), "apply"
end

local function serializeExecValue(value : any)
	local valueType = typeof(value)
	if valueType == "nil" then
		return "nil"
	end
	if valueType == "string" or valueType == "number" or valueType == "boolean" then
		return value
	end
	if valueType == "Instance" then
		local ok, fullName = pcall(function()
			return value:GetFullName()
		end)
		if ok then
			return tostring(fullName)
		end
	end
	return tostring(value)
end

local function packExecValues(...)
	local packed = { ... }
	packed.n = select("#", ...)
	return packed
end

local function serializeExecReturns(packed)
	local result = {}
	local count = tonumber(packed.n) or #packed
	for index = 1, count do
		table.insert(result, serializeExecValue(packed[index]))
	end
	return result
end

local function appendExecPrint(prints : {any}, level : string, startedAt : number, values)
	local parts = {}
	local count = tonumber(values.n) or #values
	for index = 1, count do
		parts[index] = tostring(values[index])
	end
	table.insert(prints, {
		level = level,
		text = table.concat(parts, "\t"),
		at_ms = math.floor((os.clock() - startedAt) * 1000 + 0.5),
	})
end

function SyncAPI:remoteExecHeaders()
	local token = tostring(self.remoteExecToken or "")
	if token == "" then
		return nil
	end
	return {
		Authorization = "Bearer " .. token,
	}
end

function SyncAPI:startRemoteExecLoop(runToken : number)
	if self.remoteExecLoopRunning then
		return
	end
	self.remoteExecLoopRunning = true
	task.spawn(function()
		while self.running and self.runToken == runToken do
			if self.remoteExecEnabled ~= true or self.debugState.handshake ~= true then
				task.wait(TypeList.EXEC_IDLE_WAIT_SECONDS or 0.2)
			elseif tostring(self.remoteExecToken or "") == "" then
				self.lastExecStatus = "Exec error: token required"
				self:pushDebugUpdate()
				task.wait(TypeList.EXEC_IDLE_WAIT_SECONDS or 0.2)
			elseif runService:IsRunning() then
				self.lastExecStatus = "Exec error: Play mode"
				self:pushDebugUpdate()
				self:pollRemoteExecCommand()
				if self.running and self.runToken == runToken then
					self.lastExecStatus = "Exec error: Play mode"
					self:pushDebugUpdate()
				end
				task.wait(TypeList.EXEC_IDLE_WAIT_SECONDS or 0.2)
			else
				self:pollRemoteExecCommand()
				task.wait(TypeList.EXEC_IDLE_WAIT_SECONDS or 0.2)
			end
		end
		self.remoteExecLoopRunning = false
	end)
end

function SyncAPI:pollRemoteExecCommand()
	if self.remoteExecBusy then
		return
	end
	self.remoteExecBusy = true
	local response, requestError = self:requestJsonWithHeaders("GET", TypeList.ENDPOINTS.ExecNext, nil, {
		client_id = self.clientId,
		timeout = TypeList.EXEC_POLL_TIMEOUT_SECONDS or 1,
	}, self:remoteExecHeaders())
	if not response then
		self.lastExecStatus = "Exec error: " .. tostring(requestError)
		self.remoteExecBusy = false
		self:appendDebugEvent("Remote Exec poll gagal: " .. tostring(requestError), true)
		self:pushDebugUpdate()
		return
	end
	if response.status ~= "ok" then
		self.lastExecStatus = "Exec error: " .. tostring(response.status)
		self.remoteExecBusy = false
		self:pushDebugUpdate()
		return
	end
	if typeof(response.command) ~= "table" then
		self.lastExecStatus = "Exec ready"
		self.remoteExecBusy = false
		self:pushDebugUpdate()
		return
	end

	local command = response.command
	self.lastExecStatus = "Exec running"
	self:pushDebugUpdate()
	if self.remoteExecEnabled ~= true then
		local disabledResult = {
			command_id = tostring(command.id or ""),
			client_id = self.clientId,
			ok = false,
			duration_ms = 0,
			prints = {},
			returns = {},
			error = "Remote Exec was disabled before execution",
			traceback = "",
		}
		self:postRemoteExecResult(disabledResult)
		self.remoteExecBusy = false
		self.lastExecStatus = "Exec disabled"
		self:pushDebugUpdate()
		return
	end
	local result = self:executeRemoteCommand(command)
	self:postRemoteExecResult(result)
	self.remoteExecBusy = false
	self.lastExecStatus = result.ok and "Exec ready" or "Exec error"
	self:pushDebugUpdate()
end

function SyncAPI:executeRemoteCommand(command : {[string]: any})
	local startedAt = os.clock()
	local commandId = tostring(command.id or "")
	local prints = {}
	local result = {
		command_id = commandId,
		client_id = self.clientId,
		ok = false,
		duration_ms = 0,
		prints = prints,
		returns = {},
		error = "",
		traceback = "",
	}

	local function finish(ok : boolean, errorMessage : any?, tracebackText : any?, returns : {any}?)
		result.ok = ok == true
		result.duration_ms = math.floor((os.clock() - startedAt) * 1000 + 0.5)
		result.error = tostring(errorMessage or "")
		result.traceback = tostring(tracebackText or "")
		result.returns = returns or {}
		return result
	end

	if commandId == "" then
		return finish(false, "Remote Exec command id is missing", "")
	end
	if runService:IsRunning() then
		return finish(false, "Remote Exec is disabled during Play mode", "")
	end
	if typeof(loadstring) ~= "function" then
		return finish(false, "Remote Exec loadstring is unavailable in this Studio/plugin context", "")
	end
	if typeof(setfenv) ~= "function" or typeof(getfenv) ~= "function" then
		return finish(false, "Remote Exec environment functions are unavailable in this Studio/plugin context", "")
	end

	local source = tostring(command.source or "")
	local wrappedSource = "return function()\n" .. source .. "\nend"
	local compiled, compileError = loadstring(wrappedSource)
	if not compiled then
		return finish(false, "Compile error: " .. tostring(compileError), tostring(compileError))
	end

	local baseEnvironment = getfenv(0)
	local environment = setmetatable({
		game = game,
		workspace = workspace,
		plugin = self.plugin,
		script = script,
		Instance = Instance,
		Enum = Enum,
		Color3 = Color3,
		Vector3 = Vector3,
		UDim2 = UDim2,
		task = task,
		print = function(...)
			appendExecPrint(prints, "print", startedAt, packExecValues(...))
		end,
		warn = function(...)
			appendExecPrint(prints, "warn", startedAt, packExecValues(...))
		end,
	}, {
		__index = baseEnvironment,
	})
	setfenv(compiled, environment)

	local commandFactoryOk, commandFunctionOrError = xpcall(function()
		return compiled()
	end, debug.traceback)
	if not commandFactoryOk then
		return finish(false, commandFunctionOrError, commandFunctionOrError)
	end
	if typeof(commandFunctionOrError) ~= "function" then
		return finish(false, "Compiled Remote Exec source did not return a function", "")
	end
	pcall(function()
		setfenv(commandFunctionOrError, environment)
	end)

	local ok, packedOrError = xpcall(function()
		return packExecValues(commandFunctionOrError())
	end, debug.traceback)
	if not ok then
		return finish(false, packedOrError, packedOrError)
	end
	return finish(true, "", "", serializeExecReturns(packedOrError))
end

function SyncAPI:postRemoteExecResult(result : {[string]: any})
	local response, requestError = self:requestJsonWithHeaders("POST", TypeList.ENDPOINTS.ExecResult, result, nil, self:remoteExecHeaders())
	if not response then
		self:appendDebugEvent("Remote Exec result gagal: " .. tostring(requestError), true)
		self.lastExecStatus = "Exec error: result post failed"
		return false, requestError
	end
	if response.status ~= "ok" then
		local message = "Remote Exec result rejected: " .. tostring(response.status)
		self:appendDebugEvent(message, true)
		self.lastExecStatus = "Exec error: result rejected"
		return false, message
	end
	self:appendDebugEvent("Remote Exec result terkirim id=" .. tostring(result.command_id), result.ok ~= true)
	return true
end

function SyncAPI:waitBeforeReconnect(runToken : number, reason : any)
	self.initialSyncInProgress = false
	self.debugState.handshake = false
	self.debugState.poll = false
	self.updateStatus("Reconnecting to server... " .. tostring(reason), false)
	self:appendDebugEvent("Reconnect dijadwalkan: " .. tostring(reason), true)
	self:pushDebugUpdate()

	local interval = tonumber(TypeList.RECONNECT_INTERVAL_SECONDS) or 3
	local startedAt = os.clock()
	while self.running and self.runToken == runToken and (os.clock() - startedAt) < interval do
		task.wait(0.25)
	end
end

function SyncAPI:runLoop(runToken : number)
	self:resetDebugState()
	self:appendDebugEvent("Run loop dimulai (mode=" .. tostring(self.startMode) .. ")", false)

	while self.running and self.runToken == runToken do
		self.updateStatus("Connecting to server... (10%)", false)
		local didHandshake, handshakeError = self:performHandshake()
		if not didHandshake then
			self:waitBeforeReconnect(runToken, handshakeError)
			continue
		end

		local readyForSnapshot = true
		if self.startMode == TypeList.START_MODES.StudioToFolder then
			self.updateStatus("Syncing Studio snapshot... (25%)", false)
			local didBootstrap, bootstrapError = self:pushStudioSnapshot()
			if not didBootstrap then
				readyForSnapshot = false
				self:waitBeforeReconnect(runToken, "bootstrap failed: " .. tostring(bootstrapError))
			end
		else
			if self.serverIndexedCount <= 0 then
				self.updateStatus("Syncing initial Studio snapshot... (25%)", false)
				local didBootstrap, bootstrapError = self:pushStudioSnapshot()
				if not didBootstrap then
					readyForSnapshot = false
					self:waitBeforeReconnect(runToken, "initial bootstrap failed: " .. tostring(bootstrapError))
				end
			else
				self.updateStatus("Syncing local folder to Studio... (35%)", false)
			end
		end

		if not readyForSnapshot then
			continue
		end

		self.updateStatus("Syncing snapshot... (55%)", false)
		self.initialSyncInProgress = true
		local didSnapshot, snapshotError = self:fetchSnapshot()
		self.initialSyncInProgress = false
		if not didSnapshot then
			local snapshotText = tostring(snapshotError)
			if string.find(string.lower(snapshotText), "apply gagal", 1, true) then
				self.running = false
				self.updateStatus("Not synced - snapshot failed: " .. snapshotText, true)
				return
			end
			self:waitBeforeReconnect(runToken, "snapshot failed: " .. snapshotText)
			continue
		end

		self.updateStatus("Idle - synced rev " .. tostring(self.lastAppliedRevision), false)
		self:startRemoteExecLoop(runToken)

		while self.running and self.runToken == runToken do
			local ok, pollResult, failureKind = self:pollChanges()
			if not ok then
				if failureKind == "apply" then
					self.running = false
					self.updateStatus("Not synced - " .. tostring(pollResult), true)
					return
				end
				self:waitBeforeReconnect(runToken, pollResult)
				break
			else
				if pollResult == true then
					self.updateStatus("Idle - synced rev " .. tostring(self.lastAppliedRevision), false)
				end
				task.wait(TypeList.POLL_INTERVAL_SECONDS)
			end
		end
	end
end

function SyncAPI:start()
	if self.running then
		return false, "Sync sudah jalan"
	end

	if runService:IsRunning() then
		return false, "Jalankan sync saat Edit mode, bukan Play mode"
	end

	self.running = true
	self.runToken += 1
	self.initialSyncInProgress = false
	if self.remoteExecEnabled then
		if tostring(self.remoteExecToken or "") == "" then
			self.lastExecStatus = "Exec error: token required"
		else
			self.lastExecStatus = "Exec ready"
		end
	end
	local currentToken = self.runToken
	self.updateStatus("Connecting to " .. self.baseUrl .. "... (5%)", false)
	self.debugState.lastError = ""
	self:appendDebugEvent("Start sync ke " .. self.baseUrl, false)
	self:pushDebugUpdate()

	task.spawn(function()
		self:runLoop(currentToken)
	end)

	return true
end

function SyncAPI:stop()
	self.running = false
	self.initialSyncInProgress = false
	self.remoteExecBusy = false
	self.lastExecStatus = "Exec disabled"
	self.updateStatus("Idle - sync stopped", false)
	self:appendDebugEvent("Sync berhenti", false)
	self:pushDebugUpdate()
end

function SyncAPI:forceSnapshot()
	if not self.running then
		return false, "Start sync dulu sebelum resync"
	end

	local success, snapshotError = self:fetchSnapshot()
	if success then
		self.updateStatus("Idle - resynced rev " .. tostring(self.lastAppliedRevision), false)
		return true
	end

	self.running = false
	self.updateStatus("Not synced - resync failed: " .. tostring(snapshotError), true)
	return false, snapshotError
end

function SyncAPI:pullStudioToLocal()
	if not self.running then
		return false, "Start sync dulu sebelum pull Studio"
	end

	self.updateStatus("Pulling Studio snapshot to local... (25%)", false)
	local success, responseOrError = self:pushStudioSnapshot("merge")
	if not success then
		self.updateStatus("Not synced - pull Studio failed: " .. tostring(responseOrError), true)
		return false, responseOrError
	end

	local response = responseOrError
	local serverRevision = typeof(response) == "table" and tonumber(response.server_rev) or nil
	if serverRevision then
		self.lastAppliedRevision = serverRevision
		self.debugState.lastServerRev = serverRevision
		self.debugState.lastPollTargetRev = serverRevision
		self.plugin:SetSetting(SETTING_KEYS.LastRevision, self.lastAppliedRevision)
	end

	local newCount = typeof(response) == "table" and tonumber(response.new_count) or 0
	local updatedCount = typeof(response) == "table" and tonumber(response.updated_count) or 0
	local unchangedCount = typeof(response) == "table" and tonumber(response.unchanged_count) or 0
	local errorCount = typeof(response) == "table" and tonumber(response.error_count) or 0
	local statusKind = errorCount > 0 and "conflict" or "summary"
	local summaryParts = {
		tostring(newCount) .. " new",
		tostring(updatedCount) .. " updated",
		tostring(unchangedCount) .. " unchanged",
	}
	if errorCount > 0 then
		table.insert(summaryParts, tostring(errorCount) .. " errors")
	end

	self:publishSyncHistory({
		{
			status = statusKind,
			message = "Pull Studio: " .. table.concat(summaryParts, ", "),
			path = "Studio -> local",
		},
	})
	self.updateStatus("Idle - pulled Studio rev " .. tostring(self.lastAppliedRevision), false)
	self:pushDebugUpdate()
	return true
end


-- v2.1 overrides: add UI tree sync, typed properties, stable ids, and safer apply order.
local collectionService = game:GetService("CollectionService")

local UI_PROPERTIES_FILENAME = TypeList.UI_PROPERTIES_FILENAME or "properties.init.json"
local INSTANCE_KINDS = TypeList.INSTANCE_KINDS or {
	Script = "script",
	UIInstance = "ui_instance",
}

local function decodeLocalSegment(value : string)
	if string.sub(value, 1, 3) ~= "~x_" then
		return value
	end

	local payload = string.sub(value, 4)
	if #payload == 0 or (#payload % 2) ~= 0 then
		return value
	end

	local bytes = table.create(#payload / 2)
	for index = 1, #payload, 2 do
		local hex = string.sub(payload, index, index + 1)
		local byteValue = tonumber(hex, 16)
		if not byteValue then
			return value
		end
		table.insert(bytes, string.char(byteValue))
	end

	local ok, decoded = pcall(function()
		local joined = table.concat(bytes)
		local numeric = { string.byte(joined, 1, #joined) }
		return string.char(table.unpack(numeric))
	end)
	if ok and typeof(decoded) == "string" and decoded ~= "" then
		return decoded
	end

	return table.concat(bytes)
end

local function splitBySlash(value : string)
	local segments = {}
	for segment in string.gmatch(value, "[^/]+") do
		table.insert(segments, segment)
	end
	return segments
end

local function isManagedInstance(instance : Instance)
	return instance:GetAttribute(TypeList.MANAGED_ATTRIBUTES.IsManaged) == true
end

local function markManagedInstance(instance : Instance, localPath : string?, revision : number?, stableId : string?, entityKind : string?)
	instance:SetAttribute(TypeList.MANAGED_ATTRIBUTES.IsManaged, true)
	instance:SetAttribute("ManagedByLocalSync", true)
	if typeof(localPath) == "string" then
		instance:SetAttribute(TypeList.MANAGED_ATTRIBUTES.LocalPath, localPath)
		instance:SetAttribute("SourceId", localPath)
	end
	if typeof(revision) == "number" then
		instance:SetAttribute(TypeList.MANAGED_ATTRIBUTES.LastRevision, revision)
	end
	if typeof(stableId) == "string" and stableId ~= "" then
		instance:SetAttribute(TypeList.MANAGED_ATTRIBUTES.StableId, stableId)
		instance:SetAttribute("SyncId", stableId)
	end
	if typeof(entityKind) == "string" and entityKind ~= "" then
		instance:SetAttribute(TypeList.MANAGED_ATTRIBUTES.Kind, entityKind)
	end
end

local function ensureStableId(instance : Instance)
	local existing = instance:GetAttribute(TypeList.MANAGED_ATTRIBUTES.StableId)
	if typeof(existing) == "string" and existing ~= "" then
		return existing
	end

	local generated = httpService:GenerateGUID(false)
	instance:SetAttribute(TypeList.MANAGED_ATTRIBUTES.StableId, generated)
	return generated
end

local function isUIInstanceCandidate(instance : Instance)
	return instance:IsA("LayerCollector")
		or instance:IsA("GuiObject")
		or instance:IsA("GuiBase3d")
		or instance:IsA("UIComponent")
end

local ROOT_SERVICE_CLASS = TypeList.ROOT_SERVICE_CLASS or {
	StarterGui = "StarterGui",
	Workspace = "Workspace",
	Lighting = "Lighting",
}
local LIGHTING_CHILD_CLASSES = TypeList.LIGHTING_CHILD_CLASSES or {}
local REMOTE_SYNC_ROOT_SERVICES = TypeList.REMOTE_SYNC_ROOT_SERVICES or {
	ReplicatedStorage = true,
	ReplicatedFirst = true,
	Workspace = true,
	StarterGui = true,
	StarterPlayer = true,
	StarterPack = true,
	ServerScriptService = true,
	ServerStorage = true,
}
local REMOTE_SYNC_CLASSES = TypeList.REMOTE_SYNC_CLASSES or {
	RemoteEvent = true,
	RemoteFunction = true,
	UnreliableRemoteEvent = true,
	BindableEvent = true,
	BindableFunction = true,
}
local REMOTE_SYNC_CONTAINER_CLASSES = TypeList.REMOTE_SYNC_CONTAINER_CLASSES or {
	Folder = true,
	Configuration = true,
}

local workspaceService = game:GetService("Workspace")
local lightingService = game:GetService("Lighting")

local function isGuiSyncCandidate(instance : Instance)
	if not isUIInstanceCandidate(instance) then
		return false
	end
	if instance.ClassName == "CoreGui" then
		return false
	end
	return true
end

local function isLightingSyncChild(instance : Instance)
	if LIGHTING_CHILD_CLASSES[instance.ClassName] then
		return true
	end
	return instance:IsA("PostEffect") or instance:IsA("Sky") or instance:IsA("Atmosphere")
end

local function getTopLevelService(instance : Instance)
	local current = instance
	while current and current.Parent and current.Parent ~= game do
		current = current.Parent
	end
	if current and current.Parent == game then
		return current
	end
	return nil
end

local function isInRemoteSyncScope(instance : Instance)
	local topLevelService = getTopLevelService(instance)
	if not topLevelService then
		return false
	end
	return REMOTE_SYNC_ROOT_SERVICES[topLevelService.Name] == true
end

local function isRemoteSyncCandidate(instance : Instance)
	if not isInRemoteSyncScope(instance) then
		return false
	end

	local className = instance.ClassName
	return REMOTE_SYNC_CLASSES[className] == true or REMOTE_SYNC_CONTAINER_CLASSES[className] == true
end

local function shouldTraverseRemoteScopeChild(child : Instance)
	local className = child.ClassName
	return REMOTE_SYNC_CLASSES[className] == true or REMOTE_SYNC_CONTAINER_CLASSES[className] == true
end

local function shouldTrackPropertyInstance(instance : Instance)
	if isGuiSyncCandidate(instance) then
		return true
	end
	if isScriptClass(instance.ClassName) then
		return true
	end

	if instance == workspaceService or instance == lightingService then
		return true
	end

	if instance:IsA("Terrain") and instance.Parent == workspaceService then
		return true
	end

	if instance.Parent == lightingService and isLightingSyncChild(instance) then
		return true
	end

	if isRemoteSyncCandidate(instance) then
		return true
	end

	return false
end

local function shouldTraversePropertyChild(rootServiceName : string, child : Instance)
	if rootServiceName == "StarterGui" then
		return shouldTrackPropertyInstance(child)
	end
	if rootServiceName == "Workspace" then
		return child:IsA("Terrain") or shouldTraverseRemoteScopeChild(child)
	end
	if rootServiceName == "Lighting" then
		if child.Parent == lightingService and isLightingSyncChild(child) then
			return true
		end
		return shouldTraverseRemoteScopeChild(child)
	end
	if REMOTE_SYNC_ROOT_SERVICES[rootServiceName] then
		return shouldTraverseRemoteScopeChild(child)
	end
	return false
end

local function isProtectedRootInstance(instance : Instance)
	return instance.Parent == game and ROOT_SERVICE_CLASS[instance.Name] ~= nil
end

local function isLockedParentInstance(instance : Instance)
	return instance:IsA("Terrain")
end

local function asNumber(value : any, defaultValue : number)
	local numeric = tonumber(value)
	if numeric == nil then
		return defaultValue
	end
	return numeric
end

local function roundColorChannel(value : number)
	local channel = math.floor(value + 0.5)
	if channel < 0 then
		return 0
	end
	if channel > 255 then
		return 255
	end
	return channel
end

local function toColor3Table(color : Color3)
	return {
		["$type"] = "Color3",
		r = roundColorChannel(color.R * 255),
		g = roundColorChannel(color.G * 255),
		b = roundColorChannel(color.B * 255),
	}
end

local function parseUDim(value : any)
	if typeof(value) == "table" then
		return UDim.new(asNumber(value[1] or value.scale, 0), asNumber(value[2] or value.offset, 0))
	end
	if typeof(value) == "number" then
		return UDim.new(0, value)
	end
	return nil
end

local function parseUDim2(value : any)
	if typeof(value) ~= "table" then
		return nil
	end
	return UDim2.new(
		asNumber(value[1] or value.xScale, 0),
		asNumber(value[2] or value.xOffset, 0),
		asNumber(value[3] or value.yScale, 0),
		asNumber(value[4] or value.yOffset, 0)
	)
end

local function parseColor3(value : any)
	if typeof(value) ~= "table" then
		return nil
	end
	return Color3.fromRGB(
		roundColorChannel(asNumber(value[1] or value.r, 0)),
		roundColorChannel(asNumber(value[2] or value.g, 0)),
		roundColorChannel(asNumber(value[3] or value.b, 0))
	)
end

local function parseVector2(value : any)
	if typeof(value) ~= "table" then
		return nil
	end
	return Vector2.new(asNumber(value[1] or value.x, 0), asNumber(value[2] or value.y, 0))
end

local function serializeValue(value : any)
	local valueType = typeof(value)
	if valueType == "nil" or valueType == "boolean" or valueType == "number" or valueType == "string" then
		return value
	end

	if valueType == "BrickColor" then
		return {
			["$type"] = "BrickColor",
			name = value.Name,
		}
	end

	if valueType == "Color3" then
		return toColor3Table(value)
	end

	if valueType == "UDim" then
		return {
			["$type"] = "UDim",
			scale = value.Scale,
			offset = value.Offset,
		}
	end

	if valueType == "UDim2" then
		return {
			["$type"] = "UDim2",
			xScale = value.X.Scale,
			xOffset = value.X.Offset,
			yScale = value.Y.Scale,
			yOffset = value.Y.Offset,
		}
	end

	if valueType == "Vector2" then
		return {
			["$type"] = "Vector2",
			x = value.X,
			y = value.Y,
		}
	end

	if valueType == "Vector3" then
		return {
			["$type"] = "Vector3",
			x = value.X,
			y = value.Y,
			z = value.Z,
		}
	end
	if valueType == "Rect" then
		return {
			["$type"] = "Rect",
			minX = value.Min.X,
			minY = value.Min.Y,
			maxX = value.Max.X,
			maxY = value.Max.Y,
		}
	end

	if valueType == "EnumItem" then
		local enumTypeName = ""
		local enumTypeOk, enumTypeText = pcall(function()
			return tostring(value.EnumType)
		end)
		if enumTypeOk and typeof(enumTypeText) == "string" then
			enumTypeName = string.gsub(enumTypeText, "^Enum%.", "")
		end

		local enumItemName = ""
		local enumNameOk, enumNameValue = pcall(function()
			return value.Name
		end)
		if enumNameOk and typeof(enumNameValue) == "string" then
			enumItemName = enumNameValue
		end

		return {
			["$type"] = "Enum",
			enumType = enumTypeName,
			name = enumItemName,
			value = enumItemName,
		}
	end

	if valueType == "Font" then
		local family = ""
		pcall(function()
			family = tostring(value.Family)
		end)

		local weightName = "Regular"
		pcall(function()
			weightName = value.Weight.Name
		end)

		local styleName = "Normal"
		pcall(function()
			styleName = value.Style.Name
		end)

		return {
			["$type"] = "Font",
			family = family,
			weight = weightName,
			style = styleName,
		}
	end

	if valueType == "NumberRange" then
		return {
			["$type"] = "NumberRange",
			min = value.Min,
			max = value.Max,
		}
	end

	if valueType == "NumberSequence" then
		local keypoints = {}
		for _, point in ipairs(value.Keypoints) do
			table.insert(keypoints, {
				time = point.Time,
				value = point.Value,
				envelope = point.Envelope,
			})
		end
		return {
			["$type"] = "NumberSequence",
			keypoints = keypoints,
		}
	end

	if valueType == "ColorSequence" then
		local keypoints = {}
		for _, point in ipairs(value.Keypoints) do
			table.insert(keypoints, {
				time = point.Time,
				value = toColor3Table(point.Value),
			})
		end
		return {
			["$type"] = "ColorSequence",
			keypoints = keypoints,
		}
	end

	if valueType == "CFrame" then
		local components = { value:GetComponents() }
		return {
			["$type"] = "CFrame",
			components = components,
		}
	end

	return nil
end

local function resolveEnumItem(enumTypeValue : any, enumItemValue : any)
	local enumTypeName = ""
	if enumTypeValue ~= nil then
		enumTypeName = string.gsub(tostring(enumTypeValue), "^Enum%.", "")
	end

	local enumItemName = ""
	if enumItemValue ~= nil then
		enumItemName = tostring(enumItemValue)
	end

	local fullTypeName, fullItemName = string.match(enumItemName, "^Enum%.([^%.]+)%.(.+)$")
	if fullTypeName and fullItemName then
		enumTypeName = fullTypeName
		enumItemName = fullItemName
	end

	fullTypeName, fullItemName = string.match(enumTypeName, "^([^%.]+)%.(.+)$")
	if enumItemName == "" and fullTypeName and fullItemName then
		enumTypeName = fullTypeName
		enumItemName = fullItemName
	end

	if enumTypeName == "" or enumItemName == "" then
		return nil
	end

	local enumType = nil
	local enumTypeOk, resolvedEnumType = pcall(function()
		return Enum[enumTypeName]
	end)
	if enumTypeOk then
		enumType = resolvedEnumType
	end

	if enumType then
		local enumItemOk, enumItem = pcall(function()
			return enumType[enumItemName]
		end)
		if enumItemOk and enumItem then
			return enumItem
		end
	end
	return nil
end

local function resolveFullEnumPath(value : string)
	local enumTypeName, enumItemName = string.match(value, "^Enum%.([^%.]+)%.(.+)$")
	if not enumTypeName or not enumItemName then
		return nil
	end
	return resolveEnumItem(enumTypeName, enumItemName)
end

local function deserializeEnumTable(raw : {[any]: any})
	local enumTypeValue = raw.enumType
		or raw.EnumType
		or raw.enumTypeName
		or raw.EnumTypeName
		or raw.typeName
		or raw.TypeName
	local enumItemValue = raw.name
		or raw.Name
		or raw.value
		or raw.Value
		or raw.enumItem
		or raw.EnumItem
		or raw.enumItemName
		or raw.EnumItemName

	return resolveEnumItem(enumTypeValue, enumItemValue)
end

local function parseEnum(enumTypeValue : any, enumItemValue : any?)
	if typeof(enumItemValue) ~= "nil" then
		return resolveEnumItem(enumTypeValue, enumItemValue)
	end
	if typeof(enumTypeValue) == "string" then
		return resolveFullEnumPath(enumTypeValue)
	end
	if typeof(enumTypeValue) == "table" then
		return resolveEnumItem(enumTypeValue[1], enumTypeValue[2]) or deserializeEnumTable(enumTypeValue)
	end
	return nil
end

local function getShorthandTypedValue(raw : {[any]: any})
	local shorthandKeys = {
		"UDim2",
		"UDim",
		"Vector2",
		"Color3",
		"Enum",
	}
	for _, key in ipairs(shorthandKeys) do
		if raw[key] ~= nil then
			return key, raw[key]
		end
	end
	return nil, nil
end

local function getSerializedTypeName(raw : any)
	if typeof(raw) ~= "table" then
		return nil
	end

	local shorthandType = getShorthandTypedValue(raw)
	if typeof(shorthandType) == "string" then
		return shorthandType
	end

	local taggedType = raw["$type"]
	if typeof(taggedType) ~= "string" then
		taggedType = raw.type
	end
	if typeof(taggedType) == "string" then
		return taggedType
	end
	return nil
end

local function deserializeValue(raw : any)
	if typeof(raw) == "string" then
		return resolveFullEnumPath(raw) or raw
	end

	if typeof(raw) ~= "table" then
		return raw
	end

	local shorthandType, shorthandValue = getShorthandTypedValue(raw)
	if shorthandType == "Color3" then
		return parseColor3(shorthandValue)
	end
	if shorthandType == "UDim" then
		return parseUDim(shorthandValue)
	end
	if shorthandType == "UDim2" then
		return parseUDim2(shorthandValue)
	end
	if shorthandType == "Vector2" then
		return parseVector2(shorthandValue)
	end
	if shorthandType == "Enum" then
		return parseEnum(shorthandValue)
	end

	local taggedType = getSerializedTypeName(raw)
	if typeof(taggedType) ~= "string" then
		local enumItem = deserializeEnumTable(raw)
		if enumItem then
			return enumItem
		end
		return raw
	end

	if taggedType == "Color3" then
		return Color3.fromRGB(
			roundColorChannel(asNumber(raw.r, 0)),
			roundColorChannel(asNumber(raw.g, 0)),
			roundColorChannel(asNumber(raw.b, 0))
		)
	end

	if taggedType == "BrickColor" then
		local ok, color = pcall(function()
			return BrickColor.new(tostring(raw.name or "Medium stone grey"))
		end)
		if ok then
			return color
		end
		return BrickColor.new("Medium stone grey")
	end

	if taggedType == "UDim" then
		return UDim.new(asNumber(raw.scale, 0), asNumber(raw.offset, 0))
	end

	if taggedType == "UDim2" then
		return UDim2.new(
			asNumber(raw.xScale, 0),
			asNumber(raw.xOffset, 0),
			asNumber(raw.yScale, 0),
			asNumber(raw.yOffset, 0)
		)
	end

	if taggedType == "Vector2" then
		return Vector2.new(asNumber(raw.x, 0), asNumber(raw.y, 0))
	end

	if taggedType == "Vector3" then
		return Vector3.new(asNumber(raw.x, 0), asNumber(raw.y, 0), asNumber(raw.z, 0))
	end

	if taggedType == "Rect" then
		return Rect.new(
			asNumber(raw.minX, 0),
			asNumber(raw.minY, 0),
			asNumber(raw.maxX, 0),
			asNumber(raw.maxY, 0)
		)
	end

	if taggedType == "Enum" then
		return deserializeEnumTable(raw)
	end
	if taggedType == "Font" then
		local family = tostring(raw.family or "rbxasset://fonts/families/SourceSansPro.json")
		local weight = Enum.FontWeight.Regular
		local style = Enum.FontStyle.Normal

		local weightName = tostring(raw.weight or "Regular")
		if Enum.FontWeight[weightName] then
			weight = Enum.FontWeight[weightName]
		end

		local styleName = tostring(raw.style or "Normal")
		if Enum.FontStyle[styleName] then
			style = Enum.FontStyle[styleName]
		end

		local ok, font = pcall(function()
			return Font.new(family, weight, style)
		end)
		if ok then
			return font
		end
		return nil
	end

	if taggedType == "NumberRange" then
		return NumberRange.new(asNumber(raw.min, 0), asNumber(raw.max, 0))
	end

	if taggedType == "NumberSequence" then
		local points = {}
		for _, point in ipairs(raw.keypoints or {}) do
			table.insert(
				points,
				NumberSequenceKeypoint.new(
					asNumber(point.time, 0),
					asNumber(point.value, 0),
					asNumber(point.envelope, 0)
				)
			)
		end
		if #points == 0 then
			return NumberSequence.new(0)
		end
		return NumberSequence.new(points)
	end

	if taggedType == "ColorSequence" then
		local points = {}
		for _, point in ipairs(raw.keypoints or {}) do
			table.insert(
				points,
				ColorSequenceKeypoint.new(
					asNumber(point.time, 0),
					deserializeValue(point.value) or Color3.new(1, 1, 1)
				)
			)
		end
		if #points == 0 then
			return ColorSequence.new(Color3.new(1, 1, 1))
		end
		return ColorSequence.new(points)
	end

	if taggedType == "CFrame" then
		local components = raw.components
		if typeof(components) == "table" and #components == 12 then
			local ok, cf = pcall(function()
				return CFrame.new(table.unpack(components))
			end)
			if ok then
				return cf
			end
		end
		return nil
	end

	return nil
end

local function parseRobloxPath(pathValue : string)
	local segments = splitByDot(pathValue)
	if #segments < 2 or segments[1] ~= "game" then
		return nil, nil, false, "Invalid Roblox path: " .. tostring(pathValue)
	end

	local service = getServiceByName(segments[2])
	if not service then
		return nil, nil, false, "Service not found: " .. tostring(segments[2])
	end

	if #segments == 2 then
		return nil, service, true, nil
	end

	local current = service
	for index = 3, #segments - 1 do
		local childName = segments[index]
		local child = current:FindFirstChild(childName)
		if not child then
			return nil, nil, false, "Parent not found for path: " .. tostring(pathValue)
		end
		current = child
	end

	return current, segments[#segments], false, nil
end

local function getPathDepth(pathValue : any)
	if typeof(pathValue) ~= "string" then
		return 9999
	end
	local _, dotCount = string.gsub(pathValue, "%.", "")
	return dotCount + 1
end

local function getChangeDepth(change : {[string]: any}, fallbackPath : any)
	local localPath = change.local_path
	if typeof(localPath) == "string" and localPath ~= "" then
		local _, slashCount = string.gsub(localPath, "/", "")
		return slashCount + 1
	end

	return getPathDepth(fallbackPath)
end

local function serialiseAttributes(instance : Instance)
	local out = {}
	for key, value in pairs(instance:GetAttributes()) do
		if typeof(key) == "string" then
			local lowered = string.lower(key)
			if not isManagedAttributeName(key)
				and string.sub(key, 1, 4) ~= "RBX_"
				and string.sub(lowered, 1, 4) ~= "rbx_"
			then
				local encoded = serializeValue(value)
				if encoded ~= nil then
					out[key] = encoded
				end
			end
		end
	end
	return out
end

local function serialiseTags(instance : Instance)
	local tags = collectionService:GetTags(instance)
	table.sort(tags)
	return tags
end

local function buildExtraAllowedSet(self, className : string)
	local result = {}
	local map = self.extraAllowedProperties or {}
	local wildcards = map["*"]
	if typeof(wildcards) == "table" then
		for _, propertyName in ipairs(wildcards) do
			if typeof(propertyName) == "string" and propertyName ~= "" then
				result[propertyName] = true
			end
		end
	end

	local classList = map[className]
	if typeof(classList) == "table" then
		for _, propertyName in ipairs(classList) do
			if typeof(propertyName) == "string" and propertyName ~= "" then
				result[propertyName] = true
			end
		end
	end

	return result
end

local function buildPropertyCandidateList(self, className : string)
	local candidates = {}
	local seen = {}

	local function push(propertyName : string)
		if propertyName == "" or seen[propertyName] then
			return
		end
		seen[propertyName] = true
		table.insert(candidates, propertyName)
	end

	local classWhitelist = nil
	if TypeList.getClassPropertyWhitelist then
		classWhitelist = TypeList.getClassPropertyWhitelist(className)
	end
	if classWhitelist == nil and TypeList.CLASS_PROPERTY_WHITELIST then
		classWhitelist = TypeList.CLASS_PROPERTY_WHITELIST[className]
	end

	if typeof(classWhitelist) == "table" then
		for _, propertyName in ipairs(classWhitelist) do
			if typeof(propertyName) == "string" then
				push(propertyName)
			end
		end
	end

	local strictMode = self.strictPropertyWhitelist == true
	if not strictMode then
		for _, propertyName in ipairs(TypeList.UI_PROPERTY_CANDIDATES or {}) do
			if typeof(propertyName) == "string" then
				push(propertyName)
			end
		end
	end

	local extraAllowed = buildExtraAllowedSet(self, className)
	for propertyName, _ in pairs(extraAllowed) do
		push(propertyName)
	end

	return candidates, classWhitelist, extraAllowed
end

local function serialiseUiProperties(self, instance : Instance)
	local result = {}
	local className = instance.ClassName
	local candidates = buildPropertyCandidateList(self, className)
	for _, propertyName in ipairs(candidates) do
		if propertyName ~= "Name" then
			if isScriptClass(className) and propertyName == "Source" then
				continue
			end
			local ok, value = pcall(function()
				return (instance :: any)[propertyName]
			end)
			if ok then
				local encoded = serializeValue(value)
				if encoded ~= nil then
					result[propertyName] = encoded
				end
			end
		end
	end
	return result
end

local function buildUiFolderPath(instance : Instance)
	local segments = {}
	local current = instance
	while current and current ~= game do
		if current.Parent == game then
			table.insert(segments, 1, encodeLocalSegment(current.Name))
		else
			table.insert(segments, 1, encodeLocalSegment(current.Name) .. "." .. current.ClassName)
		end
		current = current.Parent
	end
	if #segments == 0 then
		return nil
	end
	return table.concat(segments, "/")
end

local function getEntityKind(change : {[string]: any})
	local entity = change.entity
	if typeof(entity) == "string" and entity ~= "" then
		if TypeList.normaliseEntityKind then
			return TypeList.normaliseEntityKind(entity)
		end
		return entity
	end
	if typeof(change.payload) == "table" then
		return INSTANCE_KINDS.UIInstance
	end
	return INSTANCE_KINDS.Script
end

local SCRIPT_CLASS_TO_SOURCE_SUFFIX = {
	Script = ".server.luau",
	LocalScript = ".client.luau",
	ModuleScript = ".module.luau",
}

local function buildScriptSourceFilename(instance : Instance)
	local suffix = SCRIPT_CLASS_TO_SOURCE_SUFFIX[instance.ClassName]
	if not suffix then
		return nil
	end

	return encodeLocalSegment(instance.Name) .. suffix
end

local function isPropertySyncPath(pathValue : string)
	if typeof(pathValue) ~= "string" then
		return false
	end

	local roots = TypeList.PROPERTY_SYNC_ROOTS or {
		"game.StarterGui",
		"game.Workspace",
		"game.Lighting",
	}
	for _, rootPath in ipairs(roots) do
		if pathValue == rootPath then
			return true
		end
		if string.sub(pathValue, 1, #rootPath + 1) == rootPath .. "." then
			return true
		end
	end
	return false
end

local function isRemoteScopePath(pathValue : string)
	if typeof(pathValue) ~= "string" then
		return false
	end

	for serviceName, _ in pairs(REMOTE_SYNC_ROOT_SERVICES) do
		local rootPath = "game." .. serviceName
		if pathValue == rootPath then
			return true
		end
		if string.sub(pathValue, 1, #rootPath + 1) == rootPath .. "." then
			return true
		end
	end

	return false
end

local function isRemoteOrContainerClass(className : string)
	return REMOTE_SYNC_CLASSES[className] == true or REMOTE_SYNC_CONTAINER_CLASSES[className] == true
end

local function buildManagedIndexes(managedRoots : {string})
	local byStableId = {}
	local byLocalPath = {}
	for _, rootPath in ipairs(managedRoots) do
		local rootInstance = resolvePath(rootPath)
		if rootInstance then
			local instances = rootInstance:GetDescendants()
			table.insert(instances, rootInstance)
			for _, instance in ipairs(instances) do
				if isManagedInstance(instance) then
					local stableId = instance:GetAttribute(TypeList.MANAGED_ATTRIBUTES.StableId)
					if typeof(stableId) == "string" and stableId ~= "" and not byStableId[stableId] then
						byStableId[stableId] = instance
					end
					local localPath = instance:GetAttribute(TypeList.MANAGED_ATTRIBUTES.LocalPath)
					if typeof(localPath) == "string" and localPath ~= "" and not byLocalPath[localPath] then
						byLocalPath[localPath] = instance
					end
				end
			end
		end
	end
	return {
		byStableId = byStableId,
		byLocalPath = byLocalPath,
	}
end

function SyncAPI:collectStudioSnapshot()
	local files = {}
	local skippedCount = 0
	local sourceReadFailedCount = 0
	local skipBreakdown = {}
	local skipSamples = {}
	local skipDetails = {}
	local seenLocalPath = {}
	local seenPropertyStableIds = {}
	local scriptCandidates = {}
	local scriptLookup = {}

	local function noteSkip(reason : string, sample : string?)
		skippedCount += 1
		skipBreakdown[reason] = (skipBreakdown[reason] or 0) + 1
		if typeof(sample) == "string" and sample ~= "" and #skipDetails < 30 then
			table.insert(skipDetails, reason .. ": " .. sample)
		end
		if typeof(sample) == "string" and sample ~= "" and #skipSamples < 6 then
			table.insert(skipSamples, reason .. ": " .. sample)
		end
	end

	local function registerLocalPath(localPath : string, sourcePath : string)
		local existingSource = seenLocalPath[localPath]
		if typeof(existingSource) == "string" and existingSource ~= "" and existingSource ~= sourcePath then
			return false, existingSource
		end
		seenLocalPath[localPath] = sourcePath
		return true, nil
	end

	for _, rootPath in ipairs(self.managedRoots) do
		local rootInstance = resolvePath(rootPath)
		if rootInstance then
			local candidates = rootInstance:GetDescendants()
			if isScriptClass(rootInstance.ClassName) then
				table.insert(candidates, rootInstance)
			end

			for _, candidate in ipairs(candidates) do
				if isScriptClass(candidate.ClassName) and not scriptLookup[candidate] then
					scriptLookup[candidate] = true
					table.insert(scriptCandidates, candidate)
				end
			end
		end
	end

	for _, candidate in ipairs(scriptCandidates) do
		local className = candidate.ClassName
		local sourceFilename = buildScriptSourceFilename(candidate)
		if not sourceFilename then
			noteSkip("script_no_suffix", candidate:GetFullName())
			continue
		end

		local gamePath = buildGamePath(candidate)
		if isIgnoredPath(gamePath, self.ignoredRbxPaths) then
			noteSkip("script_ignored_path", gamePath)
			continue
		end

		local folderPath = buildUiFolderPath(candidate)
		local localPath = if folderPath then folderPath .. "/" .. sourceFilename else nil
		if not localPath then
			noteSkip("script_duplicate_or_invalid_local_path", gamePath)
			continue
		end
		local canRegister, conflictSource = registerLocalPath(localPath, gamePath)
		if not canRegister then
			noteSkip(
				"local_path_collision",
				localPath .. " <- " .. gamePath .. " conflicts with " .. tostring(conflictSource)
			)
			continue
		end

		local source = readStudioScriptSource(candidate)
		if typeof(source) ~= "string" then
			noteSkip("script_source_read_failed", candidate:GetFullName())
			sourceReadFailedCount += 1
			continue
		end

		table.insert(files, {
			entity = INSTANCE_KINDS.Script,
			local_path = localPath,
			rbx_path = gamePath,
			class_name = className,
			source = source,
		})
	end

	local propertyRoots = TypeList.PROPERTY_SYNC_ROOTS or {
		"game.StarterGui",
		"game.Workspace",
		"game.Lighting",
	}

	local visitedRoots = {}
	for _, rootPath in ipairs(propertyRoots) do
		if visitedRoots[rootPath] then
			continue
		end
		visitedRoots[rootPath] = true

		local rootInstance = resolvePath(rootPath)
		if not rootInstance then
			continue
		end

		local rootServiceName = rootInstance.Name

		local function exportPropertyNode(instance : Instance)
			if not shouldTrackPropertyInstance(instance) then
				return
			end

			local gamePath = buildGamePath(instance)
			if isIgnoredPath(gamePath, self.ignoredRbxPaths) then
				noteSkip("properties_ignored_path", gamePath)
				return
			end

			local folderPath = buildUiFolderPath(instance)
			if not folderPath then
				noteSkip("properties_invalid_folder_path", gamePath)
				return
			end

			local localPath = folderPath .. "/" .. UI_PROPERTIES_FILENAME
			local canRegister, conflictSource = registerLocalPath(localPath, gamePath)
			if not canRegister then
				noteSkip(
					"local_path_collision",
					localPath .. " <- " .. gamePath .. " conflicts with " .. tostring(conflictSource)
				)
				return
			end

			local stableId = ensureStableId(instance)
			if seenPropertyStableIds[stableId] and seenPropertyStableIds[stableId] ~= instance then
				repeat
					stableId = httpService:GenerateGUID(false)
				until not seenPropertyStableIds[stableId]
				instance:SetAttribute(TypeList.MANAGED_ATTRIBUTES.StableId, stableId)
			end
			seenPropertyStableIds[stableId] = instance

			local payload = {
				id = stableId,
				className = instance.ClassName,
				name = instance.Name,
				properties = serialiseUiProperties(self, instance),
				attributes = serialiseAttributes(instance),
				tags = serialiseTags(instance),
			}
			local ok, source = pcall(function()
				return httpService:JSONEncode(payload)
			end)
			if not ok then
				noteSkip("properties_json_encode_failed", gamePath)
				return
			end

			table.insert(files, {
				entity = INSTANCE_KINDS.UIInstance,
				local_path = localPath,
				rbx_path = gamePath,
				class_name = instance.ClassName,
				stable_id = stableId,
				source = source,
			})
		end

		local function visitFromRoot(instance : Instance)
			exportPropertyNode(instance)
			for _, child in ipairs(instance:GetChildren()) do
				if shouldTraversePropertyChild(rootServiceName, child) then
					visitFromRoot(child)
				end
			end
		end

		if rootServiceName == "StarterGui" then
			for _, child in ipairs(rootInstance:GetChildren()) do
				if shouldTraversePropertyChild(rootServiceName, child) then
					visitFromRoot(child)
				end
			end
		else
			visitFromRoot(rootInstance)
		end
	end

	return files, skippedCount, sourceReadFailedCount, {
		breakdown = skipBreakdown,
		samples = skipSamples,
		details = skipDetails,
	}
end

function SyncAPI:pushStudioSnapshot(bootstrapMode : string?)
	local files, skippedCount, sourceReadFailedCount, skipInfo = self:collectStudioSnapshot()
	local mode = bootstrapMode == "merge" and "merge" or "replace"

	local skipSummaryText = ""
	if typeof(skipInfo) == "table" and typeof(skipInfo.breakdown) == "table" then
		local parts = {}
		for reason, count in pairs(skipInfo.breakdown) do
			table.insert(parts, tostring(reason) .. "=" .. tostring(count))
		end
		table.sort(parts)
		if #parts > 0 then
			skipSummaryText = " [" .. table.concat(parts, ", ") .. "]"
		end
	end

	local payload = {
		client_id = self.clientId,
		session_id = self.sessionId,
		mode = mode,
		files = files,
	}

	local response, requestError = self:requestJson("POST", TypeList.ENDPOINTS.Bootstrap, payload, nil)
	if not response then
		self:setDebugError("Bootstrap request gagal: " .. tostring(requestError))
		return false, requestError
	end

	if response.status ~= "ok" then
		self:setDebugError("Bootstrap ditolak: " .. tostring(response.status))
		return false, "Bootstrap rejected"
	end

	local serverErrors = response.errors
	if typeof(serverErrors) == "table" and #serverErrors > 0 then
		warn("Bootstrap server errors: " .. table.concat(serverErrors, " | "))
	end

	local pushedCount = tonumber(response.written_count) or #files
	local actionLabel = mode == "merge" and "Pull Studio" or "Bootstrap"
	local mergeSummaryText = ""
	if mode == "merge" then
		mergeSummaryText = " [new="
			.. tostring(tonumber(response.new_count) or 0)
			.. ", updated="
			.. tostring(tonumber(response.updated_count) or 0)
			.. ", unchanged="
			.. tostring(tonumber(response.unchanged_count) or 0)
			.. "]"
	end
	self.updateStatus(
		actionLabel
			.. ": "
			.. tostring(pushedCount)
			.. " ditulis, "
			.. tostring(skippedCount)
			.. " dilewati (readFail="
			.. tostring(sourceReadFailedCount)
			.. ")"
			.. mergeSummaryText
			.. skipSummaryText,
		false
	)
	self:appendDebugEvent(
		actionLabel
			.. " sukses, written="
			.. tostring(tonumber(response.written_count) or #files)
			.. ", skipped="
			.. tostring(skippedCount)
			.. mergeSummaryText
			.. skipSummaryText,
		false
	)
	if skippedCount > 0 and typeof(skipInfo) == "table" and typeof(skipInfo.details) == "table" then
		local warnLimit = math.min(#skipInfo.details, 12)
		for index = 1, warnLimit do
			local detail = tostring(skipInfo.details[index])
			if string.sub(detail, 1, 19) == "script_ignored_path"
				or string.sub(detail, 1, 23) == "properties_ignored_path"
			then
				print("[RiftSync][bootstrap skip] " .. detail)
			else
				warn("[RiftSync][bootstrap skip] " .. detail)
			end
		end
		if #skipInfo.details > warnLimit then
			warn("[RiftSync][bootstrap skip] +" .. tostring(#skipInfo.details - warnLimit) .. " item lain")
		end
	end
	if typeof(skipInfo) == "table" and typeof(skipInfo.samples) == "table" and #skipInfo.samples > 0 then
		self:appendDebugEvent("Skip sample: " .. table.concat(skipInfo.samples, " | "), false)
	end
	self:pushDebugUpdate()
	return true, response
end

local function parseUiPayload(change : {[string]: any})
	local payload = change.payload
	if typeof(payload) ~= "table" then
		return nil, "Invalid UI payload"
	end

	local className = tostring(payload.className or change.class_name or "")
	local name = tostring(payload.name or "")
	local stableId = tostring(payload.id or payload["$id"] or payload.syncId or change.stable_id or "")
	if className == "" then
		return nil, "Missing className in UI payload"
	end
	if name == "" then
		name = tostring(change.name or "")
	end

	return {
		className = className,
		name = name,
		stableId = stableId,
		properties = typeof(payload.properties) == "table" and payload.properties or {},
		attributes = typeof(payload.attributes) == "table" and payload.attributes or {},
		tags = typeof(payload.tags) == "table" and payload.tags or {},
		raw = payload,
	}
end

local function removeFromIndexesRecursive(indexes : {[string]: any}, instance : Instance)
	local descendants = instance:GetDescendants()
	table.insert(descendants, instance)
	for _, descendant in ipairs(descendants) do
		local stableId = descendant:GetAttribute(TypeList.MANAGED_ATTRIBUTES.StableId)
		if typeof(stableId) == "string" and stableId ~= "" then
			indexes.byStableId[stableId] = nil
		end
		local localPath = descendant:GetAttribute(TypeList.MANAGED_ATTRIBUTES.LocalPath)
		if typeof(localPath) == "string" and localPath ~= "" then
			indexes.byLocalPath[localPath] = nil
		end
	end
end

local function findExistingManagedTarget(change : {[string]: any}, indexes : {[string]: any})
	local localPath = change.local_path
	if typeof(localPath) == "string" and localPath ~= "" then
		local byLocalPath = indexes.byLocalPath[localPath]
		if byLocalPath then
			return byLocalPath
		end
	end

	local pathCandidates = {
		change.new_rbx_path,
		change.rbx_path,
		change.old_rbx_path,
	}
	for _, pathValue in ipairs(pathCandidates) do
		if typeof(pathValue) == "string" then
			local existing = resolvePath(pathValue)
			if existing then
				return existing
			end
		end
	end

	local stableId = change.stable_id
	if typeof(stableId) == "string" and stableId ~= "" then
		local byId = indexes.byStableId[stableId]
		if byId then
			local existingLocalPath = byId:GetAttribute(TypeList.MANAGED_ATTRIBUTES.LocalPath)
			if tostring(change.op) == "rename" or existingLocalPath == localPath then
				return byId
			end
		end
	end

	return nil
end

local function isRestrictedAttributeName(attributeName : string)
	if isManagedAttributeName(attributeName) then
		return true
	end

	local lowered = string.lower(attributeName)
	return string.sub(attributeName, 1, 4) == "RBX_" or string.sub(lowered, 1, 4) == "rbx_"
end

local function isPermissionRestrictedWriteError(message : string)
	local lowered = string.lower(message)
	return string.find(lowered, "lacking capability robloxscript", 1, true) ~= nil
		or string.find(lowered, "corescript permission required", 1, true) ~= nil
		or string.find(lowered, "cannot write", 1, true) ~= nil
		or string.find(lowered, "read only", 1, true) ~= nil
		or string.find(lowered, "permission", 1, true) ~= nil
		or string.find(lowered, "security", 1, true) ~= nil
end

local function isKnownRestrictedProperty(className : string, propertyName : string)
	if className == "Lighting" and propertyName == "PrioritizeLightingQuality" then
		return true
	end
	return false
end

local PROPERTY_ENUM_TYPES = {
	ApplyStrokeMode = "ApplyStrokeMode",
	AspectType = "AspectType",
	AutomaticCanvasSize = "AutomaticSize",
	AutomaticSize = "AutomaticSize",
	BorderMode = "BorderMode",
	DominantAxis = "DominantAxis",
	FillDirection = "FillDirection",
	Font = "Font",
	HorizontalAlignment = "HorizontalAlignment",
	LineJoinMode = "LineJoinMode",
	ScaleType = "ScaleType",
	ScrollingDirection = "ScrollingDirection",
	SizeConstraint = "SizeConstraint",
	SortOrder = "SortOrder",
	StartCorner = "StartCorner",
	TextDirection = "TextDirection",
	TextTruncate = "TextTruncate",
	TextXAlignment = "TextXAlignment",
	TextYAlignment = "TextYAlignment",
	VerticalAlignment = "VerticalAlignment",
	ZIndexBehavior = "ZIndexBehavior",
}

local function parsePropertyValue(propertyName : string, encodedValue : any)
	local decoded = deserializeValue(encodedValue)
	if typeof(decoded) == "string" then
		local enumTypeName = PROPERTY_ENUM_TYPES[propertyName]
		if enumTypeName then
			return resolveEnumItem(enumTypeName, decoded) or decoded
		end
	end
	return decoded
end

local function applyUiProperties(self, instance : Instance, payload : {[string]: any})
	local errors = {}
	local notices = {}
	local className = tostring(payload.className or instance.ClassName)
	local classWhitelist = nil
	local extraAllowed = {}
	if self then
		local _, whitelist, extra = buildPropertyCandidateList(self, className)
		classWhitelist = whitelist
		extraAllowed = extra
	else
		if TypeList.getClassPropertyWhitelist then
			classWhitelist = TypeList.getClassPropertyWhitelist(className)
		end
		if classWhitelist == nil and TypeList.CLASS_PROPERTY_WHITELIST then
			classWhitelist = TypeList.CLASS_PROPERTY_WHITELIST[className]
		end
	end
	local allowedSet = nil
	if typeof(classWhitelist) == "table" then
		allowedSet = {}
		for _, propertyName in ipairs(classWhitelist) do
			allowedSet[propertyName] = true
		end
		for propertyName, _ in pairs(extraAllowed) do
			allowedSet[propertyName] = true
		end
	end
	local strictMode = self and self.strictPropertyWhitelist == true or false

	for propertyName, encodedValue in pairs(payload.properties) do
		if propertyName ~= "Name" and propertyName ~= "ClassName" then
			if isScriptClass(className) and propertyName == "Source" then
				table.insert(
					notices,
					"Skip Source property for script node (use source file) @ " .. instance:GetFullName()
				)
				continue
			end

			if isKnownRestrictedProperty(className, propertyName) then
				table.insert(
					notices,
					"Skip protected property " .. tostring(propertyName) .. " @ " .. instance:GetFullName()
				)
				continue
			end

			if allowedSet and not allowedSet[propertyName] then
				if strictMode then
					table.insert(
						notices,
						"Skip non-whitelist property " .. tostring(propertyName) .. " @ " .. instance:GetFullName()
					)
					continue
				end
			end

			local decoded = parsePropertyValue(propertyName, encodedValue)
			local encodedType = getSerializedTypeName(encodedValue)
			if decoded == nil and encodedType then
				table.insert(
					errors,
					"Unsupported or invalid property type "
						.. tostring(encodedType)
						.. " for "
						.. tostring(propertyName)
						.. " @ "
						.. instance:GetFullName()
				)
				continue
			end
			local ok, err = pcall(function()
				(instance :: any)[propertyName] = decoded
			end)
			if not ok then
				local message = tostring(err)
				if isPermissionRestrictedWriteError(message) then
					table.insert(
						notices,
						"Skip protected property " .. tostring(propertyName) .. " @ " .. instance:GetFullName()
					)
				else
					table.insert(
						errors,
						"Property gagal " .. tostring(propertyName) .. " @ " .. instance:GetFullName() .. " : " .. message
					)
				end
			elseif allowedSet and not allowedSet[propertyName] and not strictMode then
				table.insert(
					notices,
					"Soft-allowed property " .. tostring(propertyName) .. " @ " .. instance:GetFullName()
				)
			end
		end
	end
	return errors, notices
end

local function applyUiAttributesAndTags(instance : Instance, payload : {[string]: any})
	local errors = {}
	local notices = {}
	local attributes = payload.attributes or {}
	local currentAttributes = instance:GetAttributes()

	for key, _ in pairs(currentAttributes) do
		if attributes[key] == nil then
			if isManagedAttributeName(key) then
				continue
			end

			if typeof(key) == "string" and isRestrictedAttributeName(key) then
				table.insert(notices, "Skip protected attribute " .. tostring(key) .. " @ " .. instance:GetFullName())
				continue
			end

			local ok, err = pcall(function()
				instance:SetAttribute(key, nil)
			end)
			if not ok then
				local message = tostring(err)
				if isPermissionRestrictedWriteError(message) then
					table.insert(notices, "Skip protected attribute " .. tostring(key) .. " @ " .. instance:GetFullName())
				else
					table.insert(
						errors,
						"Clear attribute gagal " .. tostring(key) .. " @ " .. instance:GetFullName() .. " : " .. message
					)
				end
			end
		end
	end

	for key, encodedValue in pairs(attributes) do
		if isManagedAttributeName(key) then
			continue
		end

		if typeof(key) == "string" and isRestrictedAttributeName(key) then
			table.insert(notices, "Skip protected attribute " .. tostring(key) .. " @ " .. instance:GetFullName())
			continue
		end

		local decoded = deserializeValue(encodedValue)
		local encodedType = getSerializedTypeName(encodedValue)
		if decoded == nil and encodedType then
			table.insert(
				errors,
				"Unsupported or invalid attribute type "
					.. tostring(encodedType)
					.. " for "
					.. tostring(key)
					.. " @ "
					.. instance:GetFullName()
			)
			continue
		end

		local ok, err = pcall(function()
			instance:SetAttribute(key, decoded)
		end)
		if not ok then
			local message = tostring(err)
			if isPermissionRestrictedWriteError(message) then
				table.insert(notices, "Skip protected attribute " .. tostring(key) .. " @ " .. instance:GetFullName())
			else
				table.insert(
					errors,
					"Set attribute gagal " .. tostring(key) .. " @ " .. instance:GetFullName() .. " : " .. message
				)
			end
		end
	end

	local targetTagSet = {}
	for _, tag in ipairs(payload.tags or {}) do
		if typeof(tag) == "string" and tag ~= "" then
			targetTagSet[tag] = true
		end
	end

	local currentTags = collectionService:GetTags(instance)
	for _, tag in ipairs(currentTags) do
		if not targetTagSet[tag] then
			collectionService:RemoveTag(instance, tag)
		end
	end
	for tag, _ in pairs(targetTagSet) do
		if not collectionService:HasTag(instance, tag) then
			collectionService:AddTag(instance, tag)
		end
	end

	return errors, notices
end

local function markManaged(instance : Instance, syncId : string?)
	markManagedInstance(instance, nil, nil, syncId, INSTANCE_KINDS.UIInstance)
end

local function applyProperties(instance : Instance, properties : {[string]: any})
	return applyUiProperties(nil, instance, {
		className = instance.ClassName,
		properties = properties,
	})
end

local function createOrUpdateInstance(parent : Instance, config : {[string]: any})
	local className = tostring(config.className or config["$className"] or "")
	local name = tostring(config.name or config.Name or "")
	local syncId = tostring(config.syncId or config.id or config["$id"] or "")
	if className == "" then
		return nil, "Missing className"
	end
	if name == "" then
		name = className
	end

	local existing = nil
	if syncId ~= "" then
		for _, child in ipairs(parent:GetChildren()) do
			if child:GetAttribute("SyncId") == syncId or child:GetAttribute(TypeList.MANAGED_ATTRIBUTES.StableId) == syncId then
				existing = child
				break
			end
		end
	end
	if not existing then
		local named = parent:FindFirstChild(name)
		if named and named:IsA(className) then
			existing = named
		elseif named and isManagedInstance(named) then
			named:Destroy()
		elseif named then
			return nil, "Refused replacing unmanaged instance: " .. named:GetFullName()
		end
	end
	if not existing then
		local ok, createdOrError = pcall(function()
			return Instance.new(className)
		end)
		if not ok then
			return nil, "Unsupported class: " .. className
		end
		existing = createdOrError
	end

	existing.Name = name
	existing.Parent = parent
	markManaged(existing, syncId)
	local propertyErrors = applyProperties(existing, config.properties or config)
	if #propertyErrors > 0 then
		return existing, table.concat(propertyErrors, " | ")
	end
	return existing, nil
end

local function syncChildren(parent : Instance, children : {any})
	local errors = {}
	for _, childConfig in ipairs(children or {}) do
		if typeof(childConfig) == "table" then
			local child, errorMessage = createOrUpdateInstance(parent, childConfig)
			if errorMessage then
				table.insert(errors, errorMessage)
			end
			if child and typeof(childConfig.children) == "table" then
				local childErrors = syncChildren(child, childConfig.children)
				for _, childError in ipairs(childErrors) do
					table.insert(errors, childError)
				end
			end
		end
	end
	return errors
end

local function prepareUiUpsert(self, change : {[string]: any}, revision : number, indexes : {[string]: any})
	local function isProtectedByPath(instance : Instance)
		if instance:IsA("Terrain") then
			return true
		end
		if instance.Parent ~= game then
			return false
		end
		local rootServiceClass = TypeList.ROOT_SERVICE_CLASS
		if typeof(rootServiceClass) ~= "table" then
			return false
		end
		return rootServiceClass[instance.Name] ~= nil
	end

	local function findBackupName(parent : Instance, baseName : string)
		local prefix = baseName .. "__rbxlsync_unmanaged_backup"
		local candidate = prefix
		local suffix = 1
		while parent:FindFirstChild(candidate) ~= nil do
			candidate = prefix .. "_" .. tostring(suffix)
			suffix += 1
			if suffix > 200 then
				candidate = prefix .. "_" .. string.sub(httpService:GenerateGUID(false), 1, 8)
				if parent:FindFirstChild(candidate) == nil then
					break
				end
			end
		end
		return candidate
	end

	local function moveUnmanagedAside(parent : Instance, existing : Instance, targetName : string)
		if isProtectedByPath(existing) then
			return false, "Refused replacing protected instance: " .. existing:GetFullName()
		end
		local fromPath = existing:GetFullName()
		local backupName = findBackupName(parent, targetName)
		local moved, moveError = pcall(function()
			existing.Name = backupName
		end)
		if not moved then
			return false, "Failed moving unmanaged conflict at " .. fromPath .. " : " .. tostring(moveError)
		end
		return true
	end

	local payload, payloadError = parseUiPayload(change)
	if not payload then
		return false, payloadError
	end

	local targetPath = tostring(change.new_rbx_path or change.rbx_path or "")
	if targetPath == "" then
		return false, "UI target path kosong"
	end
	if isIgnoredPath(targetPath, self.ignoredRbxPaths) then
		return true, ""
	end

	local parent, targetNameOrService, isRootService, parentError = parseRobloxPath(targetPath)
	if parentError then
		if isRemoteScopePath(targetPath) and isRemoteOrContainerClass(payload.className) then
			local ensuredParent, ensuredName, ensureError = ensureParentPath(targetPath)
			if not ensuredParent then
				return false, ensureError or parentError
			end
			parent = ensuredParent
			targetNameOrService = ensuredName
			isRootService = false
			parentError = nil
		else
			return false, parentError
		end
	end

	local existing = nil
	local rootServiceTarget = nil

	if isRootService then
		rootServiceTarget = targetNameOrService
		existing = rootServiceTarget
	else
		if payload.className == "Terrain" then
			if targetPath ~= "game.Workspace.Terrain" then
				return false, "Unsupported Terrain target path: " .. targetPath
			end
			existing = workspaceService:FindFirstChildOfClass("Terrain")
		else
		existing = findExistingManagedTarget(change, indexes)
		if existing and not isManagedInstance(existing) then
			existing = nil
		end
		end

		if existing and not existing:IsA(payload.className) then
			if isProtectedRootInstance(existing) then
				return false, "Refused replacing protected root service: " .. existing:GetFullName()
			end
			removeFromIndexesRecursive(indexes, existing)
			existing:Destroy()
			existing = nil
		end

		if not existing then
			local occupied = parent:FindFirstChild(targetNameOrService)
			if occupied and occupied:IsA(payload.className) then
				existing = occupied
			elseif occupied and isManagedInstance(occupied) then
				if isProtectedRootInstance(occupied) then
					return false, "Refused replacing protected root service: " .. occupied:GetFullName()
				end
				removeFromIndexesRecursive(indexes, occupied)
				occupied:Destroy()
				occupied = nil
			else
				if occupied then
					return false,
						"Refused replacing unmanaged instance at "
							.. occupied:GetFullName()
							.. " (wanted "
							.. tostring(payload.className)
							.. ")"
				end
			end
		end

		if not existing and payload.className ~= "Terrain" then
			local createdOk, createdOrError = pcall(function()
				return Instance.new(payload.className)
			end)
			if not createdOk then
				return false, "Unsupported class: " .. tostring(payload.className)
			end
			existing = createdOrError
		end
		if not existing then
			return false, "Missing required instance for class " .. tostring(payload.className) .. " at " .. targetPath
		end

		if not isLockedParentInstance(existing) then
			existing.Name = targetNameOrService
			existing.Parent = parent
		end
	end

	if rootServiceTarget then
		if payload.className ~= "" and not existing:IsA(payload.className) then
			return false, "Root service class mismatch: " .. existing.ClassName .. " ~= " .. tostring(payload.className)
		end
		payload.name = existing.Name
		payload.className = existing.ClassName
	end

	local stableId = payload.stableId
	if stableId == "" then
		stableId = tostring(change.stable_id or "")
	end
	local generatedStableId = false
	if stableId == "" then
		local existingStableId = existing:GetAttribute(TypeList.MANAGED_ATTRIBUTES.StableId)
		stableId = ensureStableId(existing)
		generatedStableId = typeof(existingStableId) ~= "string" or existingStableId == ""
	end

	local stableIdOwner = if stableId ~= "" then indexes.byStableId[stableId] else nil
	if stableIdOwner and stableIdOwner ~= existing and tostring(change.op) ~= "rename" then
		repeat
			stableId = httpService:GenerateGUID(false)
		until not indexes.byStableId[stableId]
		existing:SetAttribute(TypeList.MANAGED_ATTRIBUTES.StableId, stableId)
		generatedStableId = true
	else
		existing:SetAttribute(TypeList.MANAGED_ATTRIBUTES.StableId, stableId)
	end

	local previousLocalPath = existing:GetAttribute(TypeList.MANAGED_ATTRIBUTES.LocalPath)
	if typeof(previousLocalPath) == "string"
		and previousLocalPath ~= ""
		and previousLocalPath ~= change.local_path
		and indexes.byLocalPath[previousLocalPath] == existing
	then
		indexes.byLocalPath[previousLocalPath] = nil
	end

	markManagedInstance(existing, change.local_path, revision, stableId, INSTANCE_KINDS.UIInstance)
	indexes.byStableId[stableId] = existing
	if typeof(change.local_path) == "string" and change.local_path ~= "" then
		indexes.byLocalPath[change.local_path] = existing
	end

	return true, {
		instance = existing,
		payload = payload,
		stableId = stableId,
		notice = generatedStableId and ("Generated stable id for " .. existing:GetFullName()) or "",
	}
end

local function applyScriptPayload(self, scriptInstance : Instance, className : string, targetName : string, rawPayload : any)
	if typeof(rawPayload) ~= "table" then
		return true
	end
	local payload = {
		className = className,
		name = targetName,
		properties = typeof(rawPayload.properties) == "table" and rawPayload.properties or {},
		attributes = typeof(rawPayload.attributes) == "table" and rawPayload.attributes or {},
		tags = typeof(rawPayload.tags) == "table" and rawPayload.tags or {},
	}
	local propertyErrors, propertyNotices = applyUiProperties(self, scriptInstance, payload)
	local metadataErrors, metadataNotices = applyUiAttributesAndTags(scriptInstance, payload)
	if #propertyErrors > 0 or #metadataErrors > 0 then
		local merged = {}
		for _, message in ipairs(propertyErrors) do
			table.insert(merged, message)
		end
		for _, message in ipairs(metadataErrors) do
			table.insert(merged, message)
		end
		return false, table.concat(merged, " | ")
	end
	local notices = {}
	for _, message in ipairs(propertyNotices) do
		table.insert(notices, message)
	end
	for _, message in ipairs(metadataNotices) do
		table.insert(notices, message)
	end
	if #notices > 0 then
		return true, table.concat(notices, " | ")
	end
	return true
end

local function applyScriptUpsert(self, change : {[string]: any}, revision : number)
	local rbxPath = change.rbx_path
	local className = change.class_name
	local source = change.source
	local localPath = change.local_path

	if typeof(rbxPath) ~= "string" or typeof(className) ~= "string" or typeof(source) ~= "string" then
		return false, "Invalid script upsert payload"
	end
	if isIgnoredPath(rbxPath, self.ignoredRbxPaths) then
		return true
	end
	if not isScriptClass(className) then
		return false, "Unsupported script class: " .. tostring(className)
	end

	local parent, targetName, parentError = ensureParentPath(rbxPath)
	if not parent then
		return false, parentError
	end

	local scriptInstance, conflictError = replaceManagedConflict(parent, targetName, className)
	if conflictError then
		return false, conflictError
	end
	if not scriptInstance then
		scriptInstance = Instance.new(className)
		scriptInstance.Name = targetName
		scriptInstance.Parent = parent
	end

	local didUpdate = setScriptSource(scriptInstance, source)
	if not didUpdate then
		return false, "Failed to update source for " .. scriptInstance:GetFullName()
	end

	markManagedInstance(scriptInstance, localPath, revision, nil, INSTANCE_KINDS.Script)
	return applyScriptPayload(self, scriptInstance, className, targetName, change.payload)
end

local function applyScriptRename(self, change : {[string]: any}, revision : number)
	local oldPath = change.old_rbx_path
	local newPath = change.new_rbx_path
	if typeof(oldPath) ~= "string" or typeof(newPath) ~= "string" then
		return false, "Invalid script rename payload"
	end
	if isIgnoredPath(oldPath, self.ignoredRbxPaths) or isIgnoredPath(newPath, self.ignoredRbxPaths) then
		return true
	end

	local existing = resolvePath(oldPath)
	if not existing then
		if typeof(change.source) == "string" then
			return applyScriptUpsert(self, change, revision)
		end
		return false, "Rename source not found: " .. oldPath
	end
	if not isManagedInstance(existing) then
		return false, "Refused to rename unmanaged script at " .. oldPath
	end

	local newParent, newName, parentError = ensureParentPath(newPath)
	if not newParent then
		return false, parentError
	end

	local occupied = newParent:FindFirstChild(newName)
	if occupied and occupied ~= existing then
		if isManagedInstance(occupied) then
			occupied:Destroy()
		else
			return false, "Rename target occupied by unmanaged instance: " .. newPath
		end
	end

	existing.Name = newName
	existing.Parent = newParent
	if typeof(change.source) == "string" then
		local didUpdate = setScriptSource(existing, change.source)
		if not didUpdate then
			return false, "Failed to update source for " .. existing:GetFullName()
		end
	end
	markManagedInstance(existing, change.local_path, revision, nil, INSTANCE_KINDS.Script)
	return applyScriptPayload(self, existing, existing.ClassName, newName, change.payload)
end

function SyncAPI:applyDelete(change : {[string]: any})
	local entity = getEntityKind(change)
	local targetPath = change.rbx_path
	if typeof(targetPath) ~= "string" then
		targetPath = change.old_rbx_path
	end
	if typeof(targetPath) ~= "string" then
		return false, "Invalid delete payload"
	end
	if isIgnoredPath(targetPath, self.ignoredRbxPaths) then
		return true
	end
	if TypeList.isRootServicePath and TypeList.isRootServicePath(targetPath) then
		return true, "Skip delete root service at " .. targetPath
	end

	local indexes = buildManagedIndexes(self.managedRoots)
	local target = findExistingManagedTarget(change, indexes)
	if not target then
		return true, "Delete target not found at " .. targetPath
	end

	if entity == INSTANCE_KINDS.Script and not isScriptClass(target.ClassName) then
		return false, "Delete target class mismatch for script: " .. target.ClassName
	end
	if target:IsA("Terrain") then
		return true, "Skip delete protected Terrain at " .. targetPath
	end
	if target.Parent == game and TypeList.ROOT_SERVICE_CLASS and TypeList.ROOT_SERVICE_CLASS[target.Name] then
		return true, "Skip delete root service instance " .. target:GetFullName()
	end

	if not isManagedInstance(target) then
		return true, "Skipped unmanaged delete at " .. targetPath
	end

	removeFromIndexesRecursive(indexes, target)
	target:Destroy()
	return true
end

function SyncAPI:applyUpsert(change : {[string]: any}, revision : number)
	local entity = getEntityKind(change)
	if entity == INSTANCE_KINDS.UIInstance then
		local indexes = buildManagedIndexes(self.managedRoots)
		local ok, resultOrError = prepareUiUpsert(self, change, revision, indexes)
		if not ok then
			return false, resultOrError
		end
		if typeof(resultOrError) == "table" and resultOrError.instance then
			local propertyErrors, propertyNotices = applyUiProperties(self, resultOrError.instance, resultOrError.payload)
			local metadataErrors, metadataNotices = applyUiAttributesAndTags(resultOrError.instance, resultOrError.payload)
			if #propertyErrors > 0 or #metadataErrors > 0 then
				local merged = {}
				for _, message in ipairs(propertyErrors) do
					table.insert(merged, message)
				end
				for _, message in ipairs(metadataErrors) do
					table.insert(merged, message)
				end
				return false, table.concat(merged, " | ")
			end
			if #propertyNotices > 0 then
				return true, table.concat(propertyNotices, " | ")
			end
			if #metadataNotices > 0 then
				return true, table.concat(metadataNotices, " | ")
			end
			if typeof(resultOrError.notice) == "string" and resultOrError.notice ~= "" then
				return true, resultOrError.notice
			end
		end
		return true
	end

	return applyScriptUpsert(self, change, revision)
end

function SyncAPI:applyRename(change : {[string]: any}, revision : number)
	local entity = getEntityKind(change)
	if entity == INSTANCE_KINDS.UIInstance then
		return self:applyUpsert(change, revision)
	end
	return applyScriptRename(self, change, revision)
end

local function getIncomingUiStableId(change : {[string]: any})
	local stableId = change.stable_id
	if typeof(stableId) == "string" and stableId ~= "" then
		return stableId
	end

	local payload = change.payload
	if typeof(payload) == "table" then
		local payloadId = payload.id
		if typeof(payloadId) == "string" and payloadId ~= "" then
			return payloadId
		end
	end

	return ""
end

local function cloneUiChangeWithoutStableId(change : {[string]: any})
	local clone = {}
	for key, value in pairs(change) do
		clone[key] = value
	end

	clone.stable_id = ""

	local payload = change.payload
	if typeof(payload) == "table" then
		local payloadClone = {}
		for key, value in pairs(payload) do
			payloadClone[key] = value
		end
		payloadClone.id = ""
		clone.payload = payloadClone
	end

	return clone
end

function SyncAPI:applyChanges(changes : {any}, targetRevision : number)
	if #changes == 0 then
		self.lastAppliedRevision = targetRevision
		self.plugin:SetSetting(SETTING_KEYS.LastRevision, self.lastAppliedRevision)
		return true, {}, {}, {
			total = 0,
			applied = 0,
		}
	end

	local incomingStableIdCounts = {}
	for _, change in ipairs(changes) do
		if getEntityKind(change) == INSTANCE_KINDS.UIInstance and tostring(change.op) ~= "delete" then
			local stableId = getIncomingUiStableId(change)
			if stableId ~= "" then
				incomingStableIdCounts[stableId] = (incomingStableIdCounts[stableId] or 0) + 1
			end
		end
	end

	local orderedChanges = {}
	for _, change in ipairs(changes) do
		local stableId = getIncomingUiStableId(change)
		if stableId ~= "" and (incomingStableIdCounts[stableId] or 0) > 1 then
			table.insert(orderedChanges, cloneUiChangeWithoutStableId(change))
		else
			table.insert(orderedChanges, change)
		end
	end

	table.sort(orderedChanges, function(a, b)
		local aOp = tostring(a.op)
		local bOp = tostring(b.op)
		local aDelete = aOp == "delete"
		local bDelete = bOp == "delete"

		if aDelete ~= bDelete then
			return not aDelete
		end

		if aDelete and bDelete then
			local aDepth = getChangeDepth(a, a.rbx_path or a.old_rbx_path)
			local bDepth = getChangeDepth(b, b.rbx_path or b.old_rbx_path)
			if aDepth ~= bDepth then
				return aDepth > bDepth
			end
		else
			local aPath = a.new_rbx_path or a.rbx_path
			local bPath = b.new_rbx_path or b.rbx_path
			local aDepth = getChangeDepth(a, aPath)
			local bDepth = getChangeDepth(b, bPath)
			if aDepth ~= bDepth then
				return aDepth < bDepth
			end
		end

		local aKey = tostring(a.new_rbx_path or a.rbx_path or a.local_path or "")
		local bKey = tostring(b.new_rbx_path or b.rbx_path or b.local_path or "")
		return aKey < bKey
	end)

	local indexes = buildManagedIndexes(self.managedRoots)
	local errors = {}
	local notices = {}
	local errorSeen = {}
	local noticeSeen = {}
	local appliedCount = 0
	local syncHistoryEvents = {}
	local uiPrepared = {}
	local deleteQueue = {}
	local totalChanges = #orderedChanges

	local function publishProgress(currentIndex : number, phase : string?)
		if self.initialSyncInProgress ~= true then
			return
		end

		local percent = 100
		if totalChanges > 0 then
			percent = math.floor((currentIndex / totalChanges) * 100 + 0.5)
		end
		self.updateStatus(
			"Syncing files... "
				.. tostring(currentIndex)
				.. "/"
				.. tostring(totalChanges)
				.. " ("
				.. tostring(percent)
				.. "%)"
				.. (phase and (" - " .. phase) or ""),
			false
		)
	end

	local function pushError(message : string)
		if not errorSeen[message] then
			errorSeen[message] = true
			table.insert(errors, message)
			if isConflictMessage(message) then
				table.insert(syncHistoryEvents, formatConflictHistoryEvent(message))
			end
		end
	end

	local function pushNotice(message : string)
		if message ~= "" and not noticeSeen[message] then
			noticeSeen[message] = true
			table.insert(notices, message)
		end
	end

	for changeIndex, change in ipairs(orderedChanges) do
		publishProgress(changeIndex, tostring(change.op or "apply"))

		local op = tostring(change.op)
		local entity = getEntityKind(change)

		if op == "delete" then
			table.insert(deleteQueue, change)
			continue
		end

		if entity == INSTANCE_KINDS.UIInstance then
			if op ~= "upsert" and op ~= "rename" then
				pushError("Unsupported UI op: " .. op)
				continue
			end

			local ok, resultOrError = prepareUiUpsert(self, change, targetRevision, indexes)
			if not ok then
				pushError(tostring(resultOrError))
			else
				appliedCount += 1
				if shouldRecordSyncHistory(self, change, resultOrError) then
					table.insert(syncHistoryEvents, formatSyncHistoryEvent(change, resultOrError))
				end
				if typeof(resultOrError) == "string" then
					pushNotice(resultOrError)
				elseif typeof(resultOrError) == "table" and resultOrError.instance then
					if typeof(resultOrError.notice) == "string" and resultOrError.notice ~= "" then
						pushNotice(resultOrError.notice)
					end
					table.insert(uiPrepared, resultOrError)
				end
			end
		else
			local ok, resultOrError
			if op == "upsert" then
				ok, resultOrError = applyScriptUpsert(self, change, targetRevision)
			elseif op == "rename" then
				ok, resultOrError = applyScriptRename(self, change, targetRevision)
			else
				ok = false
				resultOrError = "Unsupported op: " .. op
			end

			if ok then
				appliedCount += 1
				if shouldRecordSyncHistory(self, change, resultOrError) then
					table.insert(syncHistoryEvents, formatSyncHistoryEvent(change, resultOrError))
				end
				if typeof(resultOrError) == "string" then
					pushNotice(resultOrError)
				end
			else
				pushError(tostring(resultOrError))
			end
		end
	end

	if #uiPrepared > 0 and self.initialSyncInProgress == true then
		self.updateStatus("Syncing properties... (90%)", false)
	end
	for _, prepared in ipairs(uiPrepared) do
		local propertyErrors, propertyNotices = applyUiProperties(self, prepared.instance, prepared.payload)
		for _, errorMessage in ipairs(propertyErrors) do
			pushError(errorMessage)
		end
		for _, noticeMessage in ipairs(propertyNotices) do
			pushNotice(noticeMessage)
		end
	end

	if #uiPrepared > 0 and self.initialSyncInProgress == true then
		self.updateStatus("Syncing metadata... (95%)", false)
	end
	for _, prepared in ipairs(uiPrepared) do
		local metadataErrors, metadataNotices = applyUiAttributesAndTags(prepared.instance, prepared.payload)
		for _, errorMessage in ipairs(metadataErrors) do
			pushError(errorMessage)
		end
		for _, noticeMessage in ipairs(metadataNotices) do
			pushNotice(noticeMessage)
		end
	end

	if #deleteQueue > 0 and self.initialSyncInProgress == true then
		self.updateStatus("Syncing deletes... (98%)", false)
	end
	for _, change in ipairs(deleteQueue) do
		local ok, resultOrError = self:applyDelete(change)
		if ok then
			appliedCount += 1
			if shouldRecordSyncHistory(self, change, resultOrError) then
				table.insert(syncHistoryEvents, formatSyncHistoryEvent(change, resultOrError))
			end
			if typeof(resultOrError) == "string" and resultOrError ~= "" then
				pushNotice(resultOrError)
			end
		else
			pushError(tostring(resultOrError))
		end
	end

	local wasSuccessful = #errors == 0
	if wasSuccessful then
		self.lastAppliedRevision = targetRevision
		self.plugin:SetSetting(SETTING_KEYS.LastRevision, self.lastAppliedRevision)
	end

	return wasSuccessful, errors, notices, {
		total = #orderedChanges,
		applied = appliedCount,
		history = summariseSyncHistoryEvents(syncHistoryEvents),
	}
end

function SyncAPI:performHandshake()
	local payload = {
		protocol = TypeList.PROTOCOL,
		plugin_version = TypeList.VERSION,
		accept_encodings = {
			TypeList.CHANGE_ENCODINGS and TypeList.CHANGE_ENCODINGS.Compact or "compact-json-v1",
			TypeList.CHANGE_ENCODINGS and TypeList.CHANGE_ENCODINGS.Verbose or "verbose-json-v1",
		},
		client_id = self.clientId,
		last_applied_rev = self.lastAppliedRevision,
	}

	local response, requestError = self:requestJson("POST", TypeList.ENDPOINTS.Handshake, payload, nil)
	if not response then
		self.debugState.handshake = false
		self:setDebugError("Handshake gagal: " .. tostring(requestError))
		return false, requestError
	end

	if response.status ~= "ok" then
		self.debugState.handshake = false
		self:setDebugError("Handshake ditolak: " .. tostring(response.status))
		return false, "Handshake rejected"
	end

	self.sessionId = tostring(response.session_id or "")
	if typeof(response.selected_change_encoding) == "string" and response.selected_change_encoding ~= "" then
		self.changeEncoding = response.selected_change_encoding
	end
	if typeof(response.managed_roots) == "table" and #response.managed_roots > 0 then
		self.managedRoots = response.managed_roots
	end
	if typeof(response.ignored_rbx_paths) == "table" then
		self.ignoredRbxPaths = response.ignored_rbx_paths
	end
	if typeof(response.strict_property_whitelist) == "boolean" then
		self.strictPropertyWhitelist = response.strict_property_whitelist == true
	end
	if TypeList.normaliseExtraAllowedProperties then
		self.extraAllowedProperties = TypeList.normaliseExtraAllowedProperties(response.extra_allowed_properties)
	elseif typeof(response.extra_allowed_properties) == "table" then
		self.extraAllowedProperties = response.extra_allowed_properties
	end

	self.serverIndexedCount = tonumber(response.indexed_entry_count)
		or tonumber(response.indexed_script_count)
		or 0
	self.debugState.handshake = true
	self.debugState.lastServerRev = tonumber(response.server_rev) or self.debugState.lastServerRev
	self.debugState.lastError = ""
	self:appendDebugEvent(
		"Handshake ok, session="
			.. self.sessionId
			.. ", serverRev="
			.. tostring(self.debugState.lastServerRev)
			.. ", strictWhitelist="
			.. tostring(self.strictPropertyWhitelist == true),
		false
	)
	self:refreshServerDebugState(true)
	self:pushDebugUpdate()
	return true
end

function SyncAPI:refreshServerDebugState(force : boolean?)
	local nowClock = os.clock()
	if not force and (nowClock - self.lastServerDebugPullAt) < SERVER_DEBUG_PULL_INTERVAL then
		return
	end
	self.lastServerDebugPullAt = nowClock

	local response, requestError = self:requestJson("GET", TypeList.ENDPOINTS.DebugState, nil, {
		limit = 12,
	})
	if not response then
		self:appendDebugEvent("Fetch /debug/state gagal: " .. tostring(requestError), true)
		return
	end
	if response.status ~= "ok" then
		self:appendDebugEvent("Fetch /debug/state status: " .. tostring(response.status), true)
		return
	end

	local metrics = typeof(response.metrics) == "table" and response.metrics or {}
	local lastAck = typeof(metrics.last_ack) == "table" and metrics.last_ack or {}
	local lastChanges = typeof(metrics.last_changes) == "table" and metrics.last_changes or {}
	local git = typeof(response.git) == "table" and response.git or {}
	local scanWarnings = {}
	if typeof(response.scan_warnings) == "table" then
		for _, warning in ipairs(response.scan_warnings) do
			table.insert(scanWarnings, tostring(warning))
		end
	end
	local latestScanWarning = #scanWarnings > 0 and scanWarnings[#scanWarnings] or ""
	local extraAllowedClassCount = 0
	if typeof(response.extra_allowed_properties) == "table" then
		for _, value in pairs(response.extra_allowed_properties) do
			if value ~= nil then
				extraAllowedClassCount += 1
			end
		end
	end

	self.serverDebugState = {
		server_rev = tonumber(response.server_rev) or 0,
		indexed_script_count = tonumber(response.indexed_script_count) or 0,
		indexed_ui_count = tonumber(response.indexed_ui_count) or tonumber(response.indexed_props_count) or 0,
		indexed_props_count = tonumber(response.indexed_props_count) or tonumber(response.indexed_ui_count) or 0,
		indexed_entry_count = tonumber(response.indexed_entry_count) or 0,
		strict_property_whitelist = response.strict_property_whitelist == true,
		extra_allowed_class_count = extraAllowedClassCount,
		sessions_active = tonumber(response.sessions_active) or 0,
		sync_root = tostring(response.sync_root or "-"),
		scan_interval_sec = tonumber(response.scan_interval_sec) or 0,
		scan_cycles = tonumber(metrics.scan_cycles) or 0,
		last_scan_sec = tonumber(metrics.last_scan_sec) or 0,
		last_revision_change_count = tonumber(metrics.last_revision_change_count) or 0,
		ack_count = tonumber(metrics.ack_count) or 0,
		ack_error_count = tonumber(metrics.ack_error_count) or 0,
		changes_requests = tonumber(metrics.changes_requests) or 0,
		changes_timeouts = tonumber(metrics.changes_timeouts) or 0,
		last_ack_status = tostring(lastAck.status or "-"),
		last_ack_rev = tostring(lastAck.applied_rev or "-"),
		last_changes_count = tonumber(lastChanges.change_count) or 0,
		last_changes_target_rev = tonumber(lastChanges.target_rev) or 0,
		git_enabled = git.enabled == true,
		git_available = git.available == true,
		git_commit = tostring(git.last_commit_short or ""),
		git_error = tostring(git.last_error or ""),
		scan_warnings = scanWarnings,
		scan_warning = latestScanWarning,
		scan_warning_guidance = latestScanWarning ~= "" and formatGuidedError(latestScanWarning) or "",
	}
	self:pushDebugUpdate()
end

return SyncAPI
