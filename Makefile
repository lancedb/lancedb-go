# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright The LanceDB Authors

.PHONY: all build test clean install-deps install-act fmt lint lint-rust lint-go lint-go-fix lint-report examples prepare-examples run-examples run-example docs release check-libraries platform-info build-go ci-quick ci-format ci-build ci-test ci-security ci-docs ci-examples ci-local ci-list ci-stage1 ci-stage2 ci-stage3 ci-debug ci-clean ci-graph

# Default target
all: build test

# Build the Rust library and Go bindings
build:
	@echo "Building Rust library..."
	make build-native

# Platform detection variables
CURRENT_DIR := $(shell pwd)
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

# Normalize architecture names
ifeq ($(UNAME_M),x86_64)
	ARCH := amd64
else ifeq ($(UNAME_M),amd64)
	ARCH := amd64
else ifeq ($(UNAME_M),arm64)
	ARCH := arm64
else ifeq ($(UNAME_M),aarch64)
	ARCH := arm64
else
	$(error Unsupported architecture: $(UNAME_M))
endif

# Normalize platform names and set CGO flags
ifeq ($(UNAME_S),Darwin)
	PLATFORM := darwin
	FRAMEWORK_FLAGS := -framework Security -framework CoreFoundation
	CGO_LDFLAGS := $(CURRENT_DIR)/lib/darwin_$(ARCH)/liblancedb_go.a $(FRAMEWORK_FLAGS)
else ifeq ($(UNAME_S),Linux)
	PLATFORM := linux
	SYSTEM_LIBS := -lm -ldl -lpthread
	CGO_LDFLAGS := $(CURRENT_DIR)/lib/linux_$(ARCH)/liblancedb_go.a $(SYSTEM_LIBS)
else ifneq (,$(findstring MINGW,$(UNAME_S)))
	PLATFORM := windows
	ARCH := amd64
	CGO_LDFLAGS := $(CURRENT_DIR)/lib/windows_amd64/liblancedb_go.a
else ifneq (,$(findstring MSYS,$(UNAME_S)))
	PLATFORM := windows
	ARCH := amd64
	CGO_LDFLAGS := $(CURRENT_DIR)/lib/windows_amd64/liblancedb_go.a
else ifneq (,$(findstring CYGWIN,$(UNAME_S)))
	PLATFORM := windows
	ARCH := amd64
	CGO_LDFLAGS := $(CURRENT_DIR)/lib/windows_amd64/liblancedb_go.a
else
	$(error Unsupported platform: $(UNAME_S))
endif

# Set CGO flags
CGO_CFLAGS := -I$(CURRENT_DIR)/include
PLATFORM_ARCH := $(PLATFORM)_$(ARCH)

# Show platform information (useful for debugging)
platform-info:
	@echo "Platform Detection Information:"
	@echo "================================"
	@echo "Operating System: $(UNAME_S)"
	@echo "Architecture:     $(UNAME_M)"
	@echo "Normalized Platform: $(PLATFORM)"
	@echo "Normalized Arch:     $(ARCH)"
	@echo "Platform-Arch:       $(PLATFORM_ARCH)"
	@echo "Current Directory:   $(CURRENT_DIR)"
	@echo ""
	@echo "CGO Configuration:"
	@echo "=================="
	@echo "CGO_CFLAGS:  $(CGO_CFLAGS)"
	@echo "CGO_LDFLAGS: $(CGO_LDFLAGS)"
	@echo ""
	@echo "Expected Library Path: $(CURRENT_DIR)/lib/$(PLATFORM_ARCH)/"
	@if [ -d "$(CURRENT_DIR)/lib/$(PLATFORM_ARCH)" ]; then \
		echo "✅ Library directory exists"; \
		ls -la "$(CURRENT_DIR)/lib/$(PLATFORM_ARCH)/"; \
	else \
		echo "❌ Library directory not found: $(CURRENT_DIR)/lib/$(PLATFORM_ARCH)/"; \
		echo "   Run './scripts/download-artifacts.sh' to download platform libraries"; \
	fi

