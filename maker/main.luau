local RunService = game:GetService("RunService")
local UIS = game:GetService("UserInputService")
local Players = game:GetService("Players")
local CaptureService = game:GetService("CaptureService")
local Lighting = game:GetService("Lighting")
local KeyframeSequenceProvider = game:GetService("KeyframeSequenceProvider")

--  CONFIGURATION
local ROTATIONS       = 24					-- number of frames per full rotation
local DISTANCE        = 12       			-- camera distance from model center
local HEIGHT_STEP     = .1        			-- units moved per E/D press
local HEIGHT_MIN      = -10      			-- lowest the camera can go (relative to model center)
local HEIGHT_MAX      = 10       			-- highest the camera can go (relative to model center)
local PIVOT_STEP      = .1      			-- units moved per A/S press
local PIVOT_MIN       = -10      			-- leftmost pivot shift allowed
local PIVOT_MAX       = 10       			-- rightmost pivot shift allowed
local DEPTH_MIN       = -10      			-- furthest depth shift allowed
local DEPTH_MAX       = 10       			-- nearest depth shift allowed
local ZOOM_STEP       = .1        			-- units moved per R/F press
local ZOOM_MIN        = 2        			-- closest the camera can get
local ZOOM_MAX        = 50       			-- furthest the camera can get
local BACKDROP		  = true       			-- whether to use a backdrop behind the model
local BACKDROP_COLOR  = Color3.new(1, 0, 1)	-- chroma key color (magenta)
local FLAT_LIGHTING   = false     			-- set false to keep your scene lighting as is
local BLOOM_INTENSITY = 0					-- set to nil to leave bloom as is
local CLEAR_WORKSPACE = true				-- remove everything in workspace except Target model
local PREVIEW_FPS     = 10					-- frames per second during preview rotation

local player = Players.LocalPlayer
for _, gui in ipairs(player.PlayerGui:GetChildren()) do
	if gui:IsA("ScreenGui") then gui.Enabled = false end
end

local bg = Instance.new("ScreenGui", player.PlayerGui)
bg.IgnoreGuiInset = true
bg.DisplayOrder = 999

local label = Instance.new("TextLabel", bg)
label.Size = UDim2.fromOffset(700, 30)
label.Position = UDim2.new(0.5, -350, 0, 16)
label.BackgroundTransparency = 1
label.TextColor3 = Color3.new(1, 1, 1)
label.TextSize = 20
label.Font = Enum.Font.GothamBold

local subLabel = Instance.new("TextLabel", bg)
subLabel.Size = UDim2.fromOffset(700, 24)
subLabel.Position = UDim2.new(0.5, -350, 0, 44)
subLabel.BackgroundTransparency = 1
subLabel.TextColor3 = Color3.new(0.8, 0.8, 0.8)
subLabel.TextSize = 16
subLabel.Font = Enum.Font.Gotham

local previewLabel = Instance.new("TextLabel", bg)
previewLabel.Size = UDim2.fromOffset(700, 30)
previewLabel.Position = UDim2.new(0.5, -350, 0, 16)
previewLabel.BackgroundTransparency = 1
previewLabel.TextColor3 = Color3.new(0, 1, 0.5)
previewLabel.TextSize = 22
previewLabel.Font = Enum.Font.GothamBold
previewLabel.Visible = false

local previewSubLabel = Instance.new("TextLabel", bg)
previewSubLabel.Size = UDim2.fromOffset(700, 24)
previewSubLabel.Position = UDim2.new(0.5, -350, 0, 44)
previewSubLabel.BackgroundTransparency = 1
previewSubLabel.TextColor3 = Color3.new(0.7, 1, 0.8)
previewSubLabel.TextSize = 16
previewSubLabel.Font = Enum.Font.Gotham
previewSubLabel.Visible = false

local model = workspace:WaitForChild("Target", 10)
if not model then return warn("No 'Target' found in workspace!") end

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

for _, desc in ipairs(model:GetDescendants()) do
	if desc:IsA("ParticleEmitter") or desc:IsA("Trail") or desc:IsA("Beam")
		or desc:IsA("Smoke") or desc:IsA("Fire") or desc:IsA("Sparkles") then
		desc.Enabled = false
	end
