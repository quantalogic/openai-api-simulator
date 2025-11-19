# 🤖 OpenAI API Simulator with SmolLM - Makefile
# This project simulates OpenAI API responses with optional real SmolLM PyTorch inference

BINARY=server
PORT?=8090
INFERENCE_PORT?=8081
OPENWEBUI_PORT?=3000
IMAGE?=openai-api-simulator:latest
IMAGE_BAKED?=openai-api-simulator:baked
SHELL := /bin/bash

# Default target - show help
.DEFAULT_GOAL := help

.PHONY: help setup-dev build run run-sim local-dev test tidy fmt clean \
        docker-build docker-run docker-build-baked docker-run-baked docker-clean \
        compose-up compose-down compose-up-noai compose-down-noai compose-logs compose-openwebui \
        curl-stream curl-text curl-sim open stop-bg run-sim-bg wait-for-api wait-for-ui

# ─────────────────────────────────────────────────────────────────────────────
# 📚 HELP TARGET (DEFAULT)
# ─────────────────────────────────────────────────────────────────────────────

help:
	@echo ""
	@echo "  🤖 OpenAI API Simulator with SmolLM PyTorch Inference"
	@echo "  ═══════════════════════════════════════════════════════════════"
	@echo ""
	@echo "  📦 SETUP"
	@echo "     setup-dev                 • Initialize Go, Python, download SmolLM model"
	@echo ""
	@echo "  🔨 LOCAL DEVELOPMENT"
	@echo "     build                     • Build Go API server (./server)"
	@echo "     run-sim                   • Run pure simulation (blocking)"
	@echo "     run-sim-bg                • Run pure simulation in background"
	@echo "     stop-bg                   • Stop background server"
	@echo "     local-dev                 • Run with real PyTorch inference (blocking)"
	@echo "     run                       • Run with default settings"
	@echo ""
	@echo "  🐳 DOCKER (Single Service)"
	@echo "     docker-build              • Build image with PyTorch runtime"
	@echo "     docker-run                • Run container (API: 8090, Inference: 8081)"
	@echo "     docker-build-baked        • Build with SmolLM model embedded (~400MB)"
	@echo "     docker-run-baked          • Run baked image (faster startup)"
	@echo "     docker-clean              • Remove Docker images and containers"
	@echo ""
	@echo "  🐳 DOCKER-COMPOSE (Complete Stack)"
	@echo "     compose-up                • Start: API + SmolLM + Web UI"
	@echo "     compose-down              • Stop the stack"
	@echo "     compose-up-noai           • Start: API + Web UI (no inference)"
	@echo "     compose-down-noai         • Stop the no-AI stack"
	@echo "     compose-logs              • Tail logs from all services"
	@echo "     compose-openwebui         • Start only the Web UI"
	@echo ""
	@echo "  🧪 TESTING & UTILITIES"
	@echo "     test                      • Run Go test suite"
	@echo "     curl-sim                  • Test pure simulation endpoint"
	@echo "     curl-stream               • Test streaming with SmolLM"
	@echo "     curl-text                 • Test non-streaming with SmolLM"
	@echo "     open                      • Open Web UI in browser (auto-waits for ready)"
	@echo "     fmt                       • Format Go code"
	@echo "     tidy                      • Tidy Go modules"
	@echo "     clean                     • Remove binaries"
	@echo ""
	@echo "  🔄 HEALTHCHECKS & WAITING"
	@echo "     wait-for-api              • Wait for API to be ready (max 60s)"
	@echo "     wait-for-ui               • Wait for Web UI to be ready (max 60s)"
	@echo ""
	@echo "  ⚡ QUICK START"
	@echo "     Fastest (Fake AI, 2 terminals):"
	@echo "        Terminal 1: make run-sim"
	@echo "        Terminal 2: make curl-sim"
	@echo ""
	@echo "     Fastest (Fake AI, single terminal):"
	@echo "        make run-sim-bg && make curl-sim && make stop-bg"
	@echo ""
	@echo "     Web UI + Fake AI (auto-waits for init):"
	@echo "        make compose-up-noai && make open"
	@echo ""
	@echo "     Web UI + Real AI (auto-waits for init):"
	@echo "        make compose-up && make open"
	@echo ""
	@echo "  🔧 ENVIRONMENT VARIABLES"
	@echo "     PORT=<n>                  • API port (default: 8090)"
	@echo "     INFERENCE_PORT=<n>        • Inference port (default: 8081)"
	@echo "     OPENWEBUI_PORT=<n>        • Web UI port (default: 3000)"
	@echo ""
	@echo "  ═══════════════════════════════════════════════════════════════"
	@echo ""

