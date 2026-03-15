package internal

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/t7ru/chromakey/v2"
)

func getOpaqueBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	minX, minY := math.MaxInt32, math.MaxInt32
	maxX, maxY := math.MinInt32, math.MinInt32
	found := false

	var pix []uint8
	var stride int
	switch src := img.(type) {
	case *image.RGBA:
		pix, stride = src.Pix, src.Stride
	case *image.NRGBA:
		pix, stride = src.Pix, src.Stride
	}

	if pix != nil {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			off := (y-b.Min.Y)*stride + b.Min.X*4
			row := pix[off : off+b.Dx()*4]
			for x := 0; x < b.Dx()*4; x += 4 {
				if row[x+3] > 0 {
					found = true
					realX := b.Min.X + x/4
					if realX < minX {
						minX = realX
					}
					if realX > maxX {
						maxX = realX
					}
					if y < minY {
						minY = y
					}
					if y > maxY {
						maxY = y
					}
				}
			}
		}
	} else {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				if a > 0 {
					found = true
					if x < minX {
						minX = x
					}
					if x > maxX {
						maxX = x
					}
					if y < minY {
						minY = y
					}
					if y > maxY {
						maxY = y
					}
				}
			}
		}
	}
	if !found {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

func processImage(path string, tightCrop bool) (image.Image, error) {
	img := LoadImage(path)
	if img == nil {
		return nil, fmt.Errorf("failed to load %s", path)
	}

	cropped := imaging.Crop(img, image.Rect(
		GlobalSafeZone.MinX, GlobalSafeZone.MinY,
		GlobalSafeZone.MaxX, GlobalSafeZone.MaxY,
	))

	var current image.Image = cropped
	if !SkipBgRemoval {
		if ModeBg == "range" {
			current = chromakey.RemoveRange(current, ChromaKey, ThresholdMin, Threshold)
		} else {
			current = chromakey.Remove(current, ChromaKey, Threshold)
		}
		if ErodeEdges {
			if rgba, ok := current.(*image.RGBA); ok {
				current = chromakey.Erode(rgba)
			}
		}
	}

	if tightCrop {
		tight := getOpaqueBounds(current)
		if !tight.Empty() {
			current = imaging.Crop(current, tight)
		}
	}

	return current, nil
}

func RunBatchProcessing(files []string, progressCallback func(current, total int)) error {
	OutputFile = strings.TrimSuffix(OutputFile, ".png") + ".png"

	if !GlobalSafeZone.Active {
		return fmt.Errorf("no safe zone configured")
	}

	if len(files) == 0 {
		var err error
		if files, err = GetPNGFiles(InputDir); err != nil {
			return err
		}
	}

	totalFiles := len(files)
	if totalFiles == 0 {
		return fmt.Errorf("no files to process")
	}

	targetOut := OutputFile
	if totalFiles == 1 {
		targetOut = strings.TrimSuffix(OneShotFile, ".png") + ".png"
	}

	var sheet *image.RGBA

	if totalFiles == 1 {
		finalCropped, err := processImage(files[0], !SkipAutocrop)
		if err != nil {
			return err
		}
		fb := finalCropped.Bounds()
		w, h := fb.Dx(), fb.Dy()
		sq := max(w, h)
		if SizeOne {
			sq = SizeTarget
		}
		sheet = image.NewRGBA(image.Rect(0, 0, sq, sq))

		if SizeOne {
			resized := imaging.Fit(finalCropped, SizeTarget, SizeTarget, imaging.Lanczos)
			b := resized.Bounds()
			dp := image.Pt((sq-b.Dx())/2, (sq-b.Dy())/2)
			draw.Draw(sheet, image.Rectangle{Min: dp, Max: dp.Add(b.Size())}, resized, b.Min, draw.Src)
		} else {
			dp := image.Pt((sq-w)/2, (sq-h)/2)
			draw.Draw(sheet, image.Rectangle{Min: dp, Max: dp.Add(fb.Size())}, finalCropped, fb.Min, draw.Src)
		}
	} else {
		frames := make([]image.Image, totalFiles)
		globalMinX, globalMinY, globalMaxX, globalMaxY := math.MaxInt32, math.MaxInt32, math.MinInt32, math.MinInt32
		foundOpaque := false

		var wg sync.WaitGroup
		var mu sync.Mutex
		var procErr error

		for i, path := range files {
			wg.Add(1)
			go func() {
				defer wg.Done()

				frame, err := processImage(path, false)
				if err != nil {
					mu.Lock()
					if procErr == nil {
						procErr = err
					}
					mu.Unlock()
					return
				}

				var b image.Rectangle
				if !SkipAutocrop {
					b = getOpaqueBounds(frame)
				}

				mu.Lock()
				frames[i] = frame
				if !SkipAutocrop && !b.Empty() {
					foundOpaque = true
					globalMinX, globalMinY = min(globalMinX, b.Min.X), min(globalMinY, b.Min.Y)
					globalMaxX, globalMaxY = max(globalMaxX, b.Max.X), max(globalMaxY, b.Max.Y)
				}
				mu.Unlock()
			}()
		}

		wg.Wait()
		if procErr != nil {
			return procErr
		}

		globalBounds := frames[0].Bounds()
		if foundOpaque {
			globalBounds = image.Rect(globalMinX, globalMinY, globalMaxX, globalMaxY)
		}

		sheet = image.NewRGBA(image.Rect(0, 0, SizeTarget*totalFiles, SizeTarget))
		for i, frame := range frames {
			resized := imaging.Fit(imaging.Crop(frame, globalBounds), SizeTarget, SizeTarget, imaging.Lanczos)
			b := resized.Bounds()

			dp := image.Pt(i*SizeTarget+(SizeTarget-b.Dx())/2, (SizeTarget-b.Dy())/2)
			draw.Draw(sheet, image.Rectangle{Min: dp, Max: dp.Add(b.Size())}, resized, b.Min, draw.Src)

			if progressCallback != nil {
				progressCallback(i+1, totalFiles)
			}
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
			{"Author", "Paradoxum Wikis Member"},
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

	if _, err := os.Stat(targetOut); err == nil {
		if _, err := os.Stat("archive"); os.IsNotExist(err) {
			return fmt.Errorf("archive directory does not exist")
		}
		base := strings.TrimSuffix(filepath.Base(targetOut), filepath.Ext(targetOut))
		backupName := filepath.Join("archive", fmt.Sprintf("%s.png", base))
		for i := 2; ; i++ {
			if _, err := os.Stat(backupName); os.IsNotExist(err) {
				break
			}
			backupName = filepath.Join("archive", fmt.Sprintf("%s-%d.png", base, i))
		}
		if err := os.Rename(targetOut, backupName); err != nil {
			return fmt.Errorf("failed to archive %s: %w", targetOut, err)
		}
	}

	out, err := os.Create(targetOut)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()
	if _, err := out.Write(pngData); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

func applyPreviewProcessing(src image.Image) image.Image {
	if SkipBgRemoval {
		return src
	}

	if ModeBg == "range" {
		src = chromakey.RemoveRange(src, ChromaKey, ThresholdMin, Threshold)
	} else {
		src = chromakey.Remove(src, ChromaKey, Threshold)
	}

	if ErodeEdges {
		if rgba, ok := src.(*image.RGBA); ok {
			src = chromakey.Erode(rgba)
		}
	}

	return src
}
