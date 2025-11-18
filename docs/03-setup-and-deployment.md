# NanoChat PyTorch Integration - Setup & Deployment Guide

## Table of Contents

1. [Local Development Setup](#local-development-setup)
2. [Docker Deployment](#docker-deployment)
3. [Troubleshooting](#troubleshooting)
4. [Performance Tuning](#performance-tuning)
5. [Testing](#testing)

---

## Local Development Setup

### Prerequisites

- Go 1.22+
- Python 3.11+
- Git
- ~3GB free disk space for model files
- GPU (optional, but recommended)

### Step 1: Clone and Setup

```bash
# Clone repository
git clone <repo>
cd openai-api-simulator

# Download and install nanochat
./scripts/setup-nanochat.sh

# Install Python dependencies
pip install -r requirements-nanochat.txt

# Install Go dependencies
go mod download
```

### Step 2: Run Inference Server

```bash
# Terminal 1: Start Python inference server
export PYTHONPATH=.nanochat
python cmd/nanochat/inference_server.py \
  --port 8081 \
  --model-dir ~/.cache/openai-api-simulator/nanochat

# Expected output:
# 2024-11-18 11:35:00 - __main__ - INFO - Device: cuda:0 (NVIDIA A100)
# 2024-11-18 11:35:05 - __main__ - INFO - Model loaded successfully
# INFO:     Uvicorn running on http://127.0.0.1:8081
```

### Step 3: Run Go Server

```bash
# Terminal 2: Start OpenAI API simulator
export NANOCHAT_DIR=$(pwd)/.nanochat
go run ./cmd/server -port 3080

# Expected output:
# 2024-11-18 11:35:10 Starting OpenAI API simulator on :3080
# [PythonEngine] Starting inference server on http://127.0.0.1:8081
# [PythonEngine] Server ready at http://127.0.0.1:8081
```

### Step 4: Test the API

```bash
# Terminal 3: Test endpoint
curl -X POST http://localhost:3080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nanochat",
    "messages": [
      {"role": "user", "content": "What is machine learning?"}
    ],
    "temperature": 0.7,
    "max_tokens": 128
  }' | jq .

# Expected response:
# {
#   "id": "...",
#   "object": "text_completion",
#   "created": 1234567890,
#   "model": "nanochat",
#   "choices": [
#     {
#       "text": "Machine learning is...",
#       "finish_reason": "length"
#     }
#   ],
#   "usage": {
#     "prompt_tokens": 12,
#     "completion_tokens": 128,
#     "total_tokens": 140
#   }
# }
```

---

## Docker Deployment

### Option 1: Regular Image (Recommended for Development)

```bash
# Build
docker build \
  -t openai-api-simulator:pytorch \
  .

# Run
docker run \
  -p 3080:3080 \
  -p 8081:8081 \
  -e PYTHONUNBUFFERED=1 \
  --name api-simulator \
  openai-api-simulator:pytorch

# Test
sleep 5  # Wait for startup
curl http://localhost:3080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "nanochat", "messages": [{"role": "user", "content": "Hi"}]}'
```

### Option 2: Baked Image (Recommended for Production)

Includes pre-downloaded model files (~2GB added to image):

```bash
# Build (takes 5+ minutes to download models)
docker build \
  --build-arg BAKED=true \
  -t openai-api-simulator:pytorch-baked \
  .

# Run (starts in <10 seconds)
docker run \
  -p 3080:3080 \
  -p 8081:8081 \
  -e PYTHONUNBUFFERED=1 \
  --name api-simulator \
  openai-api-simulator:pytorch-baked

# First startup will be much faster
docker logs api-simulator -f
```

### Option 3: With GPU Support

```bash
# Build with CUDA support
docker build \
  --build-arg PYTORCH_INDEX_URL=https://download.pytorch.org/whl/cu118 \
  -t openai-api-simulator:pytorch-cuda \
  .

# Run with GPU access
docker run \
  --gpus all \
  -p 3080:3080 \
  -p 8081:8081 \
  -e PYTHONUNBUFFERED=1 \
  --name api-simulator \
  openai-api-simulator:pytorch-cuda

# Verify GPU is being used
docker logs api-simulator | grep -i "cuda\|gpu\|device"
```

### Docker Compose Setup

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  api-simulator:
    build:
      context: .
      args:
        BAKED: "true"
        PYTORCH_INDEX_URL: "https://download.pytorch.org/whl/cpu"
    ports:
      - "3080:3080"
      - "8081:8081"
    environment:
      - PYTHONUNBUFFERED=1
      - LOG_LEVEL=INFO
    volumes:
      - model-cache:/root/.cache/openai-api-simulator/nanochat
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3080/health"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped

volumes:
  model-cache:
    driver: local
```

Run with:
```bash
docker-compose up -d
docker-compose logs -f
docker-compose down
```

---

## Troubleshooting

### Issue: Model Download Fails

**Symptom:** `404 Not Found` when downloading model files

**Solution:**
```bash
# Check Hugging Face URL is valid
curl -I https://huggingface.co/sdobson/nanochat/resolve/main/model_000650.pt

# If 404, files may have been moved - check:
# https://huggingface.co/sdobson/nanochat

# Use Python model manager for direct download
python -c "
from internal.nanochat.model_manager import ModelManager
mm = ModelManager('/tmp/test')
mm.EnsureModel()  # Downloads with retry logic
"
```

### Issue: Out of Memory

**Symptom:** `CUDA out of memory` or Python process killed

**Solution:**
```bash
# Check available VRAM
nvidia-smi

# Reduce batch size (in inference_server.py):
# max_tokens = 256  # Reduce from 512

# Use CPU instead
export CUDA_VISIBLE_DEVICES=""

# Or limit GPU memory growth
# In Python: torch.cuda.empty_cache()
```

### Issue: Python Server Not Starting

**Symptom:** `Connection refused` when accessing `http://127.0.0.1:8081`

**Solution:**
```bash
# Check if port is in use
lsof -i :8081  # Linux/Mac
netstat -ano | findstr :8081  # Windows

# Try different port
python cmd/nanochat/inference_server.py --port 8082

# Check Python path
export PYTHONPATH=.nanochat:$PYTHONPATH

# Run with verbose logging
python -u cmd/nanochat/inference_server.py --port 8081 2>&1 | head -50
```

### Issue: Slow Token Generation

**Symptom:** Takes >5 seconds per token

**Solution:**
```bash
# 1. Check device being used
curl http://localhost:8081/info | jq .device_type

# 2. If on CPU, switch to GPU
export CUDA_VISIBLE_DEVICES=0

# 3. Reduce model precision (requires code changes)
# Use fp16 instead of fp32 in inference

# 4. Profile with:
python -m cProfile -s cumtime cmd/nanochat/inference_server.py --port 8081
```

### Issue: Docker Image Too Large

**Symptom:** Image is >3GB (too large for registry)

**Solution:**
```bash
# Don't use BAKED=true for production registry
docker build -t my-registry/api-simulator:pytorch .

# Host model files separately
# Mount model cache at runtime:
docker run \
  -v /data/models:/root/.cache/openai-api-simulator/nanochat \
  my-registry/api-simulator:pytorch
```

---

## Performance Tuning

### CPU-Only System

For development machines without GPU:

```python
# In inference_server.py, add at module level:
import os
os.environ['CUDA_VISIBLE_DEVICES'] = ''  # Disable CUDA

# Expected performance: 2-4 tokens/second
# Memory usage: 4-6GB
```

### Single GPU System

```bash
# Start inference server
python cmd/nanochat/inference_server.py \
  --port 8081 \
  --device cuda

# Monitor GPU usage
watch -n 1 nvidia-smi
```

### Multi-GPU System

Currently single-GPU. For multi-GPU support:

```python
# Modify inference_server.py:
if torch.cuda.device_count() > 1:
    model = torch.nn.DataParallel(model)
    # Expected speedup: 1.7x on 2 GPUs, 3.5x on 4 GPUs
```

### Cache Warming

Pre-load model on startup:

```python
# Add to inference_server.py startup:
@app.on_event("startup")
async def warmup():
    # Generate dummy tokens to warmup GPU cache
    dummy_tokens = torch.zeros((1, 10), device=device, dtype=torch.long)
    with torch.no_grad():
        model(dummy_tokens)
```

### Request Batching

For production throughput:

```bash
# Modify endpoint to accept batch requests
# POST /batch/completions with array of requests
# Process in parallel, return streamed results
```

---

## Testing

### Unit Tests

```bash
# Test Python inference server
python -m pytest tests/test_nanochat_inference.py -v

# Test Go components
go test ./internal/nanochat -v

# Test with coverage
go test ./internal/nanochat -cover
```

### Integration Tests

```bash
# 1. Start services
python cmd/nanochat/inference_server.py --port 8081 &
go run ./cmd/server -port 3080 &

# 2. Test endpoints
curl http://localhost:8081/health
curl http://localhost:3080/health

# 3. Test chat completion
curl -X POST http://localhost:3080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nanochat",
    "messages": [{"role": "user", "content": "Test"}]
  }'

# 4. Test streaming
curl -X POST http://localhost:3080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nanochat",
    "messages": [{"role": "user", "content": "Test"}],
    "stream": true
  }' | head -20
```

### Load Testing

```bash
# Install Apache Bench
brew install httpd  # macOS
apt install apache2-utils  # Ubuntu

# Simple load test
ab -n 100 -c 10 http://localhost:3080/health

# With POST data
ab -n 100 -c 5 \
  -T application/json \
  -p request.json \
  http://localhost:3080/v1/chat/completions

# Sustained load test
while true; do
  curl -s http://localhost:3080/health > /dev/null
  echo "$(date): Request completed"
  sleep 1
done
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NANOCHAT_DIR` | `.nanochat` | Path to nanochat repository |
| `PYTHONPATH` | (empty) | Must include nanochat directory |
| `PYTHONUNBUFFERED` | 0 | Set to 1 for real-time logs |
| `CUDA_VISIBLE_DEVICES` | (auto) | GPU selection (empty = CPU only) |
| `LOG_LEVEL` | `INFO` | Logging level (DEBUG, INFO, WARNING, ERROR) |
| `INFERENCE_SERVER_PATH` | (auto) | Path to inference_server.py |

---

## Next Steps

1. **Local Testing:** Follow local setup, verify all endpoints work
2. **Docker Build:** Build and test regular image first
3. **Baked Image:** Build production image with BAKED=true
4. **Load Testing:** Run benchmarks with expected traffic
5. **Monitoring:** Set up logging and metrics collection
6. **Deployment:** Deploy to production infrastructure

---

## Support & Debugging

### Useful Commands

```bash
# Check Python version and packages
python --version
pip list | grep -E "torch|fastapi"

# Check Go installation
go version

# Find nanochat directory
find ~ -name "nanochat" -type d 2>/dev/null

# Monitor Docker resources
docker stats api-simulator

# View server logs
docker logs api-simulator -f --tail=100

# Clean up everything
docker stop api-simulator
docker rm api-simulator
rm -rf ~/.cache/openai-api-simulator
```

### Getting Help

1. Check logs: `docker logs` or terminal output
2. Review ADR 0003 for architecture decisions
3. Check NANOCHAT_PYTORCH_IMPLEMENTATION.md for detailed examples
4. Test with simpler requests first
5. Use `--debug` or `LOG_LEVEL=DEBUG` for verbose output

---

## Production Checklist

- [ ] Model files downloaded and verified
- [ ] Both services (Go + Python) start cleanly
- [ ] All endpoints respond correctly
- [ ] Streaming responses work properly
- [ ] GPU acceleration verified (if available)
- [ ] Load test with expected traffic
- [ ] Monitoring/logging setup
- [ ] Health checks configured
- [ ] Restart policies defined
- [ ] Resource limits set (CPU, memory)
- [ ] Network policies configured
- [ ] Backup/restore procedures documented

---

## Additional Resources

- **NanoChat:** https://github.com/karpathy/nanochat
- **FastAPI:** https://fastapi.tiangolo.com/docs
- **PyTorch:** https://pytorch.org/docs/stable/index.html
- **OpenAI API:** https://platform.openai.com/docs/api-reference/chat
- **Docker:** https://docs.docker.com/
