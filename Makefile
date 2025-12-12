.PHONY: build test run clean deploy docker-build docker-run

# Default target
all: test build

# Build the Cloud Function
build:
	go build -o bin/pubsub-shovel ./cmd

# Build the fast message generator
build-generator:
	go build -o bin/generate-fast ./hack/generate-messages-fast.go

# Build all tools
build-all: build build-generator

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run the local server
run:
	go run ./cmd

# Clean build artifacts
clean:
	rm -f bin/* coverage.out coverage.html
	rm -rf bin/

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Download dependencies
deps:
	go mod download
	go mod tidy

# Deploy to Google Cloud Functions
deploy:
	./deploy.sh

# Build Docker image
docker-build:
	docker build -t pubsub-shovel:latest .

# Run Docker container locally
docker-run:
	docker run -p 8080:8080 -e GOOGLE_APPLICATION_CREDENTIALS=/path/to/your/credentials.json pubsub-shovel:latest

# Development setup
dev-setup: deps
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Help target
help:
	@echo "Available targets:"
	@echo "  build          - Build the Cloud Function"
	@echo "  build-local    - Build the local server"
	@echo "  build-all      - Build both variants"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  run            - Run the local server directly"
	@echo "  run-local      - Build and run local server binary"
	@echo "  clean          - Clean build artifacts"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  deps           - Download and tidy dependencies"
	@echo "  deploy         - Deploy to Google Cloud Functions"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container locally"
	@echo "  dev-setup      - Set up development environment"
	@echo "  help           - Show this help message"
