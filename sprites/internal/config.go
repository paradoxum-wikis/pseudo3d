package internal

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"image/color"
	"os"
	"strings"
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

func LoadConfiguration() {
	filename := "configuration.cfg"
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		var defaults strings.Builder
		defaults.WriteString("# Please see the Flags Reference section of the README for an idea on what these are!\n")
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(&defaults, "%s=%s\n", f.Name, f.DefValue)
		})
		os.WriteFile(filename, []byte(defaults.String()), 0644)
		return
	}

	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if key, val, ok := strings.Cut(line, "="); ok {
			key, val = strings.TrimSpace(key), strings.TrimSpace(val)
			if !setFlags[key] && flag.Lookup(key) != nil {
				flag.Set(key, val)
			}
		}
	}
}
