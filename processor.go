package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/schollz/progressbar/v3"
	"github.com/t7ru/chromakey"
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
	sheet := image.NewRGBA(image.Rect(0, 0, targetSize*len(files), targetSize))

	for i, path := range files {
		img := loadImage(path)

		cropped := imaging.Crop(img, image.Rect(
			globalSafeZone.MinX, globalSafeZone.MinY,
			globalSafeZone.MaxX, globalSafeZone.MaxY,
		))
		var currentImg image.Image = cropped

		if !skipBgRemoval {
			currentImg = chromakey.Remove(currentImg, chromaKey, threshold)
			if erodeEdges {
				if rgba, ok := currentImg.(*image.RGBA); ok {
					currentImg = chromakey.Erode(rgba)
				}
			}
		}

		resized := imaging.Fit(currentImg, targetSize, targetSize, imaging.Lanczos)
		b := resized.Bounds()
		dx := (targetSize - b.Dx()) / 2
		dy := (targetSize - b.Dy()) / 2
		dp := image.Pt(i*targetSize+dx, dy)

		draw.Draw(sheet, image.Rectangle{Min: dp, Max: dp.Add(b.Size())}, resized, b.Min, draw.Src)

		bar.Add(1)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, sheet); err != nil {
		log.Fatal(err)
	}

	pngData := buf.Bytes()
	if len(pngData) > 33 {
		var newPng bytes.Buffer
		newPng.Write(pngData[:33])

		chunks := []struct{ key, val string }{
			{"Software", "Pseudo3D"},
			{"Author", "Paradoxum Wikis' Pseudo3D"},
			{"Description", "Generated using Paradoxum Wikis' Pseudo3D"},
			{"Copyright", "https://github.com/paradoxum-wikis/pseudo3d"},
		}

		for _, c := range chunks {
			textData := make([]byte, 0, len(c.key)+1+len(c.val))
			textData = append(textData, c.key...)
			textData = append(textData, 0)
			textData = append(textData, c.val...)

			binary.Write(&newPng, binary.BigEndian, uint32(len(textData)))
			newPng.WriteString("tEXt")
			newPng.Write(textData)

			crc := crc32.NewIEEE()
			crc.Write([]byte("tEXt"))
			crc.Write(textData)
			binary.Write(&newPng, binary.BigEndian, crc.Sum32())
		}

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

	fmt.Printf("\nHeeho! Spritesheet saved as %s (%d frames)\n", outputFile, len(files))
	time.Sleep(3 * time.Second)
}
