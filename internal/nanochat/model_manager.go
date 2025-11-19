package nanochat

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// Hugging Face model repository for nanochat
	huggingFaceURL = "https://huggingface.co/sdobson/nanochat/resolve/main"

	// SmolLM model name
	smollmModelName = "HuggingFaceTB/SmolLM-135M"

	// Model files to download (nanochat PyTorch checkpoint)
	nanoModelFile      = "model_000650.pt"
	nanoMetaFile       = "meta_000650.json"
	nanoTokenizerFile  = "tokenizer.pkl"
	nanoTokenBytesFile = "token_bytes.pt"
)

// ModelManager handles downloading and caching nanochat model files
type ModelManager struct {
	cacheDir string
	client   *http.Client
	mu       sync.Mutex
}

// NewModelManager creates a new model manager with the given cache directory
func NewModelManager(cacheDir string) *ModelManager {
	return &ModelManager{
		cacheDir: cacheDir,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// EnsureModel ensures all required model files are present in cache
func (mm *ModelManager) EnsureModel() error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(mm.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	log.Printf("[ModelManager] Cache directory: %s", mm.cacheDir)

	// Check which files need to be downloaded
	requiredFiles := []string{nanoModelFile, nanoMetaFile, nanoTokenizerFile}
	filesToDownload := []string{}

	for _, file := range requiredFiles {
		path := filepath.Join(mm.cacheDir, file)
		if _, err := os.Stat(path); err != nil {
			filesToDownload = append(filesToDownload, file)
		} else {
			log.Printf("[ModelManager] ✓ Found cached: %s", file)
		}
	}

	// Download missing files
	if len(filesToDownload) == 0 {
		log.Printf("[ModelManager] ✓ All model files present in cache")
		return nil
	}

	log.Printf("[ModelManager] Downloading %d missing file(s)...", len(filesToDownload))

	for _, file := range filesToDownload {
		if err := mm.downloadFile(file); err != nil {
			return fmt.Errorf("failed to download %s: %w", file, err)
		}
	}

	log.Printf("[ModelManager] ✓ Model downloaded successfully")
	return nil
}

// EnsureModelAsync downloads model files asynchronously and returns a channel
// that signals completion or error
func (mm *ModelManager) EnsureModelAsync() <-chan error {
	result := make(chan error, 1)

	go func() {
		result <- mm.EnsureModel()
	}()

	return result
}

// ModelPath returns the path to the model cache directory
func (mm *ModelManager) ModelPath() string {
	return mm.cacheDir
}

// ModelExists checks if all required model files exist
func (mm *ModelManager) ModelExists() bool {
	requiredFiles := []string{nanoModelFile, nanoMetaFile, nanoTokenizerFile}

	for _, file := range requiredFiles {
		path := filepath.Join(mm.cacheDir, file)
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}

	return true
}

// CacheSize returns the total size of cached model files in bytes
func (mm *ModelManager) CacheSize() (int64, error) {
	var totalSize int64

	files := []string{nanoModelFile, nanoMetaFile, nanoTokenizerFile, nanoTokenBytesFile}
	for _, file := range files {
		path := filepath.Join(mm.cacheDir, file)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, err
		}
		totalSize += info.Size()
	}

	return totalSize, nil
}

// downloadFile downloads a single file from Hugging Face
func (mm *ModelManager) downloadFile(filename string) error {
	url := fmt.Sprintf("%s/%s", huggingFaceURL, filename)
	path := filepath.Join(mm.cacheDir, filename)

	log.Printf("[ModelManager] Downloading: %s", filename)

	// Create HTTP request with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := mm.client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}

	// Create output file
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Copy with progress tracking
	return mm.copyWithProgress(out, resp.Body, filename, resp.ContentLength)
}

// copyWithProgress copies data and logs progress
func (mm *ModelManager) copyWithProgress(dst io.Writer, src io.Reader, filename string, totalSize int64) error {
	const chunkSize = 1024 * 1024 // 1MB

	buf := make([]byte, chunkSize)
	var written int64

	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return fmt.Errorf("write error: %w", err)
			}
			written += int64(n)

			// Log progress every 10MB
			if totalSize > 0 && written%chunkSize == 0 {
				percent := (written * 100) / totalSize
				log.Printf("[ModelManager] %s: %d%%", filename, percent)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read error: %w", err)
		}
	}

	log.Printf("[ModelManager] ✓ Downloaded: %s (%d bytes)", filename, written)
	return nil
}

// VerifyIntegrity checks if downloaded files match expected properties
// (basic size checks without checksums)
func (mm *ModelManager) VerifyIntegrity() error {
	minSizes := map[string]int64{
		nanoModelFile:      1000000000, // ~1GB minimum for model
		nanoMetaFile:       100,        // ~1KB for metadata
		nanoTokenizerFile:  100000,     // ~100KB for tokenizer
		nanoTokenBytesFile: 100000,     // ~100KB for token bytes (optional)
	}

	for file, minSize := range minSizes {
		path := filepath.Join(mm.cacheDir, file)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) && file == nanoTokenBytesFile {
				// token_bytes.pt is optional
				continue
			}
			return fmt.Errorf("%s missing or unreadable: %w", file, err)
		}

		if info.Size() < minSize {
			return fmt.Errorf("%s seems truncated (size: %d)", file, info.Size())
		}
	}

	return nil
}

// Clean removes cached model files
func (mm *ModelManager) Clean() error {
	files := []string{nanoModelFile, nanoMetaFile, nanoTokenizerFile, nanoTokenBytesFile}

	for _, file := range files {
		path := filepath.Join(mm.cacheDir, file)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", file, err)
		}
	}

	log.Printf("[ModelManager] Cleaned cache directory")
	return nil
}
