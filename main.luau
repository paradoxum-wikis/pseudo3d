local RunService = game:GetService("RunService")
local UIS = game:GetService("UserInputService")
local Players = game:GetService("Players")
local CaptureService = game:GetService("CaptureService")
local Lighting = game:GetService("Lighting")
local KeyframeSequenceProvider = game:GetService("KeyframeSequenceProvider")

local ROTATIONS = 24
local DISTANCE = 12

local player = Players.LocalPlayer
for _, gui in ipairs(player.PlayerGui:GetChildren()) do
	if gui:IsA("ScreenGui") then gui.Enabled = false end
end

local bg = Instance.new("ScreenGui", player.PlayerGui)
bg.IgnoreGuiInset = true
bg.DisplayOrder = 999

local label = Instance.new("TextLabel", bg)
label.Size = UDim2.fromOffset(500, 50)
label.Position = UDim2.new(0.5, -250, 0, 20)
label.BackgroundTransparency = 1
label.TextColor3 = Color3.new(1, 1, 1)
label.TextSize = 24
label.Font = Enum.Font.GothamBold

local model = workspace:WaitForChild("Target", 10)
if not model then return warn("No 'Target' found!") end

local rootPart = model.PrimaryPart or model:FindFirstChild("HumanoidRootPart") or model:FindFirstChildWhichIsA("BasePart")
if rootPart then rootPart.Anchored = true end

for _, part in ipairs(model:GetDescendants()) do
	if part:IsA("BasePart") and part ~= rootPart then
		part.Anchored = false
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

local minY, maxY, minX, maxX, minZ, maxZ = math.huge, -math.huge, math.huge, -math.huge, math.huge, -math.huge
for _, part in ipairs(model:GetDescendants()) do
	if part:IsA("BasePart") then
		local p, s = part.Position, part.Size
		minY, maxY = math.min(minY, p.Y - s.Y/2), math.max(maxY, p.Y + s.Y/2)
		minX, maxX = math.min(minX, p.X - s.X/2), math.max(maxX, p.X + s.X/2)
		minZ, maxZ = math.min(minZ, p.Z - s.Z/2), math.max(maxZ, p.Z + s.Z/2)
	end
end

local lookAt = Vector3.new((minX + maxX)/2, (minY + maxY)/2, (minZ + maxZ)/2)
local fov = math.deg(2 * math.atan((math.max(maxY - minY, math.max(maxX - minX, maxZ - minZ)) * 0.6) / DISTANCE))

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

local backdrop = Instance.new("Part", workspace)
backdrop.Size = Vector3.new(2000, 2000, 1)
backdrop.Anchored = true
backdrop.CanCollide = false
backdrop.CastShadow = false

local surfaceGui = Instance.new("SurfaceGui", backdrop)
surfaceGui.LightInfluence = 0
local bgFrame = Instance.new("Frame", surfaceGui)
bgFrame.Size = UDim2.fromScale(1, 1)
bgFrame.BackgroundColor3 = Color3.new(1, 0, 1)

local cam = workspace.CurrentCamera
cam.CameraType = Enum.CameraType.Scriptable
cam.FieldOfView = fov

local currentFrame = 1
local startingAngleOffset = 0
local captureStarted = false
local capturing = false

local function setFrame(i)
	local angle = math.rad((i - 1) * (360 / ROTATIONS)) + startingAngleOffset
	local camPos = lookAt + Vector3.new(math.sin(angle) * DISTANCE, 0, math.cos(angle) * DISTANCE)
	cam.CFrame = CFrame.lookAt(camPos, lookAt)
	backdrop.CFrame = CFrame.lookAt(lookAt + ((lookAt - camPos).Unit * 500), camPos)
	if not captureStarted then
		local side = startingAngleOffset == 0 and "Front" or "Back"
		label.Text = string.format("Starting from: %s  —  W to flip, Q to begin", side)
	else
		label.Text = string.format("Frame %d/%d  —  Capturing...", i, ROTATIONS)
	end
end

local function captureNext()
	if capturing or currentFrame > ROTATIONS then return end
	capturing = true

	label.Visible = false
	UIS.MouseIconEnabled = false
	RunService.RenderStepped:Wait()
	RunService.RenderStepped:Wait()

	CaptureService:CaptureScreenshot(function(tempContentId)
		CaptureService:PromptSaveCapturesToGallery(
			{tempContentId},
			function(results)
				local saved = results[tempContentId]
				print(string.format("Captured frame %d/%d (saved: %s)", currentFrame, ROTATIONS, tostring(saved)))

				if currentFrame < ROTATIONS then
					currentFrame += 1
					setFrame(currentFrame)

					RunService.RenderStepped:Wait()
					RunService.RenderStepped:Wait()

					UIS.MouseIconEnabled = true
					label.Visible = true
					capturing = false
					captureNext()
				else
					currentFrame += 1
					UIS.MouseIconEnabled = true
					label.Visible = true
					label.Text = "Heeho! All frames captured!"
					print("Heeho! All frames captured!")
					capturing = false
				end
			end
		)
	end)
end

setFrame(1)

UIS.InputBegan:Connect(function(input, processed)
	if input.KeyCode == Enum.KeyCode.W and not captureStarted then
		startingAngleOffset = startingAngleOffset == 0 and math.pi or 0
		setFrame(1)
		return
	end

	if input.KeyCode == Enum.KeyCode.Q and not captureStarted then
		captureStarted = true
		setFrame(currentFrame)
		captureNext()
	end
end)
