# LanceDB Go SDK

A Go library for [LanceDB](https://github.com/lancedb/lancedb).

## Installation

```bash
go get github.com/lancedb/lancedb-go/pkg
```

## Usage

### Basic Example

```go
import "github.com/lancedb/lancedb-go/pkg/lancedb"

// Connect to a database
ctx := context.Background()
db, err := lancedb.Connect(ctx, "data/sample-lancedb", nil)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Create a table with schema
schema := lancedb.NewSchemaBuilder().
    AddInt32Field("id", false).
    AddStringField("text", false).
    AddVectorField("vector", 128, lancedb.VectorDataTypeFloat32, false).
    Build()

table, err := db.CreateTable(ctx, "my_table", *schema)
if err != nil {
    log.Fatal(err)
}
defer table.Close()

// Perform vector search
queryVector := []float32{0.1, 0.3, /* ... 128 dimensions */}
results, err := table.VectorSearch("vector", queryVector, 20)
if err != nil {
    log.Fatal(err)
}
fmt.Println(results)
```

The [quickstart guide](./examples) contains more complete examples.

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