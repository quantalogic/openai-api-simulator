package nanochat

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

// Run is the main entry point for the nanochat command
func Run(publicPort, llamaPort int) error {
	// Print platform info
	platformName := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		platformName += " (Apple Silicon)"
	}
	fmt.Printf("✓ Platform: %s\n", platformName)

	// Setup cache directory
	cacheDir := filepath.Join(os.Getenv("HOME"), cacheRoot)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Ensure llama-server and model are available
	serverPath := ensureLlamaServer(cacheDir)
	modelPath := ensureModel(cacheDir)

	// Start llama.cpp server
	fmt.Printf("→ Starting llama.cpp server on 127.0.0.1:%d", llamaPort)
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		fmt.Printf(" (Metal full GPU offload)")
	}
	fmt.Println()

	llamaCmd := exec.Command(serverPath,
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", llamaPort),
		"--model", modelPath,
		"--ctx-size", "4096",
		"--temp", "0.7",
		"--n-gpu-layers", gpuLayers(),
		"--threads", fmt.Sprintf("%d", runtime.NumCPU()),
	)

	// Capture llama-server output
	llamaCmd.Stdout = os.Stdout
	llamaCmd.Stderr = os.Stderr

	if err := llamaCmd.Start(); err != nil {
		return fmt.Errorf("failed to start llama.cpp server: %w", err)
	}

	// Ensure llama-server is killed on exit
	defer func() {
		if llamaCmd.Process != nil {
			llamaCmd.Process.Kill()
		}
	}()

	// Wait for llama.cpp to become healthy
	fmt.Printf("→ Waiting for llama.cpp server to be ready...\n")
	if !waitHealthy(llamaPort, 45*time.Second) {
		return fmt.Errorf("llama.cpp server failed to become healthy within 45 seconds")
	}
	fmt.Printf("✓ Health check passed\n")

	// Start the OpenAI API proxy
	fmt.Printf("→ Starting OpenAI API proxy on :%d\n", publicPort)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	proxyCmd := exec.Command(exe,
		"--nanochat-enabled",
		"--nanochat-upstream-url", fmt.Sprintf("http://127.0.0.1:%d", llamaPort),
		"--port", fmt.Sprintf("%d", publicPort),
	)

	proxyCmd.Stdout = os.Stdout
	proxyCmd.Stderr = os.Stderr

	if err := proxyCmd.Start(); err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
	}

	// Wait a moment for proxy to start
	time.Sleep(1 * time.Second)

	// Print ready message
	fmt.Println()
	fmt.Println("Ready! Available models:")
	fmt.Println("  • gpt-sim-1     (fake, deterministic)")
	fmt.Println("  • nanochat      (real 561M – sdobson/nanochat)")
	fmt.Println("  • smollm        (real 135M – HuggingFaceTB/SmolLM-135M)")
	fmt.Println()
	fmt.Println("Test real inference:")
	fmt.Printf("curl http://localhost:%d/v1/chat/completions \\\n", publicPort)
	fmt.Println("  -d '{\"model\":\"nanochat\",\"messages\":[{\"role\":\"user\",\"content\":\"Why is the sky blue?\"}]}'")
	fmt.Println("  -d '{\"model\":\"smollm\",\"messages\":[{\"role\":\"user\",\"content\":\"Why is the sky blue?\"}]}'")
	fmt.Println()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	<-sigChan
	fmt.Println("\n→ Shutting down...")

	// Kill both processes
	if proxyCmd.Process != nil {
		proxyCmd.Process.Kill()
	}
	if llamaCmd.Process != nil {
		llamaCmd.Process.Kill()
	}

	return nil
}
