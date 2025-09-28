#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright The LanceDB Authors

# Build script for cross-platform native binaries
# Usage: ./scripts/build-native.sh [platform] [architecture]
# Example: ./scripts/build-native.sh darwin arm64

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RUST_DIR="$PROJECT_ROOT/rust"
LIB_DIR="$PROJECT_ROOT/lib"
INCLUDE_DIR="$PROJECT_ROOT/include"

# Default to current platform if not specified
PLATFORM="${1:-$(uname -s | tr '[:upper:]' '[:lower:]')}"
ARCH="${2:-$(uname -m)}"

# Normalize architecture names
case "$ARCH" in
    "x86_64"|"amd64") ARCH="amd64" ;;
    "arm64"|"aarch64") ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Normalize platform names
case "$PLATFORM" in
    "darwin"|"macos") PLATFORM="darwin" ;;
    "linux") PLATFORM="linux" ;;
    "windows"|"win32"|"win64") PLATFORM="windows" ;;
    *) echo "Unsupported platform: $PLATFORM" >&2; exit 1 ;;
esac

TARGET_DIR="$LIB_DIR/${PLATFORM}_${ARCH}"

echo "🏗️ Building lancedb-go native library"
echo "   Platform: $PLATFORM"
echo "   Architecture: $ARCH"
echo "   Target directory: $TARGET_DIR"
echo ""

# Ensure target directory exists
mkdir -p "$TARGET_DIR"

# Set up Rust target
RUST_TARGET=""
case "$PLATFORM-$ARCH" in
    "darwin-amd64") RUST_TARGET="x86_64-apple-darwin" ;;
    "darwin-arm64") RUST_TARGET="aarch64-apple-darwin" ;;
    "linux-amd64") RUST_TARGET="x86_64-unknown-linux-gnu" ;;
    "linux-arm64") RUST_TARGET="aarch64-unknown-linux-gnu" ;;
    "windows-amd64") RUST_TARGET="x86_64-pc-windows-msvc" ;;
    *) echo "Unsupported target: $PLATFORM-$ARCH" >&2; exit 1 ;;
esac

echo "🦀 Installing Rust target: $RUST_TARGET"
rustup target add "$RUST_TARGET"

echo "🔨 Building Rust library..."
cd "$RUST_DIR"

# Build the library
CARGO_TARGET_DIR="$RUST_DIR/target" cargo build --release --target "$RUST_TARGET"

# Copy library to distribution directory
echo "📦 Copying library files..."
case "$PLATFORM" in
    "darwin"|"linux")
        cp "$RUST_DIR/target/$RUST_TARGET/release/liblancedb_go.a" "$TARGET_DIR/"
        if [ -f "$RUST_DIR/target/$RUST_TARGET/release/liblancedb_go.dylib" ]; then
            cp "$RUST_DIR/target/$RUST_TARGET/release/liblancedb_go.dylib" "$TARGET_DIR/"
        fi
        if [ -f "$RUST_DIR/target/$RUST_TARGET/release/liblancedb_go.so" ]; then
            cp "$RUST_DIR/target/$RUST_TARGET/release/liblancedb_go.so" "$TARGET_DIR/"
        fi
        ;;
    "windows")
        cp "$RUST_DIR/target/$RUST_TARGET/release/lancedb_go.lib" "$TARGET_DIR/"
        cp "$RUST_DIR/target/$RUST_TARGET/release/lancedb_go.dll" "$TARGET_DIR/"
        ;;
esac

# Generate C header (only once)
if [ ! -f "$INCLUDE_DIR/lancedb.h" ] || [ "$PLATFORM-$ARCH" = "$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')" ]; then
    echo "📝 Generating C header..."
    mkdir -p "$INCLUDE_DIR"
    cbindgen --config cbindgen.toml --crate lancedb-go --output "$INCLUDE_DIR/lancedb.h"
fi

echo "✅ Build completed successfully!"
echo "   Library: $TARGET_DIR"
echo "   Header: $INCLUDE_DIR/lancedb.h"
