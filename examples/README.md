# LanceDB Go Examples

This directory contains comprehensive examples demonstrating various LanceDB capabilities using the Go SDK. Each example is a complete, runnable program that showcases different aspects of working with LanceDB.

## 🚀 Quick Start

The easiest way to run examples is using the provided Makefile:

```bash
# Download native libraries and run all examples
make

# Or run individual examples
make basic-crud
make vector-search
make hybrid-search
```

## 📋 Prerequisites

Before running examples, you need to set up native libraries:

### Option 1: Using Makefile (Recommended)
```bash
# Download libraries automatically
make download-artifacts
```

### Option 2: Manual Download
```bash
# Download the script
curl -O https://raw.githubusercontent.com/lancedb/lancedb-go/main/scripts/download-artifacts.sh
chmod +x download-artifacts.sh

# Run it
./download-artifacts.sh
```

### Option 3: Check Your Setup
```bash
# See platform detection and CGO configuration
make platform-info
```

## 📚 Available Examples

### 1. [Basic CRUD Operations](./basic_crud/basic_crud.go)
**Fundamental database operations**

Learn the basics of LanceDB with this comprehensive example:
- Database connection and table creation
- Schema definition with multiple data types (integers, strings, vectors)
- Insert operations with Arrow builders
- Query operations with filtering
- Update and delete operations
- Proper error handling and resource management

```bash
make basic-crud
```

**Key concepts covered:**
- Arrow schema creation
- Memory management with Arrow allocators
- Batch record creation
- SQL-like filtering syntax

### 2. [Vector Search](./vector_search/vector_search.go)
**Vector similarity search and embeddings**

Explore LanceDB's core vector search capabilities:
- Creating and storing high-dimensional vector embeddings
- Basic vector similarity search (cosine similarity)
- Advanced search with distance metrics
- Performance benchmarking across different K values
- Vector search combined with metadata filtering

```bash
make vector-search
```

**Key concepts covered:**
- Vector embedding creation
- Similarity search algorithms
- Distance metrics (L2, cosine)
- Search result ranking
- Performance optimization

### 3. [Hybrid Search](./hybrid_search/hybrid_search.go)
**Combining vector and traditional search**

Learn how to build sophisticated search systems:
- E-commerce product catalog with vectors and metadata
- Vector search combined with SQL-like filters
- Multi-modal query patterns
- Recommendation system patterns
- Real-world search scenarios

```bash
make hybrid-search
```

**Key concepts covered:**
- Metadata filtering with vector search
- Complex query composition
- Search result fusion
- Recommendation algorithms

### 4. [Index Management](./index_management/index_management.go)
**Creating and managing indexes for performance**

Optimize your database performance with proper indexing:
- Vector indexes: IVF-PQ, IVF-Flat, HNSW-PQ
- Scalar indexes: BTree for range queries, Bitmap for categorical data
- Full-text search indexes
- Performance comparison and optimization strategies
- Index maintenance and monitoring

```bash
make index-management
```

**Key concepts covered:**
- Index types and use cases
- Performance tuning
- Memory vs accuracy trade-offs
- Index maintenance strategies

### 5. [Batch Operations](./batch_operations/batch_operations.go)
**Efficient bulk data operations**

Handle large-scale data efficiently:
- Different batch insertion strategies
- Memory-efficient processing of large datasets
- Concurrent batch operations with goroutines
- Error handling and recovery patterns
- Progress monitoring and reporting

```bash
make batch-operations
```

**Key concepts covered:**
- Batch processing patterns
- Memory optimization
- Concurrency patterns
- Error recovery strategies

### 6. [Storage Configuration](./storage_configuration/storage_configuration.go)
**Storage setup and optimization**

Configure LanceDB for different storage backends:
- Local file system storage optimization
- AWS S3 configuration with different authentication methods
- MinIO object storage for local development
- Performance comparison across storage types
- Storage security and access patterns

```bash
make storage-configuration
```

