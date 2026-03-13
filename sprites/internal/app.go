package internal

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/disintegration/imaging"
)

type SpriteApp struct {
	App               fyne.App
	Window            fyne.Window
	Files             []string
	NaturalSizes      []image.Point
	PreviewImages     []image.Image
	CurrentImageIndex int

	ImgWidget          *canvas.Image
	Overlay            *SelectionOverlay
	StatusLabel        *widget.Label
	FrameLabel         *widget.Label
	Slider             *widget.Slider
	SaveBtn            *widget.Button
	SaveOneBtn         *widget.Button
	ButtonBox          *fyne.Container
	ProgressBar        *widget.ProgressBar
	ColorPreview       *canvas.Rectangle
	PickColorToggleBtn *widget.Button

	pendingUpdateInfo *UpdateInfo
	UpdateChipLabel   *widget.Label
	UpdateChipAction  *widget.Hyperlink
	Username          string
	UserAvatar        *canvas.Image

	TdswPreview *canvas.Image
	AewPreview  *canvas.Image
	TdswCanvas  *image.RGBA
	AewCanvas   *image.RGBA

	LockAspect    bool
	IsNewDrag     bool
	ColorPickMode bool
	StartImgX     float32
	StartImgY     float32
	LastPreview   time.Time
}

var StartupUpdateError error

func RunUI() {
	fyneApp := app.New()
	w := fyneApp.NewWindow("Pseudo3D Sprites")
	w.Resize(fyne.NewSize(1024, 576))

	sa := &SpriteApp{
		App:        fyneApp,
		Window:     w,
		LockAspect: true,
		IsNewDrag:  true,
	}

	sa.buildUI()
	sa.setupShortcuts()

	if StartupUpdateError != nil {
		dialog.ShowInformation("Update Failed", StartupUpdateError.Error(), sa.Window)
	} else {
		sa.checkChangelog()
	}

	sa.startUpdateCheck()

	initialFiles, _ := GetPNGFiles(InputDir)
	if len(initialFiles) > 0 {
		startupProgress := widget.NewProgressBar()
		startupLabel := widget.NewLabel("Hee! I'm loading your frames.. ho!")

		loadingDialog := dialog.NewCustomWithoutButtons(
			"Starting Up",
			container.NewVBox(startupLabel, startupProgress),
			sa.Window,
		)
		loadingDialog.Show()

		go func() {
			sa.loadFrames(initialFiles, startupProgress.SetValue)

			fyne.Do(func() {
				loadingDialog.Hide()
			})
		}()
	}

	w.ShowAndRun()
}

