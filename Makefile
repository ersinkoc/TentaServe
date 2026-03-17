# Tentaserve Makefile
# Zero-dependency build system

# Build info
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo unknown)

# Directories
BIN_DIR := bin
DIST_DIR := dist
CMD_DIR := cmd/tentaserve

# Binary name
BINARY := tentaserve

# Go settings
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
GOMOD := $(GOCMD) mod
GOFMT := gofmt

# Build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE) -s -w"
CGO_ENABLED := 0

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) ./$(CMD_DIR)
	@echo "Built: $(BIN_DIR)/$(BINARY)"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter (go vet)
.PHONY: lint
lint:
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "Checking formatting..."
	$(GOFMT) -l .

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOFMT) -w .

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out coverage.html
	@echo "Cleaned."

# Build for multiple platforms
.PHONY: build-all
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p $(DIST_DIR)
	# Linux AMD64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-amd64 ./$(CMD_DIR)
	# Linux ARM64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-arm64 ./$(CMD_DIR)
	# Darwin AMD64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-amd64 ./$(CMD_DIR)
	# Darwin ARM64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-arm64 ./$(CMD_DIR)
	# Windows AMD64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-windows-amd64.exe ./$(CMD_DIR)
	@echo "All binaries built in $(DIST_DIR)/"

# Run the binary locally
.PHONY: run
run: build
	./$(BIN_DIR)/$(BINARY) serve --config tentaserve.yaml

# Validate config
.PHONY: validate
validate: build
	./$(BIN_DIR)/$(BINARY) validate --config tentaserve.yaml

# Show version
.PHONY: version
version: build
	./$(BIN_DIR)/$(BINARY) version

# Download dependencies (verify go.mod)
.PHONY: deps
deps:
	@echo "Verifying dependencies..."
	$(GOMOD) verify
	$(GOMOD) tidy
	@echo "Dependencies verified."

# Run benchmarks
.PHONY: bench
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Profile CPU
.PHONY: profile-cpu
profile-cpu: build
	@echo "Starting CPU profile..."
	./$(BIN_DIR)/$(BINARY) serve --config tentaserve.yaml --cpuprofile=cpu.prof

# Profile memory
.PHONY: profile-mem
profile-mem: build
	@echo "Starting memory profile..."
	./$(BIN_DIR)/$(BINARY) serve --config tentaserve.yaml --memprofile=mem.prof

# Docker build
.PHONY: docker
docker:
	@echo "Building Docker image..."
	docker build -t tentaserve:$(VERSION) .

# Help
.PHONY: help
help:
	@echo "Tentaserve Makefile targets:"
	@echo ""
	@echo "  build         - Build the binary"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  lint          - Run linter and format checker"
	@echo "  fmt           - Format code"
	@echo "  clean         - Clean build artifacts"
	@echo "  build-all     - Build for all platforms (linux, darwin, windows)"
	@echo "  run           - Build and run locally"
	@echo "  validate      - Validate configuration file"
	@echo "  version       - Show version"
	@echo "  deps          - Verify and tidy dependencies"
	@echo "  bench         - Run benchmarks"
	@echo "  docker        - Build Docker image"
	@echo "  help          - Show this help"
