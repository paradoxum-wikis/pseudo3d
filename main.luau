local RunService = game:GetService("RunService")
local UIS = game:GetService("UserInputService")
local Players = game:GetService("Players")
local CaptureService = game:GetService("CaptureService")
local Lighting = game:GetService("Lighting")
local KeyframeSequenceProvider = game:GetService("KeyframeSequenceProvider")

--  CONFIGURATION
local ROTATIONS       = 24					-- number of frames per full rotation
local DISTANCE        = 12       			-- camera distance from model center
local HEIGHT_STEP     = 1        			-- units moved per E/D press
local HEIGHT_MIN      = -10      			-- lowest the camera can go (relative to model center)
local HEIGHT_MAX      = 10       			-- highest the camera can go (relative to model center)
local PIVOT_STEP      = 0.5      			-- units moved per A/S press
local PIVOT_MIN       = -10      			-- leftmost pivot shift allowed
local PIVOT_MAX       = 10       			-- rightmost pivot shift allowed
local ZOOM_STEP       = 1        			-- units moved per R/F press
local ZOOM_MIN        = 2        			-- closest the camera can get
local ZOOM_MAX        = 50       			-- furthest the camera can get
local BACKDROP_COLOR  = Color3.new(1, 0, 1)	-- chroma key color (magenta)
local FLAT_LIGHTING   = true     			-- set false to keep your scene lighting as is

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

local model = workspace:WaitForChild("Target", 10)
if not model then return warn("No 'Target' found in workspace!") end

local rootPart = model.PrimaryPart or model:FindFirstChild("HumanoidRootPart") or model:FindFirstChildWhichIsA("BasePart")
if rootPart then rootPart.Anchored = true end

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

local backdrop = Instance.new("Part", workspace)
backdrop.Size = Vector3.new(2000, 2000, 1)
backdrop.Anchored = true
backdrop.CanCollide = false
backdrop.CastShadow = false

local surfaceGui = Instance.new("SurfaceGui", backdrop)
surfaceGui.LightInfluence = 0
local bgFrame = Instance.new("Frame", surfaceGui)
bgFrame.Size = UDim2.fromScale(1, 1)
bgFrame.BackgroundColor3 = BACKDROP_COLOR

local cam = workspace.CurrentCamera
cam.CameraType = Enum.CameraType.Scriptable
cam.FieldOfView = fov

local currentFrame        = 1
local heightOffset        = 0
local distance            = DISTANCE
local pivotOffset         = 0    -- horizontal shift of the rotation pivot (world X)
local startingAngleOffset = 0
local captureStarted      = false
local capturing           = false

local function getLookAt()
	return modelCenter + Vector3.new(pivotOffset, heightOffset, 0)
end

local function setFrame(i)
	local lookAt = getLookAt()
	local angle  = math.rad((i - 1) * (360 / ROTATIONS)) + startingAngleOffset
	local camPos = lookAt + Vector3.new(math.sin(angle) * distance, 0, math.cos(angle) * distance)
	cam.CFrame   = CFrame.lookAt(camPos, lookAt)
	backdrop.CFrame = CFrame.lookAt(lookAt + ((lookAt - camPos).Unit * 500), camPos)

	if not captureStarted then
		local side = startingAngleOffset == 0 and "Front" or "Back"
		label.Text = string.format("Start: %s | Height: %+.1f | Pivot: %+.1f | Zoom: %.1f | Frame %d/%d", side, heightOffset, pivotOffset, distance, i, ROTATIONS)
		subLabel.Text = "W: flip | E/D: height | A/S: pivot | R/F: zoom | Q: begin capture"
	else
		label.Text = string.format("Capturing frame %d / %d  -  Height: %+.1f  Pivot: %+.1f", i, ROTATIONS, heightOffset, pivotOffset)
		subLabel.Text = ""
	end
end

local function captureNext()
	if capturing or currentFrame > ROTATIONS then return end
	capturing = true

	label.Visible = false
	subLabel.Visible = false
	UIS.MouseIconEnabled = false
	RunService.RenderStepped:Wait()
	RunService.RenderStepped:Wait()

	CaptureService:CaptureScreenshot(function(tempContentId)
		CaptureService:PromptSaveCapturesToGallery({tempContentId}, function(results)
			local saved = results[tempContentId]
			print(string.format("Frame %d/%d captured (saved: %s)", currentFrame, ROTATIONS, tostring(saved)))

			if currentFrame < ROTATIONS then
				currentFrame += 1
				setFrame(currentFrame)
				RunService.RenderStepped:Wait()
				RunService.RenderStepped:Wait()
				UIS.MouseIconEnabled = true
				label.Visible = true
				subLabel.Visible = true
				capturing = false
				captureNext()
			else
				currentFrame += 1
				UIS.MouseIconEnabled = true
				label.Visible = true
				subLabel.Visible = true
				label.Text = "Heeho! All frames captured!"
				subLabel.Text = ""
				print("Heeho! All frames captured!")
				capturing = false
			end
		end)
	end)
end

setFrame(1)

UIS.InputBegan:Connect(function(input, processed)
	if captureStarted then return end

	if input.KeyCode == Enum.KeyCode.W then
		startingAngleOffset = startingAngleOffset == 0 and math.pi or 0
		setFrame(currentFrame)

	elseif input.KeyCode == Enum.KeyCode.E then
		heightOffset = math.min(heightOffset + HEIGHT_STEP, HEIGHT_MAX)
		setFrame(currentFrame)

	elseif input.KeyCode == Enum.KeyCode.D then
		heightOffset = math.max(heightOffset - HEIGHT_STEP, HEIGHT_MIN)
		setFrame(currentFrame)

	elseif input.KeyCode == Enum.KeyCode.A then
		pivotOffset = math.max(pivotOffset - PIVOT_STEP, PIVOT_MIN)
		setFrame(currentFrame)

	elseif input.KeyCode == Enum.KeyCode.S then
		pivotOffset = math.min(pivotOffset + PIVOT_STEP, PIVOT_MAX)
		setFrame(currentFrame)

	elseif input.KeyCode == Enum.KeyCode.R then
		distance = math.max(distance - ZOOM_STEP, ZOOM_MIN)
		setFrame(currentFrame)

	elseif input.KeyCode == Enum.KeyCode.F then
		distance = math.min(distance + ZOOM_STEP, ZOOM_MAX)
		setFrame(currentFrame)

	elseif input.KeyCode == Enum.KeyCode.Q then
		captureStarted = true
		setFrame(currentFrame)
		captureNext()
	end
end)
