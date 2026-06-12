--------------------------------------------------------------------------------------------
-- RiftSyncPlugin v3.7.0
-- One-way sync plugin: local folder -> Roblox Studio
--------------------------------------------------------------------------------------------

local apiModule = script:WaitForChild("API")
local SyncAPI = require(apiModule)
local TypeList = require(apiModule:WaitForChild("TypeList"))

if not plugin then
	warn("[RiftSync] Script berjalan di luar Plugin context, di-skip.")
	return
end

local FALLBACK_HOST = "127.0.0.1"
local FALLBACK_PORT = 8765
local START_MODES = TypeList.START_MODES or {
	StudioToFolder = "studio_to_folder",
	FolderToStudio = "folder_to_studio",
}

local function safeHostText(value)
	local text = tostring(value or FALLBACK_HOST)
	if text == "" then
		return FALLBACK_HOST
	end
	return text
end

local function safePortNumber(value)
	local numeric = tonumber(value)
	if not numeric then
		return FALLBACK_PORT
	end
	numeric = math.floor(numeric)
	if numeric < 1 or numeric > 65535 then
		return FALLBACK_PORT
	end
	return numeric
end

local function normalizeMode(value)
	if TypeList.normaliseStartMode then
		return TypeList.normaliseStartMode(value)
	end

	if value == START_MODES.StudioToFolder then
		return START_MODES.StudioToFolder
	end
	return START_MODES.FolderToStudio
end

local function getModeLabelText(value)
	if TypeList.startModeLabel then
		return TypeList.startModeLabel(value)
	end
	if normalizeMode(value) == START_MODES.StudioToFolder then
		return "Studio -> Folder (override lokal)"
	end
	return "Folder -> Studio (pakai file lokal)"
end

local toolbar = plugin:CreateToolbar("RiftSync")
local toolbarButton = toolbar:CreateButton("Sync", "Open RiftSync Studio", tostring(TypeList.PLUGIN_ICON or ""), "RiftSync")

local widgetInfo = DockWidgetPluginGuiInfo.new(
	Enum.InitialDockState.Float,
	false,
	false,
	420,
	660,
	360,
	620
)

local widget = plugin:CreateDockWidgetPluginGui("RiftSyncWidget", widgetInfo)
widget.Title = "RiftSync Studio v" .. tostring(TypeList.VERSION)
widget.Enabled = false

local COLORS = {
	AppBg = Color3.fromRGB(15, 17, 23),
	Panel = Color3.fromRGB(25, 29, 39),
	PanelSoft = Color3.fromRGB(30, 35, 47),
	Border = Color3.fromRGB(58, 66, 84),
	Title = Color3.fromRGB(247, 248, 255),
	Muted = Color3.fromRGB(158, 166, 187),
	Text = Color3.fromRGB(226, 231, 245),
	InputBg = Color3.fromRGB(20, 23, 31),
	InputBorder = Color3.fromRGB(67, 76, 96),
	StatusBg = Color3.fromRGB(24, 30, 39),
	StatusText = Color3.fromRGB(222, 247, 240),
	StatusError = Color3.fromRGB(255, 132, 160),
	StatusIdle = Color3.fromRGB(181, 190, 211),
	StatusSync = Color3.fromRGB(142, 213, 255),
	StatusGood = Color3.fromRGB(128, 232, 184),
	ProgressTrack = Color3.fromRGB(36, 42, 56),
	ProgressFill = Color3.fromRGB(138, 213, 255),
	DebugBg = Color3.fromRGB(22, 26, 36),
	DebugBorder = Color3.fromRGB(55, 64, 82),
	DebugText = Color3.fromRGB(224, 230, 246),
	DebugMuted = Color3.fromRGB(152, 163, 187),
	DebugOn = Color3.fromRGB(75, 190, 143),
	DebugOff = Color3.fromRGB(70, 78, 98),
	Start = Color3.fromRGB(92, 216, 166),
	Snapshot = Color3.fromRGB(128, 165, 255),
	Pull = Color3.fromRGB(137, 215, 255),
	Stop = Color3.fromRGB(239, 105, 137),
	ButtonText = Color3.fromRGB(14, 18, 24),
	ButtonMutedText = Color3.fromRGB(236, 240, 250),
	BadgeBg = Color3.fromRGB(42, 48, 64),
	AccentLavender = Color3.fromRGB(185, 165, 255),
	AccentBlush = Color3.fromRGB(255, 159, 181),
}

local function applyCorner(target, radius)
	local corner = Instance.new("UICorner")
	corner.CornerRadius = UDim.new(0, radius)
	corner.Parent = target
end

local function applyStroke(target, color, thickness)
	local stroke = Instance.new("UIStroke")
	stroke.Color = color
	stroke.Thickness = thickness
	stroke.ApplyStrokeMode = Enum.ApplyStrokeMode.Border
	stroke.Parent = target
end

local function applyPadding(target, left, top, right, bottom)
	local padding = Instance.new("UIPadding")
	padding.PaddingLeft = UDim.new(0, left)
	padding.PaddingTop = UDim.new(0, top)
	padding.PaddingRight = UDim.new(0, right)
	padding.PaddingBottom = UDim.new(0, bottom)
	padding.Parent = target
	return padding
end

local function applyList(target, direction, padding)
	local layout = Instance.new("UIListLayout")
	layout.FillDirection = direction
	layout.SortOrder = Enum.SortOrder.LayoutOrder
	layout.Padding = UDim.new(0, padding)
	layout.Parent = target
	return layout
end

local function makePanel(parent, height, background, order)
	local frame = Instance.new("Frame")
	frame.Size = UDim2.new(1, 0, 0, height)
	frame.BackgroundColor3 = background or COLORS.Panel
	frame.BorderSizePixel = 0
	frame.LayoutOrder = order or 0
	frame.Parent = parent
	applyCorner(frame, 10)
	applyStroke(frame, COLORS.Border, 1)
	return frame
end

local function makeText(parent, text, size, color, font, order)
	local label = Instance.new("TextLabel")
	label.BackgroundTransparency = 1
	label.Text = text
	label.TextSize = size
	label.TextColor3 = color
	label.Font = font or Enum.Font.Gotham
	label.TextXAlignment = Enum.TextXAlignment.Left
	label.TextYAlignment = Enum.TextYAlignment.Center
	label.TextTruncate = Enum.TextTruncate.AtEnd
	label.LayoutOrder = order or 0
	label.Parent = parent
	return label
end

local function styleButton(button, color, textColor)
	button.BackgroundColor3 = color
	button.BorderSizePixel = 0
	button.TextColor3 = textColor or COLORS.ButtonMutedText
	button.Font = Enum.Font.GothamBold
	button.TextSize = 12
	button.AutoButtonColor = true
	applyCorner(button, 10)
end

local container = Instance.new("Frame")
container.Size = UDim2.fromScale(1, 1)
container.BackgroundColor3 = COLORS.AppBg
container.BorderSizePixel = 0
container.Parent = widget

local card = Instance.new("Frame")
card.Size = UDim2.new(1, -24, 1, -24)
card.Position = UDim2.new(0, 12, 0, 12)
card.BackgroundTransparency = 1
card.BorderSizePixel = 0
card.Parent = container

