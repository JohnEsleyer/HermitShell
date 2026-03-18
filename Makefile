.PHONY: all build setup install build-ui build-server build-docker dev clean test lint run

# Default target - builds everything
all: build

# Setup - builds Docker image and installs dependencies
setup: build-docker
	@echo "Setup complete! Run 'make run' to start Hermit."

# Build everything (UI + Server + CLI + Docker image)
build: build-ui build-server build-cli build-docker

# Build only the frontend
build-ui:
	@echo "Building frontend..."
	cd dashboard && npm run build
	@echo "Frontend built successfully."

# Build only the Go server
build-server:
	@echo "Building server..."
	go build -o hermit ./cmd/hermit/main.go
	@echo "Server built successfully."

# Build CLI
build-cli:
	@echo "Building CLI..."
	go build -o hermit-cli ./cmd/cli/main.go
	@echo "CLI built successfully."

# Build Docker image for agents
build-docker:
	@echo "Building Docker image (hermit-agent:latest)..."
	docker build -t hermit-agent:latest .
	@echo "Docker image built successfully."

# Development server (runs Go server)
dev:
	go run ./cmd/hermit/main.go

# Run production server
run: build
	./hermit

# Run tests
test:
	go test ./... -v

# Run linter
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -f hermit
	cd dashboard && rm -rf dist
	@echo "Clean complete."

# Help target
help:
	@echo "Hermit - AI Agent Orchestrator"
	@echo ""
	@echo "Available targets:"
	@echo "  setup          - Build Docker image and prepare for first run"
	@echo "  build          - Build frontend, server, and Docker image"
	@echo "  build-ui       - Build the React dashboard"
	@echo "  build-server   - Build the Go server binary"
	@echo "  build-docker   - Build the hermit-agent Docker image"
	@echo "  dev            - Run the server in development mode"
	@echo "  run            - Build and run the production server"
	@echo "  test           - Run tests"
	@echo "  lint           - Run linter"
	@echo "  clean          - Remove build artifacts"
	@echo "  help           - Show this help message"
