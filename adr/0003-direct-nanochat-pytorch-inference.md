# ADR 0003: Direct PyTorch-based NanoChat Inference

**Status:** PROPOSED

**Date:** November 18, 2025

**Authors:** Development Team

**Deciders:** Architecture Review Board

---

## 1. Problem Statement

### Current State

The project currently attempts to use nanochat with llama.cpp (a C++ inference engine):

1. **Model Format Mismatch:** nanochat is a PyTorch model, but we tried to use it with llama.cpp (which expects GGUF format)
2. **GGUF Conversion Missing:** No official GGUF quantization of nanochat exists publicly
3. **Architecture Incompatibility:** llama.cpp releases don't provide linux-arm64 pre-built binaries, requiring x86_64 fallback that runs under emulation
4. **Download Chain Failure:** Multiple external URLs fail (Hugging Face model URL returns 404, llama.cpp arm64 builds unavailable)
5. **Complexity:** Multi-stage Docker builds, binary extraction, and runtime fallbacks add significant complexity

### Why This Matters

- **User Experience:** First startup still takes 45+ seconds due to architecture detection and binary downloads/extraction
- **Reliability:** Depends on external URLs that may change (HuggingFace, GitHub releases)
- **Build Efficiency:** Docker baking doesn't fully work due to architecture mismatches
- **Maintainability:** Complex workarounds in downloader.go for non-existent binaries

---

## 2. Solution Overview

### Core Insight

**Use nanochat's native PyTorch inference directly** instead of converting to GGUF.

