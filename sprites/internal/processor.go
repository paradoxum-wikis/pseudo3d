package internal

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/t7ru/chromakey"
)

func RunBatchProcessing(progressCallback func(current, total int)) error {
	OutputFile = strings.TrimSuffix(OutputFile, ".png") + ".png"
	LoadSafeZoneConfig()
	if !GlobalSafeZone.Active {
		return fmt.Errorf("no safe zone configured")
	}

	files, err := GetPNGFiles(InputDir)
	if err != nil {
		return err
	}

	totalFiles := len(files)
	sheet := image.NewRGBA(image.Rect(0, 0, TargetSize*totalFiles, TargetSize))

	for i, path := range files {
		img := LoadImage(path)

		cropped := imaging.Crop(img, image.Rect(
			GlobalSafeZone.MinX, GlobalSafeZone.MinY,
			GlobalSafeZone.MaxX, GlobalSafeZone.MaxY,
		))
		var currentImg image.Image = cropped

		if !SkipBgRemoval {
			currentImg = chromakey.Remove(currentImg, ChromaKey, Threshold)
			if ErodeEdges {
				if rgba, ok := currentImg.(*image.RGBA); ok {
					currentImg = chromakey.Erode(rgba)
				}
			}
		}

		resized := imaging.Fit(currentImg, TargetSize, TargetSize, imaging.Lanczos)
		b := resized.Bounds()
		dx := (TargetSize - b.Dx()) / 2
		dy := (TargetSize - b.Dy()) / 2
		dp := image.Pt(i*TargetSize+dx, dy)

		draw.Draw(sheet, image.Rectangle{Min: dp, Max: dp.Add(b.Size())}, resized, b.Min, draw.Src)

		if progressCallback != nil {
			progressCallback(i+1, totalFiles)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, sheet); err != nil {
		return fmt.Errorf("failed to encode PNG: %w", err)
	}

	pngData := buf.Bytes()
	if len(pngData) > 33 {
		var newPng bytes.Buffer
		newPng.Write(pngData[:33])

		chunkType := []byte("tEXt")
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

			var b [4]byte
			binary.BigEndian.PutUint32(b[:], uint32(len(textData)))
			newPng.Write(b[:])
			newPng.Write(chunkType)
			newPng.Write(textData)

			crc := crc32.NewIEEE()
			crc.Write(chunkType)
			crc.Write(textData)
			binary.BigEndian.PutUint32(b[:], crc.Sum32())
			newPng.Write(b[:])
		}

		newPng.Write(pngData[33:])
		pngData = newPng.Bytes()
	}

	if _, err := os.Stat(OutputFile); err == nil {
		os.MkdirAll("archive", 0755)
		base := strings.TrimSuffix(filepath.Base(OutputFile), filepath.Ext(OutputFile))
		backupName := filepath.Join("archive", fmt.Sprintf("%s.png", base))
		for i := 2; ; i++ {
			if _, err := os.Stat(backupName); os.IsNotExist(err) {
				break
			}
			backupName = filepath.Join("archive", fmt.Sprintf("%s-%d.png", base, i))
		}
		os.Rename(OutputFile, backupName)
	}

	out, err := os.Create(OutputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()
	if _, err := out.Write(pngData); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}
