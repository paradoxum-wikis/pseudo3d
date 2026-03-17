local RunService = game:GetService("RunService")
local UIS = game:GetService("UserInputService")
local Players = game:GetService("Players")
local CaptureService = game:GetService("CaptureService")
local Lighting = game:GetService("Lighting")
local AnimationClipProvider = game:GetService("AnimationClipProvider")

--  CONFIGURATION
local ROTATIONS       = 24							-- number of frames per full rotation
local DISTANCE        = 12							-- camera distance from model center
local HEIGHT_STEP     = .1							-- units moved per E/D press
local HEIGHT_MIN      = -10							-- lowest the camera can go (relative to model center)
local HEIGHT_MAX      = 10							-- highest the camera can go (relative to model center)
local PIVOT_STEP      = .1							-- units moved per A/S press
local PIVOT_MIN       = -10							-- leftmost pivot shift allowed
local PIVOT_MAX       = 10							-- rightmost pivot shift allowed
local DEPTH_MIN       = -10							-- furthest depth shift allowed
local DEPTH_MAX       = 10							-- nearest depth shift allowed
local ZOOM_STEP       = .1							-- units moved per R/F press
local ZOOM_MIN        = 2							-- closest the camera can get
local ZOOM_MAX        = 50							-- furthest the camera can get
local ANIM_STEP       = 0.1							-- animation time moved per N/M press
local BACKDROP		  = true						-- whether to use a backdrop behind the model
local BACKDROP_COLOR  = Color3.fromRGB(255, 0, 255)	-- chroma key color (magenta)
local FLAT_LIGHTING   = false						-- set false to keep your scene lighting as is
local BLOOM_INTENSITY = 0							-- set to nil to leave bloom as is
local CLEAR_WORKSPACE = true						-- remove everything in workspace except Target model
local PREVIEW_FPS     = 10							-- frames per second during preview rotation
local TARGET_NAME     = "Target"					-- name of the model to capture
local ANIM_NAME       = "Animation"					-- name of the animation to play (also prefix for variants)
local TARGET_EFFECTS  = false						-- show anything that emits from the model

local player = Players.LocalPlayer
for _, gui in ipairs(player.PlayerGui:GetChildren()) do
	if gui:IsA("ScreenGui") then gui.Enabled = false end
end

local bg = Instance.new("ScreenGui", player.PlayerGui)
bg.IgnoreGuiInset = true
bg.DisplayOrder = 999

local function makePanel(anchorY, posY, offsetY)
	local f = Instance.new("Frame", bg)
	f.AutomaticSize = Enum.AutomaticSize.XY
	f.Position = UDim2.new(0.5, 0, posY, offsetY)
	f.AnchorPoint = Vector2.new(0.5, anchorY)
	f.BackgroundColor3 = Color3.fromRGB(15, 15, 15)
	f.BackgroundTransparency = 0.35
	f.BorderSizePixel = 0
	Instance.new("UICorner", f).CornerRadius = UDim.new(0, 3)
	local l = Instance.new("UIListLayout", f)
	l.HorizontalAlignment = Enum.HorizontalAlignment.Center
	l.Padding = UDim.new(0, 2)
	local p = Instance.new("UIPadding", f)
	p.PaddingTop = UDim.new(0, 6); p.PaddingBottom = UDim.new(0, 6)
	p.PaddingLeft = UDim.new(0, 12); p.PaddingRight = UDim.new(0, 12)
	return f
end

local function makeLabel(parent, color, size, bold, visible)
	local lbl = Instance.new("TextLabel", parent)
	lbl.Size = UDim2.new(0, 0, 0, 0)
	lbl.AutomaticSize = Enum.AutomaticSize.XY
	lbl.BackgroundTransparency = 1
	lbl.TextColor3 = color
	lbl.TextSize = size
	lbl.FontFace = Font.new("rbxasset://fonts/families/Montserrat.json",
		bold and Enum.FontWeight.Bold or Enum.FontWeight.Regular)
	lbl.Visible = visible ~= false
	return lbl
