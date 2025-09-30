# LanceDB Go Examples Makefile
# This Makefile helps you download binaries and run examples

.PHONY: all download-artifacts platform-info clean help basic-crud batch-operations hybrid-search index-management storage-configuration vector-search run-all

# Default target
all: download-artifacts run-all

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
	CGO_LDFLAGS := $(CURRENT_DIR)/lib/windows_amd64/lancedb_go.lib
else ifneq (,$(findstring MSYS,$(UNAME_S)))
	PLATFORM := windows
	ARCH := amd64
	CGO_LDFLAGS := $(CURRENT_DIR)/lib/windows_amd64/lancedb_go.lib
else ifneq (,$(findstring CYGWIN,$(UNAME_S)))
	PLATFORM := windows
	ARCH := amd64
	CGO_LDFLAGS := $(CURRENT_DIR)/lib/windows_amd64/lancedb_go.lib
else
	$(error Unsupported platform: $(UNAME_S))
endif

# Set CGO flags
CGO_CFLAGS := -I$(CURRENT_DIR)/include
PLATFORM_ARCH := $(PLATFORM)_$(ARCH)

# Download native libraries and headers
download-artifacts:
	@echo "📦 Downloading LanceDB native artifacts..."
	@if [ ! -f "../scripts/download-artifacts.sh" ]; then \
		echo "❌ Download script not found. Please run from lancedb-go/examples directory."; \
		exit 1; \
	fi
	@cd .. && ./scripts/download-artifacts.sh
	@echo "✅ Artifacts downloaded to examples directory"

# Check if libraries exist
check-libraries:
	@if [ ! -d "$(CURRENT_DIR)/lib/$(PLATFORM_ARCH)" ] || [ ! -f "$(CURRENT_DIR)/lib/$(PLATFORM_ARCH)/liblancedb_go.a" ] && [ ! -f "$(CURRENT_DIR)/lib/$(PLATFORM_ARCH)/lancedb_go.lib" ]; then \
		echo "❌ Native libraries not found for platform: $(PLATFORM_ARCH)"; \
		echo "Expected: $(CURRENT_DIR)/lib/$(PLATFORM_ARCH)/"; \
		echo ""; \
		echo "Please run: make download-artifacts"; \
		exit 1; \
	fi
	@if [ ! -d "$(CURRENT_DIR)/include" ] || [ ! -f "$(CURRENT_DIR)/include/lancedb.h" ]; then \
		echo "❌ Header files not found"; \
		echo "Expected: $(CURRENT_DIR)/include/lancedb.h"; \
		echo "Please run: make download-artifacts"; \
		exit 1; \
	fi

# Show platform information
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
	@echo "Library Status:"
	@echo "==============="
	@if [ -d "$(CURRENT_DIR)/lib/$(PLATFORM_ARCH)" ]; then \
		echo "✅ Library directory exists: $(CURRENT_DIR)/lib/$(PLATFORM_ARCH)/"; \
		ls -la "$(CURRENT_DIR)/lib/$(PLATFORM_ARCH)/" | head -10; \
	else \
		echo "❌ Library directory not found: $(CURRENT_DIR)/lib/$(PLATFORM_ARCH)/"; \
		echo "   Run 'make download-artifacts' to download libraries"; \
	fi

# Individual example targets (build and run combined)
basic-crud: check-libraries
	@echo "🚀 Running Basic CRUD example..."
	@echo "Platform: $(PLATFORM_ARCH)"
	@cd basic_crud && \
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go run basic_crud.go

batch-operations: check-libraries
	@echo "🚀 Running Batch Operations example..."
	@echo "Platform: $(PLATFORM_ARCH)"
	@cd batch_operations && \
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go run batch_operations.go

hybrid-search: check-libraries
	@echo "🚀 Running Hybrid Search example..."
	@echo "Platform: $(PLATFORM_ARCH)"
	@cd hybrid_search && \
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go run hybrid_search.go

index-management: check-libraries
	@echo "🚀 Running Index Management example..."
	@echo "Platform: $(PLATFORM_ARCH)"
	@cd index_management && \
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go run index_management.go

storage-configuration: check-libraries
	@echo "🚀 Running Storage Configuration example..."
	@echo "Platform: $(PLATFORM_ARCH)"
	@cd storage_configuration && \
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go run storage_configuration.go