local headerFrame = Instance.new("Frame")
headerFrame.Size = UDim2.new(1, 0, 0, 46)
headerFrame.Position = UDim2.new(0, 0, 0, 0)
headerFrame.BackgroundTransparency = 1
headerFrame.LayoutOrder = 1
headerFrame.Parent = card

local titleLabel = makeText(headerFrame, "RiftSync Studio", 22, COLORS.Title, Enum.Font.GothamBold, 1)
titleLabel.Size = UDim2.new(0.5, -6, 0, 24)
titleLabel.Position = UDim2.new(0, 0, 0, 0)
titleLabel.Font = Enum.Font.GothamBold

local versionBadge = Instance.new("TextLabel")
versionBadge.Size = UDim2.new(0, 70, 0, 24)
versionBadge.Position = UDim2.new(1, -70, 0, 0)
versionBadge.BackgroundColor3 = COLORS.BadgeBg
versionBadge.BorderSizePixel = 0
versionBadge.Text = "v" .. tostring(TypeList.VERSION)
versionBadge.TextColor3 = COLORS.AccentLavender
versionBadge.Font = Enum.Font.GothamBold
versionBadge.TextSize = 12
versionBadge.TextXAlignment = Enum.TextXAlignment.Center
versionBadge.Parent = headerFrame
applyCorner(versionBadge, 8)

local subtitleLabel = makeText(headerFrame, "Live sync for Roblox Studio.", 12, COLORS.Muted, Enum.Font.Gotham, 2)
subtitleLabel.Size = UDim2.new(0.72, 0, 0, 18)
subtitleLabel.Position = UDim2.new(0, 0, 0, 26)
subtitleLabel.TextSize = 12

local headerHealthLabel = makeText(headerFrame, "Offline", 12, COLORS.DebugMuted, Enum.Font.GothamMedium, 3)
headerHealthLabel.Size = UDim2.new(0, 94, 0, 18)
headerHealthLabel.Position = UDim2.new(1, -94, 0, 26)
headerHealthLabel.TextXAlignment = Enum.TextXAlignment.Right

local connectionFrame = makePanel(card, 76, COLORS.Panel, 2)
connectionFrame.Position = UDim2.new(0, 0, 0, 58)
applyPadding(connectionFrame, 10, 10, 10, 10)

local connectionLayout = Instance.new("UIGridLayout")
connectionLayout.CellPadding = UDim2.new(0, 8, 0, 8)
connectionLayout.CellSize = UDim2.new(0.5, -4, 1, 0)
connectionLayout.SortOrder = Enum.SortOrder.LayoutOrder
connectionLayout.Parent = connectionFrame

local hostBox = Instance.new("Frame")
hostBox.BackgroundTransparency = 1
hostBox.LayoutOrder = 1
hostBox.Parent = connectionFrame
applyList(hostBox, Enum.FillDirection.Vertical, 4)

local hostLabel = makeText(hostBox, "Host", 11, COLORS.Muted, Enum.Font.GothamMedium, 1)
hostLabel.Size = UDim2.new(1, 0, 0, 14)

local hostInput = Instance.new("TextBox")
hostInput.Size = UDim2.new(1, 0, 0, 32)
hostInput.BackgroundColor3 = COLORS.InputBg
hostInput.BorderSizePixel = 0
hostInput.TextColor3 = COLORS.Text
hostInput.ClearTextOnFocus = false
hostInput.TextXAlignment = Enum.TextXAlignment.Center
hostInput.Font = Enum.Font.Code
hostInput.TextSize = 12
hostInput.Text = safeHostText(TypeList.DEFAULT_HOST)
hostInput.Parent = hostBox
applyCorner(hostInput, 8)
applyStroke(hostInput, COLORS.InputBorder, 1)

local portBox = Instance.new("Frame")
portBox.BackgroundTransparency = 1
portBox.LayoutOrder = 2
portBox.Parent = connectionFrame
applyList(portBox, Enum.FillDirection.Vertical, 4)

local portLabel = makeText(portBox, "Port", 11, COLORS.Muted, Enum.Font.GothamMedium, 1)
portLabel.Size = UDim2.new(1, 0, 0, 14)

local portInput = Instance.new("TextBox")
portInput.Size = UDim2.new(1, 0, 0, 32)
portInput.BackgroundColor3 = COLORS.InputBg
portInput.BorderSizePixel = 0
portInput.TextColor3 = COLORS.Text
portInput.ClearTextOnFocus = false
portInput.TextXAlignment = Enum.TextXAlignment.Center
portInput.Font = Enum.Font.Code
portInput.TextSize = 12
portInput.Text = tostring(safePortNumber(TypeList.DEFAULT_PORT))
portInput.Parent = portBox
applyCorner(portInput, 8)
applyStroke(portInput, COLORS.InputBorder, 1)

local modeLabel = Instance.new("TextLabel")
modeLabel.Size = UDim2.new(0, 0, 0, 0)
modeLabel.Position = UDim2.new(0, 12, 0, 138)
modeLabel.BackgroundTransparency = 1
modeLabel.Font = Enum.Font.GothamMedium
modeLabel.TextSize = 12
modeLabel.TextXAlignment = Enum.TextXAlignment.Left
modeLabel.TextColor3 = COLORS.Text
modeLabel.Text = "Mode"
modeLabel.Visible = false
modeLabel.LayoutOrder = 0

local modeButton = Instance.new("TextButton")
modeButton.Size = UDim2.new(0, 0, 0, 0)
modeButton.Position = UDim2.new(0.22, 0, 0, 134)
modeButton.BackgroundColor3 = COLORS.BadgeBg
modeButton.BorderSizePixel = 0
modeButton.TextColor3 = Color3.fromRGB(169, 211, 255)
modeButton.Font = Enum.Font.GothamMedium
modeButton.TextSize = 12
modeButton.Text = "Mode Start"
modeButton.Visible = false
modeButton.LayoutOrder = 0
applyCorner(modeButton, 10)
applyStroke(modeButton, COLORS.Border, 1)

local debugBox = Instance.new("Frame")
debugBox.BackgroundTransparency = 1
debugBox.LayoutOrder = 3
debugBox.Parent = connectionFrame
debugBox.Visible = false
applyList(debugBox, Enum.FillDirection.Vertical, 4)

local debugLabel = makeText(debugBox, "Debug", 11, COLORS.Muted, Enum.Font.GothamMedium, 1)
debugLabel.Size = UDim2.new(1, 0, 0, 14)

local debugToggleButton = Instance.new("TextButton")
debugToggleButton.Size = UDim2.new(1, 0, 0, 32)
debugToggleButton.BorderSizePixel = 0
debugToggleButton.TextColor3 = COLORS.ButtonText
debugToggleButton.Font = Enum.Font.GothamBold
debugToggleButton.TextSize = 12
debugToggleButton.Text = "Debug OFF"
debugToggleButton.Parent = debugBox
applyCorner(debugToggleButton, 8)
debugToggleButton.BackgroundColor3 = COLORS.DebugOff

local execFrame = makePanel(card, 58, COLORS.PanelSoft, 3)
execFrame.Position = UDim2.new(0, 0, 0, 146)

local execTitleLabel = makeText(execFrame, "Exec Token", 11, COLORS.Muted, Enum.Font.GothamMedium, 1)
execTitleLabel.Size = UDim2.new(0, 86, 0, 18)
execTitleLabel.Position = UDim2.new(0, 10, 0, 7)

