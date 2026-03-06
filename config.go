package main

import (
	"fmt"
	"image/color"
	"os"
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
	skipPrescale  bool
	skipMenu      bool
)

const previewMaxPx = 1024 // max dimension for -prescale

var chromaKey color.RGBA

type SafeZone struct {
	MinX   int  `json:"minX"`
	MinY   int  `json:"minY"`
	MaxX   int  `json:"maxX"`
	MaxY   int  `json:"maxY"`
	Active bool `json:"active"`
}

var globalSafeZone SafeZone

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