vector-search: check-libraries
	@echo "🚀 Running Vector Search example..."
	@echo "Platform: $(PLATFORM_ARCH)"
	@cd vector_search && \
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go run vector_search.go

# Build all examples (without running)
build-all: check-libraries
	@echo "🔨 Building all examples for $(PLATFORM_ARCH)..."
	@for example in basic_crud batch_operations hybrid_search index_management storage_configuration vector_search; do \
		echo "Building $$example..."; \
		cd "$$example" && \
		CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go build -o "../bin/$${example}_example" "$${example}.go" && \
		cd ..; \
	done
	@echo "✅ All examples built in bin/ directory"

# Run all examples sequentially
run-all: check-libraries
	@echo "🎯 Running all LanceDB Go examples..."
	@echo "====================================="
	@mkdir -p bin
	@echo ""
	@echo "1️⃣  Basic CRUD Operations"
	@echo "========================="
	@make basic-crud || echo "⚠️  Basic CRUD example failed"
	@echo ""
	@echo "2️⃣  Vector Search"
	@echo "================="
	@make vector-search || echo "⚠️  Vector Search example failed"
	@echo ""
	@echo "3️⃣  Hybrid Search"
	@echo "================="
	@make hybrid-search || echo "⚠️  Hybrid Search example failed"
	@echo ""
	@echo "4️⃣  Index Management"
	@echo "==================="
	@make index-management || echo "⚠️  Index Management example failed"
	@echo ""
	@echo "5️⃣  Batch Operations"
	@echo "==================="
	@make batch-operations || echo "⚠️  Batch Operations example failed"
	@echo ""
	@echo "6️⃣  Storage Configuration"
	@echo "========================="
	@make storage-configuration || echo "⚠️  Storage Configuration example failed"
	@echo ""
	@echo "🏁 All examples completed!"

# Clean built binaries and temporary files
clean:
	@echo "🧹 Cleaning examples..."
	@rm -rf bin/
	@find . -name "*.tmp" -delete 2>/dev/null || true
	@find . -name "sample-lancedb" -type d -exec rm -rf {} + 2>/dev/null || true
	@echo "✅ Cleaned build artifacts and temporary files"

# Clean everything including downloaded libraries
clean-all: clean
	@echo "🧹 Cleaning everything..."
	@rm -rf lib/ include/
	@echo "✅ Cleaned all artifacts (you'll need to run 'make download-artifacts' again)"

# Quick test - just check if examples can build
test: check-libraries
	@echo "🧪 Testing example builds..."
	@mkdir -p bin
	@for example in basic_crud vector_search; do \
		echo "Testing build: $$example"; \
		cd "$$example" && \
		CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go build -o "../bin/test_$${example}" "$${example}.go" && \
		cd .. && rm -f "bin/test_$${example}"; \
	done
	@echo "✅ Build tests passed!"

# Show help
help:
	@echo "LanceDB Go Examples - Available Targets"
	@echo "======================================="
	@echo ""
	@echo "🚀 Quick Start:"
	@echo "  all              - Download artifacts and run all examples"
	@echo "  download-artifacts - Download native libraries and headers"
	@echo ""
	@echo "📊 Individual Examples:"
	@echo "  basic-crud       - Run Basic CRUD operations example"
	@echo "  vector-search    - Run Vector Search example"
	@echo "  hybrid-search    - Run Hybrid Search example"  
	@echo "  index-management - Run Index Management example"
	@echo "  batch-operations - Run Batch Operations example"
	@echo "  storage-configuration - Run Storage Configuration example"
	@echo ""
	@echo "🔨 Build & Run:"
	@echo "  build-all        - Build all examples (don't run them)"
	@echo "  run-all          - Run all examples sequentially"
	@echo "  test             - Quick build test for key examples"
	@echo ""
	@echo "🛠️  Utilities:"
	@echo "  platform-info    - Show platform detection and CGO configuration"
	@echo "  clean            - Clean built binaries and temp files"
	@echo "  clean-all        - Clean everything including downloaded libraries"
	@echo "  help             - Show this help"
	@echo ""
	@echo "💡 Usage Examples:"
	@echo "  make                    # Download and run everything"
	@echo "  make basic-crud         # Just run basic CRUD example"
	@echo "  make download-artifacts # Just download libraries"
	@echo "  make platform-info      # Check your platform setup"