func (sa *SpriteApp) buildUI() {
	sa.ImgWidget = canvas.NewImageFromImage(nil)
	sa.ImgWidget.FillMode = canvas.ImageFillContain
	sa.Overlay = NewSelectionOverlay()
	sa.Overlay.GetImageBounds = sa.getRenderedImageBounds

	sa.StatusLabel = widget.NewLabel("No selection...")
	sa.StatusLabel.Alignment = fyne.TextAlignCenter
	sa.FrameLabel = widget.NewLabel("Frame 0 / 0")

	sa.buildUpdateControls()
	sa.buildPreviewPanel()
	sa.buildSlider()
	sa.buildProcessBox()

	dragArea := sa.buildDragArea()
	imageContainer := container.NewStack(sa.ImgWidget, sa.Overlay, dragArea)

	sa.ColorPreview = canvas.NewRectangle(ChromaKey)
	sa.ColorPreview.SetMinSize(fyne.NewSize(30, 20))

	sa.PickColorToggleBtn = widget.NewButton("Chroma Key Picker", nil)
	sa.PickColorToggleBtn.OnTapped = func() {
		sa.ColorPickMode = !sa.ColorPickMode
		if sa.ColorPickMode {
			sa.PickColorToggleBtn.SetText("(Click something on the image you fool!)")
		} else {
			sa.PickColorToggleBtn.SetText("Chroma Key Picker")
		}
	}

	colorBox := container.NewHBox(
		widget.NewLabel("Key:"),
		container.NewCenter(sa.ColorPreview),
		sa.PickColorToggleBtn,
	)

	helpBtn := widget.NewButton("?", func() {
		dialog.ShowInformation("Controls",
			"Draw a selection by dragging on the image.\n\nArrow keys: nudge selection by 1px\nShift + Arrow keys: nudge by 10px",
			sa.Window)
	})

	toggleLockBtn := widget.NewButton("Square Ratio", nil)
	toggleLockBtn.OnTapped = func() {
		sa.LockAspect = !sa.LockAspect
		if sa.LockAspect {
			toggleLockBtn.SetText("Square Ratio")
		} else {
			toggleLockBtn.SetText("Freeform")
		}
	}

	importBtn := sa.buildImportBtn()
	settingsBtn := sa.buildSettingsBtn()

	sa.ButtonBox = container.NewBorder(nil, nil, nil, sa.SaveOneBtn, sa.SaveBtn)
	processBox := container.NewStack(sa.ButtonBox, sa.ProgressBar)

	updateBox := container.NewHBox(
		container.NewCenter(sa.UserAvatar),
		sa.UpdateChipLabel,
		sa.UpdateChipAction,
	)

	footer := container.NewBorder(nil, nil, nil, updateBox, sa.StatusLabel)

	controls := container.NewVBox(
		container.NewBorder(nil, nil, nil, container.NewHBox(importBtn, settingsBtn, layout.NewSpacer(), colorBox, helpBtn), toggleLockBtn),
		container.NewBorder(nil, nil, nil, sa.FrameLabel, sa.Slider),
		processBox,
		footer,
	)

	previewPanel := container.NewVBox(
		widget.NewLabel("TDS Wiki (10:9)"),
		container.NewCenter(sa.TdswPreview),
		widget.NewLabel("ALTERPEDIA (9:10)"),
		container.NewCenter(sa.AewPreview),
	)

	sa.Window.SetContent(container.NewBorder(nil, controls, nil, nil,
		container.NewBorder(nil, nil, nil, previewPanel, imageContainer),
	))
}

func (sa *SpriteApp) loadAvatar(username string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://github.com/" + username + ".png?size=24")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if img, _, err := image.Decode(resp.Body); err == nil {
		fyne.Do(func() {
			sa.UserAvatar.Image = img
			sa.UserAvatar.Show()
			sa.UserAvatar.Refresh()
		})
	}
}

func (sa *SpriteApp) buildUpdateControls() {
	sa.UpdateChipLabel = widget.NewLabel("checking...")
	sa.UpdateChipLabel.TextStyle = fyne.TextStyle{Italic: true}

	sa.UpdateChipAction = widget.NewHyperlink("", nil)
	sa.UpdateChipAction.Hide()

	sa.UserAvatar = canvas.NewImageFromImage(nil)
	sa.UserAvatar.FillMode = canvas.ImageFillContain
	sa.UserAvatar.SetMinSize(fyne.NewSize(24, 24))
	sa.UserAvatar.Hide()
}

func (sa *SpriteApp) setChip(label, actionText string, action func()) {
	sa.UpdateChipLabel.SetText(label)
	if actionText != "" && action != nil {
		sa.UpdateChipAction.SetText(actionText)
		sa.UpdateChipAction.OnTapped = action
		sa.UpdateChipAction.Show()
	} else {
		sa.UpdateChipAction.Hide()
	}
}

func (sa *SpriteApp) startUpdateCheck() {
	go func() {
		info, username, err := CheckForUpdates(CurrentVersion)

		fyne.Do(func() {
			sa.Username = username
			prefix := ""
			if username != "" {
				prefix = "@" + username + "  -  "
				go sa.loadAvatar(username)
			}

			if err != nil {
				sa.setChip(err.Error(), "log in", sa.handleLogin)
				return
			}

			switch info.State {
			case UpdateStateLoginRequired:
				sa.setChip(prefix+"log in to check updates", "log in", sa.handleLogin)
			case UpdateStateReady:
				if info.HasUpdate {
					sa.pendingUpdateInfo = info
					sa.setChip(prefix+"update available: "+info.LatestTag, "update now", sa.handleUpdate)
				} else {
					sa.setChip(prefix+"up to date", "", nil)
				}
			}
		})
	}()
}

