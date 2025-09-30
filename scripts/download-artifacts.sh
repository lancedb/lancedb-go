#!/bin/bash

set -e

# Script to download LanceDB Go native artifacts from GitHub releases
# Usage: ./download-artifacts.sh [version]
# If version is not provided, downloads the latest release

# Configuration
GITHUB_REPO="lancedb/lancedb-go"
GITHUB_API_URL="https://api.github.com/repos/$GITHUB_REPO"
GITHUB_RELEASES_URL="https://github.com/$GITHUB_REPO/releases/download"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Function to detect current platform
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        darwin*)
            echo "darwin"
            ;;
        linux*)
            echo "linux"
            ;;
        mingw*|msys*|cygwin*)
            echo "windows"
            ;;
        *)
            print_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
}

# Function to detect current architecture
detect_architecture() {
    local arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)
            echo "amd64"
            ;;
        arm64|aarch64)
            echo "arm64"
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
}

# Function to get the latest release version
get_latest_version() {
    print_info "Fetching latest release version..." >&2
    local latest_version=$(curl -s "$GITHUB_API_URL/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
    
    if [[ -z "$latest_version" ]]; then
        print_error "Failed to fetch latest version from GitHub API" >&2
        exit 1
    fi
    
    echo "$latest_version"
}

# Function to validate version format
validate_version() {
    local version="$1"
    if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+.*$ ]]; then
        print_error "Invalid version format: $version (expected format: vX.Y.Z)"
        exit 1
    fi
}

# Function to check if release exists
check_release_exists() {
    local version="$1"
    local status_code=$(curl -s -o /dev/null -w "%{http_code}" "$GITHUB_API_URL/releases/tags/$version")
    
    if [[ "$status_code" != "200" ]]; then
        print_error "Release $version not found (HTTP $status_code)"
        exit 1
    fi
}

# Function to download file with progress
download_file() {
    local url="$1"
    local output_path="$2"
    local filename=$(basename "$output_path")
    
    print_info "Downloading $filename..."
    
    # Create directory if it doesn't exist
    mkdir -p "$(dirname "$output_path")"
    
    # Download with curl, showing progress
    if curl -L --progress-bar --fail -o "$output_path" "$url"; then
        print_success "Downloaded $filename"
        return 0
    else
        print_error "Failed to download $filename from $url"
        return 1
    fi
}

# Function to get file extension for platform
get_static_lib_extension() {
    local platform="$1"
    case "$platform" in
        windows)
            echo ".lib"
            ;;
        *)
            echo ".a"
            ;;
    esac
}

# Function to get dynamic library name and extension
get_dynamic_lib_info() {
    local platform="$1"
    case "$platform" in
        darwin)
            echo "liblancedb_go.dylib"
            ;;
        linux)
            echo "liblancedb_go.so"
            ;;
        windows)
            echo "lancedb_go.dll"
            ;;
    esac
}

# Function to get static library name
get_static_lib_name() {
    local platform="$1"
    case "$platform" in
        windows)
            echo "liblancedb_go.a"  # Use .a for better CGO compatibility
            ;;
        *)
            echo "liblancedb_go.a"
            ;;
    esac
}

# Function to set platform-specific linker flags
get_linker_flags() {
    local platform="$1"
    local arch="$2"
    local lib_path="$3"
    
    case "$platform" in
        darwin)
            echo "$lib_path -framework Security -framework CoreFoundation"
            ;;
        linux)
            echo "$lib_path -lm -ldl -lpthread"
            ;;
        windows)
            echo "$lib_path"
            ;;
        *)
            echo "$lib_path"
            ;;
    esac
}

