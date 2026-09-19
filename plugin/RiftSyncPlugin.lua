--------------------------------------------------------------------------------------------
-- RiftSyncPlugin v4.2.3
-- Multi-profile Roblox Studio client for RiftSync.
--------------------------------------------------------------------------------------------

local apiModule = script:WaitForChild("API")
local SyncAPI = require(apiModule)
local TypeList = require(apiModule:WaitForChild("TypeList"))
local TweenService = game:GetService("TweenService")

if not plugin then
	warn("[RiftSync] Plugin context is unavailable.")
	return
end

local START_MODES = TypeList.START_MODES or {
	StudioToFolder = "studio_to_folder",
	FolderToStudio = "folder_to_studio",
}

local COLORS = {
	Background = Color3.fromRGB(11, 17, 24),
	Surface = Color3.fromRGB(27, 38, 50),
	SurfaceHigh = Color3.fromRGB(37, 51, 65),
	Well = Color3.fromRGB(16, 24, 33),
	Border = Color3.fromRGB(64, 80, 100),
	BorderDark = Color3.fromRGB(7, 12, 18),
	Text = Color3.fromRGB(242, 246, 251),
	Muted = Color3.fromRGB(174, 185, 200),
	Cyan = Color3.fromRGB(104, 215, 208),
	Blue = Color3.fromRGB(113, 139, 232),
	Green = Color3.fromRGB(94, 209, 138),
	Amber = Color3.fromRGB(228, 180, 95),
	Red = Color3.fromRGB(227, 110, 120),
}

local toolbar = plugin:CreateToolbar("RiftSync")
local toolbarButton = toolbar:CreateButton(
	"Sync",
	"Open RiftSync Studio",
	tostring(TypeList.PLUGIN_ICON or ""),
	"RiftSync"
)

local widgetInfo = DockWidgetPluginGuiInfo.new(
	Enum.InitialDockState.Float,
	false,
	false,
	440,
	700,
	340,
	500
)
local widget = plugin:CreateDockWidgetPluginGui("RiftSyncWidget", widgetInfo)
widget.Title = "RiftSync Studio v" .. tostring(TypeList.VERSION)
widget.Enabled = false

local function corner(parent, radius)
	local value = Instance.new("UICorner")
	value.CornerRadius = UDim.new(0, radius or 8)
	value.Parent = parent
	return value
end

local function stroke(parent, color, thickness)
	local value = Instance.new("UIStroke")
	value.Color = color or COLORS.Border
	value.Thickness = thickness or 1
	value.ApplyStrokeMode = Enum.ApplyStrokeMode.Border
	value.Parent = parent
	return value
end

local function padding(parent, left, top, right, bottom)
	local value = Instance.new("UIPadding")
	value.PaddingLeft = UDim.new(0, left or 0)
	value.PaddingTop = UDim.new(0, top or 0)
	value.PaddingRight = UDim.new(0, right or left or 0)
	value.PaddingBottom = UDim.new(0, bottom or top or 0)
	value.Parent = parent
	return value
end

local function verticalList(parent, gap)
	local value = Instance.new("UIListLayout")
	value.FillDirection = Enum.FillDirection.Vertical
	value.SortOrder = Enum.SortOrder.LayoutOrder
	value.Padding = UDim.new(0, gap or 8)
	value.Parent = parent
	return value
end

local function horizontalList(parent, gap)
	local value = Instance.new("UIListLayout")
	value.FillDirection = Enum.FillDirection.Horizontal
	value.HorizontalAlignment = Enum.HorizontalAlignment.Left
	value.VerticalAlignment = Enum.VerticalAlignment.Center
	value.SortOrder = Enum.SortOrder.LayoutOrder
	value.Padding = UDim.new(0, gap or 8)
	value.Parent = parent
	return value
end

local function label(parent, text, size, color, bold)
	local actualSize = math.max(tonumber(size) or 13, 12)
	local value = Instance.new("TextLabel")
	value.Size = UDim2.new(1, 0, 0, actualSize + 8)
	value.BackgroundTransparency = 1
	value.Text = tostring(text or "")
	value.TextColor3 = color or COLORS.Text
	value.TextSize = actualSize
	value.Font = bold and Enum.Font.GothamBold or Enum.Font.Gotham
	value.TextXAlignment = Enum.TextXAlignment.Left
	value.TextYAlignment = Enum.TextYAlignment.Center
	value.TextTruncate = Enum.TextTruncate.AtEnd
	value.Parent = parent
	return value
end

local function wrappedLabel(parent, text, size, color, bold)
	local value = label(parent, text, size, color, bold)
	value.AutomaticSize = Enum.AutomaticSize.Y
	value.TextWrapped = true
	value.TextTruncate = Enum.TextTruncate.None
	value.TextYAlignment = Enum.TextYAlignment.Top
	return value
end

local function panel(parent, order)
	local value = Instance.new("Frame")
	value.Size = UDim2.new(1, 0, 0, 0)
	value.AutomaticSize = Enum.AutomaticSize.Y
	value.BackgroundColor3 = COLORS.Surface
	value.BackgroundTransparency = 0.08
	value.BorderSizePixel = 0
	value.LayoutOrder = order or 0
	value.Parent = parent
	corner(value, 12)
	stroke(value, COLORS.Border, 1)
	padding(value, 16, 14, 16, 14)
	verticalList(value, 12)
	return value
end

local function button(parent, text, color, order)
	local value = Instance.new("TextButton")
	value.Size = UDim2.new(0, 104, 0, 40)
	value.BackgroundColor3 = color or COLORS.SurfaceHigh
	value.BorderSizePixel = 0
	value.Text = text
	value.TextColor3 = COLORS.Text
	value.TextSize = 13
	value.Font = Enum.Font.GothamBold
	value.AutoButtonColor = true
	value.LayoutOrder = order or 0
	value.Parent = parent
	value.Selectable = true
	corner(value, 8)
	stroke(value, COLORS.Border, 1)
	return value