func (sa *SpriteApp) handleLogin() {
	verificationURI, userCode, err := BeginGitHubDeviceLogin()
	if err != nil {
		dialog.ShowError(err, sa.Window)
		return
	}

	parsedURL, _ := url.Parse(verificationURI)

	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Enter this code when prompted:\n\n%s", userCode)),
		widget.NewHyperlink("Browser didn't open? Click this!", parsedURL),
	)
	dialog.ShowCustom("GitHub Login", "OK", content, sa.Window)

	sa.setChip("waiting for GitHub login...", "", nil)

	go func() {
		time.Sleep(1 * time.Second)
		fyne.Do(func() { sa.App.OpenURL(parsedURL) })
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		timeout := time.After(2 * time.Minute)

		for {
			select {
			case <-timeout:
				fyne.Do(func() {
					sa.setChip("login timed out", "try again", sa.handleLogin)
				})
				return
			case <-ticker.C:
				err := CompleteGitHubDeviceLogin()
				if err == nil {
					fyne.Do(func() { sa.setChip("checking...", "", nil) })
					sa.startUpdateCheck()
					return
				}
				if err.Error() == "authorization_pending" || err.Error() == "slow_down" {
					continue
				}
				fyne.Do(func() {
					sa.setChip("login failed", "try again", sa.handleLogin)
					dialog.ShowError(err, sa.Window)
				})
				return
			}
		}
	}()
}

func (sa *SpriteApp) handleUpdate() {
	if sa.pendingUpdateInfo == nil {
		return
	}
	sa.setChip("downloading update...", "", nil)

	go func() {
		err := DownloadUpdate(sa.pendingUpdateInfo)
		if err == nil {
			exePath, _ := os.Executable()
			path := filepath.Join(filepath.Dir(exePath), "archive", "changelog.md")
			_ = os.WriteFile(path, []byte(sa.pendingUpdateInfo.Changelog), 0644)
		}

		fyne.Do(func() {
			if err != nil {
				sa.setChip("download failed", "retry", sa.handleUpdate)
				dialog.ShowError(err, sa.Window)
				return
			}
			sa.setChip("update ready", "restart to apply", func() {
				exePath, _ := os.Executable()
				exec.Command(exePath, os.Args[1:]...).Start()
				os.Exit(0)
			})
		})
	}()
}

func (sa *SpriteApp) checkChangelog() {
	exePath, _ := os.Executable()
	path := filepath.Join(filepath.Dir(exePath), "archive", "changelog.md")

	if data, err := os.ReadFile(path); err == nil {
		os.Remove(path)

		md := widget.NewRichTextFromMarkdown(string(data))
		md.Wrapping = fyne.TextWrapWord
		scroll := container.NewScroll(md)
		scroll.SetMinSize(fyne.NewSize(500, 400))

		dialog.ShowCustom("Update Applied! Heeho!", "Thanks", scroll, sa.Window)
	}
}

func (sa *SpriteApp) buildSlider() {
	sa.Slider = widget.NewSlider(0, 0)
	sa.Slider.OnChanged = func(v float64) {
		if len(sa.Files) == 0 {
			return
		}
		newIndex := int(v)
		if newIndex == sa.CurrentImageIndex {
			return
		}
		sa.CurrentImageIndex = newIndex

		sa.ImgWidget.Image = sa.PreviewImages[sa.CurrentImageIndex]
		sa.ImgWidget.Refresh()

		nat := sa.NaturalSizes[sa.CurrentImageIndex]
		sa.Overlay.NatW = float32(nat.X)
		sa.Overlay.NatH = float32(nat.Y)
		sa.Overlay.Refresh()
		sa.FrameLabel.SetText(fmt.Sprintf("Frame %d / %d", sa.CurrentImageIndex+1, len(sa.Files)))
		sa.updatePreview()
	}
}