local execStatusLabel = makeText(execFrame, "Exec disabled", 11, COLORS.DebugMuted, Enum.Font.GothamMedium, 2)
execStatusLabel.Size = UDim2.new(1, -106, 0, 18)
execStatusLabel.Position = UDim2.new(0, 96, 0, 7)
execStatusLabel.TextXAlignment = Enum.TextXAlignment.Right

local execTokenInput = Instance.new("TextBox")
execTokenInput.Size = UDim2.new(1, -124, 0, 28)
execTokenInput.Position = UDim2.new(0, 10, 0, 25)
execTokenInput.BackgroundColor3 = COLORS.InputBg
execTokenInput.BorderSizePixel = 0
execTokenInput.TextColor3 = COLORS.Text
execTokenInput.PlaceholderText = "remote_exec_token"
execTokenInput.PlaceholderColor3 = COLORS.DebugMuted
execTokenInput.ClearTextOnFocus = false
execTokenInput.TextXAlignment = Enum.TextXAlignment.Left
execTokenInput.Font = Enum.Font.Code
execTokenInput.TextSize = 11
execTokenInput.Text = ""
execTokenInput.Parent = execFrame
applyCorner(execTokenInput, 8)
applyStroke(execTokenInput, COLORS.InputBorder, 1)

local execToggleButton = Instance.new("TextButton")
execToggleButton.Size = UDim2.new(0, 88, 0, 28)
execToggleButton.Position = UDim2.new(1, -98, 0, 25)
execToggleButton.BorderSizePixel = 0
execToggleButton.TextColor3 = COLORS.ButtonText
execToggleButton.Font = Enum.Font.GothamBold
execToggleButton.TextSize = 11
execToggleButton.Text = "Exec OFF"
execToggleButton.BackgroundColor3 = COLORS.DebugOff
execToggleButton.Parent = execFrame
applyCorner(execToggleButton, 8)

local statusFrame = makePanel(card, 198, COLORS.StatusBg, 3)
statusFrame.Position = UDim2.new(0, 0, 0, 216)
statusFrame.Size = UDim2.new(1, 0, 0, 68)

local statusTopRow = Instance.new("Frame")
statusTopRow.Size = UDim2.new(1, -20, 0, 28)
statusTopRow.Position = UDim2.new(0, 10, 0, 10)
statusTopRow.BackgroundTransparency = 1
statusTopRow.Parent = statusFrame

local statusStateLabel = makeText(statusTopRow, "Not Synced", 16, COLORS.StatusError, Enum.Font.GothamBold, 1)
statusStateLabel.Size = UDim2.new(0.42, 0, 1, 0)

local statusLabel = makeText(statusTopRow, "Idle", 12, COLORS.StatusText, Enum.Font.GothamMedium, 2)
statusLabel.Size = UDim2.new(0.58, 0, 1, 0)
statusLabel.Position = UDim2.new(0.42, 0, 0, 0)
statusLabel.TextXAlignment = Enum.TextXAlignment.Right

local progressTrack = Instance.new("Frame")
progressTrack.Size = UDim2.new(1, -20, 0, 7)
progressTrack.Position = UDim2.new(0, 10, 0, 44)
progressTrack.BackgroundColor3 = COLORS.ProgressTrack
progressTrack.BorderSizePixel = 0
progressTrack.Visible = false
progressTrack.Parent = statusFrame
applyCorner(progressTrack, 5)

local progressFill = Instance.new("Frame")
progressFill.Size = UDim2.new(0, 0, 1, 0)
progressFill.Position = UDim2.new(0, 0, 0, 0)
progressFill.BackgroundColor3 = COLORS.ProgressFill
progressFill.BorderSizePixel = 0
progressFill.Parent = progressTrack
applyCorner(progressFill, 5)

local historyPanel = Instance.new("Frame")
historyPanel.Size = UDim2.new(1, -20, 1, -64)
historyPanel.Position = UDim2.new(0, 10, 0, 54)
historyPanel.BackgroundColor3 = COLORS.Panel
historyPanel.BorderSizePixel = 0
historyPanel.ClipsDescendants = false
historyPanel.Visible = false
historyPanel.Parent = statusFrame
applyCorner(historyPanel, 10)
applyStroke(historyPanel, COLORS.DebugBorder, 1)

local historyTitleLabel = makeText(historyPanel, "History", 12, COLORS.Text, Enum.Font.GothamBold, 1)
historyTitleLabel.Size = UDim2.new(1, -244, 0, 28)
historyTitleLabel.Position = UDim2.new(0, 10, 0, 8)

local historyDropdownButton = Instance.new("TextButton")
historyDropdownButton.Size = UDim2.new(0, 220, 0, 28)
historyDropdownButton.Position = UDim2.new(1, -230, 0, 8)
historyDropdownButton.BackgroundColor3 = COLORS.BadgeBg
historyDropdownButton.BorderSizePixel = 0
historyDropdownButton.TextColor3 = COLORS.StatusText
historyDropdownButton.Font = Enum.Font.GothamMedium
historyDropdownButton.TextSize = 12
historyDropdownButton.TextXAlignment = Enum.TextXAlignment.Center
historyDropdownButton.Text = "Rev: local history"
historyDropdownButton.ZIndex = 5
historyDropdownButton.Visible = false
historyDropdownButton.Parent = historyPanel
applyCorner(historyDropdownButton, 8)
applyStroke(historyDropdownButton, COLORS.DebugBorder, 1)

local historyDropdownFrame = Instance.new("ScrollingFrame")
historyDropdownFrame.Size = UDim2.new(0, 220, 0, 116)
historyDropdownFrame.Position = UDim2.new(1, -230, 0, 40)
historyDropdownFrame.BackgroundColor3 = COLORS.DebugBg
historyDropdownFrame.BorderSizePixel = 0
historyDropdownFrame.ScrollBarThickness = 4
historyDropdownFrame.CanvasSize = UDim2.new(0, 0, 0, 0)
historyDropdownFrame.Visible = false
historyDropdownFrame.ZIndex = 10
historyDropdownFrame.Parent = historyPanel
applyCorner(historyDropdownFrame, 8)
applyStroke(historyDropdownFrame, COLORS.DebugBorder, 1)

local historyDropdownLayout = Instance.new("UIListLayout")
historyDropdownLayout.SortOrder = Enum.SortOrder.LayoutOrder
historyDropdownLayout.Padding = UDim.new(0, 2)
historyDropdownLayout.Parent = historyDropdownFrame

local statusHistoryLabel = makeText(historyPanel, "History tersedia di RiftSync app.", 12, COLORS.DebugMuted, Enum.Font.GothamMedium, 3)
statusHistoryLabel.Size = UDim2.new(1, -20, 1, -52)
statusHistoryLabel.Position = UDim2.new(0, 10, 0, 42)
statusHistoryLabel.TextSize = 12
statusHistoryLabel.RichText = true
statusHistoryLabel.TextYAlignment = Enum.TextYAlignment.Top
statusHistoryLabel.Visible = true

local metaFrame = makePanel(card, 82, COLORS.PanelSoft, 4)
metaFrame.Position = UDim2.new(0, 0, 1, -146)
metaFrame.Visible = false
applyPadding(metaFrame, 10, 8, 10, 8)

