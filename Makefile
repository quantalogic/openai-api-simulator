# Makefile for OpenAI API Simulator with NanoChat PyTorch Inference

BINARY=server
PORT?=8090
INFERENCE_PORT?=8081
OPENWEBUI_PORT?=3000
IMAGE?=openai-api-simulator:pytorch
IMAGE_BAKED?=openai-api-simulator:pytorch-baked
SHELL := /bin/bash

.PHONY: all build run run-sim test tidy clean fmt help \
        docker-build docker-run docker-build-baked docker-run-baked docker-clean \
        compose-up compose-down compose-up-noai compose-down-noai compose-logs compose-openwebui \
        setup-dev curl-stream curl-text curl-sim open
setup-dev:
	@echo "🔧 Setting up development environment..."
	@echo "   - Enabling GO111MODULE=on"
	go env -w GO111MODULE=on
	@echo "   - Downloading Go dependencies"
	go mod download
	@echo "   - Setting up Python environment"
	./scripts/setup-nanochat.sh
	@echo "✅ Development setup complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Terminal 1: python cmd/nanochat/inference_server.py --port 8081"
	@echo "  2. Terminal 2: go run ./cmd/server -port 8090"
	@echo "  3. Terminal 3: curl -X POST http://localhost:8090/v1/chat/completions ..."
	@echo ""
	@echo "Or use: make local-dev (runs both in one terminal)"


all: setup-dev build

build:
	go build -o $(BINARY) ./cmd/server

run: build
	./$(BINARY) -port $(PORT)

run-sim: build
	@echo "🧠 Running pure simulation (fake AI model)"
	./$(BINARY) -port $(PORT)

local-dev: build
	@echo "🚀 Starting local development (API on port $(PORT), Inference on port $(INFERENCE_PORT))"
	@echo ""
	@echo "Make sure Python inference server is running in another terminal:"
	@echo "  export PYTHONPATH=.nanochat && python cmd/nanochat/inference_server.py --port $(INFERENCE_PORT)"
	@echo ""
	./$(BINARY) -port $(PORT)

test:
	go test ./... -v

tidy:
	go mod tidy

fmt:
	gofmt -w .

clean:
	rm -f $(BINARY) $(NANOCHAT_BINARY)

docker-build:
	@echo "📦 Building Docker image: $(IMAGE)"
	DOCKER_BUILDKIT=1 docker build -t $(IMAGE) .

docker-run: docker-build
	@echo "🐳 Running Docker image (API on port $(PORT))"
	docker run --rm -p $(PORT):8090 -p $(INFERENCE_PORT):8081 \
		--name openai-api-simulator \
		-e PYTHONUNBUFFERED=1 \
		$(IMAGE)

docker-build-baked:
	@echo "📦 Building Docker image with baked model: $(IMAGE_BAKED)"
	@echo "⚠️  This may take 5-10 minutes as it downloads and bakes the 1.9GB model"
	DOCKER_BUILDKIT=1 docker build --build-arg BAKED=true -t $(IMAGE_BAKED) .

docker-run-baked: docker-build-baked
	@echo "🐳 Running baked Docker image (faster startup)"
	docker run --rm -p $(PORT):8090 -p $(INFERENCE_PORT):8081 \
		--name openai-api-simulator \
		-e PYTHONUNBUFFERED=1 \
		$(IMAGE_BAKED)

docker-clean:
	-docker stop openai-api-simulator || true
	-docker rmi $(IMAGE) || true
	-docker rmi $(IMAGE_BAKED) || true
	@echo "✅ Docker cleanup complete"

open:
	@echo "🌐 Opening Web UI in browser..."
	open http://localhost:$(OPENWEBUI_PORT) || xdg-open http://localhost:$(OPENWEBUI_PORT) || echo "Please open http://localhost:$(OPENWEBUI_PORT)"

compose-up:
	@echo "🐳 Starting Docker Compose (API + NanoChat Inference + Web UI)"
	docker compose up --build -d
	@echo ""
	@echo "✅ Services started:"
	@echo "   API Simulator: http://localhost:$(PORT)"
	@echo "   Inference:    http://localhost:$(INFERENCE_PORT)"
	@echo "   Web UI:       http://localhost:$(OPENWEBUI_PORT)"
	@echo ""
	@echo "Give the UI 15s to initialize, then:"
	@echo "   make open"

compose-down:
	@echo "🛑 Stopping Docker Compose"
	docker compose down

compose-up-noai:
	@echo "🐳 Starting Docker Compose (API + Web UI, NO NanoChat)"
	docker compose -f docker-compose.noai.yml up --build -d
	@echo ""
	@echo "✅ Services started (Pure Simulation - No AI Model):"
	@echo "   API Simulator: http://localhost:$(PORT)"
	@echo "   Web UI:       http://localhost:$(OPENWEBUI_PORT)"
	@echo ""
	@echo "Give the UI 15s to initialize, then:"
	@echo "   make open"

compose-down-noai:
	@echo "🛑 Stopping Docker Compose (No AI)"
	docker compose -f docker-compose.noai.yml down

compose-logs:
	@echo "📋 Tailing compose logs..."
	docker compose logs -f --tail=100

compose-openwebui:
	@echo "🐳 Starting only Open Web UI service"
	docker compose up -d openwebui

curl-stream:
	@echo "📡 Testing streaming text generation..."
	curl http://localhost:$(PORT)/v1/chat/completions \
		-H "Content-Type: application/json" \
		-d '{"model":"nanochat","messages":[{"role":"user","content":"Say hello in 10 words or less."}],"stream":true}'

