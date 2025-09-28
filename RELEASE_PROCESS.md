# LanceDB Go SDK - Release Process

This document explains how the release process works and how to fix common issues.

## Release Process Overview

### ✅ Correct Approach (Current)

1. **Build Artifacts**: CI builds native libraries for all supported platforms
2. **Create Archive**: All binaries are packaged into a release archive
3. **GitHub Release**: Binaries are attached to GitHub Release (not committed to Git)
4. **Distribution**: Users download binaries from GitHub Releases

### ❌ Previous Approach (Deprecated)

- ~~Committing binary files directly to Git repository~~ 
- ~~Using Git LFS for large binary files~~

## Why We Don't Commit Binaries to Git

Committing large binary files to Git causes several problems:

1. **Repository Bloat**: Each release adds ~814MB to repository history
2. **Slow Clones**: New contributors download hundreds of MB unnecessarily  
3. **Push Timeouts**: Git HTTP timeout (408 errors) when pushing large files
4. **Bandwidth Waste**: Every `git clone` downloads all historical binary versions
5. **Version Control Pollution**: Binary files don't benefit from Git's text-based features

## Current Release Workflow

### Triggering a Release

```bash
# Create and push a version tag
git tag v1.0.0
git push origin v1.0.0
```

### What Happens Automatically

1. **Build Matrix**: CI builds for all platforms (darwin/linux/windows, amd64/arm64)
2. **Artifact Collection**: All platform binaries are downloaded and verified
3. **Archive Creation**: Creates `lancedb-go-native-binaries.tar.gz`
4. **GitHub Release**: Creates release with individual binaries + archive attached
5. **No Git Commits**: Binaries are NOT committed to the repository

### Release Artifacts

Each release includes:

- **Complete Archive**: `lancedb-go-native-binaries.tar.gz` (all platforms)
- **Individual Binaries**: Platform-specific `.a`, `.lib`, `.dll`, `.so`, `.dylib` files  
- **Header File**: `lancedb.h` for C integration
- **Release Notes**: Generated documentation

## Troubleshooting Common Issues

### HTTP 408 Timeout Errors

**Symptoms**: 
```
error: RPC failed; HTTP 408 curl 22 The requested URL returned error: 408
send-pack: unexpected disconnect while reading sideband packet
```

**Cause**: Attempting to push large binary files to Git

**Solution**: Don't commit binaries to Git. Use GitHub Releases instead (already implemented).

### Large Repository Size

**Symptoms**: Slow `git clone`, large `.git` folder

**Solution**: Remove historical binary files from Git history:
```bash
# WARNING: This rewrites Git history - coordinate with team
git filter-repo --path-glob 'lib/**' --invert-paths --force
```

### Missing Binaries

**Symptoms**: Users can't find pre-built binaries

**Solution**: Direct users to GitHub Releases page:
- https://github.com/lancedb/lancedb-go/releases
- Or use `go get github.com/lancedb/lancedb-go` (auto-downloads)

## Benefits of Current Approach

✅ **Fast Repository**: Only source code in Git  
✅ **Reliable Releases**: No timeout issues with GitHub Release uploads  
✅ **Proper Distribution**: Binaries where they belong (releases, not version control)  
✅ **Better UX**: Users get exactly what they need per platform  
✅ **Scalable**: Works for any binary size  

## For Developers

### Local Development
```bash
make build-native          # Build for current platform
make build-all-platforms   # Build for all platforms  
make test-dist             # Test binary distribution
```

### Testing Releases Locally
```bash
# Test with act (GitHub Actions local runner)
make ci-build              # Test build workflow
act -j create-release      # Test release creation
```

### Manual Release (Emergency)
If CI fails, you can create releases manually:

1. Build binaries locally: `make build-all-platforms`
2. Create GitHub release via web interface
3. Upload binary files as release assets
4. Update release notes

## Migration Notes

If you have historical binary files in your Git repository:

1. **Don't panic**: New releases won't add more
2. **Optional cleanup**: Use `git filter-repo` to remove historical binaries  
3. **Inform team**: Make sure everyone understands the new process
4. **Update docs**: Point users to GitHub Releases for downloads

---

For questions or issues, check:
- [BINARY_DISTRIBUTION.md](BINARY_DISTRIBUTION.md) - Technical details
- [GitHub Releases](https://github.com/lancedb/lancedb-go/releases) - Download binaries
- [GitHub Issues](https://github.com/lancedb/lancedb-go/issues) - Report problems
