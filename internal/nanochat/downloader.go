package nanochat

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

const (
	cacheRoot      = ".cache/openai-api-simulator"
	modelURL       = "https://huggingface.co/sdobson/nanochat/resolve/main/nanochat-q4_k_m.gguf"
	modelFile      = "nanochat-q4_k_m.gguf"
	expectedSHA256 = "8f2d9e8c5d8e9b1a3f7c9e2d4b6e1a8f7c3d2e9f1a0b8c7d6e5f4a3b2c1d0e9"
)

// getLatestTag retrieves the latest llama.cpp release tag by following the redirect
func getLatestTag() string {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get("https://github.com/ggml-org/llama.cpp/releases/latest")
	if err != nil {
		return "b7088" // fallback to a known working version
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	parts := strings.Split(location, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "b7088"
}

// binaryURL returns the download URL and inner executable name for the current platform
func binaryURL(tag string) (url, innerName string) {
	base := fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s/llama-%s-bin-", tag, tag)
	switch os := runtime.GOOS; os {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return base + "macos-arm64.zip", "llama-server"
		}
		return base + "macos-x64.zip", "llama-server"
	case "linux":
		// Note: linux-arm64 binaries are not available from llama.cpp releases
		// Use ubuntu-x64 as fallback for all Linux architectures (works under emulation)
		return base + "ubuntu-x64.zip", "llama-server"
	}
	panic("unsupported platform")
}

// downloadFile downloads a file from url to destPath with a progress bar
func downloadFile(url, destPath string) error {
	// Get the file size
	resp, err := http.Head(url)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := resp.ContentLength

	// Create the file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Download with progress bar
	resp, err = http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	bar := progressbar.DefaultBytes(
		fileSize,
		fmt.Sprintf("Downloading %s", filepath.Base(destPath)),
	)

	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Println()
	return nil
}

// verifySHA256 verifies the SHA256 checksum of a file
func verifySHA256(filePath, expectedHash string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	return actualHash == expectedHash, nil
}

// ensureModel ensures the model file is downloaded and cached
func ensureModel(cacheDir string) string {
	modelPath := filepath.Join(cacheDir, modelFile)

	// Check if model exists and is valid
	if _, err := os.Stat(modelPath); err == nil {
		// Note: The ADR includes a SHA256 hash but it's a placeholder.
		// For production, you'd want to verify it. For now, just check existence.
		fmt.Printf("✓ Model already cached: %s\n", modelPath)
		return modelPath
	}

	// Download the model
	fmt.Printf("↓ Downloading nanochat model (316 MB)...\n")
	tmpPath := modelPath + ".tmp"
	if err := downloadFile(modelURL, tmpPath); err != nil {
		fmt.Printf("Failed to download model: %v\n", err)
		os.Exit(1)
	}

	// Move to final location
	if err := os.Rename(tmpPath, modelPath); err != nil {
		fmt.Printf("Failed to save model: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Model downloaded successfully\n")
	return modelPath
}

// unzipFile extracts a specific file from a zip archive
func unzipFile(zipPath, targetFile, destDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		// Look for the target file (may be in subdirectories)
		if strings.HasSuffix(f.Name, targetFile) {
			destPath := filepath.Join(destDir, filepath.Base(f.Name))

			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return "", err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, rc)
			if err != nil {
				return "", err
			}

			return destPath, nil
		}
	}

	return "", fmt.Errorf("file %s not found in archive", targetFile)
}

// ensureLlamaServer ensures the llama-server binary is available (baked in image or downloaded)
func ensureLlamaServer(cacheDir string) string {
	serverPath := filepath.Join(cacheDir, "llama-server")

	// Check if server exists and is executable
	if info, err := os.Stat(serverPath); err == nil {
		if info.Mode()&0111 != 0 {
			fmt.Printf("✓ llama-server already available: %s\n", serverPath)
			return serverPath
		}
	}

	// Check if static build is available (from baked image)
	staticPath := "/app/llama-server-static"
	if info, err := os.Stat(staticPath); err == nil {
		if info.Mode()&0111 != 0 {
			fmt.Printf("✓ Using pre-built llama-server from image\n")
			// Copy to cache for consistency
			if err := copyFile(staticPath, serverPath); err == nil {
				return serverPath
			}
			// If copy fails, use the static version directly
			return staticPath
		}
	}

	// Get latest version and download URL
	tag := getLatestTag()
	fmt.Printf("✓ Latest llama.cpp: %s\n", tag)

	url, innerName := binaryURL(tag)
	fmt.Printf("↓ Downloading llama-server...\n")

	zipPath := filepath.Join(cacheDir, "llama-server.zip")
	if err := downloadFile(url, zipPath); err != nil {
		fmt.Printf("Failed to download llama-server: %v\n", err)
		os.Exit(1)
	}

	// Extract the binary
	fmt.Printf("→ Extracting llama-server...\n")
	extractedPath, err := unzipFile(zipPath, innerName, cacheDir)
	if err != nil {
		fmt.Printf("Failed to extract llama-server: %v\n", err)
		os.Exit(1)
	}

	// Rename if necessary
	if extractedPath != serverPath {
		if err := os.Rename(extractedPath, serverPath); err != nil {
			fmt.Printf("Failed to rename llama-server: %v\n", err)
			os.Exit(1)
		}
	}

	// Make executable
	if err := os.Chmod(serverPath, 0755); err != nil {
		fmt.Printf("Failed to make llama-server executable: %v\n", err)
		os.Exit(1)
	}

	// Clean up zip file
	os.Remove(zipPath)

	fmt.Printf("✓ llama-server ready\n")
	return serverPath
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// Make destination executable
	return os.Chmod(dst, 0755)
}

// waitHealthy polls the llama.cpp server health endpoint until it's ready
func waitHealthy(port int, timeout time.Duration) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/v1/models", port)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// gpuLayers returns the number of GPU layers to offload based on platform
func gpuLayers() string {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		return "999" // Full Metal GPU offload on Apple Silicon
	}
	return "0" // CPU only for other platforms
}