# Main function
main() {
    local version="$1"
    
    print_info "LanceDB Go Native Artifacts Downloader"
    print_info "======================================"
    
    # Detect platform and architecture
    local platform=$(detect_platform)
    local arch=$(detect_architecture)
    local platform_arch="${platform}_${arch}"
    
    print_info "Detected platform: $platform"
    print_info "Detected architecture: $arch"
    print_info "Target platform: $platform_arch"
    
    # Get version to download
    if [[ -z "$version" ]]; then
        version=$(get_latest_version)
        print_info "Using latest version: $version"
    else
        validate_version "$version"
        print_info "Using specified version: $version"
    fi
    
    # Check if release exists
    check_release_exists "$version"
    
    # Create lib directory structure
    local current_dir=$(pwd)
    local lib_dir="$current_dir/lib/$platform_arch"
    local include_dir="$current_dir/include"
    local static_lib_name=$(get_static_lib_name "$platform")
    local lib_file="$lib_dir/$static_lib_name"
    
    print_info "Creating directory structure..."
    mkdir -p "$lib_dir"
    mkdir -p "$include_dir"
    
    # Prepare download URLs and paths
    local base_download_url="$GITHUB_RELEASES_URL/$version"
    local dynamic_lib_name=$(get_dynamic_lib_info "$platform")
    
    # Download static library
    local static_lib_url="$base_download_url/lib/$platform_arch/$static_lib_name"
    local static_lib_path="$lib_dir/$static_lib_name"
    
    if ! download_file "$static_lib_url" "$static_lib_path"; then
        print_warning "Individual file download failed. This may be because the release doesn't contain platform-specific files."
        print_info "Falling back to complete archive download (recommended approach)..."
        # Try downloading the complete archive as fallback
        download_complete_archive "$version" "$platform_arch"
        return $?
    fi
    
    # Download dynamic library (optional for some platforms)
    if [[ -n "$dynamic_lib_name" ]]; then
        local dynamic_lib_url="$base_download_url/lib/$platform_arch/$dynamic_lib_name"
        local dynamic_lib_path="$lib_dir/$dynamic_lib_name"
        
        if ! download_file "$dynamic_lib_url" "$dynamic_lib_path"; then
            print_warning "Dynamic library not available or download failed (this might be expected for static builds)"
        fi
    fi
    
    # Download header file
    local header_url="$base_download_url/include/lancedb.h"
    local header_path="$include_dir/lancedb.h"
    
    if ! download_file "$header_url" "$header_path"; then
        print_warning "Header file download failed"
    fi
    
    # Verify downloads
    print_info "Verifying downloaded files..."
    
    if [[ -f "$static_lib_path" ]]; then
        local static_size=$(ls -lh "$static_lib_path" | awk '{print $5}')
        print_success "Static library: $static_lib_path ($static_size)"
    else
        print_error "Static library not found: $static_lib_path"
    fi
    
    if [[ -f "$dynamic_lib_path" ]]; then
        local dynamic_size=$(ls -lh "$dynamic_lib_path" | awk '{print $5}')
        print_success "Dynamic library: $dynamic_lib_path ($dynamic_size)"
    fi
    
    if [[ -f "$header_path" ]]; then
        local header_size=$(ls -lh "$header_path" | awk '{print $5}')
        print_success "Header file: $header_path ($header_size)"
    fi
    
    print_success "Download completed for $platform_arch!"
    print_info "Files are available in the lib/$platform_arch/ directory"
}

# Function to download and extract complete archive (fallback)
download_complete_archive() {
    local version="$1"
    local target_platform="$2"
    
    print_info "Downloading complete archive as fallback..."
    
    local archive_url="$GITHUB_RELEASES_URL/$version/lancedb-go-native-binaries.tar.gz"
    local archive_path="lancedb-go-native-binaries.tar.gz"
    
    if download_file "$archive_url" "$archive_path"; then
        print_info "Extracting archive..."
        
        if tar -xzf "$archive_path"; then
            print_success "Archive extracted successfully"
            
            # Clean up archive
            rm -f "$archive_path"
            
            # Check if our target platform files exist
            if [[ -d "lib/$target_platform" ]]; then
                print_success "Platform-specific files found in archive"
                return 0
            else
                print_error "Platform $target_platform not found in archive"
                return 1
            fi
        else
            print_error "Failed to extract archive"
            rm -f "$archive_path"
            return 1
        fi
    else
        print_error "Failed to download complete archive"
        return 1
    fi
}

# Show usage if requested
show_usage() {
    echo "Usage: $0 [version]"
    echo ""
    echo "Download LanceDB Go native artifacts for the current platform."
    echo ""
    echo "Arguments:"
    echo "  version    Version to download (e.g., v1.0.0). If not provided, downloads latest release."
    echo ""
    echo "Examples:"
    echo "  $0                # Download latest release"
    echo "  $0 v1.0.0         # Download specific version"
    echo ""
    echo "The script will:"
    echo "  - Detect your platform (darwin/linux/windows) and architecture (amd64/arm64)"
    echo "  - Download the appropriate native libraries"
    echo "  - Place them in lib/{platform}_{arch}/ directory"
    echo "  - Download the C header file to include/ directory"
}

# Handle command line arguments
case "${1:-}" in
    -h|--help|help)
        show_usage
        exit 0
        ;;
    *)
        main "$1"
        ;;
esac
