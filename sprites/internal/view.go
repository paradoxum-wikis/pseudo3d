package internal

import (
	"fmt"
	"image"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

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

	helpBtn := widget.NewButtonWithIcon("Controls", theme.HelpIcon(), func() {
		dialog.ShowInformation("Controls",
			"Draw a selection by dragging on the image.\n\nArrow keys: nudge selection by 1px\nShift + Arrow keys: nudge by 10px",
			sa.Window)
	})

	toggleLockBtn := widget.NewButtonWithIcon("Square Ratio", theme.ViewFullScreenIcon(), nil)
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
	fuaroundPanel := sa.buildFuaround()
	controls := container.NewVBox(
		container.NewBorder(nil, nil, nil, container.NewHBox(importBtn, settingsBtn, layout.NewSpacer(), helpBtn), toggleLockBtn),
		container.NewBorder(nil, nil, nil, sa.FrameLabel, sa.Slider),
		processBox,
		footer,
	)

	previewPanel := container.NewVBox(
		widget.NewLabelWithStyle("TDS Wiki (10:9)", fyne.TextAlignCenter, fyne.TextStyle{}),
		container.NewCenter(sa.TdswPreview),
		widget.NewLabelWithStyle("ALTERPEDIA (9:10)", fyne.TextAlignCenter, fyne.TextStyle{}),
		container.NewCenter(sa.AewPreview),
	)

	sa.Window.SetContent(container.NewBorder(
		nil,
		controls,
		container.NewPadded(fuaroundPanel),
		container.NewPadded(previewPanel),
		imageContainer,
	))
}

func (sa *SpriteApp) buildFuaround() *fyne.Container {
	skipBgCheck := widget.NewCheck("Skip BG Removal", func(b bool) {
		SkipBgRemoval = b
		sa.updatePreview()
	})
	skipBgCheck.SetChecked(SkipBgRemoval)

	erodeCheck := widget.NewCheck("Erode Edges", func(b bool) {
		ErodeEdges = b
		sa.updatePreview()
	})
	erodeCheck.SetChecked(ErodeEdges)

	autocropCheck := widget.NewCheck("Skip Autocrop", func(b bool) {
		SkipAutocrop = b
		sa.updatePreview()
	})
	autocropCheck.SetChecked(SkipAutocrop)

	threshLabel := widget.NewLabel(fmt.Sprintf("%.0f", Threshold))
	threshSlider := widget.NewSlider(0, 60000)
	threshSlider.Step, threshSlider.Value = 100, Threshold

	threshMinLabel := widget.NewLabel(fmt.Sprintf("%.0f", ThresholdMin))
	threshMinSlider := widget.NewSlider(0, 60000)
	threshMinSlider.Step, threshMinSlider.Value = 100, ThresholdMin

	threshMinRow := container.NewBorder(nil, nil, nil, threshMinLabel, threshMinSlider)
	if ModeBg != "range" {
		threshMinRow.Hide()
	}

	bgModeRadio := widget.NewRadioGroup([]string{"Hard", "Range"}, func(mode string) {
		ModeBg = strings.ToLower(mode)
		if mode == "Range" {
			threshMinRow.Show()
		} else {
			threshMinRow.Hide()
		}
		sa.updatePreview()
	})
	bgModeRadio.Horizontal = true
	bgModeRadio.SetSelected(func() string {
		if ModeBg == "range" {
			return "Range"
		}
		return "Hard"
	}())

	threshSlider.OnChanged = func(v float64) {
		Threshold = v
		threshLabel.SetText(fmt.Sprintf("%.0f", v))
		sa.updatePreview()
	}
	threshMinSlider.OnChanged = func(v float64) {
		ThresholdMin = v
		threshMinLabel.SetText(fmt.Sprintf("%.0f", v))
		sa.updatePreview()
	}

	sa.ColorEntry = widget.NewEntry()
	sa.ColorEntry.SetText(HexColor)
	sa.ColorEntry.OnChanged = func(s string) {
		HexColor = s
		if c, err := ParseHexColor(s); err == nil {
			ChromaKey = c
			if sa.ColorPreview != nil {
				sa.ColorPreview.FillColor = ChromaKey
				sa.ColorPreview.Refresh()
			}
			sa.updatePreview()
		}
	}

	form := widget.NewForm(
		widget.NewFormItem("Hex Code", sa.ColorEntry),
		widget.NewFormItem("BG Removal", bgModeRadio),
		widget.NewFormItem("Max Thresh", container.NewBorder(nil, nil, nil, threshLabel, threshSlider)),
		widget.NewFormItem("Min Thresh", threshMinRow),
	)

	colorBox := container.NewHBox(widget.NewLabel("Key:"), container.NewCenter(sa.ColorPreview), sa.PickColorToggleBtn)

	return container.NewVBox(
		colorBox,
		form,
		erodeCheck,
		skipBgCheck,
		autocropCheck,
	)
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

	sa.SaveBtn = widget.NewButtonWithIcon("Save & Process!", theme.DocumentSaveIcon(), func() {
		fname := OutputFile
		if len(sa.Files) == 1 {
			fname = OneShotFile
		}
		msg := fmt.Sprintf("Heeho! Spritesheet saved as %s (%d frames)!", fname, len(sa.Files))
		sa.processSelection(nil, msg)
	})
	sa.SaveBtn.Importance = widget.HighImportance

	sa.SaveOneBtn = widget.NewButton("One-shot current frame!", func() {
		msg := fmt.Sprintf("Heeho! One-shot saved as %s!", OneShotFile)
		sa.processSelection([]string{sa.Files[sa.CurrentImageIndex]}, msg)
	})
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

			sa.ColorPickMode = false
			sa.PickColorToggleBtn.SetText("Chroma Key Picker")

			newHex := fmt.Sprintf("%02X%02X%02X", uint8(r>>8), uint8(g>>8), uint8(b>>8))

			if sa.ColorEntry != nil {
				sa.ColorEntry.SetText(newHex)
			}
		},
	}
	dragArea.ExtendBaseWidget(dragArea)
	return dragArea
}