# Check if native libraries exist
check-libraries:
	@if [ ! -d "$(CURRENT_DIR)/lib/$(PLATFORM_ARCH)" ] || [ ! -f "$(CURRENT_DIR)/lib/$(PLATFORM_ARCH)/liblancedb_go.a" ]; then \
		echo "❌ Native libraries not found for platform: $(PLATFORM_ARCH)"; \
		echo "Expected: $(CURRENT_DIR)/lib/$(PLATFORM_ARCH)/liblancedb_go.a"; \
		echo ""; \
		echo "Please download the native libraries first:"; \
		echo "  ./scripts/download-artifacts.sh"; \
		echo ""; \
		echo "Or use the build helper script:"; \
		echo "  ./build.sh"; \
		exit 1; \
	fi
	@if [ ! -d "$(CURRENT_DIR)/include" ] || [ ! -f "$(CURRENT_DIR)/include/lancedb.h" ]; then \
		echo "⚠️ Header files not found, fallback to default one"; \
		echo "Expected: $(CURRENT_DIR)/include/lancedb.h"; \
		echo "Please download the artifacts using: ./scripts/download-artifacts.sh"; \
	fi

# Run tests
test: build check-libraries
	@echo "Running Go tests for $(PLATFORM_ARCH)..."
	@echo "CGO_CFLAGS: $(CGO_CFLAGS)"
	@echo "CGO_LDFLAGS: $(CGO_LDFLAGS)"
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go test -v ./...

# Run benchmarks
bench: build check-libraries
	@echo "Running benchmarks for $(PLATFORM_ARCH)..."
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go test -bench=. -benchmem ./...

# Quick build for user projects (convenience target)
build-go: check-libraries
	@echo "Building Go project with LanceDB native libraries..."
	@echo "Platform: $(PLATFORM_ARCH)"
	@echo "CGO_CFLAGS: $(CGO_CFLAGS)"
	@echo "CGO_LDFLAGS: $(CGO_LDFLAGS)"
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go build $(ARGS)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	cd rust && cargo clean
	go clean ./...
	rm -rf rust/target/

# Clean binary distribution files
clean-dist:
	@echo "Cleaning binary distribution files..."
	rm -rf lib/
	rm -rf include/

# Install development dependencies
install-deps:
	@echo "Installing Rust dependencies..."
	rustup update
	cargo install cbindgen
	@echo "Installing Go dependencies..."
	go mod download
	go mod tidy
	@echo "Installing golangci-lint..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.55.2)
	@echo "Development dependencies installed successfully!"

# Install act (GitHub Actions local runner)
install-act:
	@echo "Installing act (GitHub Actions local runner)..."
	@which act > /dev/null || ( \
		if [ "$$(uname)" = "Darwin" ]; then \
			echo "Installing act via Homebrew..."; \
			brew install act; \
		elif [ "$$(uname)" = "Linux" ]; then \
			echo "Installing act via script..."; \
			curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash; \
		else \
			echo "Please install act manually from https://github.com/nektos/act"; \
			exit 1; \
		fi \
	)
	@echo "Configuring act for Apple Silicon compatibility..."
	@mkdir -p "$$HOME/Library/Application Support/act"
	@if [ ! -f "$$HOME/Library/Application Support/act/actrc" ]; then \
		echo "Creating act configuration..."; \
		echo "-P ubuntu-latest=catthehacker/ubuntu:act-latest" > "$$HOME/Library/Application Support/act/actrc"; \
		echo "-P macos-latest=catthehacker/ubuntu:act-latest" >> "$$HOME/Library/Application Support/act/actrc"; \
		echo "-P macos-13=catthehacker/ubuntu:act-latest" >> "$$HOME/Library/Application Support/act/actrc"; \
		echo "--container-architecture linux/amd64" >> "$$HOME/Library/Application Support/act/actrc"; \
	fi
	@echo "✅ act installed and configured successfully!"

# Format code
fmt:
	@echo "Formatting Rust code..."
	cd rust && cargo fmt
	@echo "Formatting Go code..."
	go fmt ./...

# Lint code
lint: lint-rust lint-go

# Lint Rust code only
lint-rust:
	@echo "Linting Rust code..."
	cd rust && cargo clippy -- -D warnings

