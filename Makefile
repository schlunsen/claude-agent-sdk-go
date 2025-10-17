.PHONY: help build test fmt lint clean coverage

help:
	@echo "Claude Agent SDK for Go - Development Tasks"
	@echo ""
	@echo "Available targets:"
	@echo "  make build       - Build the SDK"
	@echo "  make test        - Run all tests"
	@echo "  make fmt         - Format code with gofmt"
	@echo "  make lint        - Run go vet and golangci-lint"
	@echo "  make coverage    - Run tests with coverage report"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make examples    - Build all examples"
	@echo ""

build:
	@echo "Building SDK..."
	go build ./...
	@echo "Build complete"

test:
	@echo "Running tests..."
	go test -v ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

lint:
	@echo "Running linters..."
	go vet ./...
	@if command -v golangci-lint > /dev/null; then golangci-lint run ./...; else echo "golangci-lint not installed, skipping"; fi

coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

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