end

local topPanel        = makePanel(0, 0, 12)
local label           = makeLabel(topPanel, Color3.fromRGB(255, 255, 255), 20, true)
local subLabel        = makeLabel(topPanel, Color3.fromRGB(204, 204, 204), 16, false)
local previewLabel    = makeLabel(topPanel, Color3.fromRGB(0, 255, 128),   22, true,  false)
local previewSubLabel = makeLabel(topPanel, Color3.fromRGB(179, 255, 204), 16, false, false)

local botPanel = makePanel(1, 1, -12)
local botLabel = makeLabel(botPanel, Color3.fromRGB(255, 255, 128), 16, true)

local animList = Instance.new("Frame", bg)
animList.AutomaticSize = Enum.AutomaticSize.Y
animList.Size = UDim2.fromOffset(220, 0)
animList.Position = UDim2.new(0, 12, 1, -12)
animList.AnchorPoint = Vector2.new(0, 1)
animList.BackgroundColor3 = Color3.fromRGB(15, 15, 15)
animList.BackgroundTransparency = 0.35
animList.BorderSizePixel = 0
animList.Visible = false
Instance.new("UICorner", animList).CornerRadius = UDim.new(0, 3)
local _l = Instance.new("UIListLayout", animList)
_l.SortOrder = Enum.SortOrder.LayoutOrder
_l.Padding = UDim.new(0, 2)
local _p = Instance.new("UIPadding", animList)
_p.PaddingTop = UDim.new(0, 6); _p.PaddingBottom = UDim.new(0, 6)
_p.PaddingLeft = UDim.new(0, 8); _p.PaddingRight = UDim.new(0, 8)
local animListHeader = Instance.new("TextLabel", animList)
animListHeader.Size = UDim2.new(1, 0, 0, 18)
animListHeader.BackgroundTransparency = 1
animListHeader.TextColor3 = Color3.fromRGB(255, 255, 255)
animListHeader.TextSize = 13
animListHeader.TextXAlignment = Enum.TextXAlignment.Left
animListHeader.FontFace = Font.new("rbxasset://fonts/families/Montserrat.json", Enum.FontWeight.Bold)
animListHeader.Text = "Change Animation: H/J"
animListHeader.LayoutOrder = 0

local model = workspace:WaitForChild(TARGET_NAME, 10)
if not model then return warn("No '" .. TARGET_NAME .. "' found in workspace!") end

local character = player.Character or player.CharacterAdded:Wait()
if CLEAR_WORKSPACE then
	for _, obj in ipairs(workspace:GetChildren()) do
		if obj ~= model and obj ~= workspace.Terrain and not obj:IsA("Camera") and obj ~= character then
			obj:Destroy()
		end
	end
end

for _, part in ipairs(character:GetDescendants()) do
	if part:IsA("BasePart") then part.Transparency = 1; part.Anchored = true end
end

local rootPart = model.PrimaryPart or model:FindFirstChild("HumanoidRootPart") or model:FindFirstChildWhichIsA("BasePart")
if rootPart then rootPart.Anchored = true end

local function getFrontAngle()
	if not rootPart then return 0 end
	return math.atan2(rootPart.CFrame.LookVector.X, rootPart.CFrame.LookVector.Z)
end

local pivot = model:GetPivot()
model:PivotTo(CFrame.new(pivot.Position) * CFrame.fromEulerAnglesYXZ(0, -getFrontAngle(), 0))

local startingAngleOffset = getFrontAngle()

for _, part in ipairs(model:GetDescendants()) do
	if part:IsA("BasePart") and part ~= rootPart then
		part.Anchored = false
	end
end

if not TARGET_EFFECTS then
	for _, desc in ipairs(model:GetDescendants()) do
		if desc:IsA("ParticleEmitter") or desc:IsA("Trail") or desc:IsA("Beam")
			or desc:IsA("Smoke") or desc:IsA("Fire") or desc:IsA("Sparkles") then
			desc.Enabled = false
		end
	end
end

