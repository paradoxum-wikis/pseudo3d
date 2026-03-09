package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/disintegration/imaging"
	"github.com/schollz/progressbar/v3"

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
	flag.StringVar(&internal.HexColor, "color-bg", "FC00EC", "Hex color code to remove as background")
	flag.BoolVar(&internal.SkipPrescale, "skip-prescale", false, "Prescale preview images to "+fmt.Sprintf("%d", internal.PreviewMaxPx)+"px max for faster UI (originals are still used for processing)")
	flag.BoolVar(&internal.SkipMenu, "skip-menu", false, "Skip the terminal startup menu and use existing files in the process folder")
}

func main() {
	flag.Parse()

	var err error
	internal.ChromaKey, err = internal.ParseHexColor(internal.HexColor)
	if err != nil {
		log.Fatalf("Error parsing chroma key color: %v\n", err)
	}

	if internal.ProcessMode {
		internal.RunBatchProcessing()
		return
	}

	files, err := internal.GetPNGFiles(internal.InputDir)
	if err != nil {
		fmt.Printf("Could not read %s: %v\n", internal.InputDir, err)
		time.Sleep(3 * time.Second)
		return
	}

	hasFiles := len(files) > 0

	if !internal.SkipMenu {
		fmt.Println("1) Use existing files in \"process\" folder")
		fmt.Println("2) Import latest Roblox capture (24 frames)")
		fmt.Println("3) Exit")
		if !hasFiles {
			fmt.Printf("\nNo PNG files found in %s\n", internal.InputDir)
			fmt.Println("TIP: Run with -help (or -h) to see all available flags.")
		}
		fmt.Print("Choose an option: ")

		var choice string
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			if !hasFiles {
				fmt.Printf("No PNG files found in %s\n", internal.InputDir)
				time.Sleep(3 * time.Second)
				return
			}
		case "2":
			imported, err := internal.ImportLatestCaptures(internal.InputDir, "archive")
			if err != nil {
				fmt.Printf("Import failed: %v\n", err)
				time.Sleep(3 * time.Second)
				return
			}
			files = imported
			fmt.Printf("Imported %d frames into %s\n\n", len(files), internal.InputDir)
		default:
			return
		}
	} else if !hasFiles {
		fmt.Printf("No PNG files found in %s\n", internal.InputDir)
		time.Sleep(3 * time.Second)
		return
	}

	a := app.New()
	w := a.NewWindow("Safe Zone Selector")
	w.Resize(fyne.NewSize(900, 700))

	if !internal.SkipPrescale {
		fmt.Printf("Preloading %d frames (prescaled to max %dpx)...\n", len(files), internal.PreviewMaxPx)
	} else {
		fmt.Printf("Preloading %d frames (full resolution)...\n", len(files))
	}

	naturalSizes := make([]image.Point, len(files))
	previewImages := make([]image.Image, len(files))

	{
		bar := progressbar.Default(int64(len(files)), "Loading frames")
		for i, f := range files {
			full := internal.LoadImage(f)
			b := full.Bounds()
			naturalSizes[i] = image.Pt(b.Dx(), b.Dy())

			if !internal.SkipPrescale {
				previewImages[i] = imaging.Fit(full, internal.PreviewMaxPx, internal.PreviewMaxPx, imaging.Lanczos)
			} else {
				previewImages[i] = full
			}
			bar.Add(1)
		}
		fmt.Println()
	}

	currentImageIndex := 0

	imgWidget := canvas.NewImageFromImage(previewImages[currentImageIndex])
	imgWidget.FillMode = canvas.ImageFillContain

	overlay := internal.NewSelectionOverlay()

	getRenderedImageBounds := func() (offX, offY, rendW, rendH float32) {
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
	nat0 := naturalSizes[0]
	overlay.NatW = float32(nat0.X)
	overlay.NatH = float32(nat0.Y)

	screenToImage := func(sx, sy float32) (float32, float32) {
		nat := naturalSizes[currentImageIndex]
		ox, oy, rw, rh := getRenderedImageBounds()
		ix := float32(math.Max(0, math.Min(float64((sx-ox)/rw*float32(nat.X)), float64(nat.X))))
		iy := float32(math.Max(0, math.Min(float64((sy-oy)/rh*float32(nat.Y)), float64(nat.Y))))
		return ix, iy
	}

	var startImgX, startImgY float32
	lockAspect := true
	isNewDrag := true
	colorPickMode := false
	var pickColorToggleBtn *widget.Button
	var colorPreview *canvas.Rectangle

	updateColor := func(c color.Color) {
		r, g, b, a := c.RGBA()
		internal.ChromaKey = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
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
		return img, container.NewStack(img)
	}

	tdswPreview, tdswClip := makeCropPreview(previewSize, previewSize*0.9)
	aewPreview, aewClip := makeCropPreview(previewSize*0.9, previewSize)
	updatePreview := func() {
		if !overlay.HasSelection {
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
		tdswPreview.Image = cropped
		aewPreview.Image = cropped
		tdswPreview.Refresh()
		aewPreview.Refresh()
	}

	updateStatus := func() {
		if !overlay.HasSelection {
			statusLabel.SetText("No selection...")
			updatePreview()
			return
		}
		statusLabel.SetText(fmt.Sprintf("Selected: %dx%d at (%d, %d)",
			int(overlay.MaxX-overlay.MinX), int(overlay.MaxY-overlay.MinY),
			int(overlay.MinX), int(overlay.MinY)))
		updatePreview()
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
				side := float32(math.Min(math.Abs(float64(rawW)), math.Abs(float64(rawH))))
				if rawW < 0 {
					minX = startImgX - side
				} else {
					minX = startImgX
				}
				if rawH < 0 {
					minY = startImgY - side
				} else {
					minY = startImgY
				}
				maxX, maxY = minX+side, minY+side
			} else {
				if rawW >= 0 {
					minX, maxX = startImgX, curImgX
				} else {
					minX, maxX = curImgX, startImgX
				}
				if rawH >= 0 {
					minY, maxY = startImgY, curImgY
				} else {
					minY, maxY = curImgY, startImgY
				}
			}
			if maxX > minX && maxY > minY {
				overlay.SetSelection(minX, minY, maxX, maxY)
				updateStatus()
			}
		},
		OnDragEnd: func() { isNewDrag = true; updateStatus() },
		OnTap: func(e *fyne.PointEvent) {
			if !colorPickMode {
				return
			}
			ix, iy := screenToImage(e.Position.X, e.Position.Y)
			nat := naturalSizes[currentImageIndex]
			previewImg := previewImages[currentImageIndex]
			pb := previewImg.Bounds()

			// natural coordinates to preview img coordinates
			px := int(float64(ix) / float64(nat.X) * float64(pb.Dx()))
			py := int(float64(iy) / float64(nat.Y) * float64(pb.Dy()))

			if px >= pb.Dx() {
				px = pb.Dx() - 1
			}
			if py >= pb.Dy() {
				py = pb.Dy() - 1
			}

			if px >= 0 && py >= 0 && px < pb.Dx() && py < pb.Dy() {
				c := previewImg.At(pb.Min.X+px, pb.Min.Y+py)
				updateColor(c)

				colorPickMode = false
				pickColorToggleBtn.SetText("Chroma Key Picker")
			}
		},
	}
	dragArea.ExtendBaseWidget(dragArea)

	imageContainer := container.NewStack(imgWidget, overlay, dragArea)

	frameLabel := widget.NewLabel(fmt.Sprintf("Frame 1 / %d", len(files)))

	slider := widget.NewSlider(0, float64(len(files)-1))
	slider.OnChanged = func(v float64) {
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

	saveBtn := widget.NewButton("Save & Process!", func() {
		if !overlay.HasSelection || overlay.MinX == overlay.MaxX {
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
		w.Close()
	})

	previewPanel := container.NewVBox(
		widget.NewLabel("TDS Wiki (10:9)"),
		tdswClip,
		widget.NewLabel("ALTERPEDIA (9:10)"),
		aewClip,
	)

	controls := container.NewVBox(
		container.NewBorder(nil, nil, nil, container.NewHBox(colorBox, helpBtn), toggleLockBtn),
		container.NewBorder(nil, nil, nil, frameLabel, slider),
		saveBtn,
		statusLabel,
	)

	w.SetContent(container.NewBorder(nil, controls, nil, nil,
		container.NewBorder(nil, nil, nil, previewPanel, imageContainer),
	))

	// arrow keys nudge
	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if !overlay.HasSelection {
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
		updateStatus()
	})

	w.ShowAndRun()

	if internal.GlobalSafeZone.Active {
		fmt.Println("\nSafe zone received. Starting batch process...")
		internal.RunBatchProcessing()
	}
}
