# Makefile for OpenAI API Simulator

BINARY=server
PORT?=3080
SIM_PORT?=3080
OPENWEBUI_PORT?=3000
IMAGE?=openai-api-simulator:latest
SHELL := /bin/bash

.PHONY: all build run test tidy clean fmt help compose-logs compose-openwebui docker-run-openwebui docker-run docker-clean open setup
setup:
	@echo "Preparing development environment..."
	@echo "Setting GOPRIVATE to prevent proxy lookups for private modules..."
	go env -w GOPRIVATE=github.com/quantalogic
	@echo "Running 'go mod tidy'..."
	go mod tidy
	@echo "✓ Setup complete! You can now run 'make build' or 'make test'."

all: setup build

build:
	go build -o $(BINARY) ./cmd/server

run: build
	./$(BINARY) -port $(PORT)

test:
	go test ./... -v

tidy:
	go mod tidy

fmt:
	gofmt -w .

clean:
	rm -f $(BINARY)

docker-build:
	@echo "Building docker image $(IMAGE)"
	DOCKER_BUILDKIT=1 docker build -t $(IMAGE) .

docker-run: docker-build
	@echo "Running docker image on port $(PORT)"
	docker run --rm -p $(PORT):$(PORT) --name openai-api-simulator $(IMAGE)

docker-clean:
	-docker rm -f openai-api-simulator || true
	-docker rmi $(IMAGE) || true

docker-run-openwebui:
	@echo "Run Open Web UI pointing at host simulator (host.docker.internal will map to host machine)"
	docker run -d -p $(OPENWEBUI_PORT):8080 \
		-e OPENAI_API_BASE_URL=http://host.docker.internal:$(SIM_PORT) \
		-e OPENAI_API_KEY=simulator \
		-e WEBUI_AUTH=False \
		-v open-webui:/app/backend/data ghcr.io/open-webui/open-webui:main

open:
	@echo "Opening Open Web UI in your default browser (http://localhost:$(OPENWEBUI_PORT))"
	open http://localhost:$(OPENWEBUI_PORT) || true

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down

compose-logs:
	@echo "Tailing compose logs..."
	docker compose logs -f --tail=200

compose-openwebui:
	@echo "Starting only Open Web UI service (will start simulator if not present)"
	docker compose up -d openwebui

curl-stream:
	@echo "Streaming example (curl):"
	curl -N -X POST http://localhost:$(PORT)/v1/chat/completions \
	  -H 'Content-Type: application/json' \
	  -d '{"model":"gpt-sim-1","messages":[{"role":"user","content":"Hello"}],"stream":true}'

curl-text:
	@echo "Non-streaming example (curl):"
	curl -X POST http://localhost:$(PORT)/v1/chat/completions \
	  -H 'Content-Type: application/json' \
	  -d '{"model":"gpt-sim-1","messages":[{"role":"user","content":"Hello"}],"stream":false}'

help:
	@echo "OpenAI API Simulator - Makefile help"
	@echo "Usage: make <target>"
	@echo "Available targets:" 
	@echo "  build                - Build the simulator binary"
	@echo "  run PORT=<port>      - Run the simulator locally (default: 3080)"
	@echo "  docker-build         - Build the docker image"
	@echo "  docker-run           - Run simulator image on port $(PORT)"
	@echo "  docker-run-openwebui - Run Open Web UI container (connects to host simulator)"
	@echo "  compose-up           - Bring up compose stack (simulator + optional services)"
	@echo "  compose-openwebui    - Start only the openwebui service via docker compose"
	@echo "  compose-logs         - Tail the compose logs"
	@echo "  compose-down         - Stop the compose stack"
	@echo "  open                 - Open the Open Web UI in your default browser (macOS 'open')"
	@echo "  test                 - Run tests"
