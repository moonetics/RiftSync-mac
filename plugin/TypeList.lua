local TypeList = {}

TypeList.VERSION = "4.2.2"
TypeList.PROTOCOL = "rbxsync/2.0.0"
TypeList.PLUGIN_ICON = "rbxassetid://72034413544662"
TypeList.CHANGE_ENCODINGS = {
	Compact = "compact-json-v1",
	Verbose = "verbose-json-v1",
}

TypeList.DEFAULT_HOST = "127.0.0.1"
TypeList.DEFAULT_PORT = 8765
TypeList.PROFILE_DISCOVERY_URL = "http://127.0.0.1:8749/riftsync/profiles"
TypeList.POLL_TIMEOUT_SECONDS = 25
TypeList.POLL_INTERVAL_SECONDS = 0.15
TypeList.RECONNECT_INTERVAL_SECONDS = 3
TypeList.EXEC_POLL_TIMEOUT_SECONDS = 1
TypeList.EXEC_IDLE_WAIT_SECONDS = 0.2
TypeList.ACTIVITY_SUMMARY_THRESHOLD = 8
TypeList.START_MODES = {
	StudioToFolder = "studio_to_folder",
	FolderToStudio = "folder_to_studio",
}
TypeList.DEFAULT_START_MODE = TypeList.START_MODES.FolderToStudio
TypeList.DEFAULT_STRICT_PROPERTY_WHITELIST = false
TypeList.DEFAULT_EXTRA_ALLOWED_PROPERTIES = {}

TypeList.ENDPOINTS = {
	Health = "/health",
	Handshake = "/handshake",
	Snapshot = "/snapshot",
	Changes = "/changes",
	Ack = "/ack",
	Activity = "/activity",
	Bootstrap = "/bootstrap",
	BootstrapPreview = "/bootstrap/preview",
	DebugState = "/debug/state",
	History = "/history",
	ExecNext = "/exec/commands/next",
	ExecResult = "/exec/commands/result",
}

TypeList.MANAGED_ATTRIBUTES = {
	IsManaged = "_rbxlsync_managed",
	LocalPath = "_rbxlsync_local_path",
	LastRevision = "_rbxlsync_last_revision",
	StableId = "_rbxlsync_stable_id",
	Kind = "_rbxlsync_kind",
}

TypeList.INSTANCE_KINDS = {
	Script = "script",
	UIInstance = "ui_instance",
}

TypeList.ENTITY_KIND_ALIASES = {
	ui_instance = "ui_instance",
	instance_props = "ui_instance",
	instance = "ui_instance",
}

TypeList.SCRIPT_SUFFIX_TO_CLASS = {
	[".server.luau"] = "Script",
	[".client.luau"] = "LocalScript",
	[".module.luau"] = "ModuleScript",
	[".server.lua"] = "Script",
	[".client.lua"] = "LocalScript",
	[".module.lua"] = "ModuleScript",
}

TypeList.CLASS_TO_PREFERRED_SUFFIX = {
	Script = ".server.luau",
	LocalScript = ".client.luau",
	ModuleScript = ".module.luau",
}

TypeList.UI_PROPERTIES_FILENAME = "properties.init.json"
TypeList.UI_ROOT_SERVICES = {
	StarterGui = true,
}

TypeList.PROPERTY_SYNC_ROOTS = {
	"game.StarterGui",
	"game.Workspace",
	"game.Lighting",
	"game.ReplicatedStorage",
	"game.ReplicatedFirst",
	"game.StarterPlayer",
	"game.StarterPack",
	"game.ServerScriptService",
	"game.ServerStorage",
}

TypeList.ROOT_SERVICE_CLASS = {
	StarterGui = "StarterGui",
	Workspace = "Workspace",
	Lighting = "Lighting",
	ReplicatedStorage = "ReplicatedStorage",
	ReplicatedFirst = "ReplicatedFirst",
	StarterPlayer = "StarterPlayer",
	StarterPack = "StarterPack",
	ServerScriptService = "ServerScriptService",
	ServerStorage = "ServerStorage",
	SoundService = "SoundService",
	TextChatService = "TextChatService",
}