func (sa *SpriteApp) buildProcessBox() {
	sa.ProgressBar = widget.NewProgressBar()
	sa.ProgressBar.Hide()

	sa.SaveBtn = widget.NewButton("Save & Process!", func() {
		fname := OutputFile
		if len(sa.Files) == 1 {
			fname = OneShotFile
		}
		msg := fmt.Sprintf("Heeho! Spritesheet saved as %s (%d frames)!", fname, len(sa.Files))
		sa.processSelection(nil, msg)
	})

	sa.SaveOneBtn = widget.NewButton("One-shot current frame!", func() {
		msg := fmt.Sprintf("Heeho! One-shot saved as %s!", OneShotFile)
		sa.processSelection([]string{sa.Files[sa.CurrentImageIndex]}, msg)
	})
}

func (sa *SpriteApp) processSelection(filesToProcess []string, successMsg string) {
	if !sa.Overlay.HasSelection || sa.Overlay.MinX == sa.Overlay.MaxX || len(sa.Files) == 0 {
		return
	}

	GlobalSafeZone = SafeZone{
		MinX:   int(math.Round(float64(sa.Overlay.MinX))),
		MinY:   int(math.Round(float64(sa.Overlay.MinY))),
		MaxX:   int(math.Round(float64(sa.Overlay.MaxX))),
		MaxY:   int(math.Round(float64(sa.Overlay.MaxY))),
		Active: true,
	}
	SaveSafeZoneConfig()
	sa.setProcessingState(true)

	go func() {
		err := RunBatchProcessing(filesToProcess, func(current, total int) {
			fyne.Do(func() { sa.ProgressBar.SetValue(float64(current) / float64(total)) })
		})

		fyne.Do(func() {
			sa.setProcessingState(false)
			if err != nil {
				dialog.ShowError(err, sa.Window)
			} else {
				dialog.ShowInformation("Success", successMsg, sa.Window)
			}
		})
	}()
}

func (sa *SpriteApp) setProcessingState(processing bool) {
	if processing {
		sa.SaveBtn.Hide()
		sa.SaveOneBtn.Hide()
		sa.ProgressBar.SetValue(0)
		sa.ProgressBar.Show()
	} else {
		sa.SaveBtn.Show()
		if len(sa.Files) > 1 {
			sa.SaveOneBtn.Show()
		} else {
			sa.SaveOneBtn.Hide() // hide oneshot if there's only 1 frame
		}
		sa.ProgressBar.Hide()
	}
	sa.ButtonBox.Refresh()
}

func (sa *SpriteApp) buildPreviewPanel() {
	const previewSize = float32(180)

	makeCropPreview := func(clipW, clipH float32) *canvas.Image {
		img := canvas.NewImageFromImage(nil)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(clipW, clipH))
		return img
	}

	sa.TdswPreview = makeCropPreview(previewSize, previewSize*0.9)
	sa.AewPreview = makeCropPreview(previewSize*0.9, previewSize)
	sa.TdswCanvas = image.NewRGBA(image.Rect(0, 0, int(previewSize), int(previewSize*0.9)))
	sa.AewCanvas = image.NewRGBA(image.Rect(0, 0, int(previewSize*0.9), int(previewSize)))
}

func (sa *SpriteApp) fillCheckerboard(canvas *image.RGBA) {
	const size = 10
	color1 := image.NewUniform(color.RGBA{220, 220, 220, 255})
	color2 := image.NewUniform(color.RGBA{255, 255, 255, 255})
	w, h := canvas.Rect.Dx(), canvas.Rect.Dy()
	for y := 0; y < h; y += size {
		for x := 0; x < w; x += size {
			c := color1
			if ((x/size)+(y/size))&1 != 0 {
				c = color2
			}
			draw.Draw(canvas, image.Rect(x, y, x+size, y+size), c, image.Point{}, draw.Src)
		}
	}
}

