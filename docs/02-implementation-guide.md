# NanoChat Inference Implementation Guide

## Overview

This guide details the step-by-step implementation of direct PyTorch-based nanochat inference as specified in ADR 0003.

---

## Phase 1: Python Inference Server

### Objective
Create a FastAPI server that:
1. Loads nanochat model from PyTorch checkpoint files
2. Accepts OpenAI-compatible chat completion requests
3. Streams tokens back as Server-Sent Events
4. Auto-detects GPU/CPU availability

### Files to Implement

#### File: `cmd/nanochat/inference_server.py` ✅ (Template Created)

**Status:** Skeleton created, needs completion

**What's Done:**
- FastAPI application structure
- Health check endpoints
- Data models for request/response
- Device auto-detection logic
- Error handling framework

**What Remains:**
1. **`load_nanochat_model()` function**
   - Clone/import nanochat repository modules
   - Load PyTorch model checkpoint (`model_000650.pt`)
   - Load tokenizer (`tokenizer.pkl`)
   - Load config (`meta_000650.json`)
   - Return model, tokenizer, and config objects

2. **`NanoChatInference.stream_completion()` method**
   - Build conversation token sequence from messages
   - Use nanochat.Engine.generate() for token generation
   - Yield tokens as Server-Sent Events JSON
   - Handle special tokens: `<|user_start|>`, `<|assistant_start|>`, etc.
   - Implement top-k sampling with temperature control

3. **Token encoding/decoding**
   - Convert chat messages → token IDs using tokenizer
   - Convert token IDs → text for streaming responses
   - Handle multi-byte UTF-8 characters (emojis, etc.)

### Implementation Steps

#### Step 1: Set Up NanoChat Dependency

**Create file:** `scripts/setup-nanochat.sh`

```bash
#!/bin/bash
# Setup nanochat dependencies in the project

set -e

NANOCHAT_DIR="${1:-.nanochat}"

echo "Cloning nanochat repository..."
git clone https://github.com/karpathy/nanochat.git "$NANOCHAT_DIR"

echo "Installing nanochat in editable mode..."
pip install -e "$NANOCHAT_DIR"

echo "✓ NanoChat setup complete"
echo "Add to PYTHONPATH:"
echo "  export PYTHONPATH=$PYTHONPATH:$(pwd)/$NANOCHAT_DIR"
```

**Then add to main setup script:**
```bash
# Install nanochat inference dependencies
pip install -r requirements-nanochat.txt
./scripts/setup-nanochat.sh
```

#### Step 2: Implement Model Loading

**In `cmd/nanochat/inference_server.py`, replace the `load_nanochat_model()` stub:**

```python
def load_nanochat_model(model_dir: Path, device: torch.device):
    """Load nanochat model and tokenizer."""
    
    try:
        # Import nanochat modules
        from nanochat.checkpoint_manager import load_model
        from nanochat.engine import Engine
    except ImportError as e:
        raise ImportError(
            "nanochat not found in PYTHONPATH. "
            "Run: ./scripts/setup-nanochat.sh"
        ) from e
    
    model_dir = Path(model_dir)
    
    # Verify required files exist
    required_files = [
        "model_000650.pt",
        "meta_000650.json",
        "tokenizer.pkl"
    ]
    for filename in required_files:
        if not (model_dir / filename).exists():
            raise FileNotFoundError(f"Missing: {filename}")
    
    # Load model checkpoint
    logger.info(f"Loading model from {model_dir}")
    model, tokenizer, meta = load_model("sft", device, phase="eval")
    
    logger.info(f"Model loaded: {meta.get('config', {})}")
    
    return model, tokenizer, meta
```

#### Step 3: Implement Token Streaming

**In `NanoChatInference.stream_completion()`, replace placeholder:**