TypeList.REMOTE_SYNC_ROOT_SERVICES = {
	ReplicatedStorage = true,
	ReplicatedFirst = true,
	Workspace = true,
	StarterGui = true,
	StarterPlayer = true,
	StarterPack = true,
	ServerScriptService = true,
	ServerStorage = true,
}

TypeList.REMOTE_SYNC_CLASSES = {
	RemoteEvent = true,
	RemoteFunction = true,
	UnreliableRemoteEvent = true,
	BindableEvent = true,
	BindableFunction = true,
}

TypeList.REMOTE_SYNC_CONTAINER_CLASSES = {
	Folder = true,
	Configuration = true,
}

TypeList.LIGHTING_CHILD_CLASSES = {
	Atmosphere = true,
	Sky = true,
	Clouds = true,
	BloomEffect = true,
	BlurEffect = true,
	ColorCorrectionEffect = true,
	DepthOfFieldEffect = true,
	SunRaysEffect = true,
	ColorGradingEffect = true,
}

TypeList.UI_PROPERTY_CANDIDATES = {
	"Active",
	"AnchorPoint",
	"Archivable",
	"AutoButtonColor",
	"AutomaticCanvasSize",
	"AutomaticSize",
	"BackgroundColor3",
	"BackgroundTransparency",
	"BorderColor3",
	"BorderMode",
	"BorderSizePixel",
	"CanvasPosition",
	"CanvasSize",
	"ClipsDescendants",
	"Enabled",
	"FontFace",
	"Image",
	"ImageColor3",
	"ImageRectOffset",
	"ImageRectSize",
	"ImageTransparency",
	"LayoutOrder",
	"LineJoinMode",
	"Looped",
	"MaxVisibleGraphemes",
	"Modal",
	"MultiLine",
	"Name",
	"PaddingBottom",
	"PaddingLeft",
	"PaddingRight",
	"PaddingTop",
	"PlaybackSpeed",
	"Position",
	"RichText",
	"Rotation",
	"ScaleType",
	"ScrollBarImageColor3",
	"ScrollBarImageTransparency",
	"ScrollBarThickness",
	"ScrollingDirection",
	"Selectable",
	"SelectionOrder",
	"Size",
	"SliceCenter",
	"SliceScale",
	"Text",
	"TextColor3",
	"TextDirection",
	"TextEditable",
	"TextScaled",
	"TextSize",
	"TextStrokeColor3",
	"TextStrokeTransparency",
	"TextTransparency",
	"TextTruncate",
	"TextWrapped",
	"TextXAlignment",
	"TextYAlignment",
	"TileSize",
	"Transparency",
	"UIOffset",
	"VerticalAlignment",
	"Visible",
	"Volume",
	"ZIndex",
	"ZIndexBehavior",
	"AspectRatio",
	"AspectType",
	"DominantAxis",
	"MinSize",
	"MaxSize",
	"ApplyStrokeMode",
	"Color",
	"Thickness",
	"Transparency",
	"Offset",
	"CellPadding",
	"CellSize",
	"FillDirection",
	"FillDirectionMaxCells",
	"HorizontalAlignment",
	"HorizontalFlex",
	"SortOrder",
	"StartCorner",
	"VerticalFlex",
	"Padding",
	"SizeConstraint",
	"HorizontalScrollBarInset",
	"VerticalScrollBarInset",
	"BottomImage",
	"MidImage",
	"TopImage",
	"Selectable",
}

TypeList.COMMON_SAFE_PROPERTIES = {
	"Archivable",
}

TypeList.CONSTRAINT_PROPERTY_CANDIDATES = {
	"ActuatorType",
	"AngularSpeed",
	"AngularVelocity",
	"Attachment0",
	"Attachment1",
	"Color",
	"Enabled",
	"FreeLength",
	"LimitsEnabled",
	"LinearResponsiveness",
	"LowerAngle",
	"LowerLimit",
	"MaxForce",
	"MaxFrictionTorque",
	"MaxLength",
	"MaxPlanarAxesForce",
	"MaxTorque",
	"MaxVelocity",
	"MinLength",
	"MotorMaxAcceleration",
	"MotorMaxTorque",
	"OrientationAlignmentMode",
	"PlaneVelocity",
	"Radius",
	"ReactionForceEnabled",
	"ReactionTorqueEnabled",
	"RelativeTo",
	"Restitution",
	"Responsiveness",
	"RigidityEnabled",
	"RopeStyle",
	"ServoMaxTorque",
	"Size",
	"Speed",
	"Stiffness",
	"Thickness",
	"UpperAngle",
	"UpperLimit",
	"VelocityConstraintMode",
	"Visible",
	"WinchEnabled",
	"WinchForce",
	"WinchResponsiveness",
	"WinchSpeed",
}

