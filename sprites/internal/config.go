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
	OneShotFile   string
	ErodeEdges    bool
	HexColor      string
	SkipPrescale  bool
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
	LoadConfiguration()
	filename := "configuration.cfg"
	content, _ := os.ReadFile(filename)

	newLines := []string{}
	for line := range strings.SplitSeq(string(content), "\n") {
		if !strings.HasPrefix(line, "safezone=") {
			newLines = append(newLines, line)
		}
	}

	safeZoneStr := fmt.Sprintf("safezone=%d,%d,%d,%d", GlobalSafeZone.MinX, GlobalSafeZone.MinY, GlobalSafeZone.MaxX, GlobalSafeZone.MaxY)
	newLines = append(newLines, safeZoneStr)

	os.WriteFile(filename, []byte(strings.Join(newLines, "\n")), 0644)
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
			if key == "safezone" {
				var minX, minY, maxX, maxY int
				if _, err := fmt.Sscanf(val, "%d,%d,%d,%d", &minX, &minY, &maxX, &maxY); err == nil {
					GlobalSafeZone = SafeZone{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY, Active: true}
				}
			} else if !setFlags[key] && flag.Lookup(key) != nil {
				flag.Set(key, val)
			}
		}
	}
}

func UpdateConfig(updates map[string]string) error {
	filename := "configuration.cfg"
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	for key, value := range updates {
		if flag.Lookup(key) != nil {
			flag.Set(key, value)
		}
	}

	var newLines []string
	for line := range strings.SplitSeq(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if key, _, ok := strings.Cut(trimmed, "="); ok {
			key = strings.TrimSpace(key)
			if val, exists := updates[key]; exists {
				newLines = append(newLines, fmt.Sprintf("%s=%s", key, val))
				delete(updates, key)
				continue
			}
		}
		newLines = append(newLines, line)
	}

	for key, value := range updates {
		newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
	}

	return os.WriteFile(filename, []byte(strings.Join(newLines, "\n")), 0644)
}