func (sa *SpriteApp) updatePreview() {
	if !sa.Overlay.HasSelection || len(sa.NaturalSizes) == 0 {
		sa.TdswPreview.Image, sa.AewPreview.Image = nil, nil
		sa.TdswPreview.Refresh()
		sa.AewPreview.Refresh()
		return
	}
	nat := sa.NaturalSizes[sa.CurrentImageIndex]
	minX := min(max(int(math.Round(float64(sa.Overlay.MinX))), 0), nat.X)
	minY := min(max(int(math.Round(float64(sa.Overlay.MinY))), 0), nat.Y)
	maxX := min(max(int(math.Round(float64(sa.Overlay.MaxX))), 0), nat.X)
	maxY := min(max(int(math.Round(float64(sa.Overlay.MaxY))), 0), nat.Y)
	if maxX <= minX || maxY <= minY {
		return
	}
	pb := sa.PreviewImages[sa.CurrentImageIndex].Bounds()
	scaleX := float64(pb.Dx()) / float64(nat.X)
	scaleY := float64(pb.Dy()) / float64(nat.Y)
	cropped := imaging.Crop(sa.PreviewImages[sa.CurrentImageIndex], image.Rect(
		int(float64(minX)*scaleX), int(float64(minY)*scaleY),
		int(float64(maxX)*scaleX), int(float64(maxY)*scaleY),
	))

	fitAndCenter := func(img image.Image, canvas *image.RGBA) image.Image {
		w, h := canvas.Rect.Dx(), canvas.Rect.Dy()
		scaled := imaging.Resize(img, w, 0, imaging.NearestNeighbor)
		sa.fillCheckerboard(canvas)
		b := scaled.Bounds()
		dx := (w - b.Dx()) / 2
		dy := (h - b.Dy()) / 2
		draw.Draw(canvas, image.Rectangle{Min: image.Pt(dx, dy), Max: image.Pt(dx, dy).Add(b.Size())}, scaled, b.Min, draw.Over)
		return canvas
	}

	sa.TdswPreview.Image = fitAndCenter(cropped, sa.TdswCanvas)
	sa.AewPreview.Image = fitAndCenter(cropped, sa.AewCanvas)
	sa.TdswPreview.Refresh()
	sa.AewPreview.Refresh()
}

func (sa *SpriteApp) updateStatus(force bool) {
	if !sa.Overlay.HasSelection {
		sa.StatusLabel.SetText("No selection...")
		sa.updatePreview()
		return
	}
	sa.StatusLabel.SetText(fmt.Sprintf("Selected: %dx%d at (%d, %d)",
		int(sa.Overlay.MaxX-sa.Overlay.MinX), int(sa.Overlay.MaxY-sa.Overlay.MinY),
		int(sa.Overlay.MinX), int(sa.Overlay.MinY)))

	if force || time.Since(sa.LastPreview) > 16*time.Millisecond {
		sa.updatePreview()
		sa.LastPreview = time.Now()
	}
}

func (sa *SpriteApp) getRenderedImageBounds() (offX, offY, rendW, rendH float32) {
	if len(sa.NaturalSizes) == 0 {
		return 0, 0, 0, 0
	}
	nat := sa.NaturalSizes[sa.CurrentImageIndex]
	containerSize := sa.ImgWidget.Size()
	scaleX := containerSize.Width / float32(nat.X)
	scaleY := containerSize.Height / float32(nat.Y)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}
	rendW = float32(nat.X) * scale
	rendH = float32(nat.Y) * scale
	offX = (containerSize.Width - rendW) / 2
	offY = (containerSize.Height - rendH) / 2
	return
}

func (sa *SpriteApp) screenToImage(sx, sy float32) (float32, float32) {
	if len(sa.NaturalSizes) == 0 {
		return 0, 0
	}
	nat := sa.NaturalSizes[sa.CurrentImageIndex]
	ox, oy, rw, rh := sa.getRenderedImageBounds()
	ix := min(max((sx-ox)/rw*float32(nat.X), 0), float32(nat.X))
	iy := min(max((sy-oy)/rh*float32(nat.Y), 0), float32(nat.Y))
	return ix, iy
}

