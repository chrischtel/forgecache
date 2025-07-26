package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	GitHubAPI      = "https://api.github.com"
	RepoOwner      = "chrischtel" // Update this to your GitHub username
	RepoName       = "forgecache" // Update this to your repo name
	UpdateCheckURL = GitHubAPI + "/repos/" + RepoOwner + "/" + RepoName + "/releases/latest"
	DevReleaseURL  = GitHubAPI + "/repos/" + RepoOwner + "/" + RepoName + "/releases/tags/latest-dev"
)

// Release represents a GitHub release
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

// Asset represents a release asset
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpdateChecker handles version checking and updates
type UpdateChecker struct {
	currentVersion string
	currentCommit  string
}

// NewUpdateChecker creates a new update checker
func NewUpdateChecker(currentVersion, currentCommit string) *UpdateChecker {
	return &UpdateChecker{
		currentVersion: currentVersion,
		currentCommit:  currentCommit,
	}
}

// CleanupOldExecutables removes old executable files left over from updates
func CleanupOldExecutables() {
	if runtime.GOOS != "windows" {
		return // Only needed on Windows
	}
	
	currentExe, err := os.Executable()
	if err != nil {
		return
	}
	
	oldPath := currentExe + ".old"
	if _, err := os.Stat(oldPath); err == nil {
		os.Remove(oldPath) // Ignore errors - it's just cleanup
	}
}

// CheckForUpdates checks if a newer version is available
func (uc *UpdateChecker) CheckForUpdates() (*Release, bool, error) {
	// Determine which release to check against
	var url string
	if strings.Contains(uc.currentVersion, "dev") || strings.Contains(uc.currentVersion, "latest-dev") {
		url = DevReleaseURL
	} else {
		url = UpdateCheckURL
	}

	// Fetch release information
	resp, err := http.Get(url)
	if err != nil {
		return nil, false, fmt.Errorf("failed tos check for updates: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, false, fmt.Errorf("failed to check for updates: HTTP %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, false, fmt.Errorf("failed to parse release information: %v", err)
	}

	// Check if update is available
	updateAvailable := uc.isUpdateAvailable(&release)
	return &release, updateAvailable, nil
}

// isUpdateAvailable determines if the fetched release is newer than current version
func (uc *UpdateChecker) isUpdateAvailable(release *Release) bool {
	// For dev versions, compare by release date (always update to latest)
	if strings.Contains(uc.currentVersion, "dev") || strings.Contains(uc.currentVersion, "latest-dev") {
		return true // Always update dev versions to latest
	}

	// For stable versions, compare version strings
	// This is a simple comparison - in production you might want semantic version comparison
	return release.TagName != uc.currentVersion
}

// DownloadAndInstall downloads and installs the update
func (uc *UpdateChecker) DownloadAndInstall(release *Release) error {
	// Find the appropriate asset for current platform
	asset := uc.findAssetForPlatform(release.Assets)
	if asset == nil {
		return fmt.Errorf("no compatible binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	fmt.Printf("Downloading %s (%d bytes)...\n", asset.Name, asset.Size)

	// Download the asset
	resp, err := http.Get(asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %v", err)
	}
	defer resp.Body.Close()

	// Create temporary file
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, asset.Name)

	out, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer out.Close()

	// Copy downloaded content
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save downloaded file: %v", err)
	}
	out.Close()

	// Extract and install
	return uc.extractAndInstall(tempFile, asset.Name)
}

// findAssetForPlatform finds the appropriate asset for the current platform
func (uc *UpdateChecker) findAssetForPlatform(assets []Asset) *Asset {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// Map Go arch names to common names used in releases
	if archName == "amd64" {
		archName = "amd64"
	} else if archName == "arm64" {
		archName = "arm64"
	}

	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, osName) && strings.Contains(name, archName) {
			return &asset
		}
	}

	return nil
}