TypeList.SOUND_EFFECT_PROPERTY_CANDIDATES = {
	"Attack",
	"Bypass",
	"DecayTime",
	"Delay",
	"Density",
	"Diffusion",
	"DryLevel",
	"Enabled",
	"Feedback",
	"GainHigh",
	"GainLow",
	"GainMakeup",
	"GainMid",
	"Intensity",
	"Mix",
	"Octave",
	"Priority",
	"Rate",
	"Ratio",
	"Release",
	"SideChain",
	"Threshold",
	"WetLevel",
}

TypeList.LIGHT_PROPERTY_CANDIDATES = {
	"Angle",
	"Brightness",
	"Color",
	"Enabled",
	"Face",
	"Range",
	"Shadows",
}

TypeList.POST_EFFECT_PROPERTY_CANDIDATES = {
	"Enabled",
}

-- These classes may appear in a local path as structural anchors, but RiftSync
-- never owns or creates them from properties metadata.
TypeList.GEOMETRY_ANCESTOR_CLASSES = {
	Model = true,
	Part = true,
	MeshPart = true,
	UnionOperation = true,
	NegateOperation = true,
	TrussPart = true,
	WedgePart = true,
	CornerWedgePart = true,
	SpawnLocation = true,
	Seat = true,
	VehicleSeat = true,
	Terrain = true,
}

