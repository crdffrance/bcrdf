# Makefile for BCRDF - Index-based backup system
# Usage: make [target]

# Variables
BINARY_NAME=bcrdf
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=${VERSION}"

# Colors for messages
GREEN=\033[0;32m
YELLOW=\033[1;33m
RED=\033[0;31m
NC=\033[0m # No Color

.PHONY: help build clean test install dev-build lint format setup clean-all build-all

# Default target
all: build

# Show help
help:
	@echo "$(GREEN)BCRDF - Index-based backup system$(NC)"
	@echo ""
	@echo "$(YELLOW)Available commands:$(NC)"
	@echo "  build          - Build the project"
	@echo "  clean          - Clean temporary files"
	@echo "  test           - Run tests"
	@echo "  install        - Install BCRDF"
	@echo "  dev-build      - Quick build for development"
	@echo "  lint           - Run linter"
	@echo "  format         - Format code"
	@echo "  setup          - Setup development environment"
	@echo "  build-all      - Build for all platforms"
	@echo "  clean-all      - Deep clean"
	@echo ""
	@echo "$(YELLOW)Usage examples:$(NC)"
	@echo "  make build     # Build BCRDF"
	@echo "  make test      # Run tests"
	@echo "  make install   # Install to system"

# Build the project
build:
	@echo "$(YELLOW)🔨 Building BCRDF...$(NC)"
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) cmd/bcrdf/main.go
	@echo "$(GREEN)✅ Binary created: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

# Quick development build
dev-build:
	@echo "$(YELLOW)⚡ Quick build...$(NC)"
	go build -o $(BINARY_NAME) cmd/bcrdf/main.go
	@echo "$(GREEN)✅ Binary created: $(BINARY_NAME)$(NC)"

# Clean temporary files
clean:
	@echo "$(YELLOW)🧹 Cleaning...$(NC)"
	rm -f $(BINARY_NAME)
	rm -f *.exe
	rm -f *.test
	rm -rf restored-*/
	rm -f *.log
	rm -f *.tmp
	@echo "$(GREEN)✅ Cleanup completed$(NC)"

# Deep clean
clean-all: clean
	@echo "$(YELLOW)🧹 Deep cleaning...$(NC)"
	rm -rf $(BUILD_DIR)
	rm -rf dist
	go clean -cache
	go clean -testcache
	go clean -modcache
	@echo "$(GREEN)✅ Deep cleanup completed$(NC)"

# Run tests
test:
	@echo "$(YELLOW)🧪 Running tests...$(NC)"
	go test -v ./...
	@echo "$(GREEN)✅ Tests completed$(NC)"

# Run tests with coverage
test-coverage:
	@echo "$(YELLOW)🧪 Running tests with coverage...$(NC)"
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✅ Coverage report generated: coverage.html$(NC)"

# Install BCRDF
install: build
	@echo "$(YELLOW)📦 Installing BCRDF...$(NC)"
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "$(GREEN)✅ BCRDF installed to /usr/local/bin/$(NC)"

# Uninstall BCRDF
uninstall:
	@echo "$(YELLOW)🗑️  Uninstalling BCRDF...$(NC)"
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "$(GREEN)✅ BCRDF uninstalled$(NC)"

# Run linter
lint:
	@echo "$(YELLOW)🔍 Running linter...$(NC)"
	golangci-lint run
	@echo "$(GREEN)✅ Linting completed$(NC)"

# Format code
format:
	@echo "$(YELLOW)📝 Formatting code...$(NC)"
	go fmt ./...
	@echo "$(GREEN)✅ Code formatted$(NC)"

# Setup development environment
setup:
	@echo "$(YELLOW)⚙️  Setting up development environment...$(NC)"
	go mod tidy
	go mod download
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@echo "$(GREEN)✅ Development environment ready$(NC)"

# Build for all platforms
build-all:
	@echo "$(YELLOW)🏗️  Building for all platforms...$(NC)"
	@mkdir -p dist
	
	# Linux x64
	@echo "Building for Linux x64..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-x64 cmd/bcrdf/main.go
	
	# Linux ARM64
	@echo "Building for Linux ARM64..."
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 cmd/bcrdf/main.go
	
	# macOS x64
	@echo "Building for macOS x64..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-x64 cmd/bcrdf/main.go
	
	# macOS ARM64 (Apple Silicon)
	@echo "Building for macOS ARM64..."
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 cmd/bcrdf/main.go
	
	# Windows x64
	@echo "Building for Windows x64..."
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-x64.exe cmd/bcrdf/main.go
	
	@echo "$(GREEN)✅ All binaries built in dist/$(NC)"
	@ls -la dist/

# Create release packages
release: build-all
	@echo "$(YELLOW)📦 Creating release packages...$(NC)"
	@mkdir -p dist/packages
	
	# Linux x64 package
	@mkdir -p dist/tmp/bcrdf-linux-x64
	@cp dist/bcrdf-linux-x64 dist/tmp/bcrdf-linux-x64/bcrdf
	@cp README.md LICENSE dist/tmp/bcrdf-linux-x64/
	@cp -r configs dist/tmp/bcrdf-linux-x64/
	@cp -r scripts dist/tmp/bcrdf-linux-x64/
	@tar -czf dist/packages/bcrdf-linux-x64.tar.gz -C dist/tmp bcrdf-linux-x64
	
	# macOS x64 package
	@mkdir -p dist/tmp/bcrdf-darwin-x64
	@cp dist/bcrdf-darwin-x64 dist/tmp/bcrdf-darwin-x64/bcrdf
	@cp README.md LICENSE dist/tmp/bcrdf-darwin-x64/
	@cp -r configs dist/tmp/bcrdf-darwin-x64/
	@cp -r scripts dist/tmp/bcrdf-darwin-x64/
	@tar -czf dist/packages/bcrdf-darwin-x64.tar.gz -C dist/tmp bcrdf-darwin-x64
	
	# Windows x64 package
	@mkdir -p dist/tmp/bcrdf-windows-x64
	@cp dist/bcrdf-windows-x64.exe dist/tmp/bcrdf-windows-x64/bcrdf.exe
	@cp README.md LICENSE dist/tmp/bcrdf-windows-x64/
	@cp -r configs dist/tmp/bcrdf-windows-x64/
	@cp -r scripts dist/tmp/bcrdf-windows-x64/
	@cd dist/tmp && zip -r ../packages/bcrdf-windows-x64.zip bcrdf-windows-x64
	
	@rm -rf dist/tmp
	@echo "$(GREEN)✅ Release packages created in dist/packages/$(NC)"
	@ls -la dist/packages/

# Show project statistics
stats:
	@echo "$(YELLOW)📊 Project Statistics$(NC)"
	@echo ""
	@echo "Lines of code:"
	@find . -name "*.go" -not -path "./vendor/*" | xargs wc -l | tail -1
	@echo ""
	@echo "Go files:"
	@find . -name "*.go" -not -path "./vendor/*" | wc -l
	@echo ""
	@echo "Packages:"
	@go list ./... | wc -l

# Run benchmarks
bench:
	@echo "$(YELLOW)🏃 Running benchmarks...$(NC)"
	go test -bench=. -benchmem ./...
	@echo "$(GREEN)✅ Benchmarks completed$(NC)"