# ─────────────────────────────────────────────────────────────────────────────
# 📦 SETUP
# ─────────────────────────────────────────────────────────────────────────────

setup-dev:
	@echo "🔧 Setting up development environment..."
	@echo "   - Enabling GO111MODULE=on"
	go env -w GO111MODULE=on
	@echo "   - Downloading Go dependencies"
	go mod download
	@echo "   - Setting up Python environment"
	./scripts/setup-smollm.sh
	@echo "✅ Development setup complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  Terminal 1: python cmd/nanochat/inference_server.py --port 8081"
	@echo "  Terminal 2: go run ./cmd/server -port 8090"
	@echo "  Terminal 3: curl http://localhost:8090/v1/chat/completions ..."

# ─────────────────────────────────────────────────────────────────────────────
# 🔨 BUILD & RUN
# ─────────────────────────────────────────────────────────────────────────────

build:
	@echo "🔨 Building Go API server..."
	go build -o $(BINARY) ./cmd/server
	@echo "✅ Built: ./$(BINARY)"

run: build
	@echo "🚀 Running API server on port $(PORT)..."
	./$(BINARY) -port $(PORT)

run-sim: build
	@echo "🧠 Running pure simulation (fake AI, instant responses)"
	./$(BINARY) -port $(PORT)

run-sim-bg: build
	@echo "🧠 Starting server in background on port $(PORT)..."
	./$(BINARY) -port $(PORT) > /tmp/openai-simulator.log 2>&1 &
	@echo "✅ Server PID: $$!"
	@sleep 1
	@echo "   To view logs: tail -f /tmp/openai-simulator.log"
	@echo "   To stop: pkill -f '\\./(BINARY)' or use: make stop-bg"

local-dev: build
	@echo "🚀 Starting local development..."
	@echo "   API port: $(PORT)"
	@echo "   Inference port: $(INFERENCE_PORT)"
	@echo ""
	@echo "⚠️  Make sure Python inference server is running:"
	@echo "   python cmd/nanochat/inference_server.py --port $(INFERENCE_PORT)"
	@echo ""
	./$(BINARY) -port $(PORT)

stop-bg:
	@echo "🛑 Stopping background server..."
	@pkill -f '\./$(BINARY)' || echo "✅ No server running"
	@echo "✅ Stopped"

# ─────────────────────────────────────────────────────────────────────────────
# 🧪 TESTING & CODE QUALITY
# ─────────────────────────────────────────────────────────────────────────────

test:
	@echo "🧪 Running tests..."
	go test ./... -v

fmt:
	@echo "📝 Formatting Go code..."
	gofmt -w .

tidy:
	@echo "📦 Tidying Go modules..."
	go mod tidy

clean:
	@echo "🧹 Cleaning up..."
	rm -f $(BINARY)
	@echo "✅ Cleaned"

# ─────────────────────────────────────────────────────────────────────────────
# 🐳 DOCKER (Single Service)
# ─────────────────────────────────────────────────────────────────────────────

docker-build:
	@echo "📦 Building Docker image: $(IMAGE)"
	DOCKER_BUILDKIT=1 docker build -t $(IMAGE) .
	@echo "✅ Built: $(IMAGE)"

docker-run: docker-build
	@echo "🐳 Running Docker image"
	@echo "   API: http://localhost:$(PORT)"
	@echo "   Inference: http://localhost:$(INFERENCE_PORT)"
	docker run --rm -p $(PORT):8090 -p $(INFERENCE_PORT):8081 \
		--name openai-api-simulator \
		-e PYTHONUNBUFFERED=1 \
		$(IMAGE)

docker-build-baked:
	@echo "📦 Building Docker image with baked SmolLM model..."
	@echo "⚠️  This may take 5-10 minutes (downloads 386MB GGUF)"
	DOCKER_BUILDKIT=1 docker build --build-arg BAKED=true -t $(IMAGE_BAKED) .
	@echo "✅ Built: $(IMAGE_BAKED)"

docker-run-baked: docker-build-baked
	@echo "🐳 Running baked Docker image (faster startup)"
	@echo "   API: http://localhost:$(PORT)"
	@echo "   Inference: http://localhost:$(INFERENCE_PORT)"
	docker run --rm -p $(PORT):8090 -p $(INFERENCE_PORT):8081 \
		--name openai-api-simulator \
		-e PYTHONUNBUFFERED=1 \
		$(IMAGE_BAKED)

docker-clean:
	@echo "🧹 Cleaning Docker images and containers..."
	-docker stop openai-api-simulator || true
	-docker rmi $(IMAGE) || true
	-docker rmi $(IMAGE_BAKED) || true
	@echo "✅ Cleanup complete"