end

local function textBox(parent, placeholder, code)
	local value = Instance.new("TextBox")
	value.Size = UDim2.new(1, 0, 0, 40)
	value.BackgroundColor3 = COLORS.Well
	value.BorderSizePixel = 0
	value.Text = ""
	value.PlaceholderText = placeholder or ""
	value.PlaceholderColor3 = COLORS.Muted
	value.TextColor3 = COLORS.Text
	value.TextSize = 13
	value.Font = code and Enum.Font.Code or Enum.Font.Gotham
	value.TextXAlignment = Enum.TextXAlignment.Left
	value.ClearTextOnFocus = false
	value.TextTruncate = Enum.TextTruncate.AtEnd
	value.Parent = parent
	value.Selectable = true
	corner(value, 8)
	stroke(value, COLORS.Border, 1)
	padding(value, 12, 0, 12, 0)
	return value
end

local function field(parent, caption, placeholder, code, order)
	local holder = Instance.new("Frame")
	holder.Size = UDim2.new(1, 0, 0, 62)
	holder.BackgroundTransparency = 1
	holder.LayoutOrder = order or 0
	holder.Parent = parent
	local title = label(holder, string.upper(caption), 11, COLORS.Muted, true)
	title.Size = UDim2.new(1, 0, 0, 18)
	local input = textBox(holder, placeholder, code)
	input.Position = UDim2.new(0, 0, 0, 22)
	return input, holder
end

local container = Instance.new("Frame")
container.Size = UDim2.fromScale(1, 1)
container.BackgroundColor3 = COLORS.Background
container.BorderSizePixel = 0
container.Parent = widget

local scroll = Instance.new("ScrollingFrame")
scroll.Size = UDim2.fromScale(1, 1)
scroll.BackgroundTransparency = 1
scroll.BorderSizePixel = 0
scroll.ScrollBarThickness = 6
scroll.ScrollBarImageColor3 = COLORS.Border
scroll.AutomaticCanvasSize = Enum.AutomaticSize.Y
scroll.CanvasSize = UDim2.new()
scroll.Parent = container
padding(scroll, 16, 16, 16, 16)
verticalList(scroll, 12)

local header = Instance.new("Frame")
header.Size = UDim2.new(1, 0, 0, 48)
header.BackgroundTransparency = 1
header.LayoutOrder = 1
header.Parent = scroll

local title = label(header, "RiftSync Studio", 20, COLORS.Text, true)
title.Size = UDim2.new(1, -90, 0, 25)
local subtitle = label(header, "MULTI-PROFILE SYNC", 12, COLORS.Muted, true)
subtitle.Size = UDim2.new(1, -90, 0, 16)
subtitle.Position = UDim2.new(0, 0, 0, 26)
local version = label(header, "v" .. tostring(TypeList.VERSION), 11, COLORS.Cyan, true)
version.Size = UDim2.new(0, 76, 0, 25)
version.Position = UDim2.new(1, -76, 0, 2)
version.TextXAlignment = Enum.TextXAlignment.Center
version.BackgroundColor3 = COLORS.Well
version.BackgroundTransparency = 0
corner(version, 7)
stroke(version, COLORS.Border, 1)

local connectionPanel = panel(scroll, 2)
label(connectionPanel, "CONNECTION", 12, COLORS.Muted, true)

local profileSwitcher = Instance.new("Frame")
profileSwitcher.Size = UDim2.new(1, 0, 0, 40)
profileSwitcher.BackgroundTransparency = 1
profileSwitcher.LayoutOrder = 1
profileSwitcher.Parent = connectionPanel

local previousProfileButton = button(profileSwitcher, "Prev", COLORS.SurfaceHigh, 1)
previousProfileButton.Size = UDim2.new(0, 52, 0, 40)
local profileNameLabel = label(profileSwitcher, "Default", 13, COLORS.Text, true)
profileNameLabel.Size = UDim2.new(1, -168, 0, 40)
profileNameLabel.Position = UDim2.new(0, 60, 0, 0)
profileNameLabel.TextXAlignment = Enum.TextXAlignment.Center
profileNameLabel.BackgroundColor3 = COLORS.Well
profileNameLabel.BackgroundTransparency = 0.08
corner(profileNameLabel, 8)
stroke(profileNameLabel, COLORS.Border, 1)
local nextProfileButton = button(profileSwitcher, "Next", COLORS.SurfaceHigh, 2)
nextProfileButton.Size = UDim2.new(0, 52, 0, 40)
nextProfileButton.Position = UDim2.new(1, -100, 0, 0)
local refreshProfilesButton = button(profileSwitcher, "", COLORS.Cyan, 3)
refreshProfilesButton.Name = "RefreshProfilesButton"
refreshProfilesButton.Size = UDim2.new(0, 40, 0, 40)
refreshProfilesButton.Position = UDim2.new(1, -40, 0, 0)
refreshProfilesButton.TextColor3 = COLORS.BorderDark
local refreshProfilesIcon = Instance.new("ImageLabel")
refreshProfilesIcon.Name = "Icon"
refreshProfilesIcon.AnchorPoint = Vector2.new(0.5, 0.5)
refreshProfilesIcon.Size = UDim2.fromOffset(22, 22)
refreshProfilesIcon.Position = UDim2.fromScale(0.5, 0.5)
refreshProfilesIcon.BackgroundTransparency = 1
refreshProfilesIcon.Image = "rbxasset://studio_svg_textures/Lua/FileSync/Light/Standard/Refresh.png"
refreshProfilesIcon.Parent = refreshProfilesButton

