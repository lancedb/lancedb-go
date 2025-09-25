# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright The LanceDB Authors

.PHONY: all build test clean install-deps fmt lint lint-rust lint-go lint-go-fix lint-report examples docs release

# Default target
all: build test

# Build the Rust library and Go bindings
build:
	@echo "Building Rust library..."
	cd rust && cargo build --release
	@echo "Generating C headers..."
	mkdir -p rust/target/generated/include
	cd rust && cbindgen --config cbindgen.toml --crate lancedb-go --output target/generated/include/lancedb.h
	@echo "Copying library files..."
	mkdir -p rust/target/generated/lib
	cp rust/target/release/liblancedb_go.a rust/target/generated/lib/
	@echo "Building Go module..."
	go build ./...

# Run tests
test: build
	@echo "Running Go tests..."
	go test -v ./...

# Run benchmarks
bench: build
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	cd rust && cargo clean
	go clean ./...
	rm -rf rust/target/

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

# Build examples
examples: build
	@echo "Building examples..."
	@mkdir -p rust/target
	@for dir in examples/*/; do \
		if [ -d "$$dir" ] && [ "$$(basename "$$dir")" != "README.md" ]; then \
			example_name=$$(basename "$$dir"); \
			echo "Building $$example_name..."; \
			if [ -f "$$dir/main.go" ]; then \
				cd "$$dir" && go build -o "../../rust/target/$${example_name}_example" . && cd ../..; \
			elif [ -f "$$dir/$${example_name}.go" ]; then \
				cd "$$dir" && go build -o "../../rust/target/$${example_name}_example" "$${example_name}.go" && cd ../..; \
			else \
				echo "Warning: No main.go or $${example_name}.go found in $$dir"; \
			fi; \
		fi; \
	done
	@echo "Examples built in rust/target/ directory"

# Run examples
run-examples: examples
	@echo "Running all examples..."
	@for example in rust/target/*_example; do \
		if [ -x "$$example" ]; then \
			example_name=$$(basename "$$example" _example); \
			echo ""; \
			echo "=== Running $$example_name example ==="; \
			"$$example" || echo "Warning: $$example_name example failed with exit code $$?"; \
		fi; \
	done
	@echo ""
	@echo "All examples completed."

# Generate documentation
docs:
	@echo "Generating Rust documentation..."
	cd rust && cargo doc --no-deps
	@echo "Generating Go documentation..."
	go doc -all ./...

# Check code formatting
check-fmt:
	@echo "Checking Rust code formatting..."
	cd rust && cargo fmt -- --check
	@echo "Checking Go code formatting..."
	test -z "$$(gofmt -l .)"

# Create a release build
release: clean
	@echo "Creating release build..."
	cd rust && cargo build --release --features remote
	@echo "Generating C headers..."
	mkdir -p rust/target/generated/include
	cd rust && cbindgen --config cbindgen.toml --crate lancedb-go --output target/generated/include/lancedb.h
	@echo "Copying library files..."
	mkdir -p rust/target/generated/lib
	cp rust/target/release/liblancedb_go.a rust/target/generated/lib/
	@echo "Release build complete"

# Install pre-commit hooks
install-hooks:
	@echo "Installing pre-commit hooks..."
	@echo "#!/bin/sh" > .git/hooks/pre-commit
	@echo "make check-fmt && make lint" >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hooks installed"

# Development setup
dev-setup: install-deps install-hooks
	@echo "Development environment setup complete"

# Check if required tools are installed
check-tools:
	@command -v cargo >/dev/null 2>&1 || { echo "Rust/Cargo is required but not installed. Please install from https://rustup.rs/"; exit 1; }
	@command -v go >/dev/null 2>&1 || { echo "Go is required but not installed. Please install from https://golang.org/"; exit 1; }
	@command -v cbindgen >/dev/null 2>&1 || { echo "cbindgen is required. Install with: cargo install cbindgen"; exit 1; }
	@echo "All required tools are installed"

# Show help
help:
	@echo "Available targets:"
	@echo "  all          - Build and test"
	@echo "  build        - Build Rust library and Go bindings"
	@echo "  test         - Run tests"
	@echo "  bench        - Run benchmarks"
	@echo "  clean        - Clean build artifacts"
	@echo "  install-deps - Install development dependencies (including golangci-lint)"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint all code (Rust + Go)"
	@echo "  lint-rust    - Lint Rust code only"
	@echo "  lint-go      - Lint Go code only"
	@echo "  lint-go-fix  - Lint and fix Go code automatically"
	@echo "  lint-report  - Generate detailed linting reports"
	@echo "  examples     - Build all examples"
	@echo "  run-examples - Run all examples"
	@echo "  docs         - Generate documentation"
	@echo "  check-fmt    - Check code formatting"
	@echo "  release      - Create release build"
	@echo "  dev-setup    - Setup development environment"
	@echo "  check-tools  - Check if required tools are installed"
	@echo "  help         - Show this help"