# ─────────────────────────────────────────────────────────────────────────────
# 🐳 DOCKER-COMPOSE (Complete Stack)
# ─────────────────────────────────────────────────────────────────────────────

wait-for-api:
	@echo "⏳ Waiting for API to be ready on port $(PORT)..."
	@for i in {1..60}; do \
		if curl -s http://localhost:$(PORT)/health > /dev/null 2>&1; then \
			echo "✅ API is ready"; \
			exit 0; \
		fi; \
		echo -n "."; \
		sleep 1; \
	done; \
	echo ""; \
	echo "❌ API failed to start"; \
	exit 1

wait-for-ui:
	@echo "⏳ Waiting for Web UI to be ready on port $(OPENWEBUI_PORT)..."
	@for i in {1..60}; do \
		if curl -s http://localhost:$(OPENWEBUI_PORT) > /dev/null 2>&1; then \
			echo "✅ Web UI is ready"; \
			exit 0; \
		fi; \
		echo -n "."; \
		sleep 1; \
	done; \
	echo ""; \
	echo "❌ Web UI failed to start"; \
	exit 1

compose-up:
	@echo "🐳 Starting Docker Compose stack..."
	@echo "   Services: API + SmolLM Inference + Open Web UI"
	docker compose up --build -d
	@echo ""
	@echo "✅ Stack started! Waiting for services to initialize..."
	@$(MAKE) wait-for-api
	@$(MAKE) wait-for-ui
	@echo ""
	@echo "🎉 All services ready!"
	@echo "   API: http://localhost:$(PORT)"
	@echo "   Inference: http://localhost:$(INFERENCE_PORT)"
	@echo "   Web UI: http://localhost:$(OPENWEBUI_PORT)"
	@echo ""
	@echo "💡 Tip: make open"

compose-down:
	@echo "🛑 Stopping Docker Compose stack..."
	docker compose down
	@echo "✅ Stopped"

compose-up-noai:
	@echo "🐳 Starting Docker Compose (without SmolLM)..."
	@echo "   Services: API + Open Web UI (pure simulation)"
	docker compose -f docker-compose.noai.yml up --build -d
	@echo ""
	@echo "✅ Stack started! Waiting for services to initialize..."
	@$(MAKE) wait-for-api
	@$(MAKE) wait-for-ui
	@echo ""
	@echo "🎉 All services ready!"
	@echo "   API: http://localhost:$(PORT)"
	@echo "   Web UI: http://localhost:$(OPENWEBUI_PORT)"
	@echo ""
	@echo "💡 Tip: make open"

compose-down-noai:
	@echo "🛑 Stopping Docker Compose (no-AI)..."
	docker compose -f docker-compose.noai.yml down
	@echo "✅ Stopped"

compose-logs:
	@echo "📋 Tailing Docker Compose logs (last 100 lines)..."
	docker compose logs -f --tail=100

compose-openwebui:
	@echo "🐳 Starting Open Web UI service..."
	docker compose up -d openwebui
	@echo "✅ Started on http://localhost:$(OPENWEBUI_PORT)"

# ─────────────────────────────────────────────────────────────────────────────
# 🔗 UTILITIES & API TESTING
# ─────────────────────────────────────────────────────────────────────────────

open:
	@echo "🌐 Opening Web UI in browser..."
	@$(MAKE) wait-for-ui
	@open http://localhost:$(OPENWEBUI_PORT) || xdg-open http://localhost:$(OPENWEBUI_PORT) || echo "Please open http://localhost:$(OPENWEBUI_PORT)"

curl-sim:
	@echo "🧠 Testing pure simulation endpoint..."
	@echo ""
	curl -s -X POST http://localhost:$(PORT)/v1/chat/completions \
		-H "Content-Type: application/json" \
		-d '{"model":"gpt-4","messages":[{"role":"user","content":"Generate a fun fact about AI."}],"stream":true}' | head -20

curl-stream:
	@echo "📡 Testing streaming with SmolLM..."
	@echo ""
	curl -s -X POST http://localhost:$(PORT)/v1/chat/completions \
		-H "Content-Type: application/json" \
		-d '{"model":"smollm","messages":[{"role":"user","content":"Say hello in 10 words or less."}],"stream":true}' | head -20

curl-text:
	@echo "📝 Testing non-streaming with SmolLM..."
	@echo ""
	curl -s -X POST http://localhost:$(PORT)/v1/chat/completions \
		-H "Content-Type: application/json" \
		-d '{"model":"smollm","messages":[{"role":"user","content":"Explain AI in one sentence."}],"stream":false}' | head -20