end

local targetAnimSeq = model:FindFirstChild("Animation")
local humOrCtrl = model:FindFirstChildOfClass("Humanoid") or model:FindFirstChildOfClass("AnimationController")

if humOrCtrl and targetAnimSeq and targetAnimSeq:IsA("KeyframeSequence") then
	local animator = humOrCtrl:FindFirstChildOfClass("Animator") or Instance.new("Animator", humOrCtrl)
	local anim = Instance.new("Animation")
	anim.AnimationId = KeyframeSequenceProvider:RegisterKeyframeSequence(targetAnimSeq)
	local track = animator:LoadAnimation(anim)
	track:Play()
	track:AdjustSpeed(0)
	track.TimePosition = 0
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
	Lighting.Ambient = Color3.new(1, 1, 1)
	Lighting.OutdoorAmbient = Color3.new(1, 1, 1)
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
pivotDot.BackgroundColor3 = Color3.new(1, 1, 0)
Instance.new("UICorner", pivotDot).CornerRadius = UDim.new(1, 0)

local cam = workspace.CurrentCamera
cam.CameraType = Enum.CameraType.Scriptable
cam.FieldOfView = fov

local currentFrame        = 1
local heightOffset        = 0
local distance            = DISTANCE
local pivotOffset         = 0
local depthOffset         = 0
local showPivot           = false
local previewing          = false
local previewConnection   = nil
local capturing           = false
local paused              = false

local function getLookAt()
	local right   = Vector3.new(math.cos(startingAngleOffset), 0, -math.sin(startingAngleOffset))
	local forward = Vector3.new(math.sin(startingAngleOffset), 0,  math.cos(startingAngleOffset))
	return modelCenter + right * pivotOffset + forward * depthOffset + Vector3.new(0, heightOffset, 0)
end

local function updateUI()
	if previewing then
		local statusText = paused and "[PAUSED]" or "[PREVIEWING]"
		previewLabel.Text = string.format("%s | Height: %+.1f | Pivot: %+.1f | Depth: %+.1f | Zoom: %.1f | Frame %d/%d",
			statusText, heightOffset, pivotOffset, depthOffset, distance, currentFrame, ROTATIONS)
		previewSubLabel.Text = "P: stop preview | Space: pause/resume"
	else
		local side = "Front"
		if rootPart then
			side = math.abs(startingAngleOffset - getFrontAngle()) < 0.01 and "Front" or "Back"
		end
		label.Text = string.format("Start: %s | Height: %+.1f | Pivot: %+.1f | Depth: %+.1f | Zoom: %.1f | Frame %d/%d",
			side, heightOffset, pivotOffset, depthOffset, distance, currentFrame, ROTATIONS)
		subLabel.Text = "W: flip | E/D: height | A/S: pivot | Z/X: depth | R/F: zoom | O: pivot point | P: preview | Q: begin capture"
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

local function captureNext()
	if capturing or currentFrame > ROTATIONS then return end
	capturing = true

	label.Visible = false
	subLabel.Visible = false
	previewLabel.Visible = false
	previewSubLabel.Visible = false
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

			if currentFrame < ROTATIONS then
				currentFrame += 1
				setFrame(currentFrame)
				capturing = false
				captureNext()
			else
				UIS.MouseIconEnabled = true
				if previewing then
					previewLabel.Visible = true
					previewSubLabel.Visible = true
					previewLabel.Text = "Heeho! All frames captured!"
					previewSubLabel.Text = ""
				else
					label.Visible = true
					subLabel.Visible = true
					label.Text = "Heeho! All frames captured!"
					subLabel.Text = ""
				end
				pivotBillboard.Enabled = showPivot
				print("Heeho! All frames captured!")
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
	[Enum.KeyCode.O] = function()
		showPivot = not showPivot
		pivotBillboard.Enabled = showPivot
	end,
	[Enum.KeyCode.P] = function()
		if previewing then
			stopPreview()
		else
			startPreview()
		end
	end,
	[Enum.KeyCode.Space] = function()
		if previewing then
			paused = not paused
			setFrame(currentFrame)
		end
	end,
	[Enum.KeyCode.Q] = function()
		captureNext()
	end,
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
