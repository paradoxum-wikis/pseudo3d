package internal

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	updateAPIURL   = "https://api.github.com/repos/paradoxum-wikis/pseudo3d/releases/latest"
	UpdateClientID = "Ov23liweYQiuwJNvXxus"
)

var (
	CurrentVersion string
	netClient      = &http.Client{Timeout: 10 * time.Second}
	GithubToken    string
	tempDeviceCode string
)

type UpdateState int

const (
	UpdateStateLoginRequired UpdateState = iota
	UpdateStateReady
)

type UpdateInfo struct {
	State       UpdateState
	HasUpdate   bool
	LatestTag   string
	DownloadURL string
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func CheckForUpdates(currentVersion string) (*UpdateInfo, string, error) {
	token := loadPreference("updates.token")
	if token == "" {
		return &UpdateInfo{State: UpdateStateLoginRequired}, "", nil
	}

	var release githubRelease
	if err := apiReq(http.MethodGet, updateAPIURL, token, nil, &release); err != nil {
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			savePreference("updates.token", "")
			return nil, "", errors.New("login required")
		}
		return nil, "", err
	}

	var user struct {
		Login string `json:"login"`
	}
	_ = apiReq(http.MethodGet, "https://api.github.com/user", token, nil, &user)

	latest := normalizeVersion(release.TagName)
	downloadURL := getZipURL(&release)

	if downloadURL == "" {
		return nil, user.Login, errors.New("no compatible release found")
	}

	info := &UpdateInfo{
		State:       UpdateStateReady,
		LatestTag:   latest,
		DownloadURL: downloadURL,
	}

	if currentVersion == "seven" {
		info.HasUpdate = false
	} else {
		info.HasUpdate = compareVersions(latest, normalizeVersion(currentVersion)) > 0
	}

	return info, user.Login, nil
}

func DownloadUpdate(info *UpdateInfo) error {
	if info == nil || info.DownloadURL == "" {
		return errors.New("invalid update info")
	}

	archivePath := getUpdatePath()
	os.MkdirAll(filepath.Dir(archivePath), 0755)

	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(info.DownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}

	return nil
}

func ApplyPendingUpdate() bool {
	archivePath := loadPreference("updates.pending_zip")
	if archivePath == "" {
		return false
	}
	defer os.Remove(getUpdatePath())

	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	exeDir := filepath.Dir(exePath)
	backupDir := filepath.Join(exeDir, "archive", "backup")

	os.RemoveAll(backupDir)
	os.MkdirAll(backupDir, 0755)

	if strings.HasSuffix(archivePath, ".tar.gz") {
		return applyTarGz(archivePath, exeDir, backupDir) == nil
	}
	return applyZip(archivePath, exeDir, backupDir) == nil
}

func backupFile(srcPath, backupDir string) error {
	if _, err := os.Stat(srcPath); err != nil {
		return nil
	}
	rel, _ := filepath.Rel(filepath.Dir(backupDir), srcPath)
	dst := filepath.Join(backupDir, rel)
	os.MkdirAll(filepath.Dir(dst), 0755)

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dstf, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstf.Close()

	_, err = io.Copy(dstf, src)
	return err
}

func applyZip(zipPath, exeDir, backupDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, file := range zr.File {
		if !filepath.IsLocal(file.Name) {
			continue
		}

		outPath := filepath.Join(exeDir, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(outPath, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(outPath), 0755)

		backupFile(outPath, backupDir)

		func() {
			src, err := file.Open()
			if err != nil {
				return
			}
			defer src.Close()

			dst, err := os.Create(outPath)
			if err != nil {
				return
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err == nil && file.Mode()&0111 != 0 {
				os.Chmod(outPath, 0755)
			}
		}()
	}
	return nil
}

func applyTarGz(tarPath, exeDir, backupDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil || !filepath.IsLocal(header.Name) {
			continue
		}

		outPath := filepath.Join(exeDir, header.Name)
		if header.FileInfo().IsDir() {
			os.MkdirAll(outPath, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(outPath), 0755)

		backupFile(outPath, backupDir)

		func() {
			dst, err := os.Create(outPath)
			if err != nil {
				return
			}
			defer dst.Close()

			if _, err := io.Copy(dst, tr); err == nil && header.FileInfo().Mode()&0111 != 0 {
				os.Chmod(outPath, 0755)
			}
		}()
	}
	return nil
}

func BeginGitHubDeviceLogin() (string, string, error) {
	var out struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
	}

	err := apiReq(http.MethodPost, "https://github.com/login/device/code", "", url.Values{
		"client_id": {UpdateClientID},
		"scope":     {"repo"},
	}, &out)

	tempDeviceCode = out.DeviceCode
	return out.VerificationURI, out.UserCode, err
}

func CompleteGitHubDeviceLogin() error {
	if tempDeviceCode == "" {
		return errors.New("missing credentials")
	}

	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}

	err := apiReq(http.MethodPost, "https://github.com/login/oauth/access_token", "", url.Values{
		"client_id":   {UpdateClientID},
		"device_code": {tempDeviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}, &out)

	if out.Error != "" {
		return errors.New(out.Error)
	}
	if err != nil {
		return err
	}
	if out.AccessToken == "" {
		return errors.New("no access token returned")
	}

	savePreference("updates.token", out.AccessToken)
	tempDeviceCode = ""
	return nil
}

func apiReq(method, reqURL, token string, form url.Values, out any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := netClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("json decode error: %w", err)
		}
	}

	return nil
}

func getZipURL(release *githubRelease) string {
	pattern := "linux-amd64.tar.gz"
	switch runtime.GOOS {
	case "windows":
		pattern = "windows-amd64.zip"
	case "darwin":
		pattern = "macos-amd64.zip"
		if runtime.GOARCH == "arm64" {
			pattern = "macos-arm64.zip"
		}
	}

	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, pattern) {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func compareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		av, bv := 0, 0
		if i < len(pa) {
			av = pa[i]
		}
		if i < len(pb) {
			bv = pb[i]
		}
		if av > bv {
			return 1
		} else if av < bv {
			return -1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = normalizeVersion(v)
	var parts []int
	for _, chunk := range strings.FieldsFunc(v, func(r rune) bool { return r < '0' || r > '9' }) {
		if n, err := strconv.Atoi(chunk); err == nil {
			parts = append(parts, n)
		}
	}
	return parts
}

func normalizeVersion(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.TrimPrefix(v, "refs/tags/")
	return strings.TrimPrefix(v, "v")
}

func loadPreference(key string) string {
	if key == "updates.token" {
		return GithubToken
	}
	if key == "updates.pending_zip" {
		archivePath := getUpdatePath()
		if _, err := os.Stat(archivePath); err == nil {
			return archivePath
		}
	}
	return ""
}

func savePreference(key, value string) {
	if key == "updates.token" {
		GithubToken = value
		UpdateConfig(map[string]string{
			"gh-token": value,
		})
	}
}

func getUpdatePath() string {
	ext := ".tar.gz"
	if runtime.GOOS != "linux" {
		ext = ".zip"
	}
	return filepath.Join(os.TempDir(), "pseudo3d-updates", "update"+ext)
}
