package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/disintegration/imaging"
	"github.com/schollz/progressbar/v3"
)

func init() {
	flag.BoolVar(&processMode, "skip-ui", false, "Skip UI and directly run batch processing")
	flag.IntVar(&targetSize, "size", 512, "Output size of the square frames (for example, 300 for 300x300)")
	flag.BoolVar(&skipBgRemoval, "skip-bg", false, "Skip chroma key background removal completely")
	flag.Float64Var(&threshold, "threshold-bg", 70.0, "Tolerance threshold for background removal (higher = more aggressive)")
	flag.StringVar(&inputDir, "in", "./process", "Input directory containing PNG frames")
	flag.StringVar(&outputFile, "out", "spritesheet.png", "Output spritesheet filename")
	flag.BoolVar(&erodeEdges, "erode", false, "Aggressively trim 1 pixel of alpha from edges to kill residue")
	flag.StringVar(&hexColor, "color-bg", "DF03DF", "Hex color code to remove as background")
	flag.BoolVar(&prescale, "skip-prescale", false, "Prescale preview images to "+fmt.Sprintf("%d", previewMaxPx)+"px max for faster UI (originals are still used for processing)")
}

func main() {
	flag.Parse()

	var err error
	chromaKey, err = parseHexColor(hexColor)
	if err != nil {
		log.Fatalf("Error parsing chroma key color: %v\n", err)
	}

	if processMode {
		runBatchProcessing()
		return
	}

	files, err := getPNGFiles(inputDir)
	if err != nil || len(files) == 0 {
		fmt.Printf("No PNG files found in %s\n", inputDir)
		time.Sleep(3 * time.Second)
		return
	}

	a := app.New()
	w := a.NewWindow("Safe Zone Selector")
	w.Resize(fyne.NewSize(900, 700))

	if !prescale {
		fmt.Printf("Preloading %d frames (prescaled to max %dpx)...\n", len(files), previewMaxPx)
	} else {
		fmt.Printf("Preloading %d frames (full resolution)...\n", len(files))
	}

	naturalSizes := make([]image.Point, len(files))
	previewImages := make([]image.Image, len(files))

	{
		bar := progressbar.Default(int64(len(files)), "Loading frames")
		for i, f := range files {
			full := loadImage(f)
			b := full.Bounds()
			naturalSizes[i] = image.Pt(b.Dx(), b.Dy())

			if prescale {
				previewImages[i] = imaging.Fit(full, previewMaxPx, previewMaxPx, imaging.Lanczos)
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

	overlay := newSelectionOverlay()

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

	overlay.getImageBounds = getRenderedImageBounds
	nat0 := naturalSizes[0]
	overlay.natW = float32(nat0.X)
	overlay.natH = float32(nat0.Y)

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

	statusLabel := widget.NewLabel("No selection...")
	statusLabel.Alignment = fyne.TextAlignCenter

	updateStatus := func() {
		if !overlay.hasSelection {
			statusLabel.SetText("No selection...")
			return
		}
		statusLabel.SetText(fmt.Sprintf("Selected: %dx%d at (%d, %d)",
			int(overlay.maxX-overlay.minX), int(overlay.maxY-overlay.minY),
			int(overlay.minX), int(overlay.minY)))
	}

	dragArea := &interactiveArea{
		onDrag: func(e *fyne.DragEvent) {
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
		onDragEnd: func() { isNewDrag = true; updateStatus() },
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
		overlay.natW = float32(nat.X)
		overlay.natH = float32(nat.Y)
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

	saveBtn := widget.NewButton("Save & Process!", func() {
		if !overlay.hasSelection || overlay.minX == overlay.maxX {
			return
		}
		globalSafeZone = SafeZone{
			MinX:   int(math.Round(float64(overlay.minX))),
			MinY:   int(math.Round(float64(overlay.minY))),
			MaxX:   int(math.Round(float64(overlay.maxX))),
			MaxY:   int(math.Round(float64(overlay.maxY))),
			Active: true,
		}
		saveSafeZoneConfig()
		w.Close()
	})

	controls := container.NewVBox(
		toggleLockBtn,
		container.NewBorder(nil, nil, nil, frameLabel, slider),
		saveBtn,
		statusLabel,
	)

	w.SetContent(container.NewBorder(nil, controls, nil, nil, imageContainer))

	// arrow keys nudge
	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if !overlay.hasSelection {
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
		selW, selH := overlay.maxX-overlay.minX, overlay.maxY-overlay.minY
		minX, minY, maxX, maxY := overlay.minX, overlay.minY, overlay.maxX, overlay.maxY

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

	if globalSafeZone.Active {
		fmt.Println("\nSafe zone received. Starting batch process...")
		runBatchProcessing()
	}
}