func (sa *SpriteApp) buildImportBtn() *widget.Button {
	return widget.NewButtonWithIcon("Import Captures", theme.FolderOpenIcon(), func() {
		opts := []string{"Single Frame (1F)", "Full Rotation (24F)", "Custom Limit..."}

		var limitSelect *widget.RadioGroup
		limitSelect = widget.NewRadioGroup(opts, func(string) {})
		limitSelect.SetSelected(opts[1])
		limitSelect.Horizontal = false

		dialog.ShowCustomConfirm("Import Latest Captures", "Start", "Cancel", limitSelect, func(b bool) {
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
	return widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		sizeEntry := widget.NewEntry()
		sizeEntry.SetText(fmt.Sprintf("%d", SizeTarget))

		sizeOneCheck := widget.NewCheck("Respect Output Size for 1F", nil)
		sizeOneCheck.SetChecked(SizeOne)

		skipUiCheck := widget.NewCheck("Skip UI on Startup", nil)
		skipUiCheck.SetChecked(ProcessMode)

		inEntry := widget.NewEntry()
		inEntry.SetText(InputDir)

		outEntry := widget.NewEntry()
		outEntry.SetText(OutputFile)

		oneShotEntry := widget.NewEntry()
		oneShotEntry.SetText(OneShotFile)

		skipPrescaleCheck := widget.NewCheck("Skip Prescale (Full Resolution Preview)", nil)
		skipPrescaleCheck.SetChecked(SkipPrescale)

		fileTab := container.NewVBox(
			widget.NewForm(
				widget.NewFormItem("Input Directory", inEntry),
				widget.NewFormItem("Output Filename", outEntry),
				widget.NewFormItem("1F Output Filename", oneShotEntry),
				widget.NewFormItem("Output Size (px)", sizeEntry),
			),
			sizeOneCheck,
		)

		appTab := container.NewVBox(
			skipUiCheck,
			skipPrescaleCheck,
		)

		tabs := container.NewAppTabs(
			container.NewTabItem("Output", container.NewPadded(fileTab)),
			container.NewTabItem("Advanced", container.NewPadded(appTab)),
		)

		settingsDialog := dialog.NewCustomConfirm("Settings ("+CurrentVersion+")", "Save", "Cancel",
			tabs,
			func(b bool) {
				if !b {
					return
				}
				updates := map[string]string{
					"skip-ui":       strconv.FormatBool(skipUiCheck.Checked),
					"size":          sizeEntry.Text,
					"size-one":      strconv.FormatBool(sizeOneCheck.Checked),
					"in":            inEntry.Text,
					"out":           outEntry.Text,
					"out-one":       oneShotEntry.Text,
					"skip-prescale": strconv.FormatBool(skipPrescaleCheck.Checked),
				}
				err := UpdateConfig(updates)
				if err != nil {
					dialog.ShowError(err, sa.Window)
				}
				dialog.ShowInformation("Settings", "Settings saved!", sa.Window)
			},
			sa.Window,
		)

		settingsDialog.Resize(fyne.NewSize(450, 500))
		settingsDialog.Show()
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