**Key concepts covered:**
- Storage backend configuration
- Authentication patterns
- Performance optimization
- Security best practices

## 🛠️ Development Workflow

### Building Examples

```bash
# Build all examples
make build-all

# Build without running
make build-all

# Test builds (quick verification)
make test
```

### Running Examples

```bash
# Run all examples sequentially
make run-all

# Run specific example
make basic-crud
make vector-search
# ... etc
```

### Platform Information

```bash
# Check your platform and CGO configuration
make platform-info
```

Example output:
```
Platform Detection Information:
================================
Operating System: Darwin
Architecture:     arm64
Normalized Platform: darwin
Normalized Arch:     arm64
Platform-Arch:       darwin_arm64
Current Directory:   /path/to/examples

CGO Configuration:
==================
CGO_CFLAGS:  -I/path/to/examples/include
CGO_LDFLAGS: /path/to/examples/lib/darwin_arm64/liblancedb_go.a -framework Security -framework CoreFoundation

Library Status:
===============
✅ Library directory exists: /path/to/examples/lib/darwin_arm64/
```

### Cleaning Up

```bash
# Clean built binaries and temp files
make clean

# Clean everything including downloaded libraries
make clean-all
```

## 🔧 Manual Setup (Advanced)

If you prefer to set up CGO manually:

```bash
# 1. Download artifacts (if not done already)
make download-artifacts

# 2. Get platform-specific CGO flags
make platform-info

# 3. Set environment variables
export CGO_CFLAGS="-I$(pwd)/include"
export CGO_LDFLAGS="$(pwd)/lib/darwin_arm64/liblancedb_go.a -framework Security -framework CoreFoundation"

# 4. Build and run manually
cd basic_crud
go run basic_crud.go
```

## 📁 Project Structure

```
examples/
├── Makefile                    # Build automation
├── README.md                   # This file
├── go.mod                      # Go module (separate from main project)
├── go.sum                      # Go module checksums
├── lib/                        # Native libraries (downloaded)
│   └── {platform}_{arch}/      # Platform-specific binaries
├── include/                    # C headers (downloaded)
│   └── lancedb.h              # Main header file
├── bin/                        # Built examples (created by make)
├── basic_crud/
│   └── basic_crud.go
├── vector_search/
│   └── vector_search.go
├── hybrid_search/
│   └── hybrid_search.go
├── index_management/
│   └── index_management.go
├── batch_operations/
│   └── batch_operations.go
└── storage_configuration/
    └── storage_configuration.go
```

## 🚨 Troubleshooting

### Common Issues

**1. "Native libraries not found"**
```bash
# Solution: Download the libraries
make download-artifacts
```

**2. "Header files not found"**
```bash
# Check if include directory exists
ls -la include/

# Re-download if missing
make download-artifacts
```

**3. "CGO compilation failed"**
```bash
# Check your platform configuration
make platform-info

# Ensure CGO environment variables are set correctly
echo $CGO_CFLAGS
echo $CGO_LDFLAGS
```

**4. "Build failed on different platform"**
- The Makefile automatically detects your platform
- Supported: macOS (amd64/arm64), Linux (amd64/arm64), Windows (amd64)
- Run `make platform-info` to verify detection

### Getting Help

1. **Check platform info**: `make platform-info`
2. **Verify library setup**: `ls -la lib/*/`
3. **Test basic build**: `make test`
4. **Clean and retry**: `make clean-all && make`

For more help, see the main [LanceDB Go repository](https://github.com/lancedb/lancedb-go) documentation.

## 🎯 Next Steps

After running the examples:

1. **Explore the source code** - Each example is well-commented
2. **Modify examples** - Try changing parameters and see the effects
3. **Build your own application** - Use examples as templates
4. **Performance testing** - Run benchmarks with your own data
5. **Join the community** - Share your use cases and get support

## 📄 License

These examples are part of the LanceDB Go SDK and follow the same license terms.