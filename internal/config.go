package internal

import (
	"fmt"
	"image/color"
	"os"
)

var (
	ProcessMode   bool
	TargetSize    int
	SkipBgRemoval bool
	Threshold     float64
	InputDir      string
	OutputFile    string
	ErodeEdges    bool
	HexColor      string
	SkipPrescale  bool
	SkipMenu      bool
)

const PreviewMaxPx = 1024 // max dimension for prescale

var ChromaKey color.RGBA

type SafeZone struct {
	MinX   int  `json:"minX"`
	MinY   int  `json:"minY"`
	MaxX   int  `json:"maxX"`
	MaxY   int  `json:"maxY"`
	Active bool `json:"active"`
}

var GlobalSafeZone SafeZone

func SaveSafeZoneConfig() {
	data := fmt.Sprintf("%d,%d,%d,%d\n",
		GlobalSafeZone.MinX, GlobalSafeZone.MinY,
		GlobalSafeZone.MaxX, GlobalSafeZone.MaxY)
	os.WriteFile("safezone.cfg", []byte(data), 0644)
}

func LoadSafeZoneConfig() {
	data, err := os.ReadFile("safezone.cfg")
	if err != nil {
		return
	}
	var minX, minY, maxX, maxY int
	fmt.Sscanf(string(data), "%d,%d,%d,%d", &minX, &minY, &maxX, &maxY)
	GlobalSafeZone = SafeZone{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY, Active: true}
}
