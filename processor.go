package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/schollz/progressbar/v3"
)

func runBatchProcessing() {
	outputFile = strings.TrimSuffix(outputFile, ".png") + ".png"
	loadSafeZoneConfig()
	if !globalSafeZone.Active {
		fmt.Println("No safe zone configured. Run without -process to open the UI first.")
		os.Exit(1)
	}

	files, err := getPNGFiles(inputDir)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Configuration:\n")
	fmt.Printf(" - Input: %s | Output: %s\n", inputDir, outputFile)
	fmt.Printf(" - Target Size: %dx%d\n", targetSize, targetSize)
	if !skipBgRemoval {
		fmt.Printf(" - BG Removal: ON (Color: #%s, Threshold: %.1f)\n", strings.ToUpper(strings.TrimPrefix(hexColor, "#")), threshold)
		if erodeEdges {
			fmt.Printf(" - Edge Erosion: ON\n")
		}
	} else {
		fmt.Printf(" - BG Removal: OFF\n")
	}

	bar := progressbar.Default(int64(len(files)), "Processing frames")
	var frames []image.Image

	for _, path := range files {
		img := loadImage(path)
		var currentImg image.Image = img

		if !skipBgRemoval {
			currentImg = chromaKeyRemove(currentImg)
			if erodeEdges {
				if rgba, ok := currentImg.(*image.RGBA); ok {
					currentImg = erodeAlpha(rgba)
				}
			}
		}

		cropped := imaging.Crop(currentImg, image.Rect(
			globalSafeZone.MinX, globalSafeZone.MinY,
			globalSafeZone.MaxX, globalSafeZone.MaxY,
		))
		resized := imaging.Fit(cropped, targetSize, targetSize, imaging.Lanczos)
		squareFrame := imaging.New(targetSize, targetSize, color.Transparent)
		squareFrame = imaging.PasteCenter(squareFrame, resized)

		frames = append(frames, squareFrame)
		bar.Add(1)
	}

	sheet := image.NewRGBA(image.Rect(0, 0, targetSize*len(frames), targetSize))
	for i, frame := range frames {
		draw.Draw(sheet, image.Rect(i*targetSize, 0, (i+1)*targetSize, targetSize), frame, image.Point{}, draw.Src)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, sheet); err != nil {
		log.Fatal(err)
	}

	pngData := buf.Bytes()
	if len(pngData) > 33 {
		textData := append([]byte("Description"), 0)
		textData = append(textData, []byte("Generated using pseudo3d-viewer 1.1 (Fyne Port)")...)
		var newPng bytes.Buffer
		newPng.Write(pngData[:33])
		binary.Write(&newPng, binary.BigEndian, uint32(len(textData)))
		newPng.Write([]byte("tEXt"))
		newPng.Write(textData)
		crc := crc32.NewIEEE()
		crc.Write([]byte("tEXt"))
		crc.Write(textData)
		binary.Write(&newPng, binary.BigEndian, crc.Sum32())
		newPng.Write(pngData[33:])
		pngData = newPng.Bytes()
	}

	out, err := os.Create(outputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	if _, err := out.Write(pngData); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nHeeho! Spritesheet saved as %s (%d frames)\n", outputFile, len(frames))
	time.Sleep(3 * time.Second)
}

func chromaKeyRemove(img image.Image) image.Image {
	bounds := img.Bounds()
	newImg := image.NewRGBA(bounds)

	switch src := img.(type) {
	case *image.RGBA:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			i := src.PixOffset(bounds.Min.X, y)
			j := newImg.PixOffset(bounds.Min.X, y)
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := color.RGBA{R: src.Pix[i], G: src.Pix[i+1], B: src.Pix[i+2], A: src.Pix[i+3]}
				if colorDiff(c, chromaKey) < threshold {
					newImg.Pix[j] = 0
					newImg.Pix[j+1] = 0
					newImg.Pix[j+2] = 0
					newImg.Pix[j+3] = 0
				} else {
					newImg.Pix[j] = c.R
					newImg.Pix[j+1] = c.G
					newImg.Pix[j+2] = c.B
					newImg.Pix[j+3] = c.A
				}
				i += 4
				j += 4
			}
		}
	case *image.NRGBA:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			i := src.PixOffset(bounds.Min.X, y)
			j := newImg.PixOffset(bounds.Min.X, y)
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				a := src.Pix[i+3]
				var c color.RGBA
				switch a {
				case 0xff:
					c = color.RGBA{R: src.Pix[i], G: src.Pix[i+1], B: src.Pix[i+2], A: 0xff}
				case 0:
					c = color.RGBA{R: 0, G: 0, B: 0, A: 0}
				default:
					r := uint32(src.Pix[i])
					r |= r << 8
					r *= uint32(a)
					r /= 0xff

					g := uint32(src.Pix[i+1])
					g |= g << 8
					g *= uint32(a)
					g /= 0xff

					b := uint32(src.Pix[i+2])
					b |= b << 8
					b *= uint32(a)
					b /= 0xff

					c = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: a}
				}

				if colorDiff(c, chromaKey) < threshold {
					newImg.Pix[j] = 0
					newImg.Pix[j+1] = 0
					newImg.Pix[j+2] = 0
					newImg.Pix[j+3] = 0
				} else {
					newImg.Pix[j] = c.R
					newImg.Pix[j+1] = c.G
					newImg.Pix[j+2] = c.B
					newImg.Pix[j+3] = c.A
				}
				i += 4
				j += 4
			}
		}
	default:
		panic(fmt.Sprintf("Unsupported image type: %T", img))
	}
	return newImg
}

func colorDiff(c1, c2 color.RGBA) float64 {
	dr := float64(c1.R) - float64(c2.R)
	dg := float64(c1.G) - float64(c2.G)
	db := float64(c1.B) - float64(c2.B)
	return dr*dr + dg*dg + db*db
}

func erodeAlpha(img *image.RGBA) *image.RGBA {
	bounds := img.Bounds()
	refined := image.NewRGBA(bounds)
	stride := img.Stride

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := img.PixOffset(x, y)
			if img.Pix[off+3] == 0 {
				continue
			}
			isEdge := (x > bounds.Min.X && img.Pix[off-4+3] == 0) ||
				(x < bounds.Max.X-1 && img.Pix[off+4+3] == 0) ||
				(y > bounds.Min.Y && img.Pix[off-stride+3] == 0) ||
				(y < bounds.Max.Y-1 && img.Pix[off+stride+3] == 0)

			if !isEdge {
				copy(refined.Pix[off:off+4], img.Pix[off:off+4])
			}
		}
	}
	return refined
}
