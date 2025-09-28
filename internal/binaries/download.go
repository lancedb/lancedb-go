package binaries

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// Default GitHub release URL pattern
	DefaultReleaseURLTemplate = "https://github.com/lancedb/lancedb-go/releases/download/%s/lancedb-go-native-binaries.tar.gz"

	// Environment variable to override release URL
	ReleaseURLEnv = "LANCEDB_RELEASE_URL"

	// Environment variable to specify version
	VersionEnv = "LANCEDB_VERSION"

	// Default version (should match current module version)
	DefaultVersion = "v0.1.1-3"
)

// PlatformInfo holds platform-specific information
type PlatformInfo struct {
	OS   string
	Arch string
	Dir  string // e.g., "darwin_amd64"
}

// GetPlatformInfo returns the current platform information
func GetPlatformInfo() PlatformInfo {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Normalize architecture names to match our naming convention
	if goarch == "amd64" {
		goarch = "amd64"
	} else if goarch == "arm64" {
		goarch = "arm64"
	}

	return PlatformInfo{
		OS:   goos,
		Arch: goarch,
		Dir:  fmt.Sprintf("%s_%s", goos, goarch),
	}
}

// EnsureBinariesExist checks if required binaries exist and downloads them if not
// Falls back to building from source if download fails
func EnsureBinariesExist(projectRoot string) error {
	platform := GetPlatformInfo()

	// Check if binaries already exist
	if binariesExist(projectRoot, platform) {
		return nil
	}

	fmt.Printf("🔄 LanceDB: Downloading native binaries for %s_%s...\n", platform.OS, platform.Arch)

	// Try to download and extract binaries
	if err := downloadAndExtractBinaries(projectRoot); err != nil {
		fmt.Printf("⚠️ LanceDB: Download failed (%v), falling back to building from source...\n", err)
		return buildFromSource(projectRoot)
	}

	// Verify binaries were extracted successfully
	if !binariesExist(projectRoot, platform) {
		fmt.Printf("⚠️ LanceDB: Download incomplete, falling back to building from source...\n")
		return buildFromSource(projectRoot)
	}

	fmt.Printf("✅ LanceDB: Native binaries ready for %s_%s\n", platform.OS, platform.Arch)
	return nil
}

// binariesExist checks if the required binary files exist for the current platform
func binariesExist(projectRoot string, platform PlatformInfo) bool {
	libDir := filepath.Join(projectRoot, "lib", platform.Dir)

	var requiredFiles []string
	switch platform.OS {
	case "darwin", "linux":
		requiredFiles = []string{"liblancedb_go.a"}
	case "windows":
		requiredFiles = []string{"lancedb_go.lib"}
	default:
		return false
	}

	for _, file := range requiredFiles {
		filePath := filepath.Join(libDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return false
		}
	}

	// Also check for header file
	headerPath := filepath.Join(projectRoot, "include", "lancedb.h")
	if _, err := os.Stat(headerPath); os.IsNotExist(err) {
		return false
	}

	return true
}

// downloadAndExtractBinaries downloads the binary archive and extracts it
func downloadAndExtractBinaries(projectRoot string) error {
	version := getVersion()
	url := getReleaseURL(version)

	// Download the archive
	// #nosec G107 - URL is constructed from trusted GitHub release template and version
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download binaries: HTTP %d from %s", resp.StatusCode, url)
	}

	// Extract the tar.gz archive directly to the project root
	if err := extractTarGz(resp.Body, projectRoot); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	return nil
}

// extractTarGz extracts a tar.gz archive to the specified directory
func extractTarGz(r io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	const maxFileSize = 500 * 1024 * 1024 // 500MB limit per file

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Validate file size to prevent decompression bombs
		if header.Size > maxFileSize {
			return fmt.Errorf("file too large: %s (%d bytes)", header.Name, header.Size)
		}

		// Construct the full path
		// #nosec G305 - Path traversal protection implemented below with validation
		path := filepath.Join(destDir, header.Name)

		// Ensure the path is within destDir (security check against path traversal)
		cleanDestDir := filepath.Clean(destDir)
		cleanPath := filepath.Clean(path)
		if !strings.HasPrefix(cleanPath, cleanDestDir+string(os.PathSeparator)) && cleanPath != cleanDestDir {
			return fmt.Errorf("invalid file path (potential path traversal): %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			// Create the directory if it doesn't exist
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}

			// Create the file
			file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// Use limited reader to prevent decompression bombs
			// #nosec G110 - Size limit implemented above to prevent decompression bombs
			limitedReader := io.LimitReader(tr, maxFileSize)
			if _, err := io.Copy(file, limitedReader); err != nil {
				file.Close()
				return err
			}
			file.Close()
		}
	}

	return nil
}

// getVersion returns the version to download, from environment or default
func getVersion() string {
	if version := os.Getenv(VersionEnv); version != "" {
		return version
	}
	return DefaultVersion
}

// getReleaseURL returns the URL to download binaries from
func getReleaseURL(version string) string {
	if url := os.Getenv(ReleaseURLEnv); url != "" {
		return url
	}
	return fmt.Sprintf(DefaultReleaseURLTemplate, version)
}

// buildFromSource builds the native libraries from source using make build-native
func buildFromSource(projectRoot string) error {
	fmt.Printf("🔨 LanceDB: Building native libraries from source...\n")
	fmt.Printf("📋 This may take several minutes on first run\n")

	// Check if we can run make
	if _, err := exec.LookPath("make"); err != nil {
		return fmt.Errorf("make not found - please install build tools or download pre-built binaries manually")
	}

	// Run make build-native
	cmd := exec.Command("make", "build-native")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build from source: %w\nPlease ensure you have Rust, cbindgen, and build tools installed", err)
	}

	// Verify the build was successful
	platform := GetPlatformInfo()
	if !binariesExist(projectRoot, platform) {
		return fmt.Errorf("build completed but binaries not found - this may indicate a build system issue")
	}

	fmt.Printf("✅ LanceDB: Successfully built native libraries from source for %s_%s\n", platform.OS, platform.Arch)
	return nil
}