TypeList.CLASS_PROPERTY_WHITELIST = {
	Script = { "Enabled", "RunContext" },
	LocalScript = { "Enabled" },
	ModuleScript = {},
	RemoteEvent = {},
	RemoteFunction = {},
	UnreliableRemoteEvent = {},
	BindableEvent = {},
	BindableFunction = {},
	Folder = {},
	Configuration = {},
	Workspace = {
		"Gravity",
		"FallenPartsDestroyHeight",
		"StreamingEnabled",
		"StreamingMinRadius",
		"StreamingTargetRadius",
		"GlobalWind",
		"AirDensity",
	},
	StarterGui = { "ScreenOrientation", "ShowDevelopmentGui" },
	StarterPlayer = {
		"CameraMaxZoomDistance",
		"CameraMinZoomDistance",
		"CameraMode",
		"CharacterJumpHeight",
		"CharacterJumpPower",
		"CharacterMaxSlopeAngle",
		"CharacterUseJumpPower",
		"DevCameraOcclusionMode",
		"DevComputerCameraMovementMode",
		"DevComputerMovementMode",
		"DevTouchCameraMovementMode",
		"DevTouchMovementMode",
		"EnableMouseLockOption",
		"HealthDisplayDistance",
		"LoadCharacterAppearance",
		"NameDisplayDistance",
		"UserEmotesEnabled",
	},
	SoundService = {
		"AcousticSimulationEnabled",
		"AmbientReverb",
		"DefaultListenerLocation",
		"DistanceFactor",
		"DopplerScale",
		"RespectFilteringEnabled",
		"RolloffScale",
	},
	TextChatService = { "ChatVersion", "CreateDefaultCommands", "CreateDefaultTextChannels" },
	Terrain = {
		"Decoration",
		"WaterColor",
		"WaterReflectance",
		"WaterTransparency",
		"WaterWaveSize",
		"WaterWaveSpeed",
	},
	Clouds = { "Color", "Cover", "Density", "Enabled" },
	Lighting = {
		"Ambient",
		"OutdoorAmbient",
		"Brightness",
		"ColorShift_Bottom",
		"ColorShift_Top",
		"ClockTime",
		"GeographicLatitude",
		"EnvironmentDiffuseScale",
		"EnvironmentSpecularScale",
		"ExposureCompensation",
		"FogColor",
		"FogEnd",
		"FogStart",
		"GlobalShadows",
		"ShadowSoftness",
		"Technology",
		"TimeOfDay",
	},
	Atmosphere = {
		"Color",
		"Decay",
		"Density",
		"Glare",
		"Haze",
		"Offset",
	},
	Sky = {
		"CelestialBodiesShown",
		"MoonAngularSize",
		"MoonTextureId",
		"SkyboxBk",
		"SkyboxDn",
		"SkyboxFt",
		"SkyboxLf",
		"SkyboxRt",
		"SkyboxUp",
		"StarCount",
		"SunAngularSize",
		"SunTextureId",
	},
	BloomEffect = {
		"Enabled",
		"Intensity",
		"Size",
		"Threshold",
	},
	BlurEffect = {
		"Enabled",
		"Size",
	},
	ColorCorrectionEffect = {
		"Enabled",
		"Brightness",
		"Contrast",
		"Saturation",
		"TintColor",
	},
	DepthOfFieldEffect = {
		"Enabled",
		"FarIntensity",
		"FocusDistance",
		"InFocusRadius",
		"NearIntensity",
	},
	SunRaysEffect = {
		"Enabled",
		"Intensity",
		"Spread",
	},
	ColorGradingEffect = { "Enabled", "TonemapperPreset" },

	BoolValue = { "Value" },
	BrickColorValue = { "Value" },
	CFrameValue = { "Value" },
	Color3Value = { "Value" },
	DoubleConstrainedValue = { "MaxValue", "MinValue", "Value" },
	IntConstrainedValue = { "MaxValue", "MinValue", "Value" },
	IntValue = { "Value" },
	NumberValue = { "Value" },
	ObjectValue = { "Value" },
	RayValue = { "Value" },
	StringValue = { "Value" },
	Vector3Value = { "Value" },

	Sound = {
		"AcousticSimulationEnabled",
		"Looped",
		"LoopRegion",
		"PlaybackRegion",
		"PlaybackRegionsEnabled",
		"PlaybackSpeed",
		"PlayOnRemove",
		"RollOffMaxDistance",
		"RollOffMinDistance",
		"RollOffMode",
		"SoundGroup",
		"SoundId",
		"Volume",
	},
	SoundGroup = { "Volume" },
	AudioPlayer = { "Asset", "AutoLoad", "Looping", "PlaybackSpeed" },
	AudioEmitter = { "AudioInteractionGroup" },
	AudioListener = { "AudioInteractionGroup" },
	AudioFader = { "Volume" },
	AudioPitchShifter = { "Pitch" },
	AudioEqualizer = { "HighGain", "LowGain", "MidGain" },
	AudioCompressor = { "Attack", "MakeupGain", "Ratio", "Release", "Threshold" },
	AudioReverb = { "DecayRatio", "DecayTime", "Density", "Diffusion", "DryLevel", "HighCutFrequency", "WetLevel" },
	AudioEcho = { "DelayTime", "DryLevel", "Feedback", "WetLevel" },
	AudioAnalyzer = { "SpectrumEnabled", "WindowSize" },
	Wire = { "SourceInstance", "SourceName", "TargetInstance", "TargetName" },

	Animation = { "AnimationId" },
	AnimationController = {},
	Animator = {},
	ParticleEmitter = {
		"Acceleration", "Brightness", "Color", "Drag", "EmissionDirection", "Enabled", "FlipbookBlendFrames", "FlipbookFramerate",
		"FlipbookLayout", "FlipbookMode", "FlipbookStartRandom", "Lifetime", "LightEmission",
		"LightInfluence", "LockedToPart", "Orientation", "Rate", "RotSpeed", "Rotation", "Shape", "ShapeInOut",
		"ShapePartial", "ShapeStyle", "Size", "Speed", "SpreadAngle", "Squash", "Texture", "TimeScale",
		"Transparency", "VelocityInheritance", "WindAffectsDrag", "ZOffset",
	},
	Beam = {
		"Attachment0", "Attachment1", "Brightness", "Color", "CurveSize0", "CurveSize1", "Enabled", "FaceCamera",
		"LightEmission", "LightInfluence", "LocalTransparencyModifier", "Segments", "Texture", "TextureLength",
		"TextureMode", "TextureSpeed", "Transparency", "Width0", "Width1", "ZOffset",
	},
	Trail = {
		"Attachment0", "Attachment1", "Brightness", "Color", "Enabled", "FaceCamera", "Lifetime", "LightEmission",
		"LightInfluence", "MaxLength", "MinLength", "Texture", "TextureLength", "TextureMode", "Transparency",
		"WidthScale",
	},
	Smoke = { "Color", "Enabled", "Opacity", "RiseVelocity", "Size", "TimeScale" },
	Fire = { "Color", "Enabled", "Heat", "SecondaryColor", "Size", "TimeScale" },
	Sparkles = { "Enabled", "SparkleColor", "TimeScale" },
	Highlight = { "Adornee", "DepthMode", "Enabled", "FillColor", "FillTransparency", "OutlineColor", "OutlineTransparency" },
	PointLight = {},
	SpotLight = {},
	SurfaceLight = {},
	Decal = { "Color3", "Face", "LocalTransparencyModifier", "Texture", "Transparency", "ZIndex" },
	Texture = { "Color3", "Face", "OffsetStudsU", "OffsetStudsV", "StudsPerTileU", "StudsPerTileV", "Texture", "Transparency", "ZIndex" },
	SurfaceAppearance = { "AlphaMode", "Color", "ColorMap", "MetalnessMap", "NormalMap", "RoughnessMap" },
	SpecialMesh = { "MeshId", "MeshType", "Offset", "Scale", "TextureId", "VertexColor" },
	BlockMesh = { "Offset", "Scale", "VertexColor" },
	CylinderMesh = { "Offset", "Scale", "VertexColor" },
	ProximityPrompt = {
		"ActionText", "AutoLocalize", "ClickablePrompt", "Enabled", "Exclusivity", "GamepadKeyCode", "HoldDuration",
		"KeyboardKeyCode", "MaxActivationDistance", "ObjectText", "RequiresLineOfSight", "RootLocalizationTable",
		"Style", "UIOffset",
	},
	ClickDetector = { "CursorIcon", "MaxActivationDistance" },
	Tool = { "CanBeDropped", "Enabled", "Grip", "GripForward", "GripPos", "GripRight", "GripUp", "ManualActivationOnly", "RequiresHandle", "ToolTip" },
	Attachment = { "Axis", "CFrame", "Orientation", "Position", "SecondaryAxis", "Visible" },
	Bone = { "CFrame", "Transform" },
	WrapLayer = { "AutoSkin", "BindOffset", "CageMeshId", "CageOrigin", "Enabled", "ImportOrigin", "Order", "Puffiness", "ReferenceMeshId", "ReferenceOrigin", "ShrinkFactor" },
	WrapTarget = { "CageMeshId", "CageOrigin", "ImportOrigin", "Stiffness" },
	WeldConstraint = { "Enabled", "Part0", "Part1" },
}