local animSequences = {}
for _, child in ipairs(model:GetChildren()) do
	if child:IsA("AnimationClip") and (child.Name == ANIM_NAME or child.Name:match("^" .. ANIM_NAME .. "%d+$")) then
		table.insert(animSequences, child)
	end
end

local humOrCtrl = model:FindFirstChildOfClass("Humanoid") or model:FindFirstChildOfClass("AnimationController")
local animator  = humOrCtrl and (humOrCtrl:FindFirstChildOfClass("Animator") or Instance.new("Animator", humOrCtrl)) or nil

local animationTrack   = nil
local currentAnimIndex = 1

local function loadAnimTrack(seq)
	if not animator or not seq then return nil end
	local anim = Instance.new("Animation")
	anim.AnimationId = AnimationClipProvider:RegisterAnimationClip(seq)
	local track = animator:LoadAnimation(anim)
	track:Play()
	track:AdjustSpeed(0)
	track.TimePosition = 0
	return track
end

if #animSequences > 0 then
	animationTrack = loadAnimTrack(animSequences[1])
end

local animRowLabels = {}
local FONT_BOLD = Font.new("rbxasset://fonts/families/Montserrat.json", Enum.FontWeight.Bold)
local FONT_REG  = Font.new("rbxasset://fonts/families/Montserrat.json", Enum.FontWeight.Regular)
animList.Visible = #animSequences >= 2
for i, seq in ipairs(animSequences) do
	local row = Instance.new("TextLabel", animList)
	row.Size = UDim2.new(1, 0, 0, 18)
	row.BackgroundTransparency = 1
	row.TextSize = 13
	row.TextXAlignment = Enum.TextXAlignment.Left
	row.LayoutOrder = i
	row.TextColor3 = i == 1 and Color3.fromRGB(80, 255, 180) or Color3.fromRGB(200, 200, 200)
	row.FontFace   = i == 1 and FONT_BOLD or FONT_REG
	row.Text       = i == 1 and "> " .. seq.Name or "  " .. seq.Name
	animRowLabels[i] = row
end