curl-text:
	@echo "📝 Testing non-streaming text generation..."
	curl http://localhost:$(PORT)/v1/chat/completions \
		-H "Content-Type: application/json" \
		-d '{"model":"nanochat","messages":[{"role":"user","content":"Explain AI in one sentence."}],"stream":false}'

curl-sim:
	@echo "🧠 Testing pure simulation (fake model)..."
	curl http://localhost:$(PORT)/v1/chat/completions \
		-H "Content-Type: application/json" \
		-d '{"model":"gpt-4","messages":[{"role":"user","content":"Generate a fun fact about AI."}],"stream":true}'

help:
	@echo "🤖 OpenAI API Simulator with NanoChat (PyTorch)"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "Usage: make <target>"
	@echo
	@echo "📦 SETUP"
	@echo "  setup-dev             One-time setup: install Go, Python, and download nanochat model"
	@echo
	@echo "🔨 LOCAL DEVELOPMENT"
	@echo "  build                 Build Go API server binary (./server)"
	@echo "  run-sim               Run pure simulation (fake AI model, instant responses)"
	@echo "  local-dev             Run both Go API + Python inference server (PyTorch)"
	@echo "  run                   Run API server with default settings"
	@echo
	@echo "🐳 DOCKER (Single Service)"
	@echo "  docker-build          Build Docker image with PyTorch (Go + Python runtime)"
	@echo "  docker-run            Run Docker container (API: 8090, Inference: 8081)"
	@echo "  docker-build-baked    Build image WITH nanochat model baked in (~2GB larger)"
	@echo "  docker-run-baked      Run baked image (faster startup, no download)"
	@echo "  docker-clean          Remove Docker images"
	@echo
	@echo "🐳 DOCKER-COMPOSE (Multi-Service Stack)"
	@echo "  💡 TWO OPTIONS:"
	@echo ""
	@echo "  Option A: WITH NanoChat (Real PyTorch AI Model):"
	@echo "  compose-up              Start: API + NanoChat Inference + Web UI"
	@echo "  compose-down            Stop the entire stack"
	@echo "                          → Real AI responses, slower inference"
	@echo ""
	@echo "  Option B: WITHOUT AI (Pure Simulation):"
	@echo "  compose-up-noai         Start: API + Web UI (NO inference server)"
	@echo "  compose-down-noai       Stop the entire stack"
	@echo "                          → Fake AI responses, instant/no model needed"
	@echo ""
	@echo "  Common:"
	@echo "  compose-logs            Tail logs from all services"
	@echo "  compose-openwebui       Start only Open Web UI service"
	@echo
	@echo "🧪 TESTING & UTILITIES"
	@echo "  test                  Run Go test suite"
	@echo "  curl-sim              Test pure simulation (fake AI model)"
	@echo "  curl-stream           Test streaming with nanochat model"
	@echo "  curl-text             Test non-streaming with nanochat model"
	@echo "  open                  Open Web UI in browser"
	@echo "  fmt                   Format Go code"
	@echo "  tidy                  Tidy Go modules"
	@echo "  clean                 Remove built binaries"
	@echo
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "🚀 QUICK START WORKFLOWS"
	@echo
	@echo "  0️⃣  Pure Simulation (Fake AI, Instant) - Local:"
	@echo "      make run-sim"
	@echo "      make curl-sim  # In another terminal"
	@echo "      → No setup needed, fast feedback for development"
	@echo
	@echo "  0️⃣  Pure Simulation with Web UI (Docker):"
	@echo "      make compose-up-noai"
	@echo "      make open      # In another terminal"
	@echo "      → Docker-based, no NanoChat model, instant responses"
	@echo
	@echo "  1️⃣  Local Dev (Real AI via PyTorch):"
	@echo "      make setup-dev"
	@echo "      make local-dev"
	@echo "      make curl-stream  # In another terminal"
	@echo "      → Real nanochat model, slower but accurate"
	@echo
	@echo "  2️⃣  Docker Regular (PyTorch, downloads on first run ~45s):"
	@echo "      make docker-build && make docker-run"
	@echo "      make curl-stream  # In another terminal"
	@echo "      → Containerized inference, fresh download each time"
	@echo
	@echo "  3️⃣  Docker Baked (PyTorch model embedded, instant startup):"
	@echo "      make docker-build-baked && make docker-run-baked"
	@echo "      make curl-stream  # In another terminal"
	@echo "      → Containerized inference with pre-baked model (~2GB larger image)"
	@echo
	@echo "  4️⃣  Full Stack with Web UI (Docker Compose - WITH NanoChat):"
	@echo "      make compose-up"
	@echo "      make open  # Opens Web UI in browser"
	@echo "      → Complete stack with API, inference, and Open Web UI"
	@echo
	@echo "  4️⃣  Full Stack with Web UI (Docker Compose - NO AI):"
	@echo "      make compose-up-noai"
	@echo "      make open  # Opens Web UI in browser"
	@echo "      → Lightweight stack with API and Open Web UI (pure simulation)"
	@echo
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "⚙️  ENVIRONMENT VARIABLES (override at runtime)"
	@echo
	@echo "  API_PORT=<port>         Go API server port (default: 8090)"
	@echo "  INFERENCE_PORT=<port>   Python inference server port (default: 8081)"
	@echo "  OPENWEBUI_PORT=<port>   Web UI port (default: 3000)"
	@echo "  BAKED=1                 Use baked image variant in compose (default: 0)"
	@echo
	@echo "  Example: make docker-run API_PORT=9000 INFERENCE_PORT=9001"
	@echo
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "📚 DOCUMENTATION"
	@echo "  README.md                   Project overview"
	@echo "  IMPLEMENTATION_SUMMARY.md   Architecture details"
	@echo
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
