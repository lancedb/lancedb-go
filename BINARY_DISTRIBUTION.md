# LanceDB Go SDK - Binary Distribution Guide

This document explains how the **Pre-built Binary Distribution** system works in lancedb-go.

## 🎯 Problem Solved

Previously, lancedb-go required users to:
- Install Rust and Cargo
- Install cbindgen  
- Manually build native libraries
- Deal with platform-specific compilation issues

**Now**: Just run `go get github.com/lancedb/lancedb-go` and it works immediately! ✨

## 🏗️ How It Works

### 1. **Platform-Specific Libraries**
The repository includes pre-built native libraries for all supported platforms:

```
lib/
├── darwin_amd64/     # macOS Intel
├── darwin_arm64/     # macOS Apple Silicon  
├── linux_amd64/      # Linux AMD64
├── linux_arm64/      # Linux ARM64
└── windows_amd64/    # Windows AMD64
```

### 2. **Smart CGO Directives**
The CGO configuration automatically selects the right library:

```go
/*
#cgo CFLAGS: -I${SRCDIR}/../../include
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/../../lib/darwin_amd64 -llancedb_go
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/../../lib/darwin_arm64 -llancedb_go
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../lib/linux_amd64 -llancedb_go
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../lib/linux_arm64 -llancedb_go
#cgo windows,amd64 LDFLAGS: -L${SRCDIR}/../../lib/windows_amd64 -llancedb_go
#include "lancedb.h"
*/
```

### 3. **Automated Release Process**
When a new version is released:

1. **GitHub Actions builds** native libraries for all platforms
2. **Libraries are committed** to the repository
3. **Release is tagged** with pre-built binaries
4. **Users can install** with just `go get`

## 🚀 For End Users

### Installation
```bash
go get github.com/lancedb/lancedb-go
```

### Usage
```go
import "github.com/lancedb/lancedb-go/pkg/lancedb"

// Works immediately - no build dependencies required!
conn, err := lancedb.Connect(ctx, "my-database", nil)
```

### Supported Platforms
- ✅ **macOS**: Intel (amd64) and Apple Silicon (arm64)
- ✅ **Linux**: Intel/AMD (amd64) and ARM (arm64)
- ✅ **Windows**: Intel/AMD (amd64)

## 🛠️ For Developers

### Building Locally

#### Single Platform (Current)
```bash
make build-native
```

#### All Platforms
```bash
make build-all-platforms
```

#### Testing Distribution
```bash
make test-dist
```

### Manual Build
```bash
# Build for specific platform
./scripts/build-native.sh darwin arm64

# Build all platforms
./scripts/build-all-platforms.sh

# Test the distribution
./scripts/test-binary-distribution.sh
```

### Directory Structure
```
lancedb-go/
├── lib/                    # Platform-specific binaries
│   ├── darwin_amd64/
│   ├── darwin_arm64/
│   ├── linux_amd64/
│   ├── linux_arm64/
│   └── windows_amd64/
├── include/                # C headers
│   └── lancedb.h
├── scripts/                # Build scripts
│   ├── build-native.sh
│   ├── build-all-platforms.sh
│   └── test-binary-distribution.sh
├── pkg/                    # Go source code
└── rust/                   # Rust source code
```

## 🔄 Release Process

### Creating a Release

1. **Build all platforms**:
   ```bash
   make build-all-platforms
   ```

2. **Test distribution**:
   ```bash
   make test-dist
   ```

3. **Commit binaries**:
   ```bash
   git add lib/ include/
   git commit -m "Add pre-built binaries for vX.Y.Z"
   git push
   ```

4. **Create and push tag**:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

5. **GitHub Actions automatically**:
   - Builds all platforms
   - Creates GitHub release
   - Updates documentation

### GitHub Actions Workflow

The release workflow (`.github/workflows/release.yml`) automatically:

- **Builds** native libraries for all platforms
- **Verifies** binary completeness
- **Creates** GitHub release with artifacts
- **Commits** binaries to the repository
- **Updates** documentation

## 🧪 Testing

### Automated Tests
```bash
# Test current platform
make test-dist

# Test all platforms (requires cross-compilation setup)
./scripts/build-all-platforms.sh
./scripts/test-binary-distribution.sh
```

### Manual Verification
```bash
# Create test project
mkdir test-lancedb && cd test-lancedb
go mod init test

# Add dependency (using local version for testing)
echo "replace github.com/lancedb/lancedb-go => ../lancedb-go" >> go.mod
go mod edit -require github.com/lancedb/lancedb-go@v0.0.0

# Test build
go build
```

## 🔧 Troubleshooting

### Missing Platform Libraries
**Error**: `fatal error: 'lancedb.h' file not found`

**Solution**:
```bash
# Build for your platform
make build-native

# Or build all platforms
make build-all-platforms
```

### CGO Linking Issues
**Error**: `undefined symbol: simple_lancedb_init`

**Possible causes**:
1. Library not built for your platform
2. Library architecture mismatch
3. Missing system dependencies

**Solution**:
```bash
# Rebuild for your specific platform
./scripts/build-native.sh $(uname -s | tr '[:upper:]' '[:lower:]') $(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')

# Test the build
./scripts/test-binary-distribution.sh
```

### Cross-Compilation Setup

For building all platforms locally, you need cross-compilation tools:

#### Linux ARM64 (on Linux AMD64)
```bash
sudo apt-get install gcc-aarch64-linux-gnu
export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=aarch64-linux-gnu-gcc
```

#### Windows (on macOS/Linux)
```bash
rustup target add x86_64-pc-windows-msvc
# Note: Full Windows cross-compilation requires additional setup
```

## 📊 Benefits

### For Users
- ✅ **No build dependencies** (Rust, cbindgen, etc.)
- ✅ **Instant installation** with `go get`
- ✅ **Cross-platform support** out of the box
- ✅ **Consistent experience** across all platforms

### For Maintainers  
- ✅ **Automated builds** via GitHub Actions
- ✅ **Quality assurance** with automated testing
- ✅ **Easy releases** with binary distribution
- ✅ **Reduced support burden** (fewer build issues)

## 🚧 Migration from Build-Required Version

If you were using the old version that required building:

### Before
```bash
# Old way - lots of dependencies
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
cargo install cbindgen
git clone https://github.com/lancedb/lancedb-go
cd lancedb-go
make build
# ... use local version with replace directive
```

### After  
```bash
# New way - just works!
go get github.com/lancedb/lancedb-go
```

### Code Changes
**No code changes required!** The API remains exactly the same.

## 🤝 Contributing

To contribute to the binary distribution system:

1. **Test locally**: Use `make test-dist` 
2. **Build all platforms**: Use `make build-all-platforms`
3. **Update scripts**: Modify files in `scripts/`
4. **Test workflow**: Use GitHub Actions for full testing

## 📚 References

- **Examples**: See `examples/` directory for usage patterns
- **Build Scripts**: See `scripts/` directory for build automation  
- **CI/CD**: See `.github/workflows/release.yml` for release process
- **Makefile**: See available targets with `make help`
