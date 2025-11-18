package nanochat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// PythonEngine manages a Python inference server subprocess
type PythonEngine struct {
	cmd       *exec.Cmd
	process   *os.Process
	url       string
	client    *http.Client
	logFile   *os.File
	mu        sync.Mutex
	isRunning bool
	pythonBin string
	modelDir  string
}

// ChatMessage represents a single chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents a chat completion request (OpenAI-compatible)
type ChatCompletionRequest struct {
	Messages    []ChatMessage `json:"messages"`
	Temperature *float32      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	TopK        *int          `json:"top_k,omitempty"`
}

// CompletionToken represents a single token from streaming completion
type CompletionToken struct {
	Token string `json:"token"`
	Done  bool   `json:"done"`
	Error string `json:"error,omitempty"`
}

// NewPythonEngine creates a new Python engine manager
func NewPythonEngine(modelDir string) *PythonEngine {
	// Find Python executable
	pythonBin := "python3"
	if err := exec.Command(pythonBin, "--version").Run(); err != nil {
		pythonBin = "python"
	}

	return &PythonEngine{
		url:       "http://127.0.0.1:8081",
		client:    &http.Client{Timeout: 30 * time.Second},
		isRunning: false,
		pythonBin: pythonBin,
		modelDir:  modelDir,
	}
}

// Start launches the Python inference server subprocess
func (pe *PythonEngine) Start(ctx context.Context, logPath string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if pe.isRunning {
		return fmt.Errorf("python engine already running")
	}

	log.Printf("[PythonEngine] Starting inference server on %s", pe.url)

	// Prepare inference server script path
	scriptPath := os.Getenv("INFERENCE_SERVER_PATH")
	if scriptPath == "" {
		// Try common locations
		candidates := []string{
			"./cmd/nanochat/inference_server.py",
			"../cmd/nanochat/inference_server.py",
			"/app/inference_server.py",
		}
		for _, path := range candidates {
			if _, err := os.Stat(path); err == nil {
				scriptPath = path
				break
			}
		}
	}

	if scriptPath == "" {
		return fmt.Errorf("inference_server.py not found; set INFERENCE_SERVER_PATH or place in cmd/nanochat/")
	}

	// Setup log file
	var err error
	if logPath != "" {
		pe.logFile, err = os.Create(logPath)
		if err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}
	}

	// Build command
	pe.cmd = exec.CommandContext(ctx,
		pe.pythonBin,
		scriptPath,
		"--port", "8081",
		"--host", "127.0.0.1",
		"--model-dir", pe.modelDir,
	)

	// Setup environment - add to PYTHONPATH
	env := os.Environ()
	nanochatDir := os.Getenv("NANOCHAT_DIR")
	if nanochatDir != "" {
		for i, e := range env {
			if strings.HasPrefix(e, "PYTHONPATH=") {
				env[i] = fmt.Sprintf("PYTHONPATH=%s:%s", nanochatDir, e[11:])
				break
			}
		}
		if !strings.HasPrefix(os.Getenv("PYTHONPATH"), nanochatDir) {
			env = append(env, fmt.Sprintf("PYTHONPATH=%s", nanochatDir))
		}
	}
	pe.cmd.Env = env

	// Setup output
	if pe.logFile != nil {
		pe.cmd.Stdout = pe.logFile
		pe.cmd.Stderr = pe.logFile
	} else {
		pe.cmd.Stdout = os.Stdout
		pe.cmd.Stderr = os.Stderr
	}

	// Start process
	if err := pe.cmd.Start(); err != nil {
		if pe.logFile != nil {
			pe.logFile.Close()
		}
		return fmt.Errorf("failed to start python server: %w", err)
	}

	pe.process = pe.cmd.Process
	pe.isRunning = true

	log.Printf("[PythonEngine] Python process started (PID: %d)", pe.process.Pid)

	// Wait for server to be ready
	if err := pe.waitHealthy(ctx, 30*time.Second); err != nil {
		pe.isRunning = false
		pe.process.Kill()
		if pe.logFile != nil {
			pe.logFile.Close()
		}
		return fmt.Errorf("python server failed to become ready: %w", err)
	}

	log.Printf("[PythonEngine] Server ready at %s", pe.url)
	return nil
}

// Stop gracefully shuts down the Python server
func (pe *PythonEngine) Stop() error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if !pe.isRunning || pe.process == nil {
		return nil
	}

	log.Printf("[PythonEngine] Stopping server (PID: %d)", pe.process.Pid)

	// Try graceful shutdown first (SIGTERM)
	if err := pe.process.Signal(os.Interrupt); err != nil {
		log.Printf("[PythonEngine] SIGTERM failed: %v, force killing", err)
		_ = pe.process.Kill()
	}

	// Wait for process to exit
	if pe.cmd != nil {
		if err := pe.cmd.Wait(); err != nil && err.Error() != "signal: interrupt" {
			log.Printf("[PythonEngine] Process wait error: %v", err)
		}
	}

	pe.isRunning = false

	// Close log file
	if pe.logFile != nil {
		pe.logFile.Close()
		pe.logFile = nil
	}

	log.Printf("[PythonEngine] Server stopped")
	return nil
}

// Health checks if the server is responding
func (pe *PythonEngine) Health(ctx context.Context) error {
	resp, err := pe.client.Get(pe.url + "/health")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %d", resp.StatusCode)
	}

	return nil
}

// Chat sends a completion request and streams tokens back
func (pe *PythonEngine) Chat(ctx context.Context, req *ChatCompletionRequest) (<-chan CompletionToken, error) {
	// Validate request
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("no messages in request")
	}

	// Create output channel
	tokens := make(chan CompletionToken, 10)

	// Encode request to JSON
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		pe.url+"/chat/completions",
		strings.NewReader(string(payload)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request in background
	go func() {
		defer close(tokens)

		resp, err := pe.client.Do(httpReq)
		if err != nil {
			tokens <- CompletionToken{Error: fmt.Sprintf("request failed: %v", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			tokens <- CompletionToken{Error: fmt.Sprintf("server returned %d", resp.StatusCode)}
			return
		}

		// Read streaming response
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and "data: " prefix
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// Parse JSON token
			var token CompletionToken
			if err := json.Unmarshal([]byte(data), &token); err != nil {
				log.Printf("[PythonEngine] Failed to parse token: %v", err)
				continue
			}

			tokens <- token

			// Stop if done
			if token.Done {
				break
			}
		}

		if err := scanner.Err(); err != nil {
			tokens <- CompletionToken{Error: fmt.Sprintf("read error: %v", err)}
		}
	}()

	return tokens, nil
}

// waitHealthy polls /health endpoint until server is ready
func (pe *PythonEngine) waitHealthy(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("server not ready after %v", timeout)
		case <-ticker.C:
			if err := pe.Health(ctx); err == nil {
				return nil
			}
		}
	}
}

// IsRunning returns whether the engine is currently running
func (pe *PythonEngine) IsRunning() bool {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	return pe.isRunning
}

// URL returns the server URL
func (pe *PythonEngine) URL() string {
	return pe.url
}

// PID returns the process ID (for debugging)
func (pe *PythonEngine) PID() int {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	if pe.process == nil {
		return 0
	}
	return pe.process.Pid
}

// StreamResponse represents a streaming response handler
type StreamResponse struct {
	channel <-chan CompletionToken
}

// Read returns the next token or error
func (sr *StreamResponse) Read(ctx context.Context) (CompletionToken, error) {
	select {
	case token, ok := <-sr.channel:
		if !ok {
			return CompletionToken{}, io.EOF
		}
		return token, nil
	case <-ctx.Done():
		return CompletionToken{}, ctx.Err()
	}
}

// CollectTokens reads all tokens and concatenates them into a single response
func (sr *StreamResponse) CollectTokens(ctx context.Context) (string, error) {
	var result strings.Builder

	for {
		token, err := sr.Read(ctx)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		if token.Error != "" {
			return "", fmt.Errorf("token error: %s", token.Error)
		}

		result.WriteString(token.Token)
	}

	return result.String(), nil
}
