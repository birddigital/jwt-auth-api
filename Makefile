.PHONY: all build run test clean coverage benchmark docker-build docker-run lint fmt vet help

# Variables
APP_NAME=jwt-auth-api
GO=go
GOFLAGS=-v
DOCKER_IMAGE=$(APP_NAME):latest
DOCKER_CONTAINER=$(APP_NAME)-container

# Default target
all: fmt vet test build

## build: Build the application
build:
	@echo "Building $(APP_NAME)..."
	$(GO) build $(GOFLAGS) -o $(APP_NAME) .

## run: Run the application
run:
	@echo "Running $(APP_NAME)..."
	$(GO) run .

## test: Run all tests
test:
	@echo "Running tests..."
	$(GO) test -v ./...

## test-cover: Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## benchmark: Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem ./...

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(APP_NAME)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

## lint: Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GO) vet ./...

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -d \
		--name $(DOCKER_CONTAINER) \
		-p 8080:8080 \
		-e JWT_SECRET=test-secret-key-12345678901234567890 \
		-e REFRESH_SECRET=test-refresh-key-12345678901234567890 \
		$(DOCKER_IMAGE)

## docker-stop: Stop Docker container
docker-stop:
	@echo "Stopping Docker container..."
	docker stop $(DOCKER_CONTAINER) || true
	docker rm $(DOCKER_CONTAINER) || true

## docker-logs: Show Docker logs
docker-logs:
	docker logs -f $(DOCKER_CONTAINER)

## install-tools: Install development tools
install-tools:
	@echo "Installing development tools..."
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Tools installed successfully"

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