local syncPathLabel = makeText(metaFrame, "Root: -", 11, COLORS.DebugText, Enum.Font.Code, 1)
syncPathLabel.Size = UDim2.new(1, 0, 0, 18)
syncPathLabel.Position = UDim2.new(0, 0, 0, 0)
syncPathLabel.TextSize = 11

local watcherLabel = makeText(metaFrame, "Server waiting | Watcher idle", 11, COLORS.DebugMuted, Enum.Font.Code, 2)
watcherLabel.Size = UDim2.new(1, 0, 0, 18)
watcherLabel.Position = UDim2.new(0, 0, 0, 20)
watcherLabel.TextSize = 11

local lastOpLabel = makeText(metaFrame, "Rev - | Git -", 11, COLORS.DebugMuted, Enum.Font.Code, 3)
lastOpLabel.Size = UDim2.new(1, 0, 0, 18)
lastOpLabel.Position = UDim2.new(0, 0, 0, 40)
lastOpLabel.TextSize = 11

local errorLabel = makeText(metaFrame, "Health: waiting", 11, COLORS.DebugMuted, Enum.Font.Code, 4)
errorLabel.Size = UDim2.new(1, 0, 0, 18)
errorLabel.Position = UDim2.new(0, 0, 0, 60)
errorLabel.TextSize = 11

local debugFrame = makePanel(card, 96, COLORS.DebugBg, 5)
debugFrame.Visible = false
debugFrame.Size = UDim2.new(1, 0, 0, 0)
debugFrame.Position = UDim2.new(0, 0, 1, -252)
applyPadding(debugFrame, 10, 8, 10, 8)

local debugChecklistLabel = makeText(debugFrame, "[ ] HTTP/server reachable  [ ] connected to server  [ ] token valid\n[ ] sync active  [ ] exec active  [ ] edit mode", 11, COLORS.DebugText, Enum.Font.Code, 1)
debugChecklistLabel.Size = UDim2.new(1, 0, 0, 28)
debugChecklistLabel.Position = UDim2.new(0, 0, 0, 0)
debugChecklistLabel.TextWrapped = true
debugChecklistLabel.TextSize = 11
debugChecklistLabel.TextYAlignment = Enum.TextYAlignment.Top

local debugServerLabel = makeText(debugFrame, "Debug nonaktif.", 11, COLORS.DebugMuted, Enum.Font.Code, 2)
debugServerLabel.Size = UDim2.new(1, 0, 0, 22)
debugServerLabel.Position = UDim2.new(0, 0, 0, 34)
debugServerLabel.TextSize = 11
debugServerLabel.TextYAlignment = Enum.TextYAlignment.Top

local debugEventsLabel = makeText(debugFrame, "Aktifkan debug untuk lihat detail request ACK/changes.", 11, COLORS.DebugMuted, Enum.Font.Code, 3)
debugEventsLabel.Size = UDim2.new(1, 0, 0, 24)
debugEventsLabel.Position = UDim2.new(0, 0, 0, 60)
debugEventsLabel.TextSize = 11
debugEventsLabel.TextYAlignment = Enum.TextYAlignment.Top

local pullPreviewFrame = makePanel(card, 118, COLORS.PanelSoft, 6)
pullPreviewFrame.Position = UDim2.new(0, 0, 0, 354)
pullPreviewFrame.Visible = false
applyPadding(pullPreviewFrame, 10, 8, 10, 8)

local pullPreviewTitleLabel = makeText(pullPreviewFrame, "Pull Studio preview", 12, COLORS.Text, Enum.Font.GothamBold, 1)
pullPreviewTitleLabel.Size = UDim2.new(0.55, 0, 0, 18)
pullPreviewTitleLabel.Position = UDim2.new(0, 0, 0, 0)

local pullPreviewCountLabel = makeText(pullPreviewFrame, "add 0 | update 0 | delete 0 | unchanged 0", 11, COLORS.DebugText, Enum.Font.Code, 2)
pullPreviewCountLabel.Size = UDim2.new(0.45, 0, 0, 18)
pullPreviewCountLabel.Position = UDim2.new(0.55, 0, 0, 0)
pullPreviewCountLabel.TextXAlignment = Enum.TextXAlignment.Right

local pullPreviewSamplesLabel = makeText(pullPreviewFrame, "Collecting preview...", 11, COLORS.DebugMuted, Enum.Font.Code, 3)
pullPreviewSamplesLabel.Size = UDim2.new(1, -126, 0, 72)
pullPreviewSamplesLabel.Position = UDim2.new(0, 0, 0, 28)
pullPreviewSamplesLabel.TextWrapped = true
pullPreviewSamplesLabel.TextYAlignment = Enum.TextYAlignment.Top

local confirmPullButton = Instance.new("TextButton")
confirmPullButton.Size = UDim2.new(0, 58, 0, 28)
confirmPullButton.Position = UDim2.new(1, -122, 1, -32)
confirmPullButton.Text = "Confirm"
confirmPullButton.Parent = pullPreviewFrame
styleButton(confirmPullButton, COLORS.Pull, COLORS.ButtonText)

local cancelPullButton = Instance.new("TextButton")
cancelPullButton.Size = UDim2.new(0, 56, 0, 28)
cancelPullButton.Position = UDim2.new(1, -56, 1, -32)
cancelPullButton.Text = "Cancel"
cancelPullButton.Parent = pullPreviewFrame
styleButton(cancelPullButton, COLORS.DebugOff, COLORS.ButtonMutedText)

local buttonRow = Instance.new("Frame")
buttonRow.Size = UDim2.new(1, 0, 0, 42)
buttonRow.Position = UDim2.new(0, 0, 0, 300)
buttonRow.BackgroundTransparency = 1
buttonRow.LayoutOrder = 7
buttonRow.Parent = card
applyList(buttonRow, Enum.FillDirection.Horizontal, 8)

local startButton = Instance.new("TextButton")
startButton.Size = UDim2.new(0.31, -6, 1, 0)
startButton.Text = "Start Sync"
startButton.Parent = buttonRow
styleButton(startButton, COLORS.Start, COLORS.ButtonText)

local snapshotButton = Instance.new("TextButton")
snapshotButton.Size = UDim2.new(0.23, -8, 1, 0)
snapshotButton.Text = "Resync"
snapshotButton.Parent = buttonRow
styleButton(snapshotButton, COLORS.Snapshot, COLORS.ButtonMutedText)

local pullStudioButton = Instance.new("TextButton")
pullStudioButton.Size = UDim2.new(0.23, -8, 1, 0)
pullStudioButton.Text = "Pull Studio"
pullStudioButton.Parent = buttonRow
styleButton(pullStudioButton, COLORS.Pull, COLORS.ButtonText)

local stopButton = Instance.new("TextButton")
stopButton.Size = UDim2.new(0.23, -8, 1, 0)
stopButton.Text = "Stop"
stopButton.Parent = buttonRow
styleButton(stopButton, COLORS.Stop, COLORS.ButtonMutedText)

local function applyAdaptivePanelLayout(debugEnabled)
	execFrame.Position = UDim2.new(0, 0, 0, 146)
	statusFrame.Position = UDim2.new(0, 0, 0, 216)
	statusFrame.Size = UDim2.new(1, 0, 0, 68)
	historyPanel.Visible = false
	metaFrame.Visible = false
	debugFrame.Visible = false
	debugFrame.Size = UDim2.new(1, 0, 0, 0)
	buttonRow.Position = UDim2.new(0, 0, 0, 300)
