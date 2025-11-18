# NanoChat - Real AI Inference in One Command

NanoChat brings real AI inference to the OpenAI API Simulator using a 561 million parameter model (sdobson/nanochat, based on Andrej Karpathy's work).

## Quick Start

```bash
# Build and run
make run-nanochat

# Or build first, then run
make build-nanochat
./openai-api-simulator-nanochat
```

That's it! The command will:

1. Detect your platform (macOS/Linux, Intel/ARM/Apple Silicon)
2. Download the llama.cpp server binary (~50MB) on first run
3. Download the nanochat GGUF model (~316MB) on first run
4. Cache everything in `~/.cache/openai-api-simulator`
5. Start llama.cpp server on port 8081
6. Start OpenAI API proxy on port 3080

**First run:** ~45 seconds (downloads ~366MB total)  
**Subsequent runs:** ~2 seconds (uses cached files)

## Usage

### Test with cURL

Non-streaming:
```bash
curl http://localhost:3080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nanochat",
    "messages": [{"role": "user", "content": "Why is the sky blue?"}]
  }'
```

Streaming:
```bash
curl http://localhost:3080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nanochat",
    "messages": [{"role": "user", "content": "Tell me a story"}],
    "stream": true
  }'
```

### Available Models

When running with nanochat, you have access to both:

- **nanochat** - Real 561M parameter AI model
  - Owner: sdobson
  - Quantization: Q4_K_M
  - Size: 316 MB
  - Context: 4096 tokens
  
- **gpt-sim-1** - Original fake deterministic model (for testing)

List available models:
```bash
curl http://localhost:3080/v1/models
```

### Custom Ports

```bash
./openai-api-simulator-nanochat -port 3080 -llama-port 8081
```

Options:
- `-port` - Public API port (default: 3080)
- `-llama-port` - Internal llama.cpp port (default: 8081)

## Platform Support

NanoChat automatically detects and downloads the correct binaries for:

- **macOS Apple Silicon** (arm64) - Full Metal GPU acceleration
- **macOS Intel** (x64) - CPU inference
- **Linux x86_64** - CPU inference
- **Linux aarch64** (ARM64) - CPU inference

On Apple Silicon, the model uses Metal for GPU acceleration, providing significantly faster inference.

## Architecture

```
User Request
    ↓
[OpenAI API Proxy :3080]
    ↓ (model="nanochat")
[llama.cpp server :8081]
    ↓
[nanochat-q4_k_m.gguf]
```

The nanochat command:
1. Ensures llama.cpp server binary is cached
2. Ensures nanochat model is cached
3. Starts llama.cpp server with the model
4. Starts the OpenAI API simulator in proxy mode
5. Routes requests for model "nanochat" to llama.cpp
6. Routes other models to the built-in fake generator

## Cache Location

All files are cached in:
```
~/.cache/openai-api-simulator/
├── llama-server           # llama.cpp server binary
└── nanochat-q4_k_m.gguf  # nanochat model
```

You can delete this directory to force re-download of all files.

## Technical Details

### Model Information

- **Name:** sdobson/nanochat
- **Parameters:** 561 million
- **Quantization:** Q4_K_M (4-bit quantization, best quality/size ratio)
- **File Size:** 316 MB
- **Context Length:** 4096 tokens
- **Temperature:** 0.7 (default)
- **Source:** Based on Andrej Karpathy's nanochat architecture

### llama.cpp Configuration

When nanochat starts llama.cpp, it uses:

```bash
llama-server \
  --host 127.0.0.1 \
  --port 8081 \
  --model nanochat-q4_k_m.gguf \
  --ctx-size 4096 \
  --temp 0.7 \
  --n-gpu-layers 999  # On Apple Silicon only
  --threads <cpu_count>
```

### GPU Acceleration

- **Apple Silicon (M1/M2/M3):** Full GPU offload via Metal (999 layers)
- **Other platforms:** CPU-only inference (0 layers)

Future versions may support CUDA for NVIDIA GPUs on Linux.

## Troubleshooting

### Download Fails

If downloads fail, delete the cache and try again:
```bash
rm -rf ~/.cache/openai-api-simulator
./openai-api-simulator-nanochat
```

### Port Already in Use

Change the ports:
```bash
./openai-api-simulator-nanochat -port 3081 -llama-port 8082
```

### Slow Inference

On non-Apple Silicon platforms, inference runs on CPU and may be slower. This is expected. For faster inference:
- Use a machine with Apple Silicon (M1/M2/M3)
- Or use a Linux machine with CUDA support (future enhancement)

### Health Check Failed

If llama.cpp fails to start within 45 seconds:
1. Check the console output for errors
2. Verify you have enough disk space (~500MB free)
3. Check that ports 8081 and 3080 are available
4. Try running with different ports

## Comparison with Fake Models

| Feature | gpt-sim-1 (fake) | nanochat (real) |
|---------|------------------|-----------------|
| Response quality | Random coherent text | Real AI responses |
| Deterministic | Yes (seed-based) | No (temperature-based) |
| Speed | Instant | 1-5 seconds |
| Setup time | 0 | 45s first run, 2s after |
| Download size | 0 | 366 MB |
| GPU acceleration | N/A | Yes (Apple Silicon) |
| Use case | Testing, CI/CD | Real conversations |

## Integration Examples

### Python (OpenAI SDK)

```python
from openai import OpenAI

client = OpenAI(
    api_key="not-needed",
    base_url="http://localhost:3080/v1"
)

response = client.chat.completions.create(
    model="nanochat",
    messages=[
        {"role": "user", "content": "Why is the sky blue?"}
    ]
)

print(response.choices[0].message.content)
```

### Node.js

```javascript
const OpenAI = require('openai');

const client = new OpenAI({
  apiKey: 'not-needed',
  baseURL: 'http://localhost:3080/v1'
});

async function chat() {
  const response = await client.chat.completions.create({
    model: 'nanochat',
    messages: [
      { role: 'user', content: 'Why is the sky blue?' }
    ]
  });
  
  console.log(response.choices[0].message.content);
}

chat();
```

### cURL with Streaming

```bash
curl -N http://localhost:3080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nanochat",
    "messages": [{"role": "user", "content": "Count to 10"}],
    "stream": true
  }'
```

## Performance Benchmarks

Approximate inference speeds (varies by hardware):

| Platform | Tokens/Second | First Token Latency |
|----------|---------------|---------------------|
| M3 Max | 80-120 | 200ms |
| M2 Pro | 60-90 | 300ms |
| M1 | 40-60 | 400ms |
| Intel i9 (CPU) | 10-20 | 1000ms |
| AMD Ryzen (CPU) | 10-20 | 1000ms |

## Future Enhancements

Planned improvements:
- [ ] CUDA support for NVIDIA GPUs on Linux
- [ ] ROCm support for AMD GPUs
- [ ] Additional model options (larger/smaller models)
- [ ] Model hot-swapping
- [ ] Batch inference support
- [ ] OpenAI embeddings endpoint with real embeddings

## See Also

- [ADR 002: NanoChat Implementation](../adr/0002-nano-chat.md)
- [Main README](../README.md)
- [llama.cpp project](https://github.com/ggerganov/llama.cpp)
- [sdobson/nanochat on Hugging Face](https://huggingface.co/sdobson/nanochat)