# Lint Go code only
lint-go:
	@echo "Linting Go code..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Run 'make install-deps' to install it." && exit 1)
	golangci-lint run --config .golangci.yml

# Lint Go code with fixes applied automatically
lint-go-fix:
	@echo "Linting and fixing Go code..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Run 'make install-deps' to install it." && exit 1)
	golangci-lint run --config .golangci.yml --fix

# Show detailed linting report
lint-report:
	@echo "Generating detailed linting reports..."
	@echo "=== Rust Clippy Report ==="
	cd rust && cargo clippy -- -D warnings
	@echo ""
	@echo "=== Go Linting Report ==="
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Run 'make install-deps' to install it." && exit 1)
	golangci-lint run --config .golangci.yml --out-format=colored-line-number

# Generate documentation
docs:
	@echo "Generating Rust documentation..."
	cd rust && cargo doc --no-deps
	@echo "Generating Go documentation..."
	@echo "Checking Go documentation generation..."
	@go doc ./pkg > /dev/null && echo "✅ Go documentation generated successfully"
	@echo "📁 Rust docs: rust/target/doc/lancedb_go/index.html"
	@echo "📁 Go docs: Use 'go doc ./pkg' to view Go documentation"
# Check code formatting
check-fmt:
	@echo "Checking Rust code formatting..."
	cd rust && cargo fmt -- --check
	@echo "Checking Go code formatting..."
	test -z "$$(gofmt -l .)"

# Build native libraries for current platform
build-native:
	@echo "Building native libraries for current platform..."
	./scripts/build-native.sh

# Build native libraries for all supported platforms
build-all-platforms:
	@echo "Building native libraries for all platforms..."
	./scripts/build-all-platforms.sh

# Create a release build (legacy - use build-native instead)
release: clean build-native
	@echo "Release build complete (using binary distribution)"

# Install pre-commit hooks
install-hooks:
	@echo "Installing pre-commit hooks..."
	@echo "#!/bin/sh" > .git/hooks/pre-commit
	@echo "make check-fmt && make lint" >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hooks installed"

# Development setup
dev-setup: install-deps install-act install-hooks
	@echo "Development environment setup complete"
	@echo ""
	@echo "✅ Your development environment is ready!"
	@echo "💡 Try these commands to get started:"
	@echo "   make ci-quick      # Quick local validation"
	@echo "   make ci-format     # Test with exact GitHub Actions environment"
	@echo "   make help          # See all available commands"

# Check if required tools are installed
check-tools:
	@command -v cargo >/dev/null 2>&1 || { echo "Rust/Cargo is required but not installed. Please install from https://rustup.rs/"; exit 1; }
	@command -v go >/dev/null 2>&1 || { echo "Go is required but not installed. Please install from https://golang.org/"; exit 1; }
	@command -v cbindgen >/dev/null 2>&1 || { echo "cbindgen is required. Install with: cargo install cbindgen"; exit 1; }
	@echo "All required tools are installed"

