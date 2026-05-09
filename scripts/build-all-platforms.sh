#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright The LanceDB Authors

# Build native libraries for all supported platforms
# This script is useful for local development and testing

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🚀 Building lancedb-go native libraries for all platforms"
echo "========================================================"

# Define all supported platforms
PLATFORMS=(
    "darwin amd64"
    "darwin arm64" 
    "linux amd64"
    "linux arm64"
    "windows amd64"  # Uncomment if building on Windows or with cross-compilation setup
    "windows arm64"
)

# Check prerequisites
echo "🔧 Checking prerequisites..."

if ! command -v rustc &> /dev/null; then
    echo "❌ Rust is not installed. Please install from https://rustup.rs/"
    exit 1
fi

if ! command -v cbindgen &> /dev/null; then
    echo "📦 Installing cbindgen..."
    cargo install cbindgen
fi

# Clean previous builds
echo "🧹 Cleaning previous builds..."
rm -rf "$PROJECT_ROOT/lib"
rm -rf "$PROJECT_ROOT/include"

# Build for each platform
for platform_arch in "${PLATFORMS[@]}"; do
    read -r platform arch <<< "$platform_arch"
    
    echo ""
    echo "🏗️ Building for $platform-$arch..."
    
    if "$SCRIPT_DIR/build-native.sh" "$platform" "$arch"; then
        echo "✅ Successfully built $platform-$arch"
    else
        echo "❌ Failed to build $platform-$arch"
        exit 1
    fi
done

echo ""
echo "🎉 All platforms built successfully!"
echo ""

# Show summary
echo "📊 Build Summary:"
echo "================"

if [ -d "$PROJECT_ROOT/include" ]; then
    echo "📝 C Header:"
    ls -la "$PROJECT_ROOT/include/"
fi

echo ""
echo "📚 Platform Libraries:"
for dir in "$PROJECT_ROOT/lib"/*; do
    if [ -d "$dir" ]; then
        platform=$(basename "$dir")
        echo "  📦 $platform:"
        ls -la "$dir" | sed 's/^/    /'
    fi
done

echo ""
echo "💡 Usage:"
echo "  The libraries are now ready for distribution."
echo "  Run 'go build' to test with the new binary layout."