TypeList.PROPERTY_FAMILY_CLASSES = {
	Constraint = true,
	Light = true,
	PostEffect = true,
	SoundEffect = true,
}

TypeList.INSTANCE_REFERENCE_PROPERTIES = {
	Adornee = true,
	Attachment0 = true,
	Attachment1 = true,
	Part0 = true,
	Part1 = true,
	RootLocalizationTable = true,
	SideChain = true,
	SoundGroup = true,
	SourceInstance = true,
	TargetInstance = true,
	Value = true,
}

TypeList.DEFAULT_MANAGED_ROOTS = {
	"game.Workspace",
	"game.ReplicatedFirst",
	"game.ReplicatedStorage",
	"game.ServerScriptService",
	"game.ServerStorage",
	"game.StarterGui",
	"game.StarterPack",
	"game.StarterPlayer",
	"game.Lighting",
	"game.SoundService",
	"game.TextChatService",
}

TypeList.DEFAULT_IGNORED_RBX_PATHS = {
	"game.ServerScriptService.RiftSyncPlugin",
	"game.ServerScriptService.RBXLSyncPlugin",
}

local function trimWhitespace(value : string)
	return (value:gsub("^%s+", ""):gsub("%s+$", ""))
end

function TypeList.normaliseHost(hostValue : string)
	local cleaned = trimWhitespace(hostValue)
	if cleaned == "" then
		return TypeList.DEFAULT_HOST
	end
	return cleaned