local profileManageActions = Instance.new("Frame")
profileManageActions.Size = UDim2.new(1, 0, 0, 40)
profileManageActions.BackgroundTransparency = 1
profileManageActions.LayoutOrder = 2
profileManageActions.Parent = connectionPanel
horizontalList(profileManageActions, 8)
local profileEditorToggleButton = button(profileManageActions, "Edit Profile", COLORS.SurfaceHigh, 1)
profileEditorToggleButton.Size = UDim2.new(0.5, -4, 0, 40)
local addProfileButton = button(profileManageActions, "Add Custom", COLORS.Blue, 2)
addProfileButton.Size = UDim2.new(0.5, -4, 0, 40)
addProfileButton.TextColor3 = COLORS.BorderDark

local execRow = Instance.new("Frame")
execRow.Size = UDim2.new(1, 0, 0, 40)
execRow.BackgroundTransparency = 1
execRow.LayoutOrder = 3
execRow.Parent = connectionPanel
local execStatusLabel = label(execRow, "Exec disabled", 13, COLORS.Muted, false)
execStatusLabel.Size = UDim2.new(1, -116, 0, 40)
local execToggleButton = button(execRow, "Exec OFF", COLORS.SurfaceHigh, 1)
execToggleButton.Size = UDim2.new(0, 108, 0, 40)
execToggleButton.Position = UDim2.new(1, -108, 0, 0)

local profileEditor = Instance.new("Frame")
profileEditor.Size = UDim2.new(1, 0, 0, 0)
profileEditor.AutomaticSize = Enum.AutomaticSize.Y
profileEditor.BackgroundColor3 = COLORS.Well
profileEditor.BackgroundTransparency = 0.18
profileEditor.BorderSizePixel = 0
profileEditor.LayoutOrder = 4
profileEditor.Visible = false
profileEditor.Parent = connectionPanel
corner(profileEditor, 8)
stroke(profileEditor, COLORS.Border, 1)
padding(profileEditor, 12, 12, 12, 12)
verticalList(profileEditor, 8)

local profileNameInput = field(profileEditor, "Profile name", "Project name", false, 1)
local hostInput = field(profileEditor, "Host", "127.0.0.1", true, 2)
local portInput = field(profileEditor, "Port", "8765", true, 3)
local execTokenInput = field(profileEditor, "Exec token", "Project Remote Exec token", true, 4)

local profileActions = Instance.new("Frame")
profileActions.Size = UDim2.new(1, 0, 0, 40)
profileActions.BackgroundTransparency = 1
profileActions.LayoutOrder = 5
profileActions.Parent = profileEditor
horizontalList(profileActions, 8)
local saveProfileButton = button(profileActions, "Save Profile", COLORS.Cyan, 1)
saveProfileButton.TextColor3 = COLORS.BorderDark
saveProfileButton.Size = UDim2.new(0.5, -4, 0, 40)
local removeProfileButton = button(profileActions, "Remove", COLORS.Red, 2)
removeProfileButton.Size = UDim2.new(0.5, -4, 0, 40)
removeProfileButton.TextColor3 = COLORS.BorderDark

local syncPanel = panel(scroll, 4)
label(syncPanel, "SYNC OPERATIONS", 12, COLORS.Muted, true)
local actionGrid = Instance.new("Frame")
actionGrid.Size = UDim2.new(1, 0, 0, 88)
actionGrid.BackgroundTransparency = 1
actionGrid.Parent = syncPanel
local grid = Instance.new("UIGridLayout")
grid.CellSize = UDim2.new(0.5, -4, 0, 40)
grid.CellPadding = UDim2.new(0, 8, 0, 8)
grid.SortOrder = Enum.SortOrder.LayoutOrder
grid.Parent = actionGrid
local startButton = button(actionGrid, "Start Sync", COLORS.Green, 1)
startButton.TextColor3 = COLORS.BorderDark
local snapshotButton = button(actionGrid, "Resync", COLORS.Blue, 2)
snapshotButton.TextColor3 = COLORS.BorderDark
local pullStudioButton = button(actionGrid, "Pull Studio", COLORS.Cyan, 3)
pullStudioButton.TextColor3 = COLORS.BorderDark
local stopButton = button(actionGrid, "Stop", COLORS.Red, 4)
stopButton.TextColor3 = COLORS.BorderDark

local startSourcePanel = panel(scroll, 5)
startSourcePanel.Visible = false
label(startSourcePanel, "CHOOSE START SOURCE", 10, COLORS.Amber, true)
local startSourceExplanation = wrappedLabel(
	startSourcePanel,
	"No compare. Use Studio replaces local after making a backup. Use Local applies local to Studio with an Undo entry. Live sync then runs Local -> Studio.",
	11,
	COLORS.Text,
	false
)
startSourceExplanation.LayoutOrder = 1
local forceIdentityRepairButton = button(startSourcePanel, "Force ID repair: OFF", COLORS.SurfaceHigh, 2)
forceIdentityRepairButton.Size = UDim2.new(1, 0, 0, 40)
local forceIdentityRepairHint = wrappedLabel(
	startSourcePanel,
	"Recovery only: clears RiftSync IDs in Studio once. Studio source creates fresh IDs; Local source restores IDs from local files.",
	10,
	COLORS.Muted,
	false
)
forceIdentityRepairHint.LayoutOrder = 3
local cleanDuplicatesStartButton = button(startSourcePanel, "Clean Duplicate IDs", COLORS.Amber, 4)
cleanDuplicatesStartButton.Size = UDim2.new(1, 0, 0, 36)
cleanDuplicatesStartButton.TextColor3 = COLORS.BorderDark
cleanDuplicatesStartButton.Visible = false
local startSourceActions = Instance.new("Frame")
startSourceActions.Size = UDim2.new(1, 0, 0, 40)
startSourceActions.BackgroundTransparency = 1
startSourceActions.LayoutOrder = 5
startSourceActions.Parent = startSourcePanel
horizontalList(startSourceActions, 8)
local startFromStudioButton = button(startSourceActions, "Use Studio", COLORS.Cyan, 1)
startFromStudioButton.TextColor3 = COLORS.BorderDark
startFromStudioButton.Size = UDim2.new(0.5, -4, 0, 40)
local startFromLocalButton = button(startSourceActions, "Use Local", COLORS.Green, 2)
startFromLocalButton.TextColor3 = COLORS.BorderDark
startFromLocalButton.Size = UDim2.new(0.5, -4, 0, 40)
local cancelStartButton = button(startSourcePanel, "Cancel", COLORS.SurfaceHigh, 6)
cancelStartButton.Size = UDim2.new(1, 0, 0, 40)