func (sa *SpriteApp) buildDragArea() fyne.Widget {
	dragArea := &InteractiveArea{
		GetCursor: func() desktop.Cursor {
			if sa.ColorPickMode {
				return desktop.CrosshairCursor
			}
			return desktop.PointerCursor
		},
		OnDrag: func(e *fyne.DragEvent) {
			if sa.IsNewDrag {
				sa.StartImgX, sa.StartImgY = sa.screenToImage(e.Position.X-e.Dragged.DX, e.Position.Y-e.Dragged.DY)
				sa.IsNewDrag = false
			}
			curImgX, curImgY := sa.screenToImage(e.Position.X, e.Position.Y)
			rawW, rawH := curImgX-sa.StartImgX, curImgY-sa.StartImgY

			var minX, minY, maxX, maxY float32
			if sa.LockAspect {
				signW := rawW < 0
				signH := rawH < 0
				if signW {
					rawW = -rawW
				}
				if signH {
					rawH = -rawH
				}
				side := min(rawW, rawH)
				if signW {
					minX = sa.StartImgX - side
				} else {
					minX = sa.StartImgX
				}
				if signH {
					minY = sa.StartImgY - side
				} else {
					minY = sa.StartImgY
				}
				maxX, maxY = minX+side, minY+side
			} else {
				minX, maxX = min(sa.StartImgX, curImgX), max(sa.StartImgX, curImgX)
				minY, maxY = min(sa.StartImgY, curImgY), max(sa.StartImgY, curImgY)
			}
			if maxX > minX && maxY > minY {
				sa.Overlay.SetSelection(minX, minY, maxX, maxY)
				sa.updateStatus(false)
			}
		},
		OnDragEnd: func() { sa.IsNewDrag = true; sa.updateStatus(true) },
		OnTap: func(e *fyne.PointEvent) {
			if !sa.ColorPickMode || len(sa.NaturalSizes) == 0 {
				return
			}
			ix, iy := sa.screenToImage(e.Position.X, e.Position.Y)
			nat := sa.NaturalSizes[sa.CurrentImageIndex]
			previewImg := sa.PreviewImages[sa.CurrentImageIndex]
			pb := previewImg.Bounds()

			px := int(float64(ix) / float64(nat.X) * float64(pb.Dx()))
			py := int(float64(iy) / float64(nat.Y) * float64(pb.Dy()))

			px = min(max(px, 0), pb.Dx()-1)
			py = min(max(py, 0), pb.Dy()-1)

			c := previewImg.At(pb.Min.X+px, pb.Min.Y+py)
			r, g, b, _ := c.RGBA()
			ChromaKey = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
			HexColor = fmt.Sprintf("%02X%02X%02X", ChromaKey.R, ChromaKey.G, ChromaKey.B)
			if sa.ColorPreview != nil {
				sa.ColorPreview.FillColor = ChromaKey
				sa.ColorPreview.Refresh()
			}
			sa.ColorPickMode = false
			sa.PickColorToggleBtn.SetText("Chroma Key Picker")
		},
	}
	dragArea.ExtendBaseWidget(dragArea)
	return dragArea
}

func (sa *SpriteApp) loadFrames(newFiles []string, progressCallback func(float64)) {
	shinNaturalSizes := make([]image.Point, len(newFiles))
	shinPreviewImages := make([]image.Image, len(newFiles))

	for i, f := range newFiles {
		if progressCallback != nil {
			fyne.Do(func() {
				progressCallback(float64(i) / float64(len(newFiles)))
			})
		}
		full := LoadImage(f)
		b := full.Bounds()
		shinNaturalSizes[i] = image.Pt(b.Dx(), b.Dy())

		if !SkipPrescale {
			shinPreviewImages[i] = imaging.Fit(full, PreviewMaxPx, PreviewMaxPx, imaging.Lanczos)
		} else {
			shinPreviewImages[i] = full
		}
	}
	if progressCallback != nil {
		fyne.Do(func() {
			progressCallback(1.0)
		})
	}

	fyne.Do(func() {
		sa.Files = newFiles
		sa.NaturalSizes = shinNaturalSizes
		sa.PreviewImages = shinPreviewImages

		if len(sa.Files) > 1 {
			sa.SaveOneBtn.Show()
		} else {
			sa.SaveOneBtn.Hide()
		}
		sa.ButtonBox.Refresh()

		if len(sa.Files) > 0 {
			sa.CurrentImageIndex = 0
			sa.ImgWidget.Image = sa.PreviewImages[0]
			sa.ImgWidget.Refresh()

			nat := sa.NaturalSizes[0]
			sa.Overlay.NatW = float32(nat.X)
			sa.Overlay.NatH = float32(nat.Y)

			if GlobalSafeZone.Active {
				sa.Overlay.SetSelection(
					float32(GlobalSafeZone.MinX),
					float32(GlobalSafeZone.MinY),
					float32(GlobalSafeZone.MaxX),
					float32(GlobalSafeZone.MaxY),
				)
			}
			sa.Overlay.Refresh()
			sa.updateStatus(true)

			sa.Slider.Max = float64(len(sa.Files) - 1)
			sa.Slider.SetValue(0)
			sa.Slider.Refresh()
			sa.FrameLabel.SetText(fmt.Sprintf("Frame 1 / %d", len(sa.Files)))
			sa.updatePreview()
		}
	})
}

