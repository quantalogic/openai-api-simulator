package nanochat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewModelManager(t *testing.T) {
	cacheDir := "/tmp/test-cache"
	mm := NewModelManager(cacheDir)

	if mm.ModelPath() != cacheDir {
		t.Errorf("Expected model path '%s', got '%s'", cacheDir, mm.ModelPath())
	}
}

func TestModelNotExistsInitially(t *testing.T) {
	cacheDir := t.TempDir()
	mm := NewModelManager(cacheDir)

	if mm.ModelExists() {
		t.Error("Model should not exist in empty cache directory")
	}
}

func TestModelExistsWhenFilesPresent(t *testing.T) {
	cacheDir := t.TempDir()
	mm := NewModelManager(cacheDir)

	// Create dummy files
	files := []string{nanoModelFile, nanoMetaFile, nanoTokenizerFile}
	for _, file := range files {
		path := filepath.Join(cacheDir, file)
		if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
			t.Fatalf("Failed to create dummy file: %v", err)
		}
	}

	if !mm.ModelExists() {
		t.Error("Model should exist when all required files are present")
	}
}

func TestCacheSize(t *testing.T) {
	cacheDir := t.TempDir()
	mm := NewModelManager(cacheDir)

	// Create dummy files with known sizes
	testData := []byte("test data")
	files := []string{nanoModelFile, nanoMetaFile, nanoTokenizerFile}
	expectedSize := int64(0)

	for _, file := range files {
		path := filepath.Join(cacheDir, file)
		if err := os.WriteFile(path, testData, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		expectedSize += int64(len(testData))
	}

	size, err := mm.CacheSize()
	if err != nil {
		t.Fatalf("Failed to get cache size: %v", err)
	}

	if size != expectedSize {
		t.Errorf("Expected cache size %d, got %d", expectedSize, size)
	}
}

func TestVerifyIntegrityMissingFiles(t *testing.T) {
	cacheDir := t.TempDir()
	mm := NewModelManager(cacheDir)

	// Don't create any files
	if err := mm.VerifyIntegrity(); err == nil {
		t.Error("VerifyIntegrity should fail when files are missing")
	}
}

func TestVerifyIntegritySmallFiles(t *testing.T) {
	cacheDir := t.TempDir()
	mm := NewModelManager(cacheDir)

	// Create files that are too small
	path := filepath.Join(cacheDir, nanoModelFile)
	if err := os.WriteFile(path, []byte("tiny"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create other required files with correct sizes
	largeDummy := make([]byte, 1000000)
	for _, file := range []string{nanoMetaFile, nanoTokenizerFile} {
		path := filepath.Join(cacheDir, file)
		if err := os.WriteFile(path, largeDummy, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	if err := mm.VerifyIntegrity(); err == nil {
		t.Error("VerifyIntegrity should fail when model file is too small")
	}
}

func TestClean(t *testing.T) {
	cacheDir := t.TempDir()
	mm := NewModelManager(cacheDir)

	// Create files
	files := []string{nanoModelFile, nanoMetaFile, nanoTokenizerFile, nanoTokenBytesFile}
	for _, file := range files {
		path := filepath.Join(cacheDir, file)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Verify files exist
	if !mm.ModelExists() {
		t.Error("Files should exist before cleaning")
	}

	// Clean
	if err := mm.Clean(); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Verify files are removed
	for _, file := range files {
		path := filepath.Join(cacheDir, file)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("File should be deleted: %s", file)
		} else if !os.IsNotExist(err) {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestHuggingFaceURLConstant(t *testing.T) {
	expected := "https://huggingface.co/sdobson/nanochat/resolve/main"
	if huggingFaceURL != expected {
		t.Errorf("Expected HF URL '%s', got '%s'", expected, huggingFaceURL)
	}
}

func TestModelFileConstant(t *testing.T) {
	if nanoModelFile != "model_000650.pt" {
		t.Errorf("Expected model file 'model_000650.pt', got '%s'", nanoModelFile)
	}
}

func TestTokenizerFileConstant(t *testing.T) {
	if nanoTokenizerFile != "tokenizer.pkl" {
		t.Errorf("Expected tokenizer file 'tokenizer.pkl', got '%s'", nanoTokenizerFile)
	}
}
