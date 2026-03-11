package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"os"
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

	"pseudo3d-sprites/internal"
)

func init() {
	flag.BoolVar(&internal.ProcessMode, "skip-ui", false, "Skip UI and directly run batch processing")
	flag.IntVar(&internal.TargetSize, "size", 512, "Output size of the square frames (for example, 300 for 300x300)")
	flag.BoolVar(&internal.SkipBgRemoval, "skip-bg", false, "Skip chroma key background removal completely")
	flag.Float64Var(&internal.Threshold, "threshold-bg", 70.0, "Tolerance threshold for background removal (higher = more aggressive)")
	flag.StringVar(&internal.InputDir, "in", "./process", "Input directory containing PNG frames")
	flag.StringVar(&internal.OutputFile, "out", "spritesheet.png", "Output spritesheet filename")
	flag.BoolVar(&internal.ErodeEdges, "erode", false, "Aggressively trim 1 pixel of alpha from edges to kill residue")
	flag.StringVar(&internal.HexColor, "color-bg", "DF03DF", "Hex color code to remove as background")
	flag.BoolVar(&internal.SkipPrescale, "skip-prescale", false, "Prescale preview images to "+fmt.Sprintf("%d", internal.PreviewMaxPx)+"px max for faster UI (originals are still used for processing)")
}

