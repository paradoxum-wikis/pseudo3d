package main

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/schollz/progressbar/v3"
)

var (
	processMode   bool
	targetSize    int
	skipBgRemoval bool
	threshold     float64
	inputDir      string
	outputFile    string
	erodeEdges    bool
	hexColor      string
)

var chromaKey color.RGBA

type SafeZone struct {
	MinX   int  `json:"minX"`
	MinY   int  `json:"minY"`
	MaxX   int  `json:"maxX"`
	MaxY   int  `json:"maxY"`
	Active bool `json:"active"`
}

var globalSafeZone SafeZone

//go:embed ui.html
var htmlUI string

func init() {
	flag.BoolVar(&processMode, "process", false, "Skip Web UI and directly run batch processing")
	flag.IntVar(&targetSize, "size", 512, "Output size of the square frames (for example, 300 for 300x300)")
	flag.BoolVar(&skipBgRemoval, "no-bg", false, "Skip chroma key background removal completely")
	flag.Float64Var(&threshold, "threshold", 70.0, "Tolerance threshold for background removal (higher = more aggressive)")
	flag.StringVar(&inputDir, "in", "./process", "Input directory containing PNG frames")
	flag.StringVar(&outputFile, "out", "spritesheet.png", "Output spritesheet filename")
	flag.BoolVar(&erodeEdges, "erode", false, "Aggressively trim 1 pixel of alpha from edges to kill residue")
	flag.StringVar(&hexColor, "color", "DF03DF", "Hex color code to remove as background")
}

func parseHexColor(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return color.RGBA{}, fmt.Errorf("invalid hex color length: %s", s)
	}
	var r, g, b uint8
	_, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}, nil
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
		log.Fatalf("No PNG files found in %s", inputDir)
	}

	shutdown := make(chan struct{})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlUI))
	})

	http.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%d", len(files))
	})

	http.HandleFunc("/image/", func(w http.ResponseWriter, r *http.Request) {
		var idx int
		fmt.Sscanf(strings.TrimPrefix(r.URL.Path, "/image/"), "%d", &idx)
		if idx < 0 || idx >= len(files) {
			http.Error(w, "out of range", http.StatusBadRequest)
			return
		}
		http.ServeFile(w, r, files[idx])
	})

	http.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request", http.StatusMethodNotAllowed)
			return
		}

		if err := json.NewDecoder(r.Body).Decode(&globalSafeZone); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		globalSafeZone.Active = true
		saveSafeZoneConfig()

		w.WriteHeader(http.StatusOK)
		close(shutdown)
	})

	fmt.Println("Starting Web UI for safe zone selection.")
	fmt.Printf("Open http://localhost:7777 in your browser.\n\n")
	fmt.Println("TIP: Run with -h to see all available flags.")

	go func() {
		if err := http.ListenAndServe(":7777", nil); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-shutdown
	fmt.Println("\nSafe zone received. Closing web server and starting batch process...")
	runBatchProcessing()
}

func runBatchProcessing() {
	outputFile = strings.TrimSuffix(outputFile, ".png") + ".png"
	loadSafeZoneConfig()
	if !globalSafeZone.Active {
		fmt.Println("No safe zone configured. Run without -process to open the Web UI first.")
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
			globalSafeZone.MinX,
			globalSafeZone.MinY,
			globalSafeZone.MaxX,
			globalSafeZone.MaxY,
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
		textData = append(textData, []byte("Generated using pseudo3d-viewer 1.1")...)

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
}

func chromaKeyRemove(img image.Image) image.Image {
	bounds := img.Bounds()
	newImg := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			if colorDiff(c, chromaKey) < threshold {
				newImg.Set(x, y, color.Transparent)
			} else {
				newImg.Set(x, y, c)
			}
		}
	}
	return newImg
}

func colorDiff(c1, c2 color.RGBA) float64 {
	return math.Sqrt(math.Pow(float64(c1.R)-float64(c2.R), 2) +
		math.Pow(float64(c1.G)-float64(c2.G), 2) +
		math.Pow(float64(c1.B)-float64(c2.B), 2))
}

func erodeAlpha(img *image.RGBA) *image.RGBA {
	bounds := img.Bounds()
	refined := image.NewRGBA(bounds)

	neighbors := [][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a > 0 {
				isEdge := false
				for _, offset := range neighbors {
					nx, ny := x+offset[0], y+offset[1]
					if nx >= bounds.Min.X && nx < bounds.Max.X && ny >= bounds.Min.Y && ny < bounds.Max.Y {
						_, _, _, na := img.At(nx, ny).RGBA()
						if na == 0 {
							isEdge = true
							break
						}
					}
				}
				if isEdge {
					refined.Set(x, y, color.Transparent)
				} else {
					refined.Set(x, y, img.At(x, y))
				}
			}
		}
	}
	return refined
}

func getPNGFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".png" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	return img
}

func saveSafeZoneConfig() {
	data := fmt.Sprintf("%d,%d,%d,%d\n",
		globalSafeZone.MinX, globalSafeZone.MinY,
		globalSafeZone.MaxX, globalSafeZone.MaxY)
	os.WriteFile("safezone.cfg", []byte(data), 0644)
}

func loadSafeZoneConfig() {
	data, err := os.ReadFile("safezone.cfg")
	if err != nil {
		return
	}

	var minX, minY, maxX, maxY int
	fmt.Sscanf(string(data), "%d,%d,%d,%d", &minX, &minY, &maxX, &maxY)
	globalSafeZone = SafeZone{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY, Active: true}
}
