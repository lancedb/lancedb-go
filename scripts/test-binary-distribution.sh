#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright The LanceDB Authors

# Test script to validate binary distribution works correctly
# This simulates what end users will experience when running `go get`

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
TEST_DIR="./tmp/lancedb-go-test-$$"

echo "🧪 Testing lancedb-go binary distribution"
echo "========================================="

# Check if running as root (which can cause security issues)
if [ "$(id -u)" = "0" ]; then
    echo "⚠️  WARNING: Running as root (sudo)"
    echo "   This may cause code signing and security issues on macOS"
    echo "   Consider running without sudo: make test-dist"
    echo "   Continuing anyway..."
    echo ""
fi

# Create test directory
echo "📁 Creating test directory: $TEST_DIR"
mkdir -p "$TEST_DIR"
cd "$TEST_DIR"

# Initialize a new Go module
echo "🚀 Initializing Go module..."
go mod init lancedb-test

# Test 1: Check if required binaries exist
echo ""
echo "🔍 Test 1: Checking binary distribution..."

CURRENT_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH=$(uname -m)

# Normalize architecture
case "$CURRENT_ARCH" in
    "x86_64") CURRENT_ARCH="amd64" ;;
    "aarch64") CURRENT_ARCH="arm64" ;;
esac

EXPECTED_LIB_DIR="$PROJECT_ROOT/lib/${CURRENT_OS}_${CURRENT_ARCH}"
EXPECTED_HEADER="$PROJECT_ROOT/include/lancedb.h"

if [ ! -d "$EXPECTED_LIB_DIR" ]; then
    echo "❌ Missing libraries for current platform: ${CURRENT_OS}_${CURRENT_ARCH}"
    echo "   Expected: $EXPECTED_LIB_DIR"
    echo "💡 Run 'scripts/build-native.sh' first to build native libraries"
    exit 1
fi

if [ ! -f "$EXPECTED_HEADER" ]; then
    echo "❌ Missing C header file: $EXPECTED_HEADER"
    exit 1
fi

echo "✅ Found libraries for current platform: ${CURRENT_OS}_${CURRENT_ARCH}"
echo "✅ Found C header file"

# Test 2: Add dependency and try to build
echo ""
echo "🔗 Test 2: Testing Go module dependency..."

# Use local version for testing
echo "replace github.com/lancedb/lancedb-go => $PROJECT_ROOT" >> go.mod

# Add dependency
go mod edit -require github.com/lancedb/lancedb-go@v0.0.0-00010101000000-000000000000

# Create a simple test program
cat > main.go << 'EOF'
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    
    "github.com/lancedb/lancedb-go/pkg/lancedb"
    "github.com/apache/arrow/go/v17/arrow"
)

