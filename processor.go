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
	var frames []image.Image

	for _, path := range files {
		img := loadImage(path)
		var currentImg image.Image = img

		if !skipBgRemoval {
			currentImg = chromakey.Remove(currentImg, chromaKey, threshold)
			if erodeEdges {
				if rgba, ok := currentImg.(*image.RGBA); ok {
					currentImg = chromakey.ErodeAlpha(rgba)
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
