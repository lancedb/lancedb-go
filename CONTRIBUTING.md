# Contributing to LanceDB Go SDK

Thank you for your interest in contributing to the LanceDB Go SDK! This document provides guidelines and instructions for contributors.

## Development Setup

### Prerequisites

- **Go 1.21+**: Download from [golang.org](https://golang.org/dl/)
- **Rust 1.78+**: Install via [rustup.rs](https://rustup.rs/)
- **Make**: Standard build tool
- **Git**: Version control

### Initial Setup

1. Fork and clone the repository:
```bash
git clone https://github.com/lancedb/lancedb-go.git
cd lancedb-go
```

2. Install all development dependencies:
```bash
make install-deps
```

This will install:
- Rust toolchain updates
- cbindgen (for C header generation)
- Go dependencies
- golangci-lint (Go linter)

3. Verify the setup:
```bash
make build
make test
```

## Code Quality and Linting

We use comprehensive linting to maintain high code quality. The project includes linting for both Go and Rust code.

### Go Linting

The project uses [golangci-lint](https://golangci-lint.run/) with a comprehensive configuration that includes:

#### Enabled Linters

- **Basic linters**: errcheck, gosimple, govet, ineffassign, staticcheck, typecheck, unused
- **Code quality**: gofmt, goimports, misspell, unconvert, unparam, gocritic
- **Style and conventions**: revive, stylecheck, gocyclo, funlen, gocognit, nestif, noctx
- **Error handling**: errorlint, wrapcheck
- **Performance**: prealloc, maligned
- **Security**: gosec
- **Maintainability**: dupl, goconst, gomnd

#### Installing golangci-lint

##### Option 1: Using the Makefile (Recommended)
```bash
make install-deps
```

##### Option 2: Manual Installation

**On macOS:**
```bash
# Using Homebrew
brew install golangci-lint

# Using Go
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

**On Linux:**
```bash
# Using the install script
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2

# Or using Go
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

**On Windows:**
```powershell
# Using Go
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

#### Running Linters

```bash
# Lint all code (Rust + Go)
make lint

# Lint only Go code
make lint-go

# Lint only Rust code
make lint-rust

# Lint Go code and automatically fix issues
make lint-go-fix

# Generate detailed linting report
make lint-report
```

#### Linting Configuration

The Go linting configuration is defined in `.golangci.yml`. Key features:

- **Comprehensive coverage**: 25+ linters enabled
- **Context-aware**: Different rules for tests and examples
- **CGO-friendly**: Special handling for CGO-related code
- **Configurable thresholds**: Reasonable complexity and length limits
- **Auto-fixable issues**: Many issues can be automatically resolved

#### Common Linting Issues and Fixes

**Unchecked errors:**
```go
// Bad
file.Close()

// Good
if err := file.Close(); err != nil {
    log.Printf("Failed to close file: %v", err)
}
```

**Missing documentation:**
```go
// Bad
func ProcessData(data []byte) error {
    // ...
}

// Good
// ProcessData processes the input data and returns an error if processing fails.
func ProcessData(data []byte) error {
    // ...
}
```

**Magic numbers:**
```go
// Bad
if len(data) > 100 {
    // ...
}

// Good
const maxDataLength = 100

if len(data) > maxDataLength {
    // ...
}
```

### Rust Linting

Rust code is linted using Clippy with strict warnings:

```bash
# Lint Rust code
make lint-rust

# Or directly
cd rust && cargo clippy -- -D warnings
```

## Code Formatting

### Go Formatting
```bash
# Format all Go code
go fmt ./...

# Or using the Makefile
make fmt
```

### Rust Formatting
```bash
# Format Rust code
cd rust && cargo fmt

# Or using the Makefile
make fmt
```

## Testing

### Running Tests
```bash
# Run all tests
make test

# Run only Go tests
go test -v ./...

# Run benchmarks
make bench
```

### Writing Tests

- Place test files next to the code they test with `_test.go` suffix
- Use table-driven tests for multiple test cases
- Include both positive and negative test cases
- Test error conditions and edge cases

Example:
```go
func TestTableOperations(t *testing.T) {
    tests := []struct {
        name    string
        input   interface{}
        want    interface{}
        wantErr bool
    }{
        {"valid input", validInput, expectedOutput, false},
        {"invalid input", invalidInput, nil, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Function() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Function() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Building and Development Workflow

### Standard Development Workflow

1. **Start development**:
   ```bash
   make dev-setup  # Set up development environment
   ```

2. **Make changes**:
   - Edit code
   - Add tests
   - Update documentation

3. **Check your changes**:
   ```bash
   make fmt        # Format code
   make lint       # Check for linting issues
   make test       # Run tests
   make build      # Build everything
   ```

4. **Fix any issues**:
   ```bash
   make lint-go-fix  # Auto-fix Go linting issues
   ```

5. **Commit and push**:
   ```bash
   git add .
   git commit -m "Your descriptive commit message"
   git push origin feature-branch
   ```

### Build Targets

```bash
make help  # Show all available targets
```

Available targets:
- `all` - Build and test
- `build` - Build Rust library and Go bindings
- `test` - Run tests
- `bench` - Run benchmarks
- `clean` - Clean build artifacts
- `install-deps` - Install development dependencies
- `fmt` - Format code
- `lint` - Lint all code
- `lint-go` - Lint Go code only
- `lint-go-fix` - Lint and fix Go code automatically
- `lint-report` - Generate detailed linting reports
- `examples` - Build all examples
- `run-examples` - Run all examples

## Submitting Changes

### Pull Request Guidelines

1. **Before submitting**:
   - Ensure all tests pass: `make test`
   - Ensure code is properly formatted: `make fmt`
   - Ensure no linting issues: `make lint`
   - Add tests for new functionality
   - Update documentation as needed

2. **PR Description**:
   - Clearly describe what changes you made
   - Explain why the changes are necessary
   - Reference any related issues
   - Include examples if applicable

3. **Code Review**:
   - Address all feedback from reviewers
   - Keep commits focused and atomic
   - Squash commits if requested

### Commit Messages

Follow conventional commit format:
```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New features
- `fix`: Bug fixes
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

Examples:
- `feat(table): add vector search functionality`
- `fix(connection): handle connection timeout properly`
- `docs(readme): update installation instructions`

## Architecture and Code Organization

### Project Structure
```
lancedb-go/
├── rust/                    # Rust CGO bindings
│   ├── src/                 # Rust source files
│   ├── Cargo.toml          # Rust project config
│   └── target/             # Rust build artifacts
├── pkg/                    # Go SDK code
│   ├── connection.go       # Database connection
│   ├── table.go           # Table operations
│   ├── query.go           # Query building
│   └── schema.go          # Schema management
├── examples/              # Usage examples
├── .golangci.yml         # Go linting configuration
└── Makefile              # Build automation
```

### Coding Standards

#### Go Code Style
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use meaningful variable and function names
- Keep functions focused and small
- Handle errors explicitly
- Use interfaces for abstractions
- Add godoc comments for exported functions

#### Rust Code Style
- Follow [Rust API Guidelines](https://rust-lang.github.io/api-guidelines/)
- Use `cargo fmt` for formatting
- Handle errors with `Result` types
- Use meaningful error messages
- Follow Rust naming conventions

## Getting Help

If you need help:
- Check existing documentation and examples
- Search existing issues on GitHub
- Create a new issue with a clear description
- Join the LanceDB community discussions

## License

By contributing to this project, you agree that your contributions will be licensed under the Apache 2.0 License.
