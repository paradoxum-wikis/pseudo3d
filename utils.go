package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func getPNGFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if strings.EqualFold(filepath.Ext(e.Name()), ".png") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func importLatestCaptures(processDir, archiveDir string) ([]string, error) {
	srcDir, err := getRobloxCaptureDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, err
	}

	type captureFile struct {
		path    string
		modTime time.Time
	}

	var captures []captureFile
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".png") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		captures = append(captures, captureFile{
			path:    filepath.Join(srcDir, e.Name()),
			modTime: info.ModTime(),
		})
	}

	if len(captures) == 0 {
		return nil, fmt.Errorf("no PNG captures found in %s", srcDir)
	}

	sort.Slice(captures, func(i, j int) bool {
		return captures[i].modTime.After(captures[j].modTime)
	})
	if len(captures) > 24 {
		captures = captures[:24]
	}
	sort.Slice(captures, func(i, j int) bool {
		return captures[i].modTime.Before(captures[j].modTime)
	})

	if err := os.MkdirAll(processDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return nil, err
	}

	existing, err := getPNGFiles(processDir)
	if err == nil && len(existing) > 0 {
		base := strings.TrimSuffix(filepath.Base(existing[0]), filepath.Ext(existing[0]))
		archiveSubdir := filepath.Join(archiveDir, base)
		for i := 2; isDirectory(archiveSubdir); i++ {
			archiveSubdir = filepath.Join(archiveDir, fmt.Sprintf("%s-%d", base, i))
		}
		if err := os.MkdirAll(archiveSubdir, 0755); err != nil {
			return nil, err
		}
		for _, path := range existing {
			dst := filepath.Join(archiveSubdir, filepath.Base(path))
			if err := os.Rename(path, dst); err != nil {
				return nil, err
			}
		}
	}

	imported := make([]string, 0, len(captures))
	for _, c := range captures {
		dst := filepath.Join(processDir, filepath.Base(c.path))
		if err := copyFile(c.path, dst); err != nil {
			return nil, err
		}
		imported = append(imported, dst)
	}

	return imported, nil
}

func getRobloxCaptureDir() (string, error) {
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		dir := filepath.Join(localAppData, "Roblox", "tmp-capture-storage")
		if isDirectory(dir) {
			return dir, nil
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		candidates := []string{
			filepath.Join(home, "Library", "Application Support", "Roblox", "tmp-capture-storage"),
			filepath.Join(home, ".var", "app", "org.vinegarhq.Vinegar", "data", "vinegar", "studio", "prefix", "drive_c", "users", os.Getenv("USER"), "AppData", "Local", "Roblox", "tmp-capture-storage"),
			filepath.Join(home, ".local", "share", "vinegar", "studio", "prefix", "drive_c", "users", os.Getenv("USER"), "AppData", "Local", "Roblox", "tmp-capture-storage"),
		}
		for _, dir := range candidates {
			if isDirectory(dir) {
				return dir, nil
			}
		}
	}

	return "", fmt.Errorf("could not find Roblox capture directory")
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return out.Close()
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
