.PHONY: help build test test-all test-integration bench fmt lint clean coverage

help:
	@echo "Claude Agent SDK for Go - Development Tasks"
	@echo ""
	@echo "Available targets:"
	@echo "  make build           - Build the SDK"
	@echo "  make test            - Run unit tests (default, fast)"
	@echo "  make test-all        - Run ALL tests including integration (spawns Claude processes)"
	@echo "  make test-integration - Run integration tests only (requires CLAUDE_API_KEY)"
	@echo "  make bench           - Run benchmarks"
	@echo "  make fmt             - Format code with gofmt"
	@echo "  make lint            - Run go vet and golangci-lint"
	@echo "  make coverage        - Run tests with coverage report"
	@echo "  make clean           - Clean build artifacts"
	@echo "  make examples        - Build all examples"
	@echo ""

build:
	@echo "Building SDK..."
	go build ./...
	@echo "Build complete"

test:
	@echo "Running unit tests (short mode)..."
	@echo "Note: Integration tests skipped. Use 'make test-all' to run all tests."
	go test -short -v ./...

test-all:
	@echo "Running ALL tests (including integration tests)..."
	@echo "WARNING: This will spawn Claude CLI processes if CLAUDE_API_KEY is set"
	go test -v ./...

test-integration:
	@echo "Running integration tests..."
	go test -v ./tests/...

bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./tests/...

fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

lint:
	@echo "Running linters..."
	go vet ./...
	@if command -v golangci-lint > /dev/null; then golangci-lint run ./...; else echo "golangci-lint not installed, skipping"; fi

coverage:
	@echo "Running tests with coverage (short mode)..."
	go test -short -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
	@echo "Note: Integration tests skipped. Use 'go test -coverprofile=coverage.out ./...' for full coverage."

clean:
	@echo "Cleaning build artifacts..."
	go clean -testcache
	rm -f coverage.out coverage.html
	@echo "Clean complete"

examples:
	@echo "Building examples..."
	@for dir in examples/*/; do \
		if [ -f "$$dir/main.go" ]; then \
			echo "Building $$dir..."; \
			(cd "$$dir" && go build -o app main.go && rm -f app); \
		fi; \
	done
	@echo "Examples built"

.DEFAULT_GOAL := help