func (sa *SpriteApp) doImport(limit int) {
	importProgress := widget.NewProgressBar()
	importLabel := widget.NewLabel("Importing and preloading images... Please wait.")

	progress := dialog.NewCustomWithoutButtons(
		"Importing",
		container.NewVBox(importLabel, importProgress),
		sa.Window,
	)
	progress.Show()

	go func() {
		imported, err := ImportLatestCaptures(InputDir, "archive", limit)
		if err == nil && len(imported) > 0 {
			GlobalSafeZone.Active = false
			SaveSafeZoneConfig()
			sa.Overlay.HasSelection = false
			sa.loadFrames(imported, importProgress.SetValue)
		}

		fyne.Do(func() {
			progress.Hide()

			if err != nil {
				dialog.ShowError(fmt.Errorf("Import failed: %v", err), sa.Window)
				return
			}

			if len(imported) > 0 {
				dialog.ShowInformation("Imported", fmt.Sprintf("Imported %d frames successfully.", len(imported)), sa.Window)
			} else {
				dialog.ShowInformation("Imported", "No new frames found to import.", sa.Window)
			}
		})
	}()
}

func (sa *SpriteApp) buildImportBtn() *widget.Button {
	return widget.NewButton("Import Latest Captures", func() {
		opts := []string{"Single Frame (1)", "Full Rotation (24)", "Custom Limit..."}

		var limitSelect *widget.RadioGroup
		limitSelect = widget.NewRadioGroup(opts, func(string) {})
		limitSelect.SetSelected(opts[1])
		limitSelect.Horizontal = false

		dialog.ShowCustomConfirm("Import Images", "Start", "Cancel", limitSelect, func(b bool) {
			if !b {
				return
			}
			switch limitSelect.Selected {
			case opts[0]:
				sa.doImport(1)
			case opts[1]:
				sa.doImport(24)
			case opts[2]:
				entry := widget.NewEntry()
				entry.SetText("24")
				dialog.ShowForm("Custom Import Limit", "Import", "Cancel", []*widget.FormItem{
					widget.NewFormItem("Frame Count:", entry),
				}, func(ok bool) {
					if ok {
						val, err := strconv.Atoi(entry.Text)
						if err == nil && val > 0 {
							sa.doImport(val)
						} else {
							dialog.ShowError(fmt.Errorf("invalid number: %s", entry.Text), sa.Window)
						}
					}
				}, sa.Window)
			}
		}, sa.Window)
	})
}