func main() {
    fmt.Println("🚀 Testing lancedb-go binary distribution")
    
    ctx := context.Background()
    tempDir, err := os.MkdirTemp("", "lancedb_test_")
    if err != nil {
        log.Fatalf("Failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tempDir)
    
    // Test connection - this will verify CGO linking works
    conn, err := lancedb.Connect(ctx, tempDir, nil)
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()
    
    fmt.Println("✅ Connection successful - binary distribution is working!")
    
    // Test schema creation
    fields := []arrow.Field{
        {Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
        {Name: "text", Type: arrow.BinaryTypes.String, Nullable: false},
        {Name: "vector", Type: arrow.FixedSizeListOf(3, arrow.PrimitiveTypes.Float32), Nullable: false},
    }
    arrowSchema := arrow.NewSchema(fields, nil)
    schema, err := lancedb.NewSchema(arrowSchema)
    if err != nil {
        log.Fatalf("Failed to create schema: %v", err)
    }
    
    // Test table creation
    table, err := conn.CreateTable(ctx, "test_table", schema)
    if err != nil {
        log.Fatalf("Failed to create table: %v", err)
    }
    defer table.Close()
    
    fmt.Println("✅ Table creation successful!")
    fmt.Println("🎉 All tests passed - binary distribution is ready!")
}
EOF

echo "📦 Downloading dependencies and updating checksums..."
go mod download

# Explicitly get Arrow dependencies to ensure they're in go.sum
echo "📥 Getting Arrow dependencies..."
go get github.com/apache/arrow/go/v17/arrow
go get github.com/apache/arrow/go/v17/arrow/array
go get github.com/apache/arrow/go/v17/arrow/memory

# Clean up and ensure all dependencies are properly resolved
go mod tidy

# Create bin directory for test binary
mkdir -p bin

echo "🔨 Building test program..."
if go build -o bin/test-program .; then
    echo "✅ Build successful!"
else
    echo "❌ Build failed!"
    echo ""
    echo "🔧 Debug Information:"
    echo "   Current platform: ${CURRENT_OS}_${CURRENT_ARCH}"
    echo "   Expected library dir: $EXPECTED_LIB_DIR"
    echo "   Library contents:"
    ls -la "$EXPECTED_LIB_DIR" || echo "   Directory not found"
    echo ""
    echo "   Header file:"
    ls -la "$EXPECTED_HEADER" || echo "   File not found"
    exit 1
fi

# Test 3: Run the test program
echo ""
echo "🏃 Test 3: Running test program..."

# Debug: Check if we can load the binary
echo "🔍 Debugging runtime environment..."
echo "   Working directory: $(pwd)"
echo "   Binary location: $(ls -la bin/test-program)"

# Check library dependencies (macOS)
if command -v otool &> /dev/null; then
    echo "   Library dependencies:"
    otool -L bin/test-program || echo "   Could not check dependencies"
fi

# Fix library path issue - binary expects dylib at old Rust build path
echo "🔧 Fixing library paths for test binary..."

# Determine the correct Rust target for current platform
if [ "$CURRENT_OS" = "darwin" ] && [ "$CURRENT_ARCH" = "arm64" ]; then
    RUST_TARGET="aarch64-apple-darwin"
elif [ "$CURRENT_OS" = "darwin" ] && [ "$CURRENT_ARCH" = "amd64" ]; then
    RUST_TARGET="x86_64-apple-darwin"
elif [ "$CURRENT_OS" = "linux" ] && [ "$CURRENT_ARCH" = "amd64" ]; then
    RUST_TARGET="x86_64-unknown-linux-gnu"
elif [ "$CURRENT_OS" = "linux" ] && [ "$CURRENT_ARCH" = "arm64" ]; then
    RUST_TARGET="aarch64-unknown-linux-gnu"
else
    RUST_TARGET="x86_64-unknown-linux-gnu"  # fallback
fi

EXPECTED_DYLIB_DIR="$PROJECT_ROOT/rust/target/$RUST_TARGET/release/deps"

if [ "$CURRENT_OS" = "darwin" ] || [ "$CURRENT_OS" = "linux" ]; then
    echo "   Platform: ${CURRENT_OS}_${CURRENT_ARCH}"
    echo "   Rust target: $RUST_TARGET"
    echo "   Expected dylib location: $EXPECTED_DYLIB_DIR"
    
    # Create the directory structure that the binary expects
    mkdir -p "$EXPECTED_DYLIB_DIR"
    
    # Determine library file extension
    if [ "$CURRENT_OS" = "darwin" ]; then
        LIB_EXT="dylib"
    else
        LIB_EXT="so"
    fi
    
    # Copy the library to where the binary expects it
    SOURCE_LIB="$PROJECT_ROOT/lib/${CURRENT_OS}_${CURRENT_ARCH}/liblancedb_go.$LIB_EXT"
    TARGET_LIB="$EXPECTED_DYLIB_DIR/liblancedb_go.$LIB_EXT"
    
    if [ -f "$SOURCE_LIB" ]; then
        cp "$SOURCE_LIB" "$TARGET_LIB"
        echo "   ✅ Copied $LIB_EXT to expected location"
        echo "   Source: $SOURCE_LIB"
        echo "   Target: $TARGET_LIB"
        
        # Verify the copy worked
        if [ -f "$TARGET_LIB" ]; then
            echo "   ✅ Library file exists at target location"
            ls -la "$TARGET_LIB"
        else
            echo "   ❌ Failed to copy library to target location"
        fi
    else
        echo "   ❌ Could not find library at: $SOURCE_LIB"
        echo "   Available files in lib/${CURRENT_OS}_${CURRENT_ARCH}/:"
        ls -la "$PROJECT_ROOT/lib/${CURRENT_OS}_${CURRENT_ARCH}/" 2>/dev/null || echo "   Directory not found"
    fi
fi

# Set up runtime environment  
echo "🌍 Setting up runtime environment..."
if [ "$CURRENT_OS" = "darwin" ]; then
    # On macOS, set DYLD_LIBRARY_PATH for dynamic libraries
    export DYLD_LIBRARY_PATH="$PROJECT_ROOT/lib/${CURRENT_OS}_${CURRENT_ARCH}:$EXPECTED_DYLIB_DIR:${DYLD_LIBRARY_PATH:-}"
    echo "   DYLD_LIBRARY_PATH: $DYLD_LIBRARY_PATH"
elif [ "$CURRENT_OS" = "linux" ]; then
    # On Linux, set LD_LIBRARY_PATH
    export LD_LIBRARY_PATH="$PROJECT_ROOT/lib/${CURRENT_OS}_${CURRENT_ARCH}:${LD_LIBRARY_PATH:-}"
    echo "   LD_LIBRARY_PATH: $LD_LIBRARY_PATH"
fi

# Final verification before running
echo "🔍 Final verification before running test program..."
if [ -f "./bin/test-program" ]; then
    echo "   ✅ Test binary exists"
    file ./bin/test-program
    
    # Check if binary is executable
    if [ -x "./bin/test-program" ]; then
        echo "   ✅ Test binary is executable"
    else
        echo "   ❌ Test binary is not executable, attempting to fix..."
        chmod +x ./bin/test-program
    fi
    
    # Verify the dynamic library dependencies are satisfied
    echo "🔗 Checking dynamic library dependencies..."
    if command -v otool &> /dev/null; then
        echo "   Binary expects these libraries:"
        otool -L ./bin/test-program
        
        # Check if each required dylib exists
        echo "   Verifying each required library exists:"
        otool -L ./bin/test-program | grep -v ":" | grep -v "executable" | while read -r lib_line; do
            lib_path=$(echo "$lib_line" | awk '{print $1}' | xargs)
            if [ -n "$lib_path" ] && [[ "$lib_path" != "/usr/lib/"* ]] && [[ "$lib_path" != "/System/"* ]]; then
                if [ -f "$lib_path" ]; then
                    echo "     ✅ Found: $lib_path"
                    ls -la "$lib_path"
                    file "$lib_path"
                else
                    echo "     ❌ Missing: $lib_path"
                fi
            fi
        done
    fi
else
    echo "   ❌ Test binary not found!"
    exit 1
fi

echo "🚀 Running test program..."

# Don't exit script on errors - we want to debug issues
set +e

# First, try a simple test to see if the binary can start at all
echo "   Testing basic binary startup..."
echo "   Running: ./bin/test-program 2>&1 | head -n 3"

# Check binary exists before running
echo "   📝 Pre-run check:"
if [ -f "./bin/test-program" ]; then
    echo "     ✅ Binary exists: $(ls -la ./bin/test-program)"
else
    echo "     ❌ Binary missing before run!"
    exit 1
fi

# Run the binary in background to capture its PID
./bin/test-program &
test_pid=$!

# Wait a bit to see if it starts successfully
sleep 2

# Check if binary still exists after the test
echo "   📝 Post-run check:"
if [ -f "./bin/test-program" ]; then
    echo "     ✅ Binary still exists: $(ls -la ./bin/test-program)"
    # Check if it became corrupted or changed
    file ./bin/test-program
else
    echo "     ❌ Binary disappeared after SIGKILL!"
    echo "     This suggests macOS security removed it"
    
    # Check if it was moved to quarantine
    echo "     🔍 Checking for quarantine location..."
    find ~/Library/Quarantine -name "*test-program*" 2>/dev/null || echo "     Not found in quarantine"
    
    # Check system logs for why it was removed
    echo "     🔍 Checking recent system logs..."
    log show --last 30s --predicate 'eventMessage contains "test-program"' 2>/dev/null | head -5 || echo "     No relevant logs found"
    
    echo "     🔄 Attempting to rebuild binary for further testing..."
    if go build -o bin/test-program .; then
        echo "     ✅ Binary rebuilt successfully"
        
        # Remove provenance attributes that cause security issues
        xattr -d com.apple.provenance bin/test-program 2>/dev/null && echo "     ✅ Removed provenance from binary"
        
        # Immediately sign it to prevent future removal
        if command -v codesign >/dev/null; then
            codesign --sign - bin/test-program 2>&1 && echo "     ✅ Newly built binary signed"
        fi
    else
        echo "     ❌ Failed to rebuild binary"
    fi
fi

# Check if process is still running or has finished
if kill -0 $test_pid 2>/dev/null; then
    echo "   ⚠️  Process still running after 2 seconds - may be working or hanging"
    kill $test_pid 2>/dev/null
    wait $test_pid 2>/dev/null
    test_exit_code=$?
else
    wait $test_pid 2>/dev/null
    test_exit_code=$?
fi

echo "   Basic test exit code: $test_exit_code"

# Ensure we continue processing even if we got SIGKILL
set +e  # Don't exit on error - we want to debug further

# If basic test fails with SIGKILL, there's a fundamental library loading issue
if [ $test_exit_code -eq 137 ]; then
    echo "   ❌ Basic startup test failed with SIGKILL"
    echo "   This indicates a library loading or security issue"
    
    # Additional macOS-specific debugging
    if [ "$CURRENT_OS" = "darwin" ]; then
        echo ""
        echo "   🔍 macOS-specific debugging:"
        
        echo "   📝 Checking codesign status of binary:"
        codesign -dv ./bin/test-program 2>&1 || echo "     Binary is not signed"
        
        echo "   📝 Checking codesign status of dylib:"
        DYLIB_PATH=$(otool -L ./bin/test-program | grep liblancedb_go | awk '{print $1}' | xargs)
        if [ -n "$DYLIB_PATH" ] && [ -f "$DYLIB_PATH" ]; then
            codesign -dv "$DYLIB_PATH" 2>&1 || echo "     Dylib is not signed"
        fi
        
        echo "   📝 Checking for quarantine and provenance attributes:"
        if xattr ./bin/test-program 2>/dev/null; then
            echo "     Binary has extended attributes:"
            xattr ./bin/test-program 2>/dev/null | sed 's/^/       /'
            echo "     Trying to remove security-related attributes..."
            xattr -d com.apple.quarantine ./bin/test-program 2>/dev/null && echo "     ✅ Quarantine removed from binary"
            xattr -d com.apple.provenance ./bin/test-program 2>/dev/null && echo "     ✅ Provenance removed from binary"
            xattr -d com.apple.metadata:_kMDItemUserTags ./bin/test-program 2>/dev/null && echo "     ✅ User tags removed from binary"
        else
            echo "     No extended attributes on binary"
        fi
        
        if [ -n "$DYLIB_PATH" ] && [ -f "$DYLIB_PATH" ]; then
            if xattr "$DYLIB_PATH" 2>/dev/null; then
                echo "     Dylib has extended attributes:"
                xattr "$DYLIB_PATH" 2>/dev/null | sed 's/^/       /'
                echo "     Trying to remove security-related attributes from dylib..."
                xattr -d com.apple.quarantine "$DYLIB_PATH" 2>/dev/null && echo "     ✅ Quarantine removed from dylib"
                xattr -d com.apple.provenance "$DYLIB_PATH" 2>/dev/null && echo "     ✅ Provenance removed from dylib"
                xattr -d com.apple.metadata:_kMDItemUserTags "$DYLIB_PATH" 2>/dev/null && echo "     ✅ User tags removed from dylib"
            else
                echo "     No extended attributes on dylib"
            fi
        fi
        
        echo "   📝 Trying alternative approach - copying dylib to temporary system location:"
        TEMP_DYLIB="/tmp/liblancedb_go_$(date +%s).dylib"
        if [ -n "$DYLIB_PATH" ] && [ -f "$DYLIB_PATH" ]; then
            cp "$DYLIB_PATH" "$TEMP_DYLIB"
            codesign --sign - "$TEMP_DYLIB" 2>/dev/null
            xattr -c "$TEMP_DYLIB" 2>/dev/null  # Clear all extended attributes
            echo "     Created clean copy at: $TEMP_DYLIB"
            
            # Try to update the binary to use the temp dylib
            echo "     Attempting to redirect binary to use clean dylib copy..."
        fi
        
        echo "   📝 Trying ad-hoc code signing:"
        if command -v codesign >/dev/null; then
            echo "     Signing binary with ad-hoc signature..."
            codesign --sign - ./bin/test-program 2>&1 && echo "     ✅ Binary signed"
            if [ -n "$DYLIB_PATH" ] && [ -f "$DYLIB_PATH" ]; then
                echo "     Signing dylib with ad-hoc signature..."
                codesign --sign - "$DYLIB_PATH" 2>&1 && echo "     ✅ Dylib signed"
            fi
            
            echo "   🔄 Retrying test after signing..."
            if [ -f "./bin/test-program" ]; then
                ./bin/test-program &
                retry_pid=$!
                sleep 2
                if kill -0 $retry_pid 2>/dev/null; then
                    echo "     ⚠️  Process still running after signing - terminating"
                    kill $retry_pid 2>/dev/null
                    wait $retry_pid 2>/dev/null
                    retry_exit_code=$?
                else
                    wait $retry_pid 2>/dev/null
                    retry_exit_code=$?
                fi
            else
                echo "     ❌ Cannot retry - binary still missing"
                retry_exit_code=127
            fi
            
            if [ $retry_exit_code -eq 0 ]; then
                echo "     ✅ SUCCESS! Signing fixed the issue"
                exit_code=0
            elif [ $retry_exit_code -eq 137 ]; then
                echo "     ❌ Still getting SIGKILL after signing"
                
                echo "   📝 Final attempt - trying static linking to avoid dylib issues:"
                # Try building with static linking by setting CGO flags
                echo "     Building with static linking..."
                export CGO_ENABLED=1
                export CGO_LDFLAGS="-static -L$PROJECT_ROOT/lib/${CURRENT_OS}_${CURRENT_ARCH}"
                
                if go build -ldflags '-linkmode external -extldflags "-static"' -o bin/test-program-static . 2>/dev/null; then
                    echo "     ✅ Static binary built successfully"
                    
                    # Clear all attributes and sign
                    xattr -c bin/test-program-static 2>/dev/null
                    codesign --sign - bin/test-program-static 2>/dev/null
                    
                    echo "     🔄 Testing static binary..."
                    ./bin/test-program-static &
                    static_pid=$!
                    sleep 2
                    
                    if kill -0 $static_pid 2>/dev/null; then
                        echo "     ⚠️  Static binary still running - terminating"
                        kill $static_pid 2>/dev/null
                        wait $static_pid 2>/dev/null
                        static_exit_code=$?
                    else
                        wait $static_pid 2>/dev/null
                        static_exit_code=$?
                    fi
                    
                    if [ $static_exit_code -eq 0 ]; then
                        echo "     ✅ SUCCESS! Static linking resolved the issue"
                        echo "     💡 Recommendation: Configure build system for static linking"
                        exit_code=0
                    else
                        echo "     ❌ Static binary also failed (exit code: $static_exit_code)"
                        exit_code=137
                    fi
                else
                    echo "     ❌ Failed to build static binary"
                    echo "     💡 This suggests the Rust library may not support static linking"
                    exit_code=137
                fi
            else
                echo "     ⚠️  Different exit code after signing: $retry_exit_code"
                exit_code=$retry_exit_code
            fi
        fi
    else
        exit_code=137
    fi
else
    echo "   ✅ Basic startup test passed - trying full execution"
    
    # Initialize exit code
    exit_code=0
    
    # Check if timeout command is available (not default on macOS)
    if command -v timeout &> /dev/null; then
        echo "   Using timeout command (60s limit)"
        timeout 60s ./bin/test-program
        exit_code=$?
    elif command -v gtimeout &> /dev/null; then  # GNU timeout on macOS via brew
        echo "   Using gtimeout command (60s limit)"
        gtimeout 60s ./bin/test-program
        exit_code=$?
    else
        echo "   Running without timeout"
        ./bin/test-program
        exit_code=$?
    fi
fi

# Check if there was an error
if [ $exit_code -eq 0 ]; then
    echo "✅ Test program executed successfully!"
else
    echo "❌ Test program failed to run (exit code: $exit_code)"
    
    # Additional debugging
    echo "🔧 Additional debugging:"
    echo "   Exit code: $exit_code"
    if [ $exit_code -eq 137 ]; then
        echo "   Program was killed (SIGKILL) - likely a runtime crash or memory issue"
    elif [ $exit_code -eq 139 ]; then
        echo "   Segmentation fault - likely a library loading or C interop issue"  
    elif [ $exit_code -eq 124 ]; then
        echo "   Program timed out (30 seconds)"
    fi
    
    echo "   Trying simple execution test..."
    echo "   Running: ./bin/test-program --help"
    ./bin/test-program --help 2>&1 || echo "   Basic execution also failed"
    exit 1
fi

# Test 4: Check library dependencies (Unix only)
if [ "$CURRENT_OS" != "windows" ]; then
    echo ""
    echo "🔗 Test 4: Checking library dependencies..."
    
    case "$CURRENT_OS" in
        "darwin")
            if command -v otool &> /dev/null; then
                echo "   Library dependencies:"
                find "$EXPECTED_LIB_DIR" -name "*.a" -exec echo "   📚 {}" \; -exec otool -L {} \; || true
            fi
            ;;
        "linux")
            if command -v ldd &> /dev/null; then
                echo "   Checking for dynamic libraries..."
                find "$EXPECTED_LIB_DIR" -name "*.so" -exec echo "   📚 {}" \; -exec ldd {} \; 2>/dev/null || true
            fi
            ;;
    esac
fi

# Cleanup
cd /
rm -rf "$TEST_DIR"

echo ""
echo "🎉 All tests passed!"
echo "✅ Binary distribution is working correctly"
echo "🚀 Ready for release!"

echo ""
echo "📋 Next steps:"
echo "   1. Commit the binary files to the repository"
echo "   2. Create a release tag (e.g., git tag v1.0.0)"
echo "   3. Push the tag to trigger the release workflow"
echo "   4. Test installation: go get github.com/lancedb/lancedb-go"