local function switchToAnim(index)
	if #animSequences == 0 then return end
	index = ((index - 1) % #animSequences) + 1
	if index == currentAnimIndex then return end
	local prevTime = animationTrack and animationTrack.TimePosition or 0
	if animationTrack then animationTrack:Stop(); animationTrack:Destroy() end
	animRowLabels[currentAnimIndex].TextColor3 = Color3.fromRGB(200, 200, 200)
	animRowLabels[currentAnimIndex].FontFace   = FONT_REG
	animRowLabels[currentAnimIndex].Text       = "  " .. animSequences[currentAnimIndex].Name
	currentAnimIndex = index
	animRowLabels[currentAnimIndex].TextColor3 = Color3.fromRGB(80, 255, 180)
	animRowLabels[currentAnimIndex].FontFace   = FONT_BOLD
	animRowLabels[currentAnimIndex].Text       = "> " .. animSequences[currentAnimIndex].Name
	animationTrack = loadAnimTrack(animSequences[currentAnimIndex])
	if animationTrack then
		animationTrack.TimePosition = math.min(prevTime, animationTrack.Length > 0 and animationTrack.Length or 10)
	end
end

task.wait(0.5)

local minY, maxY = math.huge, -math.huge
local minX, maxX = math.huge, -math.huge
local minZ, maxZ = math.huge, -math.huge
for _, part in ipairs(model:GetDescendants()) do
	if part:IsA("BasePart") then
		local p, s = part.Position, part.Size
		minY, maxY = math.min(minY, p.Y - s.Y/2), math.max(maxY, p.Y + s.Y/2)
		minX, maxX = math.min(minX, p.X - s.X/2), math.max(maxX, p.X + s.X/2)
		minZ, maxZ = math.min(minZ, p.Z - s.Z/2), math.max(maxZ, p.Z + s.Z/2)
	end
end

local modelCenter = Vector3.new((minX + maxX)/2, (minY + maxY)/2, (minZ + maxZ)/2)
local fov = math.deg(2 * math.atan((math.max(maxY - minY, math.max(maxX - minX, maxZ - minZ)) * 0.6) / DISTANCE))

if FLAT_LIGHTING then
	for _, child in ipairs(Lighting:GetChildren()) do
		if child:IsA("PostEffect") or child:IsA("Atmosphere") or child:IsA("Sky") or child:IsA("Clouds") then
			child:Destroy()
		end
	end
	Lighting.Ambient = Color3.fromRGB(255, 255, 255)
	Lighting.OutdoorAmbient = Color3.fromRGB(255, 255, 255)
	Lighting.Brightness = 0
	Lighting.GlobalShadows = false
	Lighting.FogEnd = 100000
	Lighting.EnvironmentDiffuseScale = 0
	Lighting.EnvironmentSpecularScale = 0
end

if BLOOM_INTENSITY ~= nil then
	local bloom = Lighting:FindFirstChildOfClass("BloomEffect") or Instance.new("BloomEffect", Lighting)
	bloom.Intensity = BLOOM_INTENSITY
end

local backdrop
if BACKDROP then
	backdrop = Instance.new("Part", workspace)
	backdrop.Size = Vector3.new(2000, 2000, 1)
	backdrop.Anchored = true
	backdrop.CanCollide = false
	backdrop.CastShadow = false
	local surfaceGui = Instance.new("SurfaceGui", backdrop)
	surfaceGui.LightInfluence = 0
	local bgFrame = Instance.new("Frame", surfaceGui)
	bgFrame.Size = UDim2.fromScale(1, 1)
	bgFrame.BackgroundColor3 = BACKDROP_COLOR
end

local pivotIndicator = Instance.new("Part", workspace)
pivotIndicator.Size = Vector3.new(0.1, 0.1, 0.1)
pivotIndicator.Anchored = true
pivotIndicator.CanCollide = false
pivotIndicator.Transparency = 1

local pivotBillboard = Instance.new("BillboardGui", pivotIndicator)
pivotBillboard.AlwaysOnTop = true
pivotBillboard.Size = UDim2.fromOffset(16, 16)
pivotBillboard.Enabled = false

local pivotDot = Instance.new("Frame", pivotBillboard)
pivotDot.Size = UDim2.fromScale(1, 1)
pivotDot.BackgroundColor3 = Color3.fromRGB(255, 255, 0)
Instance.new("UICorner", pivotDot).CornerRadius = UDim.new(1, 0)

local cam = workspace.CurrentCamera
cam.CameraType = Enum.CameraType.Scriptable
cam.FieldOfView = fov

local currentFrame		= 1
local heightOffset		= 0
local distance			= DISTANCE
local pivotOffset		= 0
local depthOffset		= 0
local showPivot			= false
local previewing		= false
local previewConnection	= nil
local capturing			= false
local paused            = false
local isCorpse          = false
local originalProps		= {}

local function getLookAt()
	local right   = Vector3.new(math.cos(startingAngleOffset), 0, -math.sin(startingAngleOffset))
	local forward = Vector3.new(math.sin(startingAngleOffset), 0,  math.cos(startingAngleOffset))
	return modelCenter + right * pivotOffset + forward * depthOffset + Vector3.new(0, heightOffset, 0)
end

local function updateUI()
	if capturing then return end
	if previewing then
		local statusText = paused and "[PAUSED]" or "[PREVIEWING]"
		previewLabel.Text = string.format("%s | Height: %+.1f | Pivot: %+.1f | Depth: %+.1f | Zoom: %.1f | Frame %d/%d",
			statusText, heightOffset, pivotOffset, depthOffset, distance, currentFrame, ROTATIONS)
		previewSubLabel.Text = "P: stop preview | Space: pause/resume"
		botPanel.Visible = false
	else
		local side = "Front"
		if rootPart then
			side = math.abs(startingAngleOffset - getFrontAngle()) < 0.01 and "Front" or "Back"
		end
		local animPos = animationTrack and string.format(" | Anim: %.2fs", animationTrack.TimePosition) or ""
		label.Text = string.format("Start: %s | Height: %+.1f | Pivot: %+.1f | Depth: %+.1f | Zoom: %.1f%s | Frame %d/%d",
			side, heightOffset, pivotOffset, depthOffset, distance, animPos, currentFrame, ROTATIONS)
		subLabel.Text = "W: flip | E/D: height | A/S: pivot | Z/X: depth | R/F: zoom | N/M: anim time | O: pivot | P: preview | K/L: rotate | 1: corpse"
		botLabel.Text = "Q: capture all frames | C: capture one frame"
		botPanel.Visible = true
	end
end

local function setFrame(i)
	local lookAt = getLookAt()
	local angle  = math.rad((i - 1) * (360 / ROTATIONS)) + startingAngleOffset
	local camPos = lookAt + Vector3.new(math.sin(angle) * distance, 0, math.cos(angle) * distance)
	cam.CFrame   = CFrame.lookAt(camPos, lookAt)
	if BACKDROP then
		backdrop.CFrame = CFrame.lookAt(lookAt + ((lookAt - camPos).Unit * 500), camPos)
	end
	pivotIndicator.CFrame = CFrame.new(lookAt)
	updateUI()
end

local function stopPreview()
	previewLabel.Visible = false
	previewSubLabel.Visible = false
	label.Visible = true
	subLabel.Visible = true
	previewing = false
	paused = false
	if previewConnection then
		previewConnection:Disconnect()
		previewConnection = nil
	end
	currentFrame = 1
	setFrame(currentFrame)
end

local function startPreview()
	label.Visible = false
	subLabel.Visible = false
	previewLabel.Visible = true
	previewSubLabel.Visible = true
	previewing = true
	local elapsed = 0
	previewConnection = RunService.RenderStepped:Connect(function(dt)
		if paused then return end
		elapsed += dt
		if elapsed >= 1 / PREVIEW_FPS then
			elapsed = 0
			currentFrame = (currentFrame % ROTATIONS) + 1
			setFrame(currentFrame)
		end
	end)
end

local function captureNext(oneShot)
	if capturing or currentFrame > ROTATIONS then return end
	capturing = true

	topPanel.Visible = false
	botPanel.Visible = false
	animList.Visible = false
	pivotBillboard.Enabled = false
	UIS.MouseIconEnabled = false
	RunService.RenderStepped:Wait()

	CaptureService:TakeScreenshotCaptureAsync(function(result, screenshotCapture)
		if result ~= Enum.ScreenshotCaptureResult.Success or not screenshotCapture then
			warn("Screenshot failed: " .. tostring(result))
			capturing = false
			return
		end

		CaptureService:PromptSaveCapturesToGallery({screenshotCapture}, function(results)
			local saved = results[screenshotCapture]
			print(string.format("Frame %d/%d captured (saved: %s)", currentFrame, ROTATIONS, tostring(saved)))

			if currentFrame < ROTATIONS and not oneShot then
				currentFrame += 1
				setFrame(currentFrame)
				capturing = false
				captureNext()
			else
				UIS.MouseIconEnabled = true
				local text = oneShot and "Heeho! Single frame captured!" or "Heeho! All frames captured!"
				topPanel.Visible = true
				animList.Visible = #animSequences >= 2
				if previewing then
					previewLabel.Text = text
					previewSubLabel.Visible = false
				else
					label.Text = text
					subLabel.Visible = false
				end
				pivotBillboard.Enabled = showPivot
				print(text)
				capturing = false
			end
		end)
	end)
end

setFrame(1)

local keyActions = {
	[Enum.KeyCode.W] = function()
		local base = getFrontAngle()
		startingAngleOffset = math.abs(startingAngleOffset - base) < 0.01 and base + math.pi or base
	end,
	[Enum.KeyCode.E] = function() heightOffset = math.min(heightOffset + HEIGHT_STEP, HEIGHT_MAX) end,
	[Enum.KeyCode.D] = function() heightOffset = math.max(heightOffset - HEIGHT_STEP, HEIGHT_MIN) end,
	[Enum.KeyCode.A] = function() pivotOffset  = math.max(pivotOffset  - PIVOT_STEP,  PIVOT_MIN)  end,
	[Enum.KeyCode.S] = function() pivotOffset  = math.min(pivotOffset  + PIVOT_STEP,  PIVOT_MAX)  end,
	[Enum.KeyCode.Z] = function() depthOffset  = math.max(depthOffset  - PIVOT_STEP,  DEPTH_MIN)  end,
	[Enum.KeyCode.X] = function() depthOffset  = math.min(depthOffset  + PIVOT_STEP,  DEPTH_MAX)  end,
	[Enum.KeyCode.R] = function() distance     = math.max(distance     - ZOOM_STEP,   ZOOM_MIN)   end,
	[Enum.KeyCode.F] = function() distance     = math.min(distance     + ZOOM_STEP,   ZOOM_MAX)   end,
	[Enum.KeyCode.N] = function()
		if animationTrack then
			animationTrack.TimePosition = math.max(0, animationTrack.TimePosition - ANIM_STEP)
		end
	end,
	[Enum.KeyCode.M] = function()
		if animationTrack then
			local maxTime = animationTrack.Length > 0 and animationTrack.Length or 10
			animationTrack.TimePosition = math.min(maxTime, animationTrack.TimePosition + ANIM_STEP)
		end
	end,
	[Enum.KeyCode.H] = function() switchToAnim(currentAnimIndex - 1) end,
	[Enum.KeyCode.J] = function() switchToAnim(currentAnimIndex + 1) end,
	[Enum.KeyCode.O] = function()
		showPivot = not showPivot
		pivotBillboard.Enabled = showPivot
	end,
	[Enum.KeyCode.P] = function()
		if previewing then stopPreview() else startPreview() end
	end,
	[Enum.KeyCode.Space] = function()
		if previewing then
			paused = not paused
			setFrame(currentFrame)
		end
	end,
	[Enum.KeyCode.K] = function() startingAngleOffset -= math.pi * 2 / ROTATIONS end,
	[Enum.KeyCode.L] = function() startingAngleOffset += math.pi * 2 / ROTATIONS end,
	[Enum.KeyCode.One] = function()
		isCorpse = not isCorpse
		if isCorpse then
			table.clear(originalProps)
			for _, desc in ipairs(model:GetDescendants()) do
				if desc:IsA("BasePart") then
					originalProps[desc] = { Color = desc.Color, Material = desc.Material, TextureID = desc:IsA("MeshPart") and desc.TextureID or nil }
					desc.Color = Color3.fromRGB(40, 54, 64)
					desc.Material = Enum.Material.Basalt
					if desc:IsA("MeshPart") then desc.TextureID = "" end
				elseif desc:IsA("Decal") or desc:IsA("Texture") or desc:IsA("SurfaceAppearance") or desc:IsA("Clothing") then
					originalProps[desc] = desc.Parent
					desc.Parent = nil
				end
			end
		else
			for obj, data in pairs(originalProps) do
				if typeof(data) == "table" then
					obj.Color = data.Color
					obj.Material = data.Material
					if data.TextureID then obj.TextureID = data.TextureID end
				else
					obj.Parent = data
				end
			end
			table.clear(originalProps)
		end
	end,
	[Enum.KeyCode.Q] = function() captureNext() end,
	[Enum.KeyCode.C] = function() captureNext(true) end,
}

UIS.InputBegan:Connect(function(input, processed)
	if processed then return end
	if previewing and input.KeyCode ~= Enum.KeyCode.P and input.KeyCode ~= Enum.KeyCode.Space then return end
	local action = keyActions[input.KeyCode]
	if action then
		action()
		if input.KeyCode ~= Enum.KeyCode.P and input.KeyCode ~= Enum.KeyCode.Space then
			setFrame(currentFrame)
		end
	end
end)