local pullPreviewPanel = panel(scroll, 6)
pullPreviewPanel.Visible = false
label(pullPreviewPanel, "PULL STUDIO PREVIEW", 10, COLORS.Amber, true)
local pullPreviewCountLabel = wrappedLabel(pullPreviewPanel, "", 11, COLORS.Text, true)
local pullPreviewSamplesLabel = wrappedLabel(pullPreviewPanel, "", 10, COLORS.Muted, false)
pullPreviewSamplesLabel.Font = Enum.Font.Code
local pullActions = Instance.new("Frame")
pullActions.Size = UDim2.new(1, 0, 0, 40)
pullActions.BackgroundTransparency = 1
pullActions.Parent = pullPreviewPanel
horizontalList(pullActions, 8)
local confirmPullButton = button(pullActions, "Confirm", COLORS.Cyan, 1)
confirmPullButton.TextColor3 = COLORS.BorderDark
confirmPullButton.Size = UDim2.new(0.5, -4, 0, 40)
local cancelPullButton = button(pullActions, "Cancel", COLORS.SurfaceHigh, 2)
cancelPullButton.Size = UDim2.new(0.5, -4, 0, 40)

local statusPanel = panel(scroll, 3)
label(statusPanel, "PROJECT STATUS", 12, COLORS.Muted, true)
local statusHeadline = wrappedLabel(statusPanel, "● Stopped", 16, COLORS.Text, true)
local statusDetail = wrappedLabel(statusPanel, "Choose a profile and start sync.", 13, COLORS.Muted, false)
local progressTrack = Instance.new("Frame")
progressTrack.Name = "SyncProgress"
progressTrack.Size = UDim2.new(1, 0, 0, 8)
progressTrack.BackgroundColor3 = COLORS.Well
progressTrack.BorderSizePixel = 0
progressTrack.Visible = false
progressTrack.Parent = statusPanel
corner(progressTrack, 4)
stroke(progressTrack, COLORS.Border, 1)
local progressFill = Instance.new("Frame")
progressFill.Name = "Fill"
progressFill.Size = UDim2.new(0, 0, 1, 0)
progressFill.BackgroundColor3 = COLORS.Cyan
progressFill.BorderSizePixel = 0
progressFill.Parent = progressTrack
corner(progressFill, 4)
local progressCaption = label(statusPanel, "", 12, COLORS.Muted, false)
progressCaption.Visible = false
local cleanDuplicatesStatusButton = button(statusPanel, "Clean Duplicate IDs", COLORS.Amber, 5)
cleanDuplicatesStatusButton.Size = UDim2.new(1, 0, 0, 36)
cleanDuplicatesStatusButton.TextColor3 = COLORS.BorderDark
cleanDuplicatesStatusButton.Visible = false

local historyPanel = panel(scroll, 7)
label(historyPanel, "RECENT SYNC ACTIVITY", 10, COLORS.Muted, true)
local historyLabel = wrappedLabel(historyPanel, "History is available after sync begins.", 10, COLORS.Muted, false)
historyLabel.Font = Enum.Font.Code

local debugPanel = panel(scroll, 8)
local debugDisclosureButton = button(debugPanel, "Diagnostics  +", COLORS.SurfaceHigh, 1)
debugDisclosureButton.Size = UDim2.new(1, 0, 0, 40)
debugDisclosureButton.TextXAlignment = Enum.TextXAlignment.Left

local debugBody = Instance.new("Frame")
debugBody.Size = UDim2.new(1, 0, 0, 0)
debugBody.AutomaticSize = Enum.AutomaticSize.Y
debugBody.BackgroundTransparency = 1
debugBody.LayoutOrder = 2
debugBody.Visible = false
debugBody.Parent = debugPanel
verticalList(debugBody, 8)
local debugToggleButton = button(debugBody, "Debug OFF", COLORS.SurfaceHigh, 1)
debugToggleButton.Size = UDim2.new(1, 0, 0, 40)
local statusPath = wrappedLabel(debugBody, "No server activity yet.", 12, COLORS.Muted, false)
statusPath.LayoutOrder = 2
statusPath.Font = Enum.Font.Code
local debugLabel = wrappedLabel(debugBody, "HTTP [ ]  Sync [ ]  Token [ ]  Edit mode [ ]", 12, COLORS.Muted, false)
debugLabel.LayoutOrder = 3
debugLabel.Font = Enum.Font.Code

local client = nil
local profiles = {}
local selectedProfileIndex = 1
local historyLines = {}
local MAX_HISTORY_LINES = 12
local progressTween = nil
local updateStatus
local forceIdentityRepairEnabled = false

local function setProfileEditorVisible(visible)
	profileEditor.Visible = visible == true
	profileEditorToggleButton.Text = profileEditor.Visible and "Hide Settings" or "Edit Profile"
end

local function setDebugBodyVisible(visible)
	debugBody.Visible = visible == true
	debugDisclosureButton.Text = debugBody.Visible and "Diagnostics  −" or "Diagnostics  +"
end

local function setControlEnabled(control, enabled)
	if control:IsA("TextBox") then
		control.TextEditable = enabled
		control.BackgroundColor3 = enabled and COLORS.Well or COLORS.Background
	else
		control.Active = enabled
		control.AutoButtonColor = enabled
		control.Selectable = enabled
	end
	control.TextTransparency = enabled and 0 or 0.45
