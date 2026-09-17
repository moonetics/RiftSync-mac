local Identity = {}

function Identity.isExplicitStableId(value)
	return type(value) == "string" and value ~= "" and string.sub(value, 1, 5) ~= "path:"
end

function Identity.localSegmentHasStableId(value)
	if type(value) ~= "string" then
		return false
	end
	local marker = "~rid_"
	local index = string.find(value, marker, 1, true)
	return index ~= nil and index > 1 and string.sub(value, index + #marker) ~= ""
end

function Identity.identityKey(entity, stableId, rbxPath, localPath)
	if Identity.isExplicitStableId(stableId) then
		return tostring(entity or "") .. "\0stable_id\0" .. stableId
	end
	return tostring(entity or "") .. "\0rbx_path\0" .. tostring(rbxPath or "") .. "\0" .. tostring(localPath or "")
end

local function incomingClassName(change)
	if type(change.class_name) == "string" and change.class_name ~= "" then
		return change.class_name
	end
	local payload = change.payload
	if type(payload) == "table" then
		local value = payload.className or payload["$className"]
		if type(value) == "string" then
			return value
		end
	end
	return ""
end

local function incomingStableId(change)
	if Identity.isExplicitStableId(change.stable_id) then
		return change.stable_id
	end
	local payload = change.payload
	if type(payload) == "table" then
		for _, key in ipairs({ "id", "$id", "syncId" }) do
			if Identity.isExplicitStableId(payload[key]) then
				return payload[key]
			end
		end
	end
	return ""
end

-- An exact same-class upsert can safely adopt an object that predates RiftSync
-- metadata. Destructive operations, moves, class replacement, ambiguous paths,
-- and conflicting explicit identities still require manual resolution.
function Identity.canAdoptExactTarget(change, candidate)
	if type(change) ~= "table" or type(candidate) ~= "table" then
		return false
	end
	if tostring(change.op or "upsert") ~= "upsert" then
		return false
	end

	local desiredPath = change.new_rbx_path or change.rbx_path or ""
	local expectedClass = incomingClassName(change)
	if desiredPath == ""
		or candidate.rbxPath ~= desiredPath
		or expectedClass == ""
		or candidate.className ~= expectedClass
	then
		return false
	end

	local desiredStableId = incomingStableId(change)
	local candidateStableId = candidate.stableId
	if Identity.isExplicitStableId(desiredStableId)
		and Identity.isExplicitStableId(candidateStableId)
		and desiredStableId ~= candidateStableId
	then
		return false
	end

	return true
end

function Identity.resolveName(candidates, name, stableId)
	local matches = {}
	for _, candidate in ipairs(candidates or {}) do
		if candidate.name == name and (stableId == "" or candidate.stableId == stableId) then
			table.insert(matches, candidate)
		end
	end
	if #matches == 1 then
		return matches[1], nil
	end
	if #matches > 1 then
		return nil, "multiple candidates share the same identity"
	end
	return nil, nil
end

local function lastPathPart(path)
	if type(path) ~= "string" then
		return ""
	end
	return string.match(path, "([^.]+)$") or ""
end

local function conflict(kind, operation, change, message, hint, conflicting)
	return {
		kind = kind,
		operation = operation,
		entity = change.entity or "",
		local_path = change.local_path or "",
		rbx_path = change.new_rbx_path or change.rbx_path or "",
		stable_id = change.stable_id or "",
		conflicting_path = conflicting and (conflicting.localPath or conflicting.rbxPath) or nil,
		message = message,
		hint = hint or "Fix the identity conflict and retry.",
	}
end

-- Build a read-only identity plan from plain tables. Roblox Instance objects
-- are deliberately not required here, so this logic can be tested by Lune.
function Identity.buildApplyPlan(changes, candidates)
	local plan = { steps = {}, conflicts = {} }
	local byStableId = {}
	local byLocalPath = {}
	local byRbxPath = {}

	for _, candidate in ipairs(candidates or {}) do
		if Identity.isExplicitStableId(candidate.stableId) then
			byStableId[candidate.stableId] = byStableId[candidate.stableId] or {}
			table.insert(byStableId[candidate.stableId], candidate)
		end
		if candidate.localPath and candidate.localPath ~= "" then
			byLocalPath[candidate.localPath] = byLocalPath[candidate.localPath] or {}
			table.insert(byLocalPath[candidate.localPath], candidate)
		end
		if candidate.rbxPath and candidate.rbxPath ~= "" then
			byRbxPath[candidate.rbxPath] = byRbxPath[candidate.rbxPath] or {}
			table.insert(byRbxPath[candidate.rbxPath], candidate)
		end
	end

	local function add(conflictValue)
		for _, existing in ipairs(plan.conflicts) do
			if existing.kind == conflictValue.kind
				and existing.local_path == conflictValue.local_path
				and existing.rbx_path == conflictValue.rbx_path
				and existing.stable_id == conflictValue.stable_id
			then
				return
			end
		end
		table.insert(plan.conflicts, conflictValue)
	end

	for stableId, matches in pairs(byStableId) do
		if #matches > 1 then
			for _, candidate in ipairs(matches) do
				add({
					kind = "duplicate_stable_id",
					operation = "upsert",
					entity = candidate.entity or "",
					local_path = candidate.localPath or "",
					rbx_path = candidate.rbxPath or "",
					stable_id = stableId,
					message = "StableId " .. stableId .. " belongs to multiple managed instances.",
					hint = "Give every managed instance a unique StableId.",
				})
			end
		end
	end

	local claimedCandidates = {}

	for _, change in ipairs(changes or {}) do
		local operation = change.op or "upsert"
		local stableId = change.stable_id or ""
		local target = nil
		local targetMatches = nil

		if Identity.isExplicitStableId(stableId) then
			targetMatches = byStableId[stableId] or {}
			if #targetMatches > 1 then
				add(conflict("duplicate_stable_id", operation, change,
					"StableId " .. stableId .. " resolves to multiple candidates.",
					"Fix the duplicate StableId before syncing."))
			elseif #targetMatches == 1 then
				target = targetMatches[1]
				claimedCandidates[target] = true
			end
		end

		if not target and change.local_path and change.local_path ~= "" then
			targetMatches = byLocalPath[change.local_path] or {}
			if #targetMatches > 1 then
				add(conflict("duplicate_local_path", operation, change,
					"LocalPath resolves to multiple managed instances.",
					"Give every managed instance a unique LocalPath."))
			elseif #targetMatches == 1 then
				target = targetMatches[1]
				claimedCandidates[target] = true
			end
		end

		local desiredPath = change.new_rbx_path or change.rbx_path or ""
		local destinationMatches = byRbxPath[desiredPath] or {}

		if not target and desiredPath ~= "" and operation == "upsert" and #destinationMatches > 0 then
			for _, candidate in ipairs(destinationMatches) do
				if not claimedCandidates[candidate] and Identity.canAdoptExactTarget(change, candidate) then
					target = candidate
					claimedCandidates[candidate] = true
					break
				end
			end
		end

		local desiredName = change.name or lastPathPart(desiredPath)
		if not target and desiredName ~= "" then
			local nameMatches = {}
			for _, candidate in ipairs(candidates or {}) do
				if candidate.name == desiredName and not claimedCandidates[candidate] then
					table.insert(nameMatches, candidate)
				end
			end
			local resolved, resolveError = Identity.resolveName(nameMatches, desiredName, "")
			if resolveError then
				add(conflict("ambiguous_target", operation, change,
					"Name-only resolution found multiple candidates for " .. desiredName .. ".",
					"Use StableId to select the intended instance."))
			elseif resolved then
				target = resolved
				claimedCandidates[target] = true
			end
		end

		if target and target.managed ~= true and not Identity.canAdoptExactTarget(change, target) then
			add(conflict("ambiguous_target", operation, change,
				"Resolved target is unmanaged.",
				"Do not delete or repurpose unmanaged instances; choose another target."))
		end

		if #destinationMatches > 1 then
			local targetIsAtDestination = false
			for _, candidate in ipairs(destinationMatches) do
				if candidate == target then
					targetIsAtDestination = true
					break
				end
			end
			if not targetIsAtDestination then
				local hasUnclaimedOccupant = false
				for _, candidate in ipairs(destinationMatches) do
					if not claimedCandidates[candidate] then
						hasUnclaimedOccupant = true
						break
					end
				end
				if hasUnclaimedOccupant then
					add(conflict("ambiguous_target", operation, change,
						"Destination path is occupied by multiple candidates.",
						"Use a StableId and the ~rid_<StableId> local suffix."))
				end
			end
		elseif #destinationMatches == 1 and destinationMatches[1] ~= target then
			local occupant = destinationMatches[1]
			if (occupant.managed ~= true or not target) and not claimedCandidates[occupant] then
				add(conflict("ambiguous_target", operation, change,
					"Destination is occupied by another or unmanaged instance.",
					"Move the destination or resolve it with a unique StableId.", occupant))
			end
		end

		local action = "create"
		if target then
			action = target.rbxPath ~= desiredPath and "rename_move" or "reuse"
		end
		if operation == "delete" then
			action = target and "delete" or "delete_missing"
		end
		table.insert(plan.steps, { operation = operation, action = action, target = target, change = change })
	end

	if #plan.conflicts > 0 then
		for _, step in ipairs(plan.steps) do
			step.action = "reject"
		end
	end
	return plan
end

return Identity