func main() {
	flag.Parse()
	internal.LoadConfiguration()

	var err error
	internal.ChromaKey, err = internal.ParseHexColor(internal.HexColor)
	if err != nil {
		log.Fatalf("Error parsing chroma key color: %v\n", err)
	}

	if internal.ProcessMode {
		if err := internal.RunBatchProcessing(nil); err != nil {
			fmt.Printf("Processing failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	a := app.New()
	w := a.NewWindow("Safe Zone Selector")
	w.Resize(fyne.NewSize(900, 700))

	var files []string
	var naturalSizes []image.Point
	var previewImages []image.Image
	currentImageIndex := 0

	imgWidget := canvas.NewImageFromImage(nil)
	imgWidget.FillMode = canvas.ImageFillContain

	overlay := internal.NewSelectionOverlay()

	getRenderedImageBounds := func() (offX, offY, rendW, rendH float32) {
		if len(naturalSizes) == 0 {
			return 0, 0, 0, 0
		}
		nat := naturalSizes[currentImageIndex]
		containerSize := imgWidget.Size()
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

	overlay.GetImageBounds = getRenderedImageBounds

	screenToImage := func(sx, sy float32) (float32, float32) {
		if len(naturalSizes) == 0 {
			return 0, 0
		}
		nat := naturalSizes[currentImageIndex]
		ox, oy, rw, rh := getRenderedImageBounds()
		ix := min(max((sx-ox)/rw*float32(nat.X), 0), float32(nat.X))
		iy := min(max((sy-oy)/rh*float32(nat.Y), 0), float32(nat.Y))
		return ix, iy
	}

	var startImgX, startImgY float32
	lockAspect := true
	isNewDrag := true
	colorPickMode := false
	var pickColorToggleBtn *widget.Button
	var colorPreview *canvas.Rectangle

	updateColor := func(c color.Color) {
		r, g, b, _ := c.RGBA()
		internal.ChromaKey = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
		internal.HexColor = fmt.Sprintf("%02X%02X%02X", internal.ChromaKey.R, internal.ChromaKey.G, internal.ChromaKey.B)
		if colorPreview != nil {
			colorPreview.FillColor = internal.ChromaKey
			colorPreview.Refresh()
		}
	}

	statusLabel := widget.NewLabel("No selection...")
	statusLabel.Alignment = fyne.TextAlignCenter

	const previewSize = float32(180)

	makeCropPreview := func(clipW, clipH float32) (*canvas.Image, *fyne.Container) {
		img := canvas.NewImageFromImage(nil)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(clipW, clipH))
		return img, container.NewCenter(img)
	}

	tdswPreview, tdswClip := makeCropPreview(previewSize, previewSize*0.9) // 10:9
	aewPreview, aewClip := makeCropPreview(previewSize*0.9, previewSize)   // 9:10

	tdswCanvas := image.NewRGBA(image.Rect(0, 0, int(previewSize), int(previewSize*0.9)))
	aewCanvas := image.NewRGBA(image.Rect(0, 0, int(previewSize*0.9), int(previewSize)))

	fillCheckerboard := func(canvas *image.RGBA) {
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

	updatePreview := func() {
		if !overlay.HasSelection || len(naturalSizes) == 0 {
			tdswPreview.Image, aewPreview.Image = nil, nil
			tdswPreview.Refresh()
			aewPreview.Refresh()
			return
		}
		nat := naturalSizes[currentImageIndex]
		minX := min(max(int(math.Round(float64(overlay.MinX))), 0), nat.X)
		minY := min(max(int(math.Round(float64(overlay.MinY))), 0), nat.Y)
		maxX := min(max(int(math.Round(float64(overlay.MaxX))), 0), nat.X)
		maxY := min(max(int(math.Round(float64(overlay.MaxY))), 0), nat.Y)
		if maxX <= minX || maxY <= minY {
			return
		}
		pb := previewImages[currentImageIndex].Bounds()
		scaleX := float64(pb.Dx()) / float64(nat.X)
		scaleY := float64(pb.Dy()) / float64(nat.Y)
		cropped := imaging.Crop(previewImages[currentImageIndex], image.Rect(
			int(float64(minX)*scaleX), int(float64(minY)*scaleY),
			int(float64(maxX)*scaleX), int(float64(maxY)*scaleY),
		))

		fitAndCenter := func(img image.Image, canvas *image.RGBA) image.Image {
			w, h := canvas.Rect.Dx(), canvas.Rect.Dy()
			scaled := imaging.Resize(img, w, 0, imaging.NearestNeighbor)
			fillCheckerboard(canvas)
			b := scaled.Bounds()
			dx := (w - b.Dx()) / 2
			dy := (h - b.Dy()) / 2
			draw.Draw(canvas, image.Rectangle{Min: image.Pt(dx, dy), Max: image.Pt(dx, dy).Add(b.Size())}, scaled, b.Min, draw.Over)
			return canvas
		}

		tdswPreview.Image = fitAndCenter(cropped, tdswCanvas)
		aewPreview.Image = fitAndCenter(cropped, aewCanvas)
		tdswPreview.Refresh()
		aewPreview.Refresh()
	}

	var lastPreview time.Time
	updateStatus := func(force bool) {
		if !overlay.HasSelection {
			statusLabel.SetText("No selection...")
			updatePreview()
			return
		}
		statusLabel.SetText(fmt.Sprintf("Selected: %dx%d at (%d, %d)",
			int(overlay.MaxX-overlay.MinX), int(overlay.MaxY-overlay.MinY),
			int(overlay.MinX), int(overlay.MinY)))

		if force || time.Since(lastPreview) > 16*time.Millisecond {
			updatePreview()
			lastPreview = time.Now()
		}
	}

	dragArea := &internal.InteractiveArea{
		GetCursor: func() desktop.Cursor {
			if colorPickMode {
				return desktop.CrosshairCursor
			}
			return desktop.PointerCursor
		},
		OnDrag: func(e *fyne.DragEvent) {
			if isNewDrag {
				startImgX, startImgY = screenToImage(e.Position.X-e.Dragged.DX, e.Position.Y-e.Dragged.DY)
				isNewDrag = false
			}
			curImgX, curImgY := screenToImage(e.Position.X, e.Position.Y)
			rawW, rawH := curImgX-startImgX, curImgY-startImgY

			var minX, minY, maxX, maxY float32
			if lockAspect {
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
					minX = startImgX - side
				} else {
					minX = startImgX
				}
				if signH {
					minY = startImgY - side
				} else {
					minY = startImgY
				}
				maxX, maxY = minX+side, minY+side
			} else {
				minX, maxX = min(startImgX, curImgX), max(startImgX, curImgX)
				minY, maxY = min(startImgY, curImgY), max(startImgY, curImgY)
			}
			if maxX > minX && maxY > minY {
				overlay.SetSelection(minX, minY, maxX, maxY)
				updateStatus(false)
			}
		},
		OnDragEnd: func() { isNewDrag = true; updateStatus(true) },
		OnTap: func(e *fyne.PointEvent) {
			if !colorPickMode || len(naturalSizes) == 0 {
				return
			}
			ix, iy := screenToImage(e.Position.X, e.Position.Y)
			nat := naturalSizes[currentImageIndex]
			previewImg := previewImages[currentImageIndex]
			pb := previewImg.Bounds()

			// natural coordinates to preview img coordinates
			px := int(float64(ix) / float64(nat.X) * float64(pb.Dx()))
			py := int(float64(iy) / float64(nat.Y) * float64(pb.Dy()))

			px = min(max(px, 0), pb.Dx()-1)
			py = min(max(py, 0), pb.Dy()-1)

			updateColor(previewImg.At(pb.Min.X+px, pb.Min.Y+py))
			colorPickMode = false
			pickColorToggleBtn.SetText("Chroma Key Picker")
		},
	}
	dragArea.ExtendBaseWidget(dragArea)

	imageContainer := container.NewStack(imgWidget, overlay, dragArea)

	frameLabel := widget.NewLabel("Frame 0 / 0")

	slider := widget.NewSlider(0, 0)
	slider.OnChanged = func(v float64) {
		if len(files) == 0 {
			return
		}
		newIndex := int(v)
		if newIndex == currentImageIndex {
			return
		}
		currentImageIndex = newIndex

		imgWidget.Image = previewImages[currentImageIndex]
		imgWidget.Refresh()

		nat := naturalSizes[currentImageIndex]
		overlay.NatW = float32(nat.X)
		overlay.NatH = float32(nat.Y)
		overlay.Refresh()
		frameLabel.SetText(fmt.Sprintf("Frame %d / %d", currentImageIndex+1, len(files)))
		updatePreview()
	}

	toggleLockBtn := widget.NewButton("Square Ratio", nil)
	toggleLockBtn.OnTapped = func() {
		lockAspect = !lockAspect
		if lockAspect {
			toggleLockBtn.SetText("Square Ratio")
		} else {
			toggleLockBtn.SetText("Freeform")
		}
	}

	colorPreview = canvas.NewRectangle(internal.ChromaKey)
	colorPreview.SetMinSize(fyne.NewSize(30, 20))

	pickColorToggleBtn = widget.NewButton("Chroma Key Picker", nil)
	pickColorToggleBtn.OnTapped = func() {
		colorPickMode = !colorPickMode
		if colorPickMode {
			pickColorToggleBtn.SetText("(Click something on the image you fool!)")
		} else {
			pickColorToggleBtn.SetText("Chroma Key Picker")
		}
	}

	colorBox := container.NewHBox(
		widget.NewLabel("Key:"),
		container.NewCenter(colorPreview),
		pickColorToggleBtn,
	)

	helpBtn := widget.NewButton("?", func() {
		dialog.ShowInformation("Controls",
			"Draw a selection by dragging on the image.\n\nArrow keys: nudge selection by 1px\nShift + Arrow keys: nudge by 10px",
			w)
	})

	var saveBtn *widget.Button
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	processBox := container.NewStack(saveBtn)

	saveBtn = widget.NewButton("Save & Process!", func() {
		if !overlay.HasSelection || overlay.MinX == overlay.MaxX || len(files) == 0 {
			return
		}
		internal.GlobalSafeZone = internal.SafeZone{
			MinX:   int(math.Round(float64(overlay.MinX))),
			MinY:   int(math.Round(float64(overlay.MinY))),
			MaxX:   int(math.Round(float64(overlay.MaxX))),
			MaxY:   int(math.Round(float64(overlay.MaxY))),
			Active: true,
		}
		internal.SaveSafeZoneConfig()

		saveBtn.Hide()
		progressBar.SetValue(0)
		progressBar.Show()

		go func() {
			err := internal.RunBatchProcessing(func(current, total int) {
				progressBar.SetValue(float64(current) / float64(total))
			})

			fyne.Do(func() {
				saveBtn.Show()
				progressBar.Hide()

				if err != nil {
					dialog.ShowError(err, w)
				} else {
					dialog.ShowInformation("Success", fmt.Sprintf("Heeho! Spritesheet saved as %s (%d frames)!", internal.OutputFile, len(files)), w)
				}
			})
		}()
	})

	processBox.Objects = []fyne.CanvasObject{saveBtn, progressBar}

	previewPanel := container.NewVBox(
		widget.NewLabel("TDS Wiki (10:9)"),
		tdswClip,
		widget.NewLabel("ALTERPEDIA (9:10)"),
		aewClip,
	)

	loadFrames := func(newFiles []string, progressCallback func(float64)) {
		shinNaturalSizes := make([]image.Point, len(newFiles))
		shinPreviewImages := make([]image.Image, len(newFiles))

		for i, f := range newFiles {
			if progressCallback != nil {
				progressCallback(float64(i) / float64(len(newFiles)))
			}
			full := internal.LoadImage(f)
			b := full.Bounds()
			shinNaturalSizes[i] = image.Pt(b.Dx(), b.Dy())

			if !internal.SkipPrescale {
				shinPreviewImages[i] = imaging.Fit(full, internal.PreviewMaxPx, internal.PreviewMaxPx, imaging.Lanczos)
			} else {
				shinPreviewImages[i] = full
			}
		}
		if progressCallback != nil {
			progressCallback(1.0)
		}

		fyne.Do(func() {
			files = newFiles
			naturalSizes = shinNaturalSizes
			previewImages = shinPreviewImages

			if len(files) > 0 {
				currentImageIndex = 0
				imgWidget.Image = previewImages[0]
				imgWidget.Refresh()

				nat := naturalSizes[0]
				overlay.NatW = float32(nat.X)
				overlay.NatH = float32(nat.Y)

				if internal.GlobalSafeZone.Active {
					overlay.SetSelection(
						float32(internal.GlobalSafeZone.MinX),
						float32(internal.GlobalSafeZone.MinY),
						float32(internal.GlobalSafeZone.MaxX),
						float32(internal.GlobalSafeZone.MaxY),
					)
				}
				overlay.Refresh()

				slider.Max = float64(len(files) - 1)
				slider.SetValue(0)
				slider.Refresh()
				frameLabel.SetText(fmt.Sprintf("Frame 1 / %d", len(files)))
				updatePreview()
			}
		})
	}

	importBtn := widget.NewButton("Import Latest Captures", func() {
		importProgress := widget.NewProgressBar()
		importLabel := widget.NewLabel("Importing and preloading images... Please wait.")

		progress := dialog.NewCustomWithoutButtons(
			"Importing",
			container.NewVBox(importLabel, importProgress),
			w,
		)
		progress.Show()

		go func() {
			imported, err := internal.ImportLatestCaptures(internal.InputDir, "archive")
			if err == nil && len(imported) > 0 {
				internal.GlobalSafeZone.Active = false
				internal.SaveSafeZoneConfig()
				overlay.HasSelection = false
				loadFrames(imported, importProgress.SetValue)
			}

			fyne.Do(func() {
				progress.Hide()

				if err != nil {
					dialog.ShowError(fmt.Errorf("Import failed: %v", err), w)
					return
				}

				if len(imported) > 0 {
					dialog.ShowInformation("Imported", fmt.Sprintf("Imported %d frames successfully.", len(imported)), w)
				} else {
					dialog.ShowInformation("Imported", "No new frames found to import.", w)
				}
			})
		}()
	})

	controls := container.NewVBox(
		container.NewBorder(nil, nil, nil, container.NewHBox(importBtn, layout.NewSpacer(), colorBox, helpBtn), toggleLockBtn),
		container.NewBorder(nil, nil, nil, frameLabel, slider),
		processBox,
		statusLabel,
	)

	w.SetContent(container.NewBorder(nil, controls, nil, nil,
		container.NewBorder(nil, nil, nil, previewPanel, imageContainer),
	))

	// arrow keys nudge
	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if !overlay.HasSelection || len(naturalSizes) == 0 {
			return
		}

		step := float32(1.0)
		if d, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
			if d.CurrentKeyModifiers()&fyne.KeyModifierShift != 0 {
				step = 10.0
			}
		}

		nat := naturalSizes[currentImageIndex]
		natW, natH := float32(nat.X), float32(nat.Y)
		selW, selH := overlay.MaxX-overlay.MinX, overlay.MaxY-overlay.MinY
		minX, minY, maxX, maxY := overlay.MinX, overlay.MinY, overlay.MaxX, overlay.MaxY

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

		overlay.SetSelection(minX, minY, maxX, maxY)
		updateStatus(true)
	})

	initialFiles, _ := internal.GetPNGFiles(internal.InputDir)
	if len(initialFiles) > 0 {
		loadFrames(initialFiles, nil)
	}

	w.ShowAndRun()
}