func (sa *SpriteApp) buildSettingsBtn() *widget.Button {
	return widget.NewButton("Settings", func() {
		sizeEntry := widget.NewEntry()
		sizeEntry.SetText(fmt.Sprintf("%d", TargetSize))

		skipUiCheck := widget.NewCheck("Skip UI on Startup", nil)
		skipUiCheck.SetChecked(ProcessMode)

		skipBgCheck := widget.NewCheck("Skip Background Removal", nil)
		skipBgCheck.SetChecked(SkipBgRemoval)

		thresholdEntry := widget.NewEntry()
		thresholdEntry.SetText(fmt.Sprintf("%.1f", Threshold))

		inEntry := widget.NewEntry()
		inEntry.SetText(InputDir)

		outEntry := widget.NewEntry()
		outEntry.SetText(OutputFile)

		oneShotEntry := widget.NewEntry()
		oneShotEntry.SetText(OneShotFile)

		erodeCheck := widget.NewCheck("Erode Edges", nil)
		erodeCheck.SetChecked(ErodeEdges)

		colorEntry := widget.NewEntry()
		colorEntry.SetText(HexColor)

		skipPrescaleCheck := widget.NewCheck("Skip Prescale (Full Res Preview)", nil)
		skipPrescaleCheck.SetChecked(SkipPrescale)

		items := []*widget.FormItem{
			widget.NewFormItem("", skipUiCheck),
			widget.NewFormItem("Output Size (px)", sizeEntry),
			widget.NewFormItem("", skipBgCheck),
			widget.NewFormItem("BG Threshold", thresholdEntry),
			widget.NewFormItem("Input Directory", inEntry),
			widget.NewFormItem("Output Filename", outEntry),
			widget.NewFormItem("One Frame Filename", oneShotEntry),
			widget.NewFormItem("", erodeCheck),
			widget.NewFormItem("Chroma Key Color", colorEntry),
			widget.NewFormItem("", skipPrescaleCheck),
		}

		dialog.ShowForm("Settings", "Save", "Cancel", items, func(b bool) {
			if !b {
				return
			}
			updates := map[string]string{
				"skip-ui":       strconv.FormatBool(skipUiCheck.Checked),
				"size":          sizeEntry.Text,
				"skip-bg":       strconv.FormatBool(skipBgCheck.Checked),
				"threshold-bg":  thresholdEntry.Text,
				"in":            inEntry.Text,
				"out":           outEntry.Text,
				"out-one":       oneShotEntry.Text,
				"erode":         strconv.FormatBool(erodeCheck.Checked),
				"color-bg":      colorEntry.Text,
				"skip-prescale": strconv.FormatBool(skipPrescaleCheck.Checked),
			}
			err := UpdateConfig(updates)
			if err != nil {
				dialog.ShowError(err, sa.Window)
			} else {
				var parseErr error
				ChromaKey, parseErr = ParseHexColor(HexColor)
				if parseErr == nil && sa.ColorPreview != nil {
					sa.ColorPreview.FillColor = ChromaKey
					sa.ColorPreview.Refresh()
				}
				dialog.ShowInformation("Settings", "Settings saved!", sa.Window)
			}
		}, sa.Window)
	})
}

func (sa *SpriteApp) setupShortcuts() {
	sa.Window.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if !sa.Overlay.HasSelection || len(sa.NaturalSizes) == 0 {
			return
		}

		step := float32(1.0)
		if d, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
			if d.CurrentKeyModifiers()&fyne.KeyModifierShift != 0 {
				step = 10.0
			}
		}

		nat := sa.NaturalSizes[sa.CurrentImageIndex]
		natW, natH := float32(nat.X), float32(nat.Y)
		selW, selH := sa.Overlay.MaxX-sa.Overlay.MinX, sa.Overlay.MaxY-sa.Overlay.MinY
		minX, minY, maxX, maxY := sa.Overlay.MinX, sa.Overlay.MinY, sa.Overlay.MaxX, sa.Overlay.MaxY

		switch k.Name {
		case fyne.KeyLeft:
			minX -= step
			maxX -= step
		case fyne.KeyRight:
			minX += step
			maxX += step
		case fyne.KeyUp:
			minY -= step
			maxY -= step
		case fyne.KeyDown:
			minY += step
			maxY += step
		default:
			return
		}

		if minX < 0 {
			minX, maxX = 0, selW
		}
		if minY < 0 {
			minY, maxY = 0, selH
		}
		if maxX > natW {
			maxX, minX = natW, natW-selW
		}
		if maxY > natH {
			maxY, minY = natH, natH-selH
		}

		sa.Overlay.SetSelection(minX, minY, maxX, maxY)
		sa.updateStatus(true)
	})
}
