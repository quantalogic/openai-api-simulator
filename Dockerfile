## Stage 1: Build Go binary
FROM golang:1.22-alpine AS go-build
RUN apk add --no-cache git
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG BINARY=server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /workspace/${BINARY} ./cmd/${BINARY}

## Stage 2: Runtime image with Python + PyTorch
FROM python:3.11-slim
ARG BINARY=server
ARG BAKED=false

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
	ca-certificates \
	curl \
	git \
	build-essential \
	&& rm -rf /var/lib/apt/lists/*

# Install llama-cpp-python and FastAPI dependencies
# llama-cpp-python provides efficient GGUF model inference via llama.cpp
RUN pip install --no-cache-dir \
	llama-cpp-python>=0.2.0 \
	fastapi>=0.104.0 \
	uvicorn>=0.24.0 \
	pydantic>=2.0.0 \
	pydantic-settings

# Create app directory
WORKDIR /app

# Copy Go binary from build stage
COPY --from=go-build /workspace/${BINARY} /app/server

# Copy Python inference server
COPY cmd/nanochat/inference_server.py /app/inference_server.py

# Set PYTHONPATH and model path
ENV PYTHONPATH="/app:${PYTHONPATH}"
ENV INFERENCE_SERVER_PATH="/app/inference_server.py"
ENV SMOLLM_MODEL_PATH="/root/.cache/openai-api-simulator/smollm"

# Optional: Bake model files into image (saves download time on first run)
RUN if [ "${BAKED}" = "true" ]; then \
	echo "→ Baking SmolLM GGUF model into image..."; \
	mkdir -p /root/.cache/openai-api-simulator/smollm && \
	cd /root/.cache/openai-api-simulator/smollm && \
	\
	echo "↓ Downloading SmolLM2-360M-Instruct GGUF from Hugging Face..."; \
	echo "  Model: HuggingFaceTB/SmolLM2-360M-Instruct-GGUF"; \
	echo "  Size: ~386MB (Q8_0 quantization)..."; \
	\
	curl -fsSL -o smollm2-360m-instruct-q8_0.gguf "https://huggingface.co/HuggingFaceTB/SmolLM2-360M-Instruct-GGUF/resolve/main/smollm2-360m-instruct-q8_0.gguf" && \
	\
	echo "✓ Model file downloaded"; \
	echo "Cache contents:"; \
	du -sh /root/.cache/openai-api-simulator/smollm/; \
fi

# Expose ports
EXPOSE 8090 8081

# Set default Go server entrypoint
# (Python inference server is managed by Go server via subprocess)
# Note: docker-compose will provide the full command including port and other flags
ENTRYPOINT ["/app/server"]
CMD []