# Show help
help:
	@echo "Available targets:"
	@echo ""
	@echo "=== Build & Test ==="
	@echo "  all               - Build and test"
	@echo "  build             - Build Rust library and Go bindings (legacy)"
	@echo "  build-native      - Build native libraries for current platform"
	@echo "  build-all-platforms - Build native libraries for all platforms"
	@echo "  build-go          - Build Go project with proper CGO configuration"
	@echo "  test              - Run tests"
	@echo "  bench             - Run benchmarks"
	@echo "  clean             - Clean build artifacts"
	@echo "  clean-dist        - Clean binary distribution files"
	@echo ""
	@echo "=== Code Quality ==="
	@echo "  fmt          - Format code"
	@echo "  check-fmt    - Check code formatting"
	@echo "  lint         - Lint all code (Rust + Go)"
	@echo "  lint-rust    - Lint Rust code only"
	@echo "  lint-go      - Lint Go code only"
	@echo "  lint-go-fix  - Lint and fix Go code automatically"
	@echo "  lint-report  - Generate detailed linting reports"
	@echo ""
	@echo "=== Local CI Testing ==="
	@echo "  ci-quick     - Quick validation (format + lint, no Docker)"
	@echo "  ci-format    - Run format and basic checks using act"
	@echo "  ci-build     - Run build artifacts workflow (includes linting)"
	@echo "  ci-test      - Run test suite using act"
	@echo "  ci-security  - Run security scan using act"
	@echo "  ci-docs      - Run documentation check using act"
	@echo "  ci-local     - Run complete optimized CI pipeline"
	@echo "  ci-list      - List all available CI jobs"
	@echo "  ci-stage1    - Run Stage 1 (quick-checks only)"
	@echo "  ci-stage2    - Run Stage 2 (build, security, docs)"
	@echo "  ci-debug     - Run CI with verbose debug output"
	@echo "  ci-clean     - Clean Docker containers and images"
	@echo "  ci-graph     - Show CI workflow dependencies"	@echo ""
	@echo ""
	@echo "=== Setup & Tools ==="
	@echo "  install-deps - Install development dependencies"
	@echo "  install-act  - Install act (GitHub Actions local runner)"
	@echo "  dev-setup    - Setup development environment"
	@echo "  check-tools  - Check if required tools are installed"
	@echo "  platform-info - Show detected platform and CGO configuration"
	@echo "  check-libraries - Check if native libraries are available"
	@echo ""
	@echo "=== Release ==="
	@echo "  release      - Create release build"
	@echo ""
	@echo "=== Help ==="
	@echo "  help         - Show this help"

# === Local CI Testing with act ===

# Quick local validation (format + lint, no Docker required)
ci-quick: check-fmt lint
	@echo "✅ Quick local validation completed!"
	@echo "   • Code formatting: ✓"
	@echo "   • Linting: ✓"
	@echo ""
	@echo "💡 Run 'make ci-format' to test with the exact same environment as GitHub Actions"

# Run format and basic checks locally (fast)
ci-format: install-act
	@echo "🔍 Running format and basic checks locally..."
	act -j quick-checks

# Run build artifacts workflow locally (includes linting)
ci-build: install-act
	@echo "🏗️ Running build artifacts workflow locally..."
	@echo "⚠️  Note: This downloads large Docker images and may take several minutes on first run"
	act -j build-artifacts

# Run test suite locally (requires build artifacts)
ci-test: install-act
	@echo "🧪 Running test suite locally..."
	@echo "⚠️  This requires build-artifacts to run first or will build them automatically"
	act -j test

# Run security scan locally
ci-security: install-act
	@echo "🔒 Running security scan locally..."
	act -j security

# Run documentation check locally
ci-docs: install-act
	@echo "📚 Running documentation check locally..."
	act -j docs

# Run complete optimized CI pipeline (all jobs)
ci-local: install-act
	@echo "🚀 Running complete optimized CI pipeline locally..."
	@echo "This will run all GitHub Actions jobs in the optimized workflow"
	act

# List all available CI jobs
ci-list: install-act
	@echo "📋 Available GitHub Actions jobs:"
	act --list

# Run specific stages of the CI pipeline
ci-stage1: install-act
	@echo "🚦 Running CI Stage 1 (Quick Checks)..."
	act -j quick-checks

ci-stage2: install-act
	@echo "🚦 Running CI Stage 2 (Build, Security, Docs)..."
	act -j build-artifacts -j security -j docs

# Debug CI workflow with verbose output
ci-debug: install-act
	@echo "🐞 Running CI with debug output..."
	act --verbose

# Clean act Docker containers and images
ci-clean:
	@echo "🧹 Cleaning act Docker containers and images..."
	@docker container prune -f
	@docker image prune -f
	@echo "✅ Docker cleanup completed"

# Show CI workflow graph/dependencies
ci-graph: install-act
	@echo "📊 CI Workflow Dependencies:"
	@echo "Stage 0: quick-checks"
	@echo "Stage 1: build-artifacts, security, docs (depends on quick-checks)"
	@echo "Stage 2: test (depends on build-artifacts)"
	@echo "Stage 3: ci-success (depends on all previous)"
	@echo "Stage 4: cleanup (depends on ci-success)"
	@echo ""
	@echo "📋 Available jobs:"
	@act --list