end

local function refreshForceIdentityRepairUI()
	forceIdentityRepairButton.Text = forceIdentityRepairEnabled
		and "Force ID repair: ON"
		or "Force ID repair: OFF"
	forceIdentityRepairButton.BackgroundColor3 = forceIdentityRepairEnabled and COLORS.Amber or COLORS.SurfaceHigh
	forceIdentityRepairButton.TextColor3 = forceIdentityRepairEnabled and COLORS.BorderDark or COLORS.Text
end

local function setProfileControlsLocked(locked)
	for _, control in ipairs({
		profileNameInput,
		hostInput,
		portInput,
		execTokenInput,
		previousProfileButton,
		nextProfileButton,
		refreshProfilesButton,
		addProfileButton,
		profileEditorToggleButton,
		saveProfileButton,
		removeProfileButton,
	}) do
		setControlEnabled(control, not locked)
	end
	if locked then
		setProfileEditorVisible(false)
	end
	if not locked and #profiles <= 1 then
		removeProfileButton.Active = false
		removeProfileButton.AutoButtonColor = false
		removeProfileButton.Selectable = false
		removeProfileButton.TextTransparency = 0.45
	end
end

local function findSelectedProfileIndex()
	local selectedId = client and client.getSelectedConnectionProfileId
		and client:getSelectedConnectionProfileId()
		or ""
	for index, profile in ipairs(profiles) do
		if profile.id == selectedId then
			return index
		end
	end
	return 1
end

