# Build
.PHONY: build build-linux build-darwin clean test lint

# Binary names
NODE_BINARY = kerrigan-node
CLI_BINARY = kerrigan-cli

# Build directories
BUILD_DIR = ./build
DIST_DIR = ./dist

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod
GOFMT = $(GOCMD) fmt
GOVET = $(GOCMD) vet
GOLINT = golangci-lint run

# Version info
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# OS/ARCH
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Test coverage
COVERAGE_DIR = coverage
COVERAGE_FILE = coverage.out

## help: Show this help message
help:
	@echo "Kerrigan v2 Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## build: Build all binaries for current platform
build: build-node build-cli

## build-node: Build node binary
build-node:
	@echo "Building node binary..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(NODE_BINARY) ./cmd/node
	@echo "Node binary built: $(BUILD_DIR)/$(NODE_BINARY)"

## build-cli: Build CLI binary
build-cli:
	@echo "Building CLI binary..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_BINARY) ./cmd/cli
	@echo "CLI binary built: $(BUILD_DIR)/$(CLI_BINARY)"

## build-linux: Build for Linux (amd64)
build-linux:
	@echo "Cross-compiling for Linux..."
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(NODE_BINARY)-linux-amd64 ./cmd/node
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(CLI_BINARY)-linux-amd64 ./cmd/cli
	@echo "Linux binaries built in $(DIST_DIR)/"

## build-darwin: Build for macOS
build-darwin:
	@echo "Cross-compiling for macOS..."
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(NODE_BINARY)-darwin-amd64 ./cmd/node
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(CLI_BINARY)-darwin-amd64 ./cmd/cli
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(NODE_BINARY)-darwin-arm64 ./cmd/node
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(CLI_BINARY)-darwin-arm64 ./cmd/cli
	@echo "macOS binaries built in $(DIST_DIR)/"

## test: Run all tests
test:
	@echo "Running tests..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -race -coverprofile=$(COVERAGE_DIR)/$(COVERAGE_FILE) -covermode=atomic ./...

## test-unit: Run unit tests only
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v -short ./...

## test-integration: Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./...

## coverage: Generate coverage report
coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/$(COVERAGE_FILE) -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report: $(COVERAGE_DIR)/coverage.html"

## lint: Run linters
lint:
	@echo "Running linters..."
	$(GOFMT) ./...
	$(GOVET) ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

## tidy: Tidy go modules
tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy
	$(GOMOD) verify

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR) $(COVERAGE_DIR)
	@echo "Clean complete"

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t kerrigan/node:$(VERSION) -f Dockerfile .
	docker tag kerrigan/node:$(VERSION) kerrigan/node:latest

## docker-push: Push Docker image
docker-push: docker-build
	@echo "Pushing Docker image..."
	docker push kerrigan/node:$(VERSION)
	docker push kerrigan/node:latest

## run-node: Run node locally
run-node:
	@echo "Running node..."
	$(GOCMD) run ./cmd/node

## run-cli: Run CLI
run-cli:
	@echo "Running CLI..."
	$(GOCMD) run ./cmd/cli

## deps: Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

## check: Run all checks (fmt, vet, lint, test)
check: fmt lint test

## gen-proto: Generate protobuf files
gen-proto:
	@echo "Generating protobuf files..."
	protoc --go_out=. --go-grpc_out=. ./internal/api/grpc/**/*.proto

## version: Show version info
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(shell go version)"
	@echo "GOOS: $(GOOS)"
	@echo "GOARCH: $(GOARCH)"