end

applyAdaptivePanelLayout(false)

local MAX_STATUS_LENGTH = 220
local MAX_HISTORY_LINE_LENGTH = 420
local MAX_HISTORY_TEXT_LENGTH = 180000
local MAX_SYNC_HISTORY_LINES = 30
local MAX_REVISION_OPTIONS = 10
local APP_HISTORY_HINT = "History tersedia di RiftSync app."
local lastLoggedError = ""
local syncHistoryLines = {}
local activeHistoryLines = syncHistoryLines
local activeHistoryEmptyText = "Belum ada riwayat sync."
local revisionSummaries = {}
local selectedRevision = nil
local historyRefreshBusy = false
local client = nil

local function updateExecUi(statusOverride)
	local enabled = false
	local statusText = tostring(statusOverride or "Exec disabled")
	if client then
		if client.isRemoteExecEnabled then
			enabled = client:isRemoteExecEnabled()
		end
		if not statusOverride and client.getRemoteExecStatus then
			statusText = tostring(client:getRemoteExecStatus())
		end
	end

	execToggleButton.Text = enabled and "Exec ON" or "Exec OFF"
	execToggleButton.BackgroundColor3 = enabled and COLORS.DebugOn or COLORS.DebugOff
	execToggleButton.TextColor3 = enabled and COLORS.ButtonText or COLORS.ButtonMutedText
	execStatusLabel.Text = statusText

	local lowerStatus = string.lower(statusText)
	if string.find(lowerStatus, "error", 1, true) then
		execStatusLabel.TextColor3 = COLORS.StatusError
	elseif enabled and string.find(lowerStatus, "running", 1, true) then
		execStatusLabel.TextColor3 = COLORS.StatusSync
	elseif enabled then
		execStatusLabel.TextColor3 = COLORS.StatusGood
	else
		execStatusLabel.TextColor3 = COLORS.DebugMuted
	end
end

local function compactStatusText(textValue)
	if #textValue <= MAX_STATUS_LENGTH then
		return textValue
	end
	return string.sub(textValue, 1, MAX_STATUS_LENGTH - 3) .. "..."
end

local function compactHistoryText(textValue)
	if #textValue <= MAX_HISTORY_LINE_LENGTH then
		return textValue
	end
	return string.sub(textValue, 1, MAX_HISTORY_LINE_LENGTH - 3) .. "..."
end

local function escapeRichText(textValue)
	local escaped = tostring(textValue)
	escaped = string.gsub(escaped, "&", "&amp;")
	escaped = string.gsub(escaped, "<", "&lt;")
	escaped = string.gsub(escaped, ">", "&gt;")
	escaped = string.gsub(escaped, "\"", "&quot;")
	return escaped
end

local function mutedRichText(textValue)
	return "<font color=\"#99A8C6\">" .. escapeRichText(textValue) .. "</font>"
end

local function joinHistoryLinesForLabel(lines)
	local selected = {}
	local totalLength = 0
	local hiddenCount = 0
	local reserveLength = 180

	for index, line in ipairs(lines) do
		local lineText = tostring(line)
		local separatorLength = #selected > 0 and 1 or 0
		if totalLength + separatorLength + #lineText + reserveLength > MAX_HISTORY_TEXT_LENGTH then
			hiddenCount = #lines - index + 1
			break
		end

		table.insert(selected, lineText)
		totalLength += separatorLength + #lineText
	end

	if hiddenCount > 0 then
		table.insert(
			selected,
			mutedRichText(
				"... "
					.. tostring(hiddenCount)
					.. " item disembunyikan agar panel history tidak melewati limit teks Roblox."
			)
		)
	end

	return table.concat(selected, "\n")
end

local function compactGuidedError(message)
	local text = tostring(message or "")
	if text == "" then
		return ""
	end
	local first, second = string.match(text, "^([^|]+)%s*|%s*([^|]+)")
	if first and second then
		return compactStatusText(first .. " | " .. second)
	end
	return compactStatusText(text)
end

local function clampProgress(value)
	local numeric = tonumber(value) or 0
	if numeric < 0 then
		return 0
	end
	if numeric > 1 then
		return 1
	end
	return numeric
end

local function extractProgress(rawText, isError)
	local percentText = string.match(rawText, "%((%d+)%%%)")
	if percentText then
		return clampProgress((tonumber(percentText) or 0) / 100)
	end

	local lowered = string.lower(rawText)
	if isError then
		return 1
	end
	if string.find(lowered, "connecting", 1, true) then
		return 0.12
	end
	if string.find(lowered, "checking", 1, true) then
		return 0.18
	end
	if string.find(lowered, "syncing", 1, true) then
		return 0.5
	end
	return 0
end

local function isProgressStatus(rawText, isError)
	if isError then
		return false
	end

	local lowered = string.lower(rawText)
	return string.match(rawText, "%((%d+)%%%)") ~= nil
		and (
			string.find(lowered, "connecting", 1, true) ~= nil
			or string.find(lowered, "syncing", 1, true) ~= nil
		)
end

local function setActiveHistoryLines(lines, emptyText)
	activeHistoryLines = lines or {}
	activeHistoryEmptyText = tostring(emptyText or "Belum ada riwayat sync.")
end

local function refreshSyncHistoryLabel()
	if #activeHistoryLines == 0 then
		statusHistoryLabel.TextColor3 = COLORS.DebugMuted
		statusHistoryLabel.Text = activeHistoryEmptyText
		return
	end

	statusHistoryLabel.TextColor3 = COLORS.StatusText
	statusHistoryLabel.Text = joinHistoryLinesForLabel(activeHistoryLines)
end

local function getHistoryBadge(status)
	local normalized = string.lower(tostring(status or "synced"))
	if normalized == "deleted" or normalized == "delete" then
		return "DEL", "#FF889F"
	elseif normalized == "renamed" or normalized == "rename" then
		return "REN", "#A9D3FF"
	elseif normalized == "conflict" then
		return "WARN", "#FFD166"
	elseif normalized == "error" then
		return "ERR", "#FF889F"
	elseif normalized == "skipped" then
		return "SKIP", "#99A8C6"
	elseif normalized == "summary" then
		return "BATCH", "#7ED5FF"
	end
	return "SYNC", "#7BE1B2"
end

local function formatHistoryLine(stamp, status, message)
	local badgeText, badgeColor = getHistoryBadge(status)
	return "["
		.. stamp
		.. "] <font color=\""
		.. badgeColor
		.. "\">["
		.. badgeText
		.. "]</font> "
		.. escapeRichText(compactHistoryText(message))
end

local function revisionOptionLabel(summary)
	if typeof(summary) ~= "table" then
		return "Local history"
	end

	local rev = tostring(summary.rev or "-")
	local count = tostring(summary.change_count or 0)
	local hash = tostring(summary.git_commit_short or "")
	local label = "Rev " .. rev .. " | " .. count .. " changes"
	if hash ~= "" then
		label ..= " | " .. hash
	end
	return label
end

local function updateHistoryButton(summary)
	if selectedRevision then
		historyDropdownButton.Text = revisionOptionLabel(summary or { rev = selectedRevision })
	else
		historyDropdownButton.Text = "Rev: local history"
	end