local function refreshProfileUI()
	profiles = client and client.listConnectionProfiles and client:listConnectionProfiles() or {}
	if #profiles == 0 then
		return
	end
	selectedProfileIndex = math.clamp(findSelectedProfileIndex(), 1, #profiles)
	local profile = profiles[selectedProfileIndex]
	local isLocal = profile.kind == "local"
	profileNameLabel.Text = tostring(profile.name or "Profile") .. (isLocal and "  ·  LOCAL" or "  ·  CUSTOM")
	profileNameInput.Text = tostring(profile.name or "")
	hostInput.Text = tostring(profile.host or TypeList.DEFAULT_HOST or "127.0.0.1")
	portInput.Text = tostring(profile.port or TypeList.DEFAULT_PORT or 8765)
	execTokenInput.Text = tostring(profile.token or "")
	if isLocal then
		setProfileEditorVisible(false)
	end
	profileEditorToggleButton.Text = isLocal and "Refresh Local" or (profileEditor.Visible and "Hide Settings" or "Edit Custom")
	local editable = not isLocal and not client:isRunning()
	setControlEnabled(profileNameInput, editable)
	setControlEnabled(hostInput, editable)
	setControlEnabled(portInput, editable)
	setControlEnabled(execTokenInput, editable)
	setControlEnabled(saveProfileButton, editable)
	local canRemove = editable and #profiles > 1
	removeProfileButton.Text = isLocal and "Managed by App" or (canRemove and "Remove" or "Keep one profile")
	setControlEnabled(removeProfileButton, canRemove)
end

local function selectProfileAt(index)
	if not client or #profiles == 0 then
		return
	end
	if index < 1 then
		index = #profiles
	elseif index > #profiles then
		index = 1
	end
	local ok, err = client:selectConnectionProfile(profiles[index].id)
	if not ok then
		statusDetail.Text = tostring(err)
		statusDetail.TextColor3 = COLORS.Red
		return
	end
	selectedProfileIndex = index
	refreshProfileUI()
end

local function saveCurrentProfile()
	if not client then
		return false
	end
	local current = profiles[selectedProfileIndex]
	if not current then
		return false
	end
	if current.kind == "local" then
		return true
	end
	local saved, err = client:upsertConnectionProfile({
		id = current.id,
		name = profileNameInput.Text ~= "" and profileNameInput.Text or "Profile",
		host = hostInput.Text,
		port = portInput.Text,
		token = execTokenInput.Text,
	})
	if not saved then
		statusDetail.Text = tostring(err)
		statusDetail.TextColor3 = COLORS.Red
		return false
	end
	local ok, selectError = client:selectConnectionProfile(saved.id)
	if not ok then
		statusDetail.Text = tostring(selectError)
		statusDetail.TextColor3 = COLORS.Red
		return false
	end
	refreshProfileUI()
	statusDetail.Text = "Profile saved for this Place."
	statusDetail.TextColor3 = COLORS.Green
	setProfileEditorVisible(false)
	return true
end

local function refreshLocalProfilesFromApp(showStatus)
	if not client then
		return
	end
	if client:isRunning() then
		if showStatus then
			updateStatus("Stop sync sebelum refresh profile.", true)
		end
		return
	end
	setControlEnabled(refreshProfilesButton, false)
	refreshProfilesIcon.ImageTransparency = 0.35
	local refreshTween = TweenService:Create(
		refreshProfilesIcon,
		TweenInfo.new(0.65, Enum.EasingStyle.Linear, Enum.EasingDirection.InOut, -1, false),
		{ Rotation = 360 }
	)
	refreshTween:Play()
	local ok, err = client:refreshLocalProfiles()
	refreshTween:Cancel()
	refreshProfilesIcon.Rotation = 0
	if ok then
		if showStatus then
			updateStatus("Profile Local diperbarui dari aplikasi RiftSync.", false)
		end
	elseif showStatus then
		updateStatus(tostring(err), true)
	end
	refreshProfileUI()
	refreshProfilesIcon.ImageTransparency = 0
	setControlEnabled(refreshProfilesButton, not client:isRunning())
end

local function updateExecUI()
	if not client then
		return
	end
	local enabled = client:isRemoteExecEnabled()
	execToggleButton.Text = enabled and "Exec ON" or "Exec OFF"
	execToggleButton.BackgroundColor3 = enabled and COLORS.Green or COLORS.SurfaceHigh
	execToggleButton.TextColor3 = enabled and COLORS.BorderDark or COLORS.Text
	execStatusLabel.Text = tostring(client:getRemoteExecStatus())
	execStatusLabel.TextColor3 = string.find(string.lower(execStatusLabel.Text), "error", 1, true)
			and COLORS.Red
		or (enabled and COLORS.Green or COLORS.Muted)
end

local function appendHistory(meta)
	if typeof(meta) ~= "table" or typeof(meta.events) ~= "table" then
		return
	end
	for _, event in ipairs(meta.events) do
		local message = typeof(event) == "table"
				and tostring(event.message or event.path or event.status or "sync")
			or tostring(event)
		table.insert(historyLines, 1, os.date("%H:%M:%S") .. "  " .. message)
	end
	while #historyLines > MAX_HISTORY_LINES do
		table.remove(historyLines)
	end
	historyLabel.Text = #historyLines > 0 and table.concat(historyLines, "\n") or "No sync activity yet."
end

local function updateProgress(meta, isError)
	local value = typeof(meta) == "table" and math.clamp(tonumber(meta.progress) or 0, 0, 100) or 0
	local indeterminate = typeof(meta) == "table" and meta.indeterminate == true
	local phase = typeof(meta) == "table" and tostring(meta.phase or "") or ""
	local current = typeof(meta) == "table" and math.max(0, math.floor(tonumber(meta.current) or 0)) or 0
	local total = typeof(meta) == "table" and math.max(0, math.floor(tonumber(meta.total) or 0)) or 0
	if progressTween then
		progressTween:Cancel()
		progressTween = nil
	end
	progressTrack.Visible = indeterminate or value > 0
	progressCaption.Visible = progressTrack.Visible
	progressFill.BackgroundColor3 = isError and COLORS.Red or COLORS.Cyan
	progressFill.Position = UDim2.new(0, 0, 0, 0)
	if indeterminate then
		progressFill.Size = UDim2.new(0.34, 0, 1, 0)
		progressTween = TweenService:Create(
			progressFill,
			TweenInfo.new(1.1, Enum.EasingStyle.Linear, Enum.EasingDirection.InOut, -1, false),
			{ Position = UDim2.new(0.66, 0, 0, 0) }
		)
		progressTween:Play()
	else
		progressFill.Size = UDim2.new(value / 100, 0, 1, 0)
	end
	local bits = {}
	if phase ~= "" then
		local phaseLabel = string.gsub(phase, "_", " ")
		table.insert(bits, phaseLabel)
	end
	if total > 0 then
		table.insert(bits, tostring(current) .. "/" .. tostring(total))
	end
	if not indeterminate and value > 0 then
		table.insert(bits, tostring(math.floor(value + 0.5)) .. "%")
	end
	progressCaption.Text = table.concat(bits, "  ·  ")
end

updateStatus = function(text, isError, meta)
	local raw = tostring(text or "")
	statusHeadline.Text = isError and "● Issue" or (client and client:isRunning() and "● Connected" or "● Ready")
	statusHeadline.TextColor3 = isError and COLORS.Red or (client and client:isRunning() and COLORS.Green or COLORS.Text)
	statusDetail.Text = raw
	statusDetail.TextColor3 = isError and COLORS.Red or COLORS.Muted
	updateProgress(meta, isError)
	appendHistory(meta)
	if isError then
		warn("[RiftSync] " .. raw)
	end
	if client then
		setProfileControlsLocked(client:isRunning())
		updateExecUI()
	end
	if isError and (string.find(raw, "duplicate_stable_id", 1, true) or string.find(raw, "dipakai oleh lebih dari satu instance", 1, true)) then
		local lastDups = client and client.getLastDuplicateInstances and client:getLastDuplicateInstances() or {}
		if #lastDups > 0 then
			local labelText = string.format("Clean Duplicate IDs (%d objects)", #lastDups)
			cleanDuplicatesStatusButton.Text = labelText
			cleanDuplicatesStatusButton.Visible = true
			cleanDuplicatesStartButton.Text = labelText
			cleanDuplicatesStartButton.Visible = true
		elseif client and client.detectStudioDuplicateIds then
			task.spawn(function()
				local detected = client:detectStudioDuplicateIds()
				if detected.count > 0 then
					local labelText = string.format("Clean Duplicate IDs (%d objects)", detected.count)
					cleanDuplicatesStatusButton.Text = labelText
					cleanDuplicatesStatusButton.Visible = true
					cleanDuplicatesStartButton.Text = labelText
					cleanDuplicatesStartButton.Visible = true
				end
			end)
		end
	end
end

local function refreshDuplicateButtons()
	if not client then
		cleanDuplicatesStatusButton.Visible = false
		cleanDuplicatesStartButton.Visible = false
		return
	end
	local lastDups = client.getLastDuplicateInstances and client:getLastDuplicateInstances() or {}
	local count = #lastDups
	if count > 0 then
		local btnText = string.format("Clean Duplicate IDs (%d objects)", count)
		cleanDuplicatesStatusButton.Text = btnText
		cleanDuplicatesStatusButton.Visible = true
		cleanDuplicatesStartButton.Text = btnText
		cleanDuplicatesStartButton.Visible = true
	else
		cleanDuplicatesStatusButton.Visible = false
		cleanDuplicatesStartButton.Visible = false
	end
end

local function onCleanDuplicatesClicked()
	if not client or not client.cleanDuplicateStudioIds then
		return
	end
	cleanDuplicatesStatusButton.Active = false
	cleanDuplicatesStartButton.Active = false
	cleanDuplicatesStatusButton.Text = "Cleaning duplicate IDs..."
	cleanDuplicatesStartButton.Text = "Cleaning duplicate IDs..."
	task.spawn(function()
		local cleaned, cleanErr = client:cleanDuplicateStudioIds()
		cleanDuplicatesStatusButton.Active = true
		cleanDuplicatesStartButton.Active = true
		if cleanErr then
			updateStatus("Failed to clean duplicates: " .. tostring(cleanErr), true)
		elseif cleaned and cleaned > 0 then
			refreshDuplicateButtons()
			updateStatus(
				string.format("Selesai membersihkan %d objek duplikat! Silakan klik Start lagi.", cleaned),
				false
			)
		else
			refreshDuplicateButtons()
			updateStatus("Tidak ditemukan objek duplikat. Silakan klik Start.", false)
		end
	end)
end

cleanDuplicatesStatusButton.MouseButton1Click:Connect(onCleanDuplicatesClicked)
cleanDuplicatesStartButton.MouseButton1Click:Connect(onCleanDuplicatesClicked)

local function mark(value)
	return value and "[x]" or "[ ]"
end

local function updateDebug(payload)
	local enabled = typeof(payload) == "table" and payload.enabled == true
	debugToggleButton.Text = enabled and "Debug ON" or "Debug OFF"
	debugToggleButton.BackgroundColor3 = enabled and COLORS.Green or COLORS.SurfaceHigh
	debugToggleButton.TextColor3 = enabled and COLORS.BorderDark or COLORS.Text
	if typeof(payload) ~= "table" then
		return
	end
	local checks = typeof(payload.checks) == "table" and payload.checks or {}
	local server = typeof(payload.server) == "table" and payload.server or {}
	local stats = typeof(payload.stats) == "table" and payload.stats or {}
	statusPath.Text = "root="
		.. tostring(server.sync_root or "-")
		.. "  rev="
		.. tostring(stats.last_applied_rev or server.server_rev or "-")
	debugLabel.Text = table.concat({
		mark(checks.http_server_reachable) .. " HTTP",
		mark(checks.connected_to_server) .. " connected",
		mark(checks.token_valid) .. " token",
		mark(checks.sync_active) .. " sync",
		mark(checks.exec_active) .. " exec",
		mark(checks.edit_mode) .. " edit mode",
	}, "   ")
	if stats.remote_exec_status then
		execStatusLabel.Text = tostring(stats.remote_exec_status)
	end
end

client = SyncAPI.new(plugin, updateStatus, updateDebug)
refreshProfileUI()
updateExecUI()
refreshForceIdentityRepairUI()

toolbarButton.Click:Connect(function()
	widget.Enabled = not widget.Enabled
	if widget.Enabled then
		pcall(function()
			widget:RequestRaise()
		end)
		task.spawn(function()
			refreshLocalProfilesFromApp(false)
		end)
	end
end)

previousProfileButton.MouseButton1Click:Connect(function()
	selectProfileAt(selectedProfileIndex - 1)
end)

nextProfileButton.MouseButton1Click:Connect(function()
	selectProfileAt(selectedProfileIndex + 1)
end)

refreshProfilesButton.MouseButton1Click:Connect(function()
	task.spawn(function()
		refreshLocalProfilesFromApp(true)
	end)
end)

profileEditorToggleButton.MouseButton1Click:Connect(function()
	if client:isRunning() then
		return
	end
	local current = profiles[selectedProfileIndex]
	if current and current.kind == "local" then
		task.spawn(function()
			refreshLocalProfilesFromApp(true)
		end)
		return
	end
	setProfileEditorVisible(not profileEditor.Visible)
	if profileEditor.Visible then
		profileNameInput:CaptureFocus()
	end
end)

addProfileButton.MouseButton1Click:Connect(function()
	if client:isRunning() then
		return
	end
	local created, err = client:upsertConnectionProfile({
		name = "Project " .. tostring(#profiles + 1),
		host = "127.0.0.1",
		port = 8765 + #profiles,
		token = "",
	})
	if not created then
		updateStatus(tostring(err), true)
		return
	end
	client:selectConnectionProfile(created.id)
	refreshProfileUI()
	setProfileEditorVisible(true)
	profileNameInput:CaptureFocus()
end)

saveProfileButton.MouseButton1Click:Connect(saveCurrentProfile)

removeProfileButton.MouseButton1Click:Connect(function()
	local profile = profiles[selectedProfileIndex]
	if not profile then
		return
	end
	local ok, err = client:removeConnectionProfile(profile.id)
	if not ok then
		updateStatus(tostring(err), true)
		return
	end
	refreshProfileUI()
	setProfileEditorVisible(false)
	updateStatus("Connection profile removed. Project files were not changed.", false)
end)

local function detectWorkspaceState()
	local studioItemCount = 0
	local rootsToCheck = {
		game:GetService("Workspace"),
		game:GetService("ReplicatedStorage"),
		game:GetService("ServerScriptService"),
		game:GetService("StarterGui"),
		game:GetService("ServerStorage"),
	}
	for _, root in ipairs(rootsToCheck) do
		for _, child in ipairs(root:GetChildren()) do
			if root == game.Workspace then
				if not child:IsA("Terrain") and not child:IsA("Camera") and child.Name ~= "Baseplate" then
					studioItemCount += 1
				end
			else
				studioItemCount += 1
			end
		end
	end

	pcall(function()
		client:fetchHealth()
	end)

	local localCount = tonumber(client.serverIndexedCount)
	return localCount, studioItemCount
end

local function refreshStartSourceGuidance()
	local localCount, studioCount = detectWorkspaceState()
	startFromStudioButton.Text = "Use Studio"
	startFromStudioButton.BackgroundColor3 = COLORS.Cyan
	startFromLocalButton.Text = "Use Local"
	startFromLocalButton.BackgroundColor3 = COLORS.Green

	if localCount ~= nil and localCount > 0 then
		startSourceExplanation.Text = "Folder lokal: "
			.. tostring(localCount)
			.. " file. Studio: "
			.. tostring(studioCount)
			.. " objek.\n• Use Studio: Mengekspor objek Studio ke folder lokal (backup lokal dibuat).\n• Use Local: Menerapkan file folder lokal ke Studio (Undo tersedia).\nLive sync berjalan otomatis setelah start."
	else
		startSourceExplanation.Text = "Pilih sumber data awal:\n• Use Studio: Mengekspor objek Studio ke folder lokal (backup lokal dibuat).\n• Use Local: Menerapkan file folder lokal ke Studio (Undo tersedia).\nLive sync berjalan otomatis setelah start."
	end
end

startButton.MouseButton1Click:Connect(function()
	if client:isRunning() then
		return
	end
	local current = profiles[selectedProfileIndex]
	if (not current or current.kind ~= "local") and not saveCurrentProfile() then
		return
	end
	pullPreviewPanel.Visible = false
	forceIdentityRepairEnabled = false
	client:setForceIdentityRepair(false)
	refreshForceIdentityRepairUI()
	refreshStartSourceGuidance()
	refreshDuplicateButtons()
	startSourcePanel.Visible = true
	setProfileControlsLocked(true)
	updateStatus("Choose Studio or Local as the source for this start. Nothing has changed yet.", false)
end)

forceIdentityRepairButton.MouseButton1Click:Connect(function()
	forceIdentityRepairEnabled = not forceIdentityRepairEnabled
	refreshForceIdentityRepairUI()
	if forceIdentityRepairEnabled then
		updateStatus(
			"Force ID repair armed for this Start only. Now choose Studio or Local as the source.",
			false
		)
	else
		updateStatus("Force ID repair disabled. Start will use existing identities.", false)
	end
end)

local function startWithMode(mode : string)
	startSourcePanel.Visible = false
	client:setStartMode(mode)
	client:setForceIdentityRepair(forceIdentityRepairEnabled)
	local ok, err = client:start()
	forceIdentityRepairEnabled = false
	refreshForceIdentityRepairUI()
	if not ok then
		client:setForceIdentityRepair(false)
		setProfileControlsLocked(false)
		updateStatus(tostring(err), true)
	else
		setProfileControlsLocked(true)
		updateExecUI()
	end
end

startFromStudioButton.MouseButton1Click:Connect(function()
	startWithMode(START_MODES.StudioToFolder)
end)

startFromLocalButton.MouseButton1Click:Connect(function()
	startWithMode(START_MODES.FolderToStudio)
end)

cancelStartButton.MouseButton1Click:Connect(function()
	startSourcePanel.Visible = false
	forceIdentityRepairEnabled = false
	client:setForceIdentityRepair(false)
	refreshForceIdentityRepairUI()
	setProfileControlsLocked(false)
	refreshProfileUI()
	updateStatus("Start cancelled. Studio and local files were not changed.", false)
end)

stopButton.MouseButton1Click:Connect(function()
	startSourcePanel.Visible = false
	pullPreviewPanel.Visible = false
	forceIdentityRepairEnabled = false
	refreshForceIdentityRepairUI()
	client:stop()
	setProfileControlsLocked(false)
	refreshProfileUI()
	updateExecUI()
end)

snapshotButton.MouseButton1Click:Connect(function()
	local ok, err = client:forceSnapshot()
	if not ok then
		updateStatus("Resync failed: " .. tostring(err), true)
	end
end)

local function previewList(response, key)
	local values = typeof(response[key]) == "table" and response[key] or {}
	local result = {}
	for index = 1, math.min(#values, 3) do
		table.insert(result, tostring(values[index]))
	end
	if #values > 3 then
		table.insert(result, "+" .. tostring(#values - 3) .. " more")
	end
	return #result > 0 and table.concat(result, ", ") or "-"
end

pullStudioButton.MouseButton1Click:Connect(function()
	local ok, previewOrError = client:previewPullStudioToLocal()
	if not ok then
		updateStatus("Pull Studio preview failed: " .. tostring(previewOrError), true)
		return
	end
	local response = typeof(previewOrError) == "table" and (previewOrError.response or previewOrError) or {}
	pullPreviewCountLabel.Text = "add "
		.. tostring(response.add_count or 0)
		.. "  update "
		.. tostring(response.update_count or 0)
		.. "  delete "
		.. tostring(response.delete_count or 0)
		.. "  unchanged "
		.. tostring(response.unchanged_count or 0)
	pullPreviewSamplesLabel.Text = "add: "
		.. previewList(response, "files_to_add")
		.. "\nupdate: "
		.. previewList(response, "files_to_update")
		.. "\ndelete: "
		.. previewList(response, "files_to_delete")
	pullPreviewPanel.Visible = true
end)

confirmPullButton.MouseButton1Click:Connect(function()
	local ok, err = client:confirmPullStudioToLocal()
	pullPreviewPanel.Visible = false
	if not ok then
		updateStatus("Pull Studio failed: " .. tostring(err), true)
	end
end)

cancelPullButton.MouseButton1Click:Connect(function()
	client:cancelPullStudioToLocal()
	pullPreviewPanel.Visible = false
	updateStatus("Pull Studio cancelled. Local files were not changed.", false)
end)

execToggleButton.MouseButton1Click:Connect(function()
	if client:isRunning() == false then
		updateStatus("Start sync before enabling Remote Exec.", true)
		return
	end
	client:setRemoteExecToken(execTokenInput.Text)
	client:setRemoteExecEnabled(not client:isRemoteExecEnabled())
	updateExecUI()
end)

debugToggleButton.MouseButton1Click:Connect(function()
	client:setDebugEnabled(not client:isDebugEnabled())
	if client:isDebugEnabled() and client.refreshServerDebugState then
		client:refreshServerDebugState(true)
	end
end)

debugDisclosureButton.MouseButton1Click:Connect(function()
	setDebugBodyVisible(not debugBody.Visible)
end)

if plugin.Unloading then
	plugin.Unloading:Connect(function()
		client:stop()
	end)
end