// extractAndInstall extracts the downloaded archive and replaces the current binary
func (uc *UpdateChecker) extractAndInstall(archivePath, assetName string) error {
	// Get current executable path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %v", err)
	}

	var newBinaryPath string

	// Extract based on file extension
	if strings.HasSuffix(assetName, ".tar.gz") {
		newBinaryPath, err = uc.extractTarGz(archivePath)
	} else if strings.HasSuffix(assetName, ".zip") {
		newBinaryPath, err = uc.extractZip(archivePath)
	} else {
		return fmt.Errorf("unsupported archive format: %s", assetName)
	}

	if err != nil {
		return fmt.Errorf("failed to extract archive: %v", err)
	}

	// On Windows, we need a different strategy because we can't replace a running executable
	if runtime.GOOS == "windows" {
		return uc.installOnWindows(currentExe, newBinaryPath, archivePath)
	}

	// Create backup
	backupPath := currentExe + ".backup"
	if err := uc.copyFile(currentExe, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %v", err)
	}

	// Replace current binary
	if err := uc.copyFile(newBinaryPath, currentExe); err != nil {
		// Restore backup on failure
		uc.copyFile(backupPath, currentExe)
		return fmt.Errorf("failed to replace binary: %v", err)
	}

	// Make executable on Unix systems
	if err := os.Chmod(currentExe, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %v", err)
	}

	// Clean up
	os.Remove(archivePath)
	os.Remove(newBinaryPath)
	os.Remove(backupPath)

	fmt.Println("Update installed successfully!")
	fmt.Println("Please restart ForgeCache to use the new version.")

	return nil
}

// installOnWindows handles the Windows-specific update process
func (uc *UpdateChecker) installOnWindows(currentExe, newBinaryPath, archivePath string) error {
	// On Windows, we can't replace a running executable, so we:
	// 1. Move the current exe to .old
	// 2. Move the new exe to the current location
	// 3. The .old file will be cleaned up on next run
	
	oldPath := currentExe + ".old"
	
	// Remove any existing .old file
	os.Remove(oldPath)
	
	// Move current executable to .old (this works even if it's running)
	if err := os.Rename(currentExe, oldPath); err != nil {
		return fmt.Errorf("failed to move current executable: %v", err)
	}
	
	// Move new executable to current location
	if err := uc.copyFile(newBinaryPath, currentExe); err != nil {
		// Try to restore the original if this fails
		os.Rename(oldPath, currentExe)
		return fmt.Errorf("failed to install new executable: %v", err)
	}
	
	// Clean up
	os.Remove(archivePath)
	os.Remove(newBinaryPath)
	
	fmt.Println("Update installed successfully!")
	fmt.Println("The old version will be cleaned up automatically.")
	fmt.Println("Please restart ForgeCache to use the new version.")
	
	return nil
}

// extractTarGz extracts a tar.gz file and returns the path to the binary
func (uc *UpdateChecker) extractTarGz(archivePath string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	tempDir := filepath.Dir(archivePath)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// Look for the binary (usually named "forge" or similar)
		if header.Typeflag == tar.TypeReg && (strings.Contains(header.Name, "forge") && !strings.Contains(header.Name, ".")) {
			outPath := filepath.Join(tempDir, "forge_new")
			outFile, err := os.Create(outPath)
			if err != nil {
				return "", err
			}

			_, err = io.Copy(outFile, tr)
			outFile.Close()
			if err != nil {
				return "", err
			}

			return outPath, nil
		}
	}

	return "", fmt.Errorf("binary not found in archive")
}

// extractZip extracts a zip file and returns the path to the binary
func (uc *UpdateChecker) extractZip(archivePath string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	tempDir := filepath.Dir(archivePath)

	for _, f := range r.File {
		// Look for the binary (usually .exe on Windows)
		if strings.Contains(f.Name, "forge") && strings.HasSuffix(f.Name, ".exe") {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}

			outPath := filepath.Join(tempDir, "forge_new.exe")
			outFile, err := os.Create(outPath)
			if err != nil {
				rc.Close()
				return "", err
			}

			_, err = io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()
			if err != nil {
				return "", err
			}

			return outPath, nil
		}
	}

	return "", fmt.Errorf("binary not found in archive")
}

// copyFile copies a file from src to dst
func (uc *UpdateChecker) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