end

local function clearHistoryDropdownOptions()
	for _, child in ipairs(historyDropdownFrame:GetChildren()) do
		if child:IsA("TextButton") then
			child:Destroy()
		end
	end
end

local function makeHistoryOption(text, order, onClick)
	local option = Instance.new("TextButton")
	option.Size = UDim2.new(1, -8, 0, 22)
	option.BackgroundColor3 = COLORS.InputBg
	option.BorderSizePixel = 0
	option.TextColor3 = COLORS.Text
	option.Font = Enum.Font.Code
	option.TextSize = 11
	option.TextXAlignment = Enum.TextXAlignment.Left
	option.TextTruncate = Enum.TextTruncate.AtEnd
	option.Text = "  " .. tostring(text)
	option.LayoutOrder = order
	option.ZIndex = 11
	option.Parent = historyDropdownFrame
	applyCorner(option, 6)
	option.MouseButton1Click:Connect(onClick)
	return option
end

local function formatServerChangeLine(revision, change)
	local op = tostring(change.op or "upsert")
	local entity = tostring(change.entity or "script")
	local className = tostring(change.class_name or "")
	local path = tostring(change.new_rbx_path or change.rbx_path or change.old_rbx_path or change.local_path or "-")
	local detail = op .. " " .. entity
	if className ~= "" then
		detail ..= " " .. className
	end
	detail ..= " @ " .. path
	if typeof(change.source_summary) == "table" then
		detail ..= " | " .. tostring(change.source_summary.line_count or 0) .. " lines"
	elseif typeof(change.payload_summary) == "table" then
		local props = tonumber(change.payload_summary.property_count) or 0
		local attrs = tonumber(change.payload_summary.attribute_count) or 0
		local tags = tonumber(change.payload_summary.tag_count) or 0
		detail ..= " | props=" .. tostring(props) .. " attrs=" .. tostring(attrs) .. " tags=" .. tostring(tags)
	end

	local status = "synced"
	if op == "delete" then
		status = "deleted"
	elseif op == "rename" then
		status = "renamed"
	end
	return formatHistoryLine("rev " .. tostring(revision), status, detail)
end

local function renderRevisionDetail(response)
	local revision = tonumber(response.rev) or selectedRevision or 0
	local lines = {}
	local changes = typeof(response.changes) == "table" and response.changes or {}
	for _, change in ipairs(changes) do
		if typeof(change) == "table" then
			table.insert(lines, formatServerChangeLine(revision, change))
		end
	end

	local summary = {
		rev = revision,
		change_count = tonumber(response.change_count) or #lines,
		git_commit_short = tostring(response.git_commit_short or ""),
	}
	selectedRevision = revision
	updateHistoryButton(summary)
	setActiveHistoryLines(lines, "Server belum punya detail untuk Rev " .. tostring(revision) .. ".")
	refreshSyncHistoryLabel()
end

local function showLocalHistory()
	selectedRevision = nil
	updateHistoryButton(nil)
	setActiveHistoryLines({}, APP_HISTORY_HINT)
	refreshSyncHistoryLabel()
end

local function fetchRevisionDetail(revision)
	if not client or not client.fetchRevisionDetail then
		showLocalHistory()
		return
	end

	task.spawn(function()
		local response, err = client:fetchRevisionDetail(revision)
		if not response or response.found == false then
			selectedRevision = tonumber(revision)
			updateHistoryButton({ rev = revision })
			setActiveHistoryLines({}, "History Rev " .. tostring(revision) .. " belum tersedia di server.")
			refreshSyncHistoryLabel()
			if err then
				warn("[RiftSync] Fetch history detail failed: " .. tostring(err))
			end
			return
		end
		renderRevisionDetail(response)
	end)
end