The nanochat repository ([karpathy/nanochat](https://github.com/karpathy/nanochat)) provides:
- **Engine**: Efficient token-by-token inference with KV cache optimization
- **Chat Web Server**: FastAPI-based chat completion API compatible with OpenAI format
- **Tokenizer**: BPE tokenizer for encoding/decoding
- **PyTorch Model**: Direct access to model.forward() for inference

### Architecture

```
User Request (OpenAI API format)
         ↓
┌─────────────────────────────────────┐
│  Go OpenAI API Proxy (:3080)        │
│  (existing server)                  │
└──────────────┬──────────────────────┘
               ↓ (model="nanochat")
┌─────────────────────────────────────┐
│  Python Subprocess or HTTP Proxy    │
│  - Bridges Go ↔ Python              │
│  - Translates OpenAI format         │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│  Python FastAPI Chat Server         │
│  (nanochat/scripts/chat_web.py)     │
│  - Uses nanochat.Engine             │
│  - KV cache optimization            │
│  - Streaming responses              │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│  PyTorch Inference Engine           │
│  - nanochat model (561M params)     │
│  - GPU acceleration (if available)  │
│  - Metal on Apple Silicon           │
│  - CPU fallback on other platforms  │
└─────────────────────────────────────┘
```

---

## 3. Detailed Design

### 3.1 Components to Add

#### A. Python Inference Server (`cmd/nanochat/inference_server.py`)

```python
"""
FastAPI server bridging Go simulator to nanochat PyTorch inference.

Features:
- Receives OpenAI-format chat completions requests
- Uses nanochat.Engine for efficient inference
- Streams tokens back as Server-Sent Events
- Manages model loading and GPU/CPU selection
"""
```

**Key Responsibilities:**
- Load nanochat model (from local checkpoint or Hugging Face)
- Manage tokenizer (special tokens: `<|user_start|>`, `<|assistant_start|>`, etc.)
- Implement streaming token generation
- Handle device selection (CUDA, MPS, CPU)
- Cache KV states for efficiency

**Endpoint:**
```
POST /chat/completions
{
  "messages": [{"role": "user", "content": "Hello"}],
  "temperature": 0.7,
  "max_tokens": 512,
  "top_k": 50
}
```

Response: Server-Sent Events stream of tokens

#### B. Go Subprocess Wrapper (`internal/nanochat/python_engine.go`)

```go
/*
Manages Python subprocess for inference:
- Starts Python FastAPI server on internal port
- Forwards OpenAI API calls to Python backend
- Handles process lifecycle (startup, shutdown)
- Translates between Go and Python message formats
*/
```

**Key Responsibilities:**
- Launch Python inference server with correct PYTHONPATH
- Wait for server readiness (health check polling)
- Proxy requests from Go to Python FastAPI
- Manage subprocess cleanup on exit
- Cache long-lived connection to reduce latency

#### C. Model Download Manager (`internal/nanochat/model_manager.go`)

```go
/*
Downloads and manages nanochat model files:
- Detects if model already cached locally
- Downloads from Hugging Face if needed
- Verifies checksums
- Extracts/organizes model structure
*/
```

**Model Structure:**
```
~/.cache/openai-api-simulator/nanochat/
├── model_000650.pt           # Model weights (~1.9 GB)
├── meta_000650.json          # Model config
├── tokenizer.pkl             # BPE tokenizer
├── token_bytes.pt            # Token byte mappings
├── weights_000650.safetensors (optional, if available)
└── inference_ready.lock      # Marker file for complete download
```

**Download Source:**
- Primary: `https://huggingface.co/sdobson/nanochat/` (PyTorch model files)
- Alternative: `https://huggingface.co/karpathy/nanochat/` (if sdobson version removed)

#### D. Updated Dockerfile

```dockerfile
# Stage 1: Build Go binary (unchanged)
FROM golang:1.22-alpine AS build
# ... build openai-api-simulator binary

# Stage 2: Runtime with Python + PyTorch
FROM python:3.11-slim
# Install PyTorch, nanochat dependencies
# Copy nanochat source code
# Copy Go binary
# Pre-download model if BAKED=true
```

### 3.2 Model Loading Strategy

#### For Local Development

```bash
# User runs:
./openai-api-simulator-nanochat

# On first startup:
1. Check if model exists in ~/.cache/openai-api-simulator/nanochat/
2. If not, download from Hugging Face (shows progress bar)
3. Start Python FastAPI server as subprocess
4. Wait for readiness health check
5. Serve chat requests through Go proxy
```

**Timeline:**
- First run: ~60-90 seconds (model download + inference server startup)
- Subsequent runs: ~5-10 seconds (model cached, fast server startup)

#### For Docker (Baked Image)

```bash
# During build with BAKED=true:
1. Download model files into container cache
2. Model is included in final image (~2GB added)

# At container startup:
1. Model already available in /root/.cache/
2. Python server starts immediately
3. Fast first request (no download needed)
```

**Image Size Impact:**
- Base image: ~400MB
- + PyTorch: ~800MB
- + Model: ~1.9GB
- **Total: ~3.1GB** (acceptable for baked scenario)

### 3.3 Implementation Phases

#### Phase 1: Core Python Inference Server

**Files to Create:**
1. `cmd/nanochat/inference_server.py` - FastAPI server with model loading
2. `scripts/install-nanochat.sh` - Updated to install Python deps
3. `pyproject.toml` (or `requirements.txt`) - Python dependencies

**Python Dependencies:**
```
torch>=2.0.0
transformers>=4.30.0
fastapi>=0.104.0
uvicorn>=0.24.0
pydantic>=2.0.0
```

**Key Features:**
- Load model with `torch.load()` and device auto-detection
- Use nanochat's tokenizer and Engine classes
- Implement `/chat/completions` endpoint compatible with OpenAI format
- Support streaming Server-Sent Events

#### Phase 2: Go Subprocess Manager

**Files to Create:**
1. `internal/nanochat/python_engine.go` - Subprocess lifecycle + health checks
2. Update `internal/nanochat/launcher.go` - Use new Python engine instead of llama.cpp

**Go Code Structure:**
```go
type PythonEngine struct {
    cmd *exec.Cmd
    url string // e.g., "http://127.0.0.1:8081"
    client *http.Client
}

func (pe *PythonEngine) Start() error {
    // Start subprocess
    // Wait for health check
    // Verify readiness
}

func (pe *PythonEngine) Stop() error {
    // Graceful shutdown
    // Cleanup resources
}

func (pe *PythonEngine) Chat(ctx context.Context, req OpenAIRequest) (stream io.Reader, err error) {
    // Forward to Python FastAPI endpoint
    // Return streaming response
}
```

#### Phase 3: Model Manager

**Files to Create:**
1. `internal/nanochat/model_manager.go` - Download and cache logic

**Key Functions:**
```go
func EnsureModel(cacheDir string) error {
    // Check local cache
    // Download if missing
    // Verify integrity
}

func DownloadModelFile(url, dest string) error {
    // With progress bar
    // Resume on failure
    // Checksum verification
}
```

#### Phase 4: Docker Integration

**Files to Update:**
1. `Dockerfile` - Multi-stage with Python + PyTorch
2. `docker-compose.yml` - Environment variables for model path
3. `Makefile` - New targets for Python-based builds

**Docker Build Process:**
```dockerfile
# With BAKED=true:
RUN if [ "$BAKED" = "true" ]; then \
    python -m scripts.download_model \
    --output /root/.cache/openai-api-simulator/nanochat; \
fi
```

---

## 4. Technology Stack

### PyTorch Inference
- **Model Format:** Native PyTorch checkpoint (`.pt` files)
- **Inference Mode:** `torch.inference_mode()` for optimized inference
- **GPU Support:** CUDA (NVIDIA), MPS (Apple Silicon)
- **CPU Fallback:** Automatic if GPU unavailable

### FastAPI for Python Server
- **Async:** Full async/await support for concurrent requests
- **Streaming:** Server-Sent Events for token streaming
- **OpenAI Compatible:** Implements `/chat/completions` endpoint
- **Lightweight:** Minimal dependencies, fast startup

### Go Subprocess Management
- `os/exec` - Standard library subprocess handling
- `net/http` - Client for communicating with Python server
- `context` - Timeout and cancellation support

---

## 5. Design Decisions & Rationales

| Decision | Rationale |
|----------|-----------|
| Use PyTorch directly instead of GGUF | GGUF doesn't exist for nanochat; PyTorch is native format; simpler conversion path |
| Python subprocess instead of embedding | Avoids C/Python binding complexity; easier to maintain; PyTorch already requires Python |
| FastAPI for Python server | OpenAI-compatible API; streaming support; modern async; easy to test |
| Keep Go proxy layer | Maintains unified entry point; existing OpenAI API compatibility; can add other models |
| Model cache in `~/.cache/` | Standard XDG location; consistent with other tools; user-writable |
| KV cache optimization | Reduces inference time per token by 2-3x; nanochat Engine already implements it |

---

## 6. Advantages Over Current Approach

### ✅ **Correctness**
- Uses model in its native PyTorch format (no conversion needed)
- Leverages tested nanochat inference code from karpathy/nanochat
- No architecture mismatches (arm64 vs x86_64)

### ✅ **Reliability**
- Single model download source (Hugging Face PyTorch files)
- No external binary dependencies (llama.cpp releases)
- Predictable, deterministic behavior

### ✅ **Performance**
- GPU acceleration works seamlessly (CUDA + MPS)
- KV cache optimization already implemented in nanochat.Engine
- Faster token generation than llama.cpp in many cases

### ✅ **Simplicity**
- Fewer moving parts (no zip extraction, binary detection)
- Cleaner Docker builds (no llama.cpp compilation)
- Easier to debug (Python code vs subprocess black boxes)

### ✅ **Maintainability**
- Code closely mirrors nanochat repository structure
- Easy to update when nanochat improves
- Clear separation of concerns (Go proxy ↔ Python inference)

---

## 7. Challenges & Mitigations

| Challenge | Mitigation |
|-----------|-----------|
| Go ↔ Python overhead | Subprocess amortized over many requests; pipelined requests |
| Python startup time | Cache subprocess; health check polling; show progress to user |
| Model download size (~2GB) | Cached locally; progress bar; resume on failure; optional baking |
| PyTorch dependency size | Acceptable in Docker (~800MB); users on x86_64 can use CPU |
| First request latency | Offline usage docs; pre-warming subprocess in background |

---

## 8. Backward Compatibility

### Breaking Changes
- ❌ Removes `llama-server` binary dependency
- ❌ Requires Python 3.11+ (new requirement)

### Non-Breaking
- ✅ OpenAI API format unchanged (same `/chat/completions` endpoint)
- ✅ Model selection unchanged (`model="nanochat"`)
- ✅ Docker image still works (new layers added)
- ✅ Makefile targets compatible

### Migration Path
- Old: `make run-nanochat` → calls llama.cpp
- New: `make run-nanochat` → calls Python inference (transparent to user)

---

## 9. Implementation Timeline

| Phase | Duration | Blockers |
|-------|----------|----------|
| Phase 1: Python server | 2-3 days | Learning nanochat codebase |
| Phase 2: Go wrapper | 1-2 days | Subprocess handling edge cases |
| Phase 3: Model manager | 1 day | Hugging Face API integration |
| Phase 4: Docker | 1 day | Testing multi-platform builds |
| Testing & Docs | 1-2 days | Integration testing |
| **Total** | **~1 week** | None |

---

## 10. Testing Strategy

### Unit Tests
```go
// internal/nanochat/model_manager_test.go
- Test model cache detection
- Test download with mock server
- Test checksum verification
- Test error handling

// internal/nanochat/python_engine_test.go
- Test subprocess startup/shutdown
- Test health check polling
- Test request forwarding
- Test streaming response parsing
```

### Integration Tests
```bash
# Full flow test
1. Clean cache
2. Start simulator
3. Wait for model download + server startup
4. Send chat request
5. Verify response format
6. Verify tokens are valid
```

### Performance Tests
```bash
# Benchmark inference
- First request (no cache): ~90s total (60s download + 30s inference)
- Cached requests: <5s total
- Token latency: <500ms per token (GPU), <2s per token (CPU)
```

---

## 11. Deployment Checklist

- [ ] Create Python inference server (`cmd/nanochat/inference_server.py`)
- [ ] Create Go subprocess wrapper (`internal/nanochat/python_engine.go`)
- [ ] Create model manager (`internal/nanochat/model_manager.go`)
- [ ] Update Dockerfile for Python + PyTorch
- [ ] Update Makefile build targets
- [ ] Add Python dependencies to docker-compose
- [ ] Create comprehensive tests
- [ ] Update documentation (README, NANOCHAT_QUICKSTART)
- [ ] Performance benchmarking on x86_64 and ARM64
- [ ] Docker multi-platform build testing
- [ ] Remove old downloader.go llama.cpp logic (Phase 2 cleanup)

---

## 12. Success Criteria

✅ **Functional**
- [ ] NanoChat inference works on macOS Apple Silicon
- [ ] NanoChat inference works on Linux x86_64
- [ ] Docker image builds and runs successfully
- [ ] Model downloads on first run with progress
- [ ] Subsequent runs use cached model

✅ **Performance**
- [ ] First startup: <2 minutes (model download + inference server)
- [ ] Subsequent startups: <10 seconds
- [ ] Inference latency: <1 token/second (GPU), <5 tokens/sec (CPU)
- [ ] Memory usage: <8GB GPU, <4GB CPU

✅ **Reliability**
- [ ] Graceful failure on network errors
- [ ] Automatic retry on partial downloads
- [ ] Subprocess crash recovery

✅ **UX**
- [ ] Clear progress messages during download
- [ ] Documented setup process
- [ ] Identical OpenAI API interface as before

---

## 13. Future Enhancements

### Phase 2 (Future)
- [ ] Support for other nanochat model sizes (d20, d26, d32)
- [ ] Quantized variants (int8, float16) for reduced memory
- [ ] Model fine-tuning API
- [ ] Batch inference for multiple simultaneous requests

### Phase 3 (Future)
- [ ] Support for other PyTorch models (Llama 2, Mistral, etc.)
- [ ] ONNX export option for edge deployment
- [ ] Web UI enhancement with model selection dropdown

---

## 14. References

### NanoChat Repository
- **Repo:** https://github.com/karpathy/nanochat
- **Key Files:**
  - `nanochat/engine.py` - Efficient inference with KV cache
  - `scripts/chat_web.py` - FastAPI chat server
  - `nanochat/tokenizer.py` - BPE tokenizer wrapper
  - `nanochat/gpt.py` - Model architecture

### Model Repository
- **Hugging Face:** https://huggingface.co/sdobson/nanochat
- **Size:** Model ~1.9GB, Tokenizer ~900KB, Config ~300B

### Documentation
- **FastAPI:** https://fastapi.tiangolo.com/
- **PyTorch Inference:** https://pytorch.org/docs/stable/inference_mode.html
- **Server-Sent Events:** https://html.spec.whatwg.org/multipage/server-sent-events.html

---

## 15. Conclusion

By using nanochat's native PyTorch inference instead of attempting to convert to GGUF and use llama.cpp, we:

1. **Eliminate complexity** - No binary extraction, architecture detection, or external URL dependencies
2. **Improve reliability** - Use the model in its intended format with tested inference code
3. **Gain flexibility** - Can easily support other PyTorch models in the future
4. **Maintain compatibility** - Same OpenAI API interface for users

This decision record advocates for leveraging the nanochat repository's existing, well-tested inference infrastructure rather than fighting against format conversions and missing binaries.

---

**ADR Status:** Ready for implementation

**Next Steps:** Begin Phase 1 development