end

function TypeList.normalisePort(portValue : string | number)
	local numericPort = tonumber(portValue)
	if not numericPort then
		return TypeList.DEFAULT_PORT
	end

	numericPort = math.floor(numericPort)
	if numericPort < 1 or numericPort > 65535 then
		return TypeList.DEFAULT_PORT
	end

	return numericPort
end

function TypeList.buildBaseUrl(hostValue : string, portValue : string | number)
	local host = TypeList.normaliseHost(hostValue)
	local port = TypeList.normalisePort(portValue)
	return "http://" .. host .. ":" .. tostring(port), host, port
end

function TypeList.isIgnoredRbxPath(pathValue : string, ignoredPaths : {string}?)
	local path = trimWhitespace(tostring(pathValue or ""))
	if path == "" then
		return false
	end

	local list = ignoredPaths or TypeList.DEFAULT_IGNORED_RBX_PATHS
	for _, ignored in ipairs(list) do
		local normalizedIgnored = trimWhitespace(tostring(ignored or ""))
		if normalizedIgnored ~= "" then
			if path == normalizedIgnored then
				return true
			end
			if string.sub(path, 1, #normalizedIgnored + 1) == normalizedIgnored .. "." then
				return true
			end
		end
	end

	return false
end

function TypeList.normaliseStartMode(modeValue : string?)
	local value = trimWhitespace(tostring(modeValue or ""))
	if value == TypeList.START_MODES.StudioToFolder then
		return TypeList.START_MODES.StudioToFolder
	end
	if value == TypeList.START_MODES.FolderToStudio then
		return TypeList.START_MODES.FolderToStudio
	end

	if value == "replace" then
		return TypeList.START_MODES.StudioToFolder
	end

	return TypeList.DEFAULT_START_MODE
end

function TypeList.startModeLabel(modeValue : string)
	local mode = TypeList.normaliseStartMode(modeValue)
	if mode == TypeList.START_MODES.StudioToFolder then
		return "Studio -> Folder (override lokal)"
	end
	return "Folder -> Studio (pakai file lokal)"
end

function TypeList.isScriptClass(className : string)
	return className == "Script" or className == "LocalScript" or className == "ModuleScript"
end

function TypeList.isUIRootPath(rbxPath : string)
	if typeof(rbxPath) ~= "string" then
		return false
	end
	for serviceName, _ in pairs(TypeList.UI_ROOT_SERVICES) do
		if rbxPath == "game." .. serviceName then
			return true
		end
		if string.sub(rbxPath, 1, #serviceName + 6) == "game." .. serviceName .. "." then
			return true
		end
	end
	return false
end

function TypeList.normaliseEntityKind(kindValue : any)
	local value = tostring(kindValue or "")
	local mapped = TypeList.ENTITY_KIND_ALIASES[value]
	if mapped then
		return mapped
	end
	return value
end

function TypeList.getClassPropertyWhitelist(className : string)
	return TypeList.CLASS_PROPERTY_WHITELIST[className]
end

function TypeList.normaliseExtraAllowedProperties(rawValue : any)
	local result = {}
	if typeof(rawValue) ~= "table" then
		return result
	end

	for className, listValue in pairs(rawValue) do
		if typeof(className) == "string" and typeof(listValue) == "table" then
			local clean = {}
			local seen = {}
			for _, propertyName in ipairs(listValue) do
				if typeof(propertyName) == "string" and propertyName ~= "" and not seen[propertyName] then
					seen[propertyName] = true
					table.insert(clean, propertyName)
				end
			end
			result[className] = clean
		end
	end

	return result
end

function TypeList.isRootServicePath(rbxPath : string)
	if typeof(rbxPath) ~= "string" then
		return false
	end
	for serviceName, _ in pairs(TypeList.ROOT_SERVICE_CLASS) do
		if rbxPath == "game." .. serviceName then
			return true
		end
	end
	return false
end

return TypeList
