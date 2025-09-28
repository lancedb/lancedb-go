# LanceDB Go SDK

A Go library for [LanceDB](https://github.com/lancedb/lancedb) with **pre-built native binaries**.

✨ **Now works out-of-the-box with `go get`** - no build dependencies required!

## Installation

```bash
go get github.com/lancedb/lancedb-go
```

**That's it!** Native libraries are pre-built and included. No need to install Rust, cbindgen, or any other dependencies.

### Supported Platforms

- **macOS**: Intel (amd64) and Apple Silicon (arm64)
- **Linux**: Intel/AMD (amd64) and ARM (arm64)  
- **Windows**: Intel/AMD (amd64)

## Usage

### Basic Example

```go
import (
    "context"
    "log"
    
    "github.com/lancedb/lancedb-go/pkg/lancedb"
    "github.com/apache/arrow/go/v17/arrow"
    "github.com/apache/arrow/go/v17/arrow/array"
    "github.com/apache/arrow/go/v17/arrow/memory"
)

// Connect to a database
ctx := context.Background()
conn, err := lancedb.Connect(ctx, "data/sample-lancedb", nil)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// Create a table with Arrow schema
fields := []arrow.Field{
    {Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
    {Name: "text", Type: arrow.BinaryTypes.String, Nullable: false},
    {Name: "vector", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32), Nullable: false},
}
arrowSchema := arrow.NewSchema(fields, nil)
schema, err := lancedb.NewSchema(arrowSchema)
if err != nil {
    log.Fatal(err)
}

table, err := conn.CreateTable(ctx, "my_table", schema)
if err != nil {
    log.Fatal(err)
}
defer table.Close()

// Insert some data
pool := memory.NewGoAllocator()
// ... prepare your data arrays using Arrow builders ...

// Perform vector search
queryVector := []float32{0.1, 0.3, /* ... 128 dimensions */}
results, err := table.VectorSearch(ctx, "vector", queryVector, 20)
if err != nil {
    log.Fatal(err)
}
fmt.Println(results)
```

## Examples

The [`examples/`](./examples) directory contains comprehensive examples demonstrating various LanceDB capabilities:

### 📚 Available Examples

1. **[Basic CRUD Operations](./examples/basic_crud/basic_crud.go)** - Fundamental database operations
   - Database connection and table creation
   - Schema definition with multiple data types
   - Insert, query, update, and delete operations
   - Error handling and resource management

2. **[Vector Search](./examples/vector_search/vector_search.go)** - Vector similarity search
   - Creating and storing vector embeddings
   - Basic and advanced vector similarity search
   - Performance benchmarking across different K values
   - Vector search with metadata filtering

3. **[Hybrid Search](./examples/hybrid_search/hybrid_search.go)** - Combining vector and traditional search
   - E-commerce product catalog with vectors and metadata
   - Vector search combined with SQL-like filters
   - Multi-modal query patterns and recommendations
   - Real-world search scenarios

4. **[Index Management](./examples/index_management/index_management.go)** - Creating and managing indexes
   - Vector indexes: IVF-PQ, IVF-Flat, HNSW-PQ
   - Scalar indexes: BTree for range queries, Bitmap for categorical data
   - Full-text search indexes
   - Performance comparison and optimization

5. **[Batch Operations](./examples/batch_operations/batch_operations.go)** - Efficient bulk data operations
   - Different batch insertion strategies
   - Memory-efficient processing of large datasets
   - Concurrent batch operations with goroutines
   - Error handling and recovery patterns

6. **[Storage Configuration](./examples/storage_configuration/storage_configuration.go)** - Storage setup
   - Local file system storage optimization
   - AWS S3 configuration with authentication methods
   - MinIO object storage for local development
   - Performance comparison and optimization

### 🚀 Quick Start

```bash
# Run any example
cd examples/basic_crud
go run basic_crud.go

# Or run from the examples directory
go run examples/basic_crud/basic_crud.go
```

See the detailed [examples README](./examples/README.md) for comprehensive documentation, configuration options, and advanced usage patterns.

## 🚀 Binary Distribution

This package uses **pre-built native binaries** to eliminate build dependencies:

### ✅ What This Means for You
- **No Rust installation required**
- **No cbindgen or other build tools needed**  
- **Works immediately** after `go get`
- **Cross-platform** support out of the box
- **Consistent experience** across all environments

### 🔧 For Contributors & Maintainers
- **Build locally**: `make build-native`
- **Build all platforms**: `make build-all-platforms`  
- **Test distribution**: `make test-dist`
- **See detailed guide**: [BINARY_DISTRIBUTION.md](./BINARY_DISTRIBUTION.md)

### 📦 How It Works
The repository includes pre-built libraries for all supported platforms in the `lib/` directory. Go's CGO system automatically selects the correct library for your platform during compilation.

**No setup required** - it just works! 🎉

## Development

### Quick Start

```shell
# Install all development dependencies
make install-deps

# Build the project
make build

# Run tests
make test

# Lint code (requires golangci-lint)
make lint

# Format code
make fmt
```

### Go Linting

This project uses [golangci-lint](https://golangci-lint.run/) for comprehensive Go code linting:

```shell
# Install golangci-lint (included in install-deps)
make install-deps

# Lint Go code
make lint-go

# Lint and auto-fix issues
make lint-go-fix
```

See [CONTRIBUTING.md](./CONTRIBUTING.md) for detailed development guidelines, linting configuration, and contribution instructions.