```python
async def stream_completion(
    self,
    messages: List[ChatMessage],
    temperature: float = 0.7,
    max_tokens: int = 512,
    top_k: int = 50
) -> AsyncGenerator[str, None]:
    """Stream chat completion tokens."""
    
    try:
        # Build conversation tokens
        conversation_tokens = self._build_conversation_tokens(messages)
        
        # Initialize engine
        from nanochat.engine import Engine
        engine = Engine(self.model, self.tokenizer)
        
        # Generate and stream tokens
        token_count = 0
        with self.autocast_ctx:
            for token_column, token_masks in engine.generate(
                conversation_tokens,
                num_samples=1,
                max_tokens=max_tokens,
                temperature=temperature,
                top_k=top_k,
                seed=42
            ):
                token_id = token_column[0]
                
                # Decode and emit token
                token_text = self.tokenizer.decode([token_id])
                
                yield f"data: {json.dumps({'token': token_text})}\n\n"
                
                token_count += 1
                
                # Prevent runaway generation
                if token_count >= max_tokens:
                    break
        
        yield "data: {\"done\": true}\n\n"
    
    except Exception as e:
        logger.error(f"Generation error: {e}")
        yield f"data: {json.dumps({'error': str(e)})}\n\n"

def _build_conversation_tokens(self, messages: List[ChatMessage]) -> List[int]:
    """Convert chat messages to token sequence."""
    
    # Special tokens from nanochat
    bos = self.tokenizer.get_bos_token_id()
    user_start = self.tokenizer.encode_special("<|user_start|>")
    user_end = self.tokenizer.encode_special("<|user_end|>")
    assistant_start = self.tokenizer.encode_special("<|assistant_start|>")
    assistant_end = self.tokenizer.encode_special("<|assistant_end|>")
    
    tokens = [bos]
    
    for msg in messages:
        if msg.role == "user":
            tokens.extend(user_start)
            tokens.extend(self.tokenizer.encode(msg.content))
            tokens.extend(user_end)
        elif msg.role == "assistant":
            tokens.extend(assistant_start)
            tokens.extend(self.tokenizer.encode(msg.content))
            tokens.extend(assistant_end)
    
    # Start generation from assistant
    tokens.extend(assistant_start)
    
    return tokens
```

#### Step 4: Testing

**Create file:** `tests/test_inference_server.py`

```python
import pytest
import asyncio
from cmd.nanochat.inference_server import NanoChatInference, ChatMessage

@pytest.mark.asyncio
async def test_stream_completion():
    """Test token streaming."""
    messages = [
        ChatMessage(role="user", content="Hello, how are you?")
    ]
    
    # Mock model and tokenizer (will be real in integration tests)
    tokens = []
    async for chunk in inference_engine.stream_completion(
        messages,
        temperature=0.7,
        max_tokens=10
    ):
        tokens.append(chunk)
    
    assert len(tokens) > 0
    assert any("done" in t for t in tokens)

@pytest.mark.asyncio
async def test_health_check(client):
    """Test /health endpoint."""
    response = await client.get("/health")
    assert response.status_code == 200
    assert response.json()["ready"] is True
```

---

## Phase 2: Go Subprocess Wrapper

### Files to Create

#### File: `internal/nanochat/python_engine.go`

```go
package nanochat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PythonEngine manages the Python inference subprocess
type PythonEngine struct {
	cmd     *exec.Cmd
	url     string // e.g., "http://127.0.0.1:8081"
	client  *http.Client
	process *os.Process
}

// Start launches the Python inference server
func (pe *PythonEngine) Start(ctx context.Context, modelDir string) error {
	// Determine Python path
	pythonBin := "python3"
	if err := exec.Command(pythonBin, "--version").Run(); err != nil {
		pythonBin = "python"
	}

	// Build command
	pythonScript := filepath.Join(os.Getenv("GOBIN"), "../cmd/nanochat/inference_server.py")
	pe.cmd = exec.CommandContext(ctx,
		pythonBin,
		"-m", "cmd.nanochat.inference_server",
		"--port", "8081",
		"--model-dir", modelDir,
	)

	// Setup output
	pe.cmd.Stdout = os.Stdout
	pe.cmd.Stderr = os.Stderr
	pe.cmd.Env = os.Environ()

	// Start process
	if err := pe.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start python server: %w", err)
	}

	pe.process = pe.cmd.Process
	pe.url = "http://127.0.0.1:8081"
	pe.client = &http.Client{Timeout: 30 * time.Second}

	// Wait for readiness
	return pe.waitHealthy(30 * time.Second)
}

// Stop gracefully shuts down the Python server
func (pe *PythonEngine) Stop() error {
	if pe.process == nil {
		return nil
	}

	// Send SIGTERM
	if err := pe.process.Signal(os.Interrupt); err != nil {
		// Force kill if SIGTERM fails
		_ = pe.process.Kill()
	}

	// Wait for process to exit
	pe.cmd.Wait()
	return nil
}

// Chat sends a completion request and returns streaming response
func (pe *PythonEngine) Chat(ctx context.Context, req *OpenAIChatRequest) (io.ReadCloser, error) {
	// Convert to Python request format
	pythonReq := map[string]interface{}{
		"messages":   req.Messages,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"top_k":       req.TopK,
	}

	payload, err := json.Marshal(pythonReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		pe.url+"/chat/completions",
		strings.NewReader(string(payload)),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := pe.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("inference server returned %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// waitHealthy polls /health endpoint until ready
func (pe *PythonEngine) waitHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := pe.client.Get(pe.url + "/health")
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("python server not ready after %v", timeout)
}
```

---

## Phase 3: Model Manager

### Files to Create

#### File: `internal/nanochat/model_manager.go`

```go
package nanochat

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	modelURL = "https://huggingface.co/sdobson/nanochat/resolve/main/"
	modelFile = "model_000650.pt"
	metaFile = "meta_000650.json"
	tokenizerFile = "tokenizer.pkl"
	tokenBytesFile = "token_bytes.pt"
)

// EnsureModel ensures nanochat model files are available
func EnsureModel(cacheDir string) error {
	// Check if all files exist
	if modelExists(cacheDir) {
		fmt.Printf("✓ Model already cached: %s\n", cacheDir)
		return nil
	}

	fmt.Printf("↓ Downloading nanochat model files...\n")

	files := []string{modelFile, metaFile, tokenizerFile, tokenBytesFile}
	for _, file := range files {
		if err := downloadFile(modelURL+file, filepath.Join(cacheDir, file)); err != nil {
			return fmt.Errorf("failed to download %s: %w", file, err)
		}
	}

	fmt.Printf("✓ Model downloaded successfully\n")
	return nil
}

func modelExists(cacheDir string) bool {
	requiredFiles := []string{modelFile, metaFile, tokenizerFile}
	for _, file := range requiredFiles {
		if _, err := os.Stat(filepath.Join(cacheDir, file)); err != nil {
			return false
		}
	}
	return true
}

func downloadFile(url, dest string) error {
	os.MkdirAll(filepath.Dir(dest), 0755)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
```

---

## Phase 4: Docker Integration

### Files to Update

#### File: `Dockerfile` (Multi-stage with Python + PyTorch)

```dockerfile
# Stage 1: Build Go binary
FROM golang:1.22-alpine AS build
RUN apk add --no-cache git
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG BINARY=server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /workspace/${BINARY} ./cmd/${BINARY}

# Stage 2: Python runtime with PyTorch
FROM python:3.11-slim
ARG BINARY=server
ARG BAKED=false

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    && rm -rf /var/lib/apt/lists/*

# Install PyTorch
RUN pip install --no-cache-dir torch>=2.0.0 torchvision torchaudio --index-url https://download.pytorch.org/whl/cpu

# Install FastAPI dependencies
RUN pip install --no-cache-dir \
    fastapi>=0.104.0 \
    uvicorn>=0.24.0 \
    pydantic>=2.0.0

# Clone nanochat
RUN git clone https://github.com/karpathy/nanochat.git /opt/nanochat && \
    pip install --no-cache-dir -e /opt/nanochat

# Copy Go binary and Python inference server
COPY --from=build /workspace/${BINARY} /app/server
COPY cmd/nanochat/inference_server.py /app/inference_server.py

# Optional: Bake model into image
RUN if [ "${BAKED}" = "true" ]; then \
    mkdir -p /root/.cache/openai-api-simulator/nanochat && \
    python -c "from internal.nanochat.model_manager import EnsureModel; EnsureModel('/root/.cache/openai-api-simulator/nanochat')" ; \
fi

WORKDIR /app
EXPOSE 3080
ENTRYPOINT ["/app/server"]
CMD ["-port", "3080"]
```

---

## Testing Checklist

- [ ] Model loading works (unit test with mock model)
- [ ] Token streaming produces valid JSON
- [ ] Device auto-detection works (cuda/mps/cpu)
- [ ] Subprocess startup/shutdown clean
- [ ] Health check polling works
- [ ] Chat completion request/response format correct
- [ ] Streaming response parses correctly in Go
- [ ] Docker image builds successfully
- [ ] Baked image contains model files
- [ ] First startup with download works
- [ ] Performance: <1 token/sec on GPU, <5 tokens/sec on CPU

---

## Debugging Commands

```bash
# Test inference server directly
python -m cmd.nanochat.inference_server --port 8081 --model-dir ~/.cache/openai-api-simulator/nanochat

# Test health endpoint
curl http://localhost:8081/health

# Test chat completion
curl -X POST http://localhost:8081/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "Hello"}]}'

# Check model files
ls -lh ~/.cache/openai-api-simulator/nanochat/

# Docker build with baking
docker build --build-arg BINARY=nanochat --build-arg BAKED=true \
  -t openai-api-simulator:nanochat-pytorch .
```

---

## Success Criteria

✅ Model loads and initializes without errors
✅ Tokens stream in real-time (no buffering)
✅ OpenAI API format fully compatible
✅ GPU acceleration works on CUDA/MPS systems
✅ CPU fallback works for other systems
✅ First startup: <2 min (model download + server start)
✅ Cached startup: <10 seconds
✅ Docker images build and run correctly
✅ All tests pass
✅ Performance meets latency targets

---

## References

- **NanoChat Repo:** https://github.com/karpathy/nanochat
- **Model:** https://huggingface.co/sdobson/nanochat
- **FastAPI Docs:** https://fastapi.tiangolo.com/
- **PyTorch Inference:** https://pytorch.org/docs/stable/inference_mode.html
- **Server-Sent Events:** https://html.spec.whatwg.org/multipage/server-sent-events.html