local function populateHistoryDropdown()
	clearHistoryDropdownOptions()
	makeHistoryOption("Local history fallback", 0, function()
		historyDropdownFrame.Visible = false
		showLocalHistory()
	end)

	for index, summary in ipairs(revisionSummaries) do
		if index > MAX_REVISION_OPTIONS then
			break
		end
		makeHistoryOption(revisionOptionLabel(summary), index, function()
			historyDropdownFrame.Visible = false
			fetchRevisionDetail(tonumber(summary.rev) or 0)
		end)
	end

	local rows = math.min(MAX_REVISION_OPTIONS + 1, #revisionSummaries + 1)
	historyDropdownFrame.CanvasSize = UDim2.new(0, 0, 0, rows * 24)
end

local function refreshRevisionHistory(autoSelectLatest)
	revisionSummaries = {}
	selectedRevision = nil
	historyDropdownFrame.Visible = false
	setActiveHistoryLines({}, APP_HISTORY_HINT)
	refreshSyncHistoryLabel()
end

local function appendSyncHistory(events)
	if typeof(events) ~= "table" then
		return
	end

	local stamp = os.date("%H:%M:%S")
	for _, event in ipairs(events) do
		local status = "synced"
		local message = tostring(event)
		if typeof(event) == "table" then
			status = string.lower(tostring(event.status or "synced"))
			message = tostring(event.message or event.path or "")
		end
		if status == "conflict" or status == "error" then
			message = compactGuidedError(message)
		end

		table.insert(syncHistoryLines, 1, formatHistoryLine(stamp, status, message))
	end

	while #syncHistoryLines > MAX_SYNC_HISTORY_LINES do
		table.remove(syncHistoryLines)
	end

	if not selectedRevision then
		setActiveHistoryLines({}, APP_HISTORY_HINT)
		refreshSyncHistoryLabel()
	end
end

local function applyStatusVisual(rawText, isError, meta)
	local lowered = string.lower(rawText)
	local stateText = "Idle"
	local frameColor = COLORS.StatusBg
	local stateColor = COLORS.StatusIdle
	local detailColor = COLORS.StatusText
	local fillColor = COLORS.ProgressFill
	local headerText = "Offline"
	local headerColor = COLORS.DebugMuted
	local showProgress = isProgressStatus(rawText, isError)

	if isError then
		stateText = "Not Synced"
		frameColor = Color3.fromRGB(48, 30, 42)
		stateColor = COLORS.StatusError
		detailColor = COLORS.StatusError
		fillColor = COLORS.StatusError
		headerText = "Issue"
		headerColor = COLORS.StatusError
	elseif string.find(lowered, "syncing", 1, true)
		or string.find(lowered, "connecting", 1, true)
		or string.find(lowered, "checking", 1, true)
	then
		stateText = "Syncing"
		frameColor = Color3.fromRGB(24, 36, 50)
		stateColor = COLORS.StatusSync
		fillColor = COLORS.ProgressFill
		headerText = "Syncing"
		headerColor = COLORS.StatusSync
	elseif string.find(lowered, "synced", 1, true) then
		stateText = "Ready"
		stateColor = COLORS.StatusGood
		fillColor = COLORS.StatusGood
		headerText = "Online"
		headerColor = COLORS.StatusGood
	elseif string.find(lowered, "stopped", 1, true) or string.find(lowered, "idle", 1, true) then
		stateText = "Ready"
		headerText = "Idle"
		headerColor = COLORS.StatusIdle
	end

	statusFrame.BackgroundColor3 = frameColor
	statusStateLabel.Text = stateText
	statusStateLabel.TextColor3 = stateColor
	statusLabel.TextColor3 = detailColor
	headerHealthLabel.Text = headerText
	headerHealthLabel.TextColor3 = headerColor
	progressFill.BackgroundColor3 = fillColor
	progressTrack.Visible = showProgress
	historyPanel.Visible = not showProgress
	if showProgress then
		historyDropdownFrame.Visible = false
		progressFill:TweenSize(
			UDim2.new(extractProgress(rawText, isError), 0, 1, 0),
			Enum.EasingDirection.Out,
			Enum.EasingStyle.Quad,
			0.12,
			true
		)
	else
		progressFill.Size = UDim2.new(0, 0, 1, 0)
		refreshSyncHistoryLabel()
	end
end

local function updateStatus(text, isError, meta)
	local rawText = tostring(text)
	if typeof(meta) == "table" and meta.kind == "history" then
		appendSyncHistory(meta.events)
	end
	statusLabel.Text = compactStatusText(rawText)
	applyStatusVisual(rawText, isError == true, meta)
	if isError then
		if rawText ~= lastLoggedError then
			warn("[RiftSync] " .. rawText)
			lastLoggedError = rawText
		end
	else
		lastLoggedError = ""
	end
end

local function checkMark(value)
	if value then
		return "[x]"
	end
	return "[ ]"
end

local function previewCount(response, keyName)
	if typeof(response) ~= "table" then
		return 0
	end
	return tonumber(response[keyName]) or 0
end

local function previewSamples(response, keyName, label)
	local list = typeof(response) == "table" and response[keyName] or nil
	if typeof(list) ~= "table" or #list == 0 then
		return label .. ": -"
	end

	local samples = {}
	local limit = math.min(#list, 3)
	for index = 1, limit do
		table.insert(samples, tostring(list[index]))
	end
	if #list > limit then
		table.insert(samples, "+" .. tostring(#list - limit) .. " more")
	end
	return label .. ": " .. table.concat(samples, ", ")
end

local function showPullPreview(preview)
	local response = typeof(preview) == "table" and preview.response or preview
	local addCount = previewCount(response, "add_count")
	local updateCount = previewCount(response, "update_count")
	local deleteCount = previewCount(response, "delete_count")
	local unchangedCount = previewCount(response, "unchanged_count")
	pullPreviewFrame.Visible = true
	pullPreviewCountLabel.Text = "add "
		.. tostring(addCount)
		.. " | update "
		.. tostring(updateCount)
		.. " | delete "
		.. tostring(deleteCount)
		.. " | unchanged "
		.. tostring(unchangedCount)
	pullPreviewSamplesLabel.Text = table.concat({
		previewSamples(response, "files_to_add", "add"),
		previewSamples(response, "files_to_update", "update"),
		previewSamples(response, "files_to_delete", "delete"),
	}, "\n")
end

local function hidePullPreview()
	pullPreviewFrame.Visible = false
	pullPreviewSamplesLabel.Text = "Collecting preview..."
	pullPreviewCountLabel.Text = "add 0 | update 0 | delete 0 | unchanged 0"
end

local function updateDebugToggleUi(enabled)
	if enabled then
		debugToggleButton.BackgroundColor3 = COLORS.DebugOn
		debugToggleButton.Text = "Debug ON"
	else
		debugToggleButton.BackgroundColor3 = COLORS.DebugOff
		debugToggleButton.Text = "Debug OFF"
	end
end

local function updateHealthPanel(payload)
	local stats = typeof(payload) == "table" and typeof(payload.stats) == "table" and payload.stats or {}
	local server = typeof(payload) == "table" and typeof(payload.server) == "table" and payload.server or {}
	local checks = typeof(payload) == "table" and typeof(payload.checks) == "table" and payload.checks or {}

	local syncRoot = tostring(server.sync_root or "-")
	local scanCycles = tonumber(server.scan_cycles) or 0
	local lastScanSec = tonumber(server.last_scan_sec) or 0
	local lastChange = tonumber(server.last_revision_change_count) or 0
	local lastError = tostring(stats.last_error or "")
	local pollErrors = tonumber(stats.poll_error_count) or 0
	local clientRev = tostring(stats.last_applied_rev or "-")
	local serverRev = tostring(stats.last_server_rev or server.server_rev or "-")
	local changeCount = tostring(stats.last_change_count or server.last_changes_count or 0)
	local ackStatus = tostring(server.last_ack_status or "-")
	local requestCount = tostring(server.changes_requests or "-")
	local gitCommit = tostring(server.git_commit or "")
	local gitError = tostring(server.git_error or "")
	local scanWarningGuidance = tostring(server.scan_warning_guidance or "")
	local watcherState = scanCycles > 0 and "active" or "idle"
	local serverState = checks.handshake and "online" or "waiting"

	headerHealthLabel.Text = checks.handshake and "Online" or "Waiting"
	headerHealthLabel.TextColor3 = checks.handshake and COLORS.StatusGood or COLORS.DebugMuted
	syncPathLabel.Text = "Root " .. compactStatusText(syncRoot)
	watcherLabel.Text = "Server "
		.. serverState
		.. " | Watcher "
		.. watcherState
		.. " | scans "
		.. tostring(scanCycles)
		.. (lastScanSec > 0 and (" | " .. string.format("%.2fs", lastScanSec)) or "")
	lastOpLabel.Text = "Rev c"
		.. clientRev
		.. " / s"
		.. serverRev
		.. " | Δ "
		.. changeCount
		.. " | ack "
		.. ackStatus
		.. " | req "
		.. requestCount
		.. (gitCommit ~= "" and (" | git=" .. gitCommit) or "")

	if gitError ~= "" then
		errorLabel.Text = "Git issue | " .. compactGuidedError(gitError)
		errorLabel.TextColor3 = COLORS.StatusError
	elseif lastError ~= "" then
		errorLabel.Text = compactGuidedError(lastError)
		errorLabel.TextColor3 = COLORS.StatusError
	elseif scanWarningGuidance ~= "" then
		errorLabel.Text = compactGuidedError(scanWarningGuidance)
		errorLabel.TextColor3 = COLORS.StatusError
	elseif pollErrors > 0 then
		errorLabel.Text = "Reconnecting | poll errors " .. tostring(pollErrors)
		errorLabel.TextColor3 = COLORS.StatusError
	elseif lastChange > 0 then
		errorLabel.Text = "Healthy | last scan Δ " .. tostring(lastChange)
		errorLabel.TextColor3 = COLORS.StatusGood
	else
		errorLabel.Text = "Healthy | no pending changes"
		errorLabel.TextColor3 = COLORS.StatusGood
	end
end

local function updateDebugPanel(payload)
	local enabled = typeof(payload) == "table" and payload.enabled == true
	updateDebugToggleUi(enabled)
	updateHealthPanel(payload)
	applyAdaptivePanelLayout(enabled)
	local execStats = typeof(payload) == "table" and typeof(payload.stats) == "table" and payload.stats or nil
	if execStats and execStats.remote_exec_status then
		updateExecUi(tostring(execStats.remote_exec_status))
	else
		updateExecUi()
	end

	if not enabled then
		debugChecklistLabel.Text = "[ ] HTTP/server reachable  [ ] connected to server  [ ] token valid\n[ ] sync active  [ ] exec active  [ ] edit mode"
		debugServerLabel.TextColor3 = COLORS.DebugMuted
		debugServerLabel.Text = "Debug nonaktif."
		debugEventsLabel.TextColor3 = COLORS.DebugMuted
		debugEventsLabel.Text = "Aktifkan debug untuk lihat detail request ACK/changes."
		return
	end

	local checks = typeof(payload.checks) == "table" and payload.checks or {}
	local stats = typeof(payload.stats) == "table" and payload.stats or {}
	local server = typeof(payload.server) == "table" and payload.server or {}

	local checklistLineOne = table.concat({
		checkMark(checks.http_server_reachable) .. " HTTP/server reachable",
		checkMark(checks.connected_to_server) .. " connected to server",
		checkMark(checks.token_valid) .. " token valid",
	}, "  ")
	local checklistLineTwo = table.concat({
		checkMark(checks.sync_active) .. " sync active",
		checkMark(checks.exec_active) .. " exec active",
		checkMark(checks.edit_mode) .. " edit mode",
	}, "  ")
	debugChecklistLabel.Text = checklistLineOne .. "\n" .. checklistLineTwo

	local clientRev = tostring(stats.last_applied_rev or "-")
	local targetRev = tostring(stats.last_target_rev or "-")
	local serverRev = tostring(stats.last_server_rev or server.server_rev or "-")
	local changeCount = tostring(stats.last_change_count or 0)
	local ackStatus = tostring(server.last_ack_status or "-")
	local changesReq = tostring(server.changes_requests or "-")
	local lastError = tostring(stats.last_error or "")
	local strictWhitelist = server.strict_property_whitelist == true
	local extraAllowedClasses = tonumber(server.extra_allowed_class_count) or 0

	debugServerLabel.Text = "clientRev="
		.. clientRev
		.. " target="
		.. targetRev
		.. " server="
		.. serverRev
		.. " changes="
		.. changeCount
		.. " ack="
		.. ackStatus
		.. " req="
		.. changesReq

	if lastError ~= "" then
		debugServerLabel.TextColor3 = COLORS.StatusError
	else
		debugServerLabel.TextColor3 = COLORS.DebugMuted
	end
	lastOpLabel.Text = lastOpLabel.Text
		.. " | strict="
		.. tostring(strictWhitelist)
		.. " | extra="
		.. tostring(extraAllowedClasses)

	local events = typeof(payload.events) == "table" and payload.events or {}
	local recent = {}
	local startIndex = math.max(1, #events)
	for index = startIndex, #events do
		table.insert(recent, tostring(events[index]))
	end
	if #recent == 0 then
		table.insert(recent, "Belum ada event debug.")
	end
	debugEventsLabel.TextColor3 = COLORS.DebugMuted
	debugEventsLabel.Text = table.concat(recent, "\n")
end

client = SyncAPI.new(plugin, updateStatus, updateDebugPanel)
local currentStartMode = START_MODES.FolderToStudio

local function applyStartModeUi()
	modeButton.Text = getModeLabelText(START_MODES.FolderToStudio)
end

do
	local host, port = client:getConnection()
	hostInput.Text = safeHostText(host)
	portInput.Text = tostring(safePortNumber(port))
	if client.getRemoteExecToken then
		execTokenInput.Text = tostring(client:getRemoteExecToken())
	end
	currentStartMode = START_MODES.FolderToStudio
	if client.setStartMode then
		client:setStartMode(currentStartMode)
	end
	applyStartModeUi()
	if client.isDebugEnabled then
		local debugEnabled = client:isDebugEnabled()
		updateDebugToggleUi(debugEnabled)
		applyAdaptivePanelLayout(debugEnabled)
	end
	updateExecUi()
	showLocalHistory()
	populateHistoryDropdown()
end

toolbarButton.Click:Connect(function()
	widget.Enabled = not widget.Enabled
	if widget.Enabled then
		pcall(function()
			widget:RequestRaise()
		end)
		refreshRevisionHistory(false)
	end
end)

historyDropdownButton.MouseButton1Click:Connect(function()
	populateHistoryDropdown()
	historyDropdownFrame.Visible = not historyDropdownFrame.Visible
end)

startButton.MouseButton1Click:Connect(function()
	client:setConnection(hostInput.Text, portInput.Text)
	if client.setStartMode then
		currentStartMode = START_MODES.FolderToStudio
		client:setStartMode(currentStartMode)
	end
	local ok, err = client:start()
	if not ok then
		updateStatus(tostring(err), true)
	else
		updateExecUi()
		refreshRevisionHistory(false)
	end
end)

stopButton.MouseButton1Click:Connect(function()
	client:stop()
	updateExecUi()
end)

snapshotButton.MouseButton1Click:Connect(function()
	local ok, err = client:forceSnapshot()
	if not ok then
		updateStatus("Resync gagal: " .. tostring(err), true)
	else
		refreshRevisionHistory(true)
	end
end)

pullStudioButton.MouseButton1Click:Connect(function()
	if not client.previewPullStudioToLocal then
		updateStatus("Pull Studio preview tidak tersedia di API ini", true)
		return
	end

	hidePullPreview()
	local ok, previewOrError = client:previewPullStudioToLocal()
	if not ok then
		updateStatus("Pull Studio preview gagal: " .. tostring(previewOrError), true)
	else
		showPullPreview(previewOrError)
		updateStatus("Review Pull Studio preview, lalu Confirm atau Cancel.", false)
	end
end)

confirmPullButton.MouseButton1Click:Connect(function()
	if not client.confirmPullStudioToLocal then
		updateStatus("Pull Studio confirm tidak tersedia di API ini", true)
		return
	end

	local ok, err = client:confirmPullStudioToLocal()
	hidePullPreview()
	if not ok then
		updateStatus("Pull Studio gagal: " .. tostring(err), true)
	else
		refreshRevisionHistory(true)
	end
end)

cancelPullButton.MouseButton1Click:Connect(function()
	if client.cancelPullStudioToLocal then
		client:cancelPullStudioToLocal()
	end
	hidePullPreview()
	updateStatus("Pull Studio dibatalkan. Local files tidak berubah.", false)
end)

debugToggleButton.MouseButton1Click:Connect(function()
	local isEnabled = false
	if client.isDebugEnabled then
		isEnabled = client:isDebugEnabled()
	end

	local nextState = not isEnabled
	if client.setDebugEnabled then
		client:setDebugEnabled(nextState)
	end

	if nextState and client.refreshServerDebugState then
		client:refreshServerDebugState(true)
		refreshRevisionHistory(false)
	end
end)

execTokenInput.FocusLost:Connect(function()
	if client and client.setRemoteExecToken then
		client:setRemoteExecToken(execTokenInput.Text)
	end
	updateExecUi()
end)

execToggleButton.MouseButton1Click:Connect(function()
	if not client then
		return
	end
	if client.setRemoteExecToken then
		client:setRemoteExecToken(execTokenInput.Text)
	end
	local enabled = false
	if client.isRemoteExecEnabled then
		enabled = client:isRemoteExecEnabled()
	end
	if client.setRemoteExecEnabled then
		client:setRemoteExecEnabled(not enabled)
	end
	updateExecUi()
end)

if plugin.Unloading then
	plugin.Unloading:Connect(function()
		client:stop()
	end)
end
