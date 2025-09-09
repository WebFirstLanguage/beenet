# Beenet Build System
# Phase 0 - Project Bootstrap Infrastructure

.PHONY: all build test clean fmt lint vet race fuzz deps tidy check install cross-compile release help

# Build configuration
BINARY_NAME := bee
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go configuration
GO := go
GOFLAGS := -trimpath
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.commitHash=$(COMMIT_HASH)

# Directories
BUILD_DIR := build
DIST_DIR := dist
COVERAGE_DIR := coverage

# Cross-compilation targets
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

# Default target
all: check build

# Build the binary
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/bee

# Install the binary
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./cmd/bee

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test -v -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report generated: $(COVERAGE_DIR)/coverage.html"

# Run race detector tests
race:
	@echo "Running race detector tests..."
	$(GO) test -race -v ./...

# Run fuzz tests
fuzz:
	@echo "Running fuzz tests..."
	@for pkg in $$($(GO) list ./... | grep -E "(codec|wire|identity)"); do \
		echo "Fuzzing $$pkg..."; \
		$(GO) test -fuzz=. -fuzztime=30s $$pkg || true; \
	done

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, installing..."; \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run; \
	fi

# Vet code
vet:
	@echo "Vetting code..."
	$(GO) vet ./...

# Check code quality (format, lint, vet)
check: fmt vet lint

# Update dependencies
deps:
	@echo "Updating dependencies..."
	$(GO) get -u ./...
	$(GO) mod tidy

# Tidy go.mod
tidy:
	@echo "Tidying go.mod..."
	$(GO) mod tidy

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(COVERAGE_DIR)
	$(GO) clean -cache -testcache -modcache

# Cross-compile for all platforms
cross-compile:
	@echo "Cross-compiling for all platforms..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output_name=$(BINARY_NAME)-$(VERSION)-$$os-$$arch; \
		if [ $$os = "windows" ]; then output_name=$$output_name.exe; fi; \
		echo "Building for $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$$output_name ./cmd/bee; \
	done

# Create release archives
release: cross-compile
	@echo "Creating release archives..."
	@cd $(DIST_DIR) && for file in $(BINARY_NAME)-$(VERSION)-*; do \
		if [[ $$file == *.exe ]]; then \
			zip $$file.zip $$file; \
		else \
			tar -czf $$file.tar.gz $$file; \
		fi; \
	done
	@echo "Release archives created in $(DIST_DIR)/"

# Run all quality checks and tests
ci: check test race
	@echo "All CI checks passed!"

# Development workflow
dev: clean deps check test build
	@echo "Development build complete!"

# Benchmark tests
bench:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem ./...

# Generate golden test vectors
golden:
	@echo "Generating golden test vectors..."
	$(GO) test -v -run=TestCanonicalEncoding ./pkg/codec/cborcanon
	$(GO) test -v -run=TestHoneytagGeneration ./pkg/identity

# Security scan
security:
	@echo "Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not found, installing..."; \
		$(GO) install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
		gosec ./...; \
	fi

# Show help
help:
	@echo "Beenet Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  all            - Run checks and build (default)"
	@echo "  build          - Build the binary"
	@echo "  install        - Install the binary"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  race           - Run race detector tests"
	@echo "  fuzz           - Run fuzz tests"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  vet            - Vet code"
	@echo "  check          - Run all code quality checks"
	@echo "  deps           - Update dependencies"
	@echo "  tidy           - Tidy go.mod"
	@echo "  clean          - Clean build artifacts"
	@echo "  cross-compile  - Cross-compile for all platforms"
	@echo "  release        - Create release archives"
	@echo "  ci             - Run all CI checks"
	@echo "  dev            - Development workflow"
	@echo "  bench          - Run benchmarks"
	@echo "  golden         - Generate golden test vectors"
	@echo "  security       - Run security scan"
	@echo "  help           - Show this help"
	@echo ""
	@echo "Build info:"
	@echo "  Version:     $(VERSION)"
	@echo "  Build time:  $(BUILD_TIME)"
	@echo "  Commit:      $(COMMIT_HASH)"
