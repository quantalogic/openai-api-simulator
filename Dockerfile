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

# Install PyTorch (CPU by default, GPU support can be enabled via build args)
# For CUDA support, use: torch>=2.0.0 --index-url https://download.pytorch.org/whl/cu118
# For CPU only:
RUN pip install --no-cache-dir \
	torch>=2.0.0 \
	torchvision \
	torchaudio \
	--index-url https://download.pytorch.org/whl/cpu

# Install FastAPI dependencies
RUN pip install --no-cache-dir \
	fastapi>=0.104.0 \
	uvicorn>=0.24.0 \
	pydantic>=2.0.0 \
	pydantic-settings

# Clone nanochat repository
RUN git clone https://github.com/karpathy/nanochat.git /opt/nanochat && \
	pip install --no-cache-dir -e /opt/nanochat

# Create app directory
WORKDIR /app

# Copy Go binary from build stage
COPY --from=go-build /workspace/${BINARY} /app/server

# Copy Python inference server
COPY cmd/nanochat/inference_server.py /app/inference_server.py

# Set PYTHONPATH for nanochat imports
ENV PYTHONPATH="/opt/nanochat:${PYTHONPATH}"
ENV INFERENCE_SERVER_PATH="/app/inference_server.py"

# Optional: Bake model files into image (saves download time on first run)
RUN if [ "${BAKED}" = "true" ]; then \
	echo "→ Baking nanochat model files into image..."; \
	mkdir -p /root/.cache/openai-api-simulator/nanochat && \
	cd /root/.cache/openai-api-simulator/nanochat && \
	\
	echo "↓ Downloading nanochat PyTorch model files from Hugging Face..."; \
	echo "  This may take several minutes (model is ~1.9GB)..."; \
	\
	curl -fsSL -o model_000650.pt "https://huggingface.co/sdobson/nanochat/resolve/main/model_000650.pt" && \
	curl -fsSL -o meta_000650.json "https://huggingface.co/sdobson/nanochat/resolve/main/meta_000650.json" && \
	curl -fsSL -o tokenizer.pkl "https://huggingface.co/sdobson/nanochat/resolve/main/tokenizer.pkl" && \
	\
	echo "✓ Model files downloaded"; \
	echo "Cache contents:"; \
	du -sh /root/.cache/openai-api-simulator/nanochat/; \
fi

# Expose ports
EXPOSE 8090 8081

# Set default Go server entrypoint
# (Python inference server is managed by Go server via subprocess)
# Note: docker-compose will provide the full command including port and other flags
ENTRYPOINT ["/app/server"]
CMD []
