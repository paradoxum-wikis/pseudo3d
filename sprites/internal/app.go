package internal

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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

	LockAspect     bool
	IsNewDrag      bool
	ColorPickMode  bool
	StartImgX      float32
	StartImgY      float32
	LastPreview    time.Time
	PreviewTrigger chan struct{}
}

var StartupUpdateError error

func RunUI() {
	fyneApp := app.New()
	w := fyneApp.NewWindow("Pseudo3D Sprites")
	w.Resize(fyne.NewSize(1152, 648))

	sa := &SpriteApp{
		App:            fyneApp,
		Window:         w,
		LockAspect:     true,
		IsNewDrag:      true,
		PreviewTrigger: make(chan struct{}, 1),
	}
	go func() {
		for range sa.PreviewTrigger {
			sa.renderPreview()
			time.Sleep(24 * time.Millisecond)
		}
	}()

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
		startupLabel := widget.NewLabel("Hee! I'm loading your frames... Ho!")

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

func (sa *SpriteApp) processAndRenderPreview() {
	if !sa.Overlay.HasSelection || len(sa.NaturalSizes) == 0 {
		fyne.Do(func() {
			sa.TdswPreview.Image, sa.AewPreview.Image = nil, nil
			sa.TdswPreview.Refresh()
			sa.AewPreview.Refresh()
		})
		return
	}

	nat := sa.NaturalSizes[sa.CurrentImageIndex]
	minX, minY := max(int(sa.Overlay.MinX), 0), max(int(sa.Overlay.MinY), 0)
	maxX, maxY := min(int(sa.Overlay.MaxX), nat.X), min(int(sa.Overlay.MaxY), nat.Y)

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
	processed := applyPreviewProcessing(cropped)

	fitAndCenter := func(img image.Image, w, h int) image.Image {
		scaled := imaging.Resize(img, w, 0, imaging.NearestNeighbor)
		canvas := image.NewRGBA(image.Rect(0, 0, w, h))
		sa.fillCheckerboard(canvas)
		b := scaled.Bounds()
		dx, dy := (w-b.Dx())/2, (h-b.Dy())/2
		draw.Draw(canvas, image.Rectangle{Min: image.Pt(dx, dy), Max: image.Pt(dx, dy).Add(b.Size())}, scaled, b.Min, draw.Over)
		return canvas
	}

	tdswW, tdswH := sa.TdswCanvas.Rect.Dx(), sa.TdswCanvas.Rect.Dy()
	aewW, aewH := sa.AewCanvas.Rect.Dx(), sa.AewCanvas.Rect.Dy()

	tdswFinal := fitAndCenter(processed, tdswW, tdswH)
	aewFinal := fitAndCenter(processed, aewW, aewH)

	fyne.Do(func() {
		sa.TdswPreview.Image = tdswFinal
		sa.AewPreview.Image = aewFinal
		sa.TdswPreview.Refresh()
		sa.AewPreview.Refresh()
	})
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
	importLabel := widget.NewLabel("Importing and preloading images... Please wait..!")

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
	select {
	case sa.PreviewTrigger <- struct{}{}:
	default:
	}
}

func (sa *SpriteApp) renderPreview() {
	if !sa.Overlay.HasSelection || len(sa.NaturalSizes) == 0 {
		fyne.Do(func() {
			sa.TdswPreview.Image, sa.AewPreview.Image = nil, nil
			sa.TdswPreview.Refresh()
			sa.AewPreview.Refresh()
		})
		return
	}

	nat := sa.NaturalSizes[sa.CurrentImageIndex]
	minX, minY := max(int(sa.Overlay.MinX), 0), max(int(sa.Overlay.MinY), 0)
	maxX, maxY := min(int(sa.Overlay.MaxX), nat.X), min(int(sa.Overlay.MaxY), nat.Y)
	if maxX <= minX || maxY <= minY {
		return
	}

	pb := sa.PreviewImages[sa.CurrentImageIndex].Bounds()
	scX, scY := float64(pb.Dx())/float64(nat.X), float64(pb.Dy())/float64(nat.Y)
	cropRect := image.Rect(int(float64(minX)*scX), int(float64(minY)*scY), int(float64(maxX)*scX), int(float64(maxY)*scY))

	processed := applyPreviewProcessing(imaging.Crop(sa.PreviewImages[sa.CurrentImageIndex], cropRect))

	fit := func(c *image.RGBA) image.Image {
		w, h := c.Rect.Dx(), c.Rect.Dy()
		scaled := imaging.Resize(processed, w, 0, imaging.NearestNeighbor)
		out, b := image.NewRGBA(c.Rect), scaled.Bounds()
		sa.fillCheckerboard(out)
		dp := image.Pt((w-b.Dx())/2, (h-b.Dy())/2)
		draw.Draw(out, image.Rectangle{Min: dp, Max: dp.Add(b.Size())}, scaled, b.Min, draw.Over)
		return out
	}

	img1, img2 := fit(sa.TdswCanvas), fit(sa.AewCanvas)

	fyne.Do(func() {
		sa.TdswPreview.Image, sa.AewPreview.Image = img1, img2
		sa.TdswPreview.Refresh()
		sa.AewPreview.Refresh()
	})
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
