package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"pseudo3d-sprites/internal"
)

var version = "seven"

func init() {
	flag.BoolVar(&internal.ProcessMode, "skip-ui", false, "Skip UI and directly run batch processing")
	flag.IntVar(&internal.TargetSize, "size", 512, "Output size of the square frames (for example, 300 for 300x300)")
	flag.BoolVar(&internal.SkipBgRemoval, "skip-bg", false, "Skip chroma key background removal completely")
	flag.Float64Var(&internal.Threshold, "threshold-bg", 7000.0, "Tolerance threshold for background removal (higher = more aggressive)")
	flag.StringVar(&internal.InputDir, "in", "./process", "Input directory containing PNG frames")
	flag.StringVar(&internal.OutputFile, "out", "spritesheet.png", "Output spritesheet filename")
	flag.StringVar(&internal.OneShotFile, "out-one", "oneshot.png", "Output filename for single frame exports")
	flag.BoolVar(&internal.ErodeEdges, "erode", false, "Aggressively trim 1 pixel of alpha from edges to kill residue")
	flag.StringVar(&internal.HexColor, "color-bg", "DF03DF", "Hex color code to remove as background")
	flag.BoolVar(&internal.SkipPrescale, "skip-prescale", false, "Prescale preview images to "+fmt.Sprintf("%d", internal.PreviewMaxPx)+"px max for faster UI (originals are still used for processing)")
}

func main() {
	internal.CurrentVersion = version

	flag.Parse()
	internal.LoadConfiguration()

	var err error
	internal.ChromaKey, err = internal.ParseHexColor(internal.HexColor)
	if err != nil {
		log.Fatalf("Error parsing chroma key color: %v\n", err)
	}

	if internal.ProcessMode {
		if err := internal.RunBatchProcessing(nil, nil); err != nil {
			fmt.Printf("Processing failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	internal.RunUI()
}
