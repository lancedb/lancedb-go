// Package artifactmeta parses the packaging metadata a generated companion
// module declares in artifact.go.
//
// The origin generator treats candidate modules as untrusted input: it
// re-reads the declared metadata, checks its shape, and compares the claims
// with the payload bytes it is about to serve.
package artifactmeta

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
)

// Metadata is the verified metadata of one generated artifact module.
type Metadata struct {
	Target             string
	LibraryName        string
	PayloadName        string
	CompressedSize     int64
	CompressedSHA256   string
	UncompressedSize   int64
	UncompressedSHA256 string
	UpstreamRepo       string
	UpstreamCommit     string
	HeaderSHA256       string
}

var requiredStrings = []string{
	"Target",
	"LibraryName",
	"CompressedSHA256",
	"UncompressedSHA256",
	"UpstreamRepo",
	"UpstreamCommit",
	"HeaderSHA256",
}

var requiredInts = []string{
	"CompressedSize",
	"UncompressedSize",
}

// ParseFile reads and parses an artifact.go from disk.
func ParseFile(path string) (Metadata, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return Metadata{}, err
	}
	meta, err := ParseSource(source)
	if err != nil {
		return Metadata{}, fmt.Errorf("%s: %w", path, err)
	}
	return meta, nil
}

// ParseSource parses artifact.go source and returns the declared metadata.
func ParseSource(source []byte) (Metadata, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "artifact.go", source, parser.ParseComments)
	if err != nil {
		return Metadata{}, fmt.Errorf("parse artifact.go: %w", err)
	}
	if file.Name.Name != "artifact" {
		return Metadata{}, fmt.Errorf("artifact.go declares package %q, want artifact", file.Name.Name)
	}

	stringValues := map[string]string{}
	intValues := map[string]int64{}
	embedCount := 0
	payloadName := ""

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			return Metadata{}, errors.New("artifact.go must contain only imports and constant or variable declarations")
		}
		switch gen.Tok {
		case token.IMPORT:
			for _, spec := range gen.Specs {
				imp, ok := spec.(*ast.ImportSpec)
				if !ok {
					return Metadata{}, errors.New("artifact.go has an invalid import")
				}
				if imp.Path.Value != `"embed"` {
					return Metadata{}, fmt.Errorf("artifact.go imports %s; the module must import only embed", imp.Path.Value)
				}
			}
		case token.VAR:
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					return Metadata{}, errors.New("artifact.go has an invalid variable declaration")
				}
				if len(value.Names) != 1 || value.Names[0].Name != "Payload" {
					return Metadata{}, errors.New("artifact.go must declare exactly one variable, Payload")
				}
				if len(value.Values) != 0 {
					return Metadata{}, errors.New("artifact.go Payload must not have an initializer")
				}
				if !isByteSlice(value.Type) {
					return Metadata{}, errors.New("artifact.go Payload must have type []byte")
				}
				for _, comment := range commentsFor(gen, value) {
					text := strings.TrimSpace(comment.Text)
					if !strings.HasPrefix(text, "//go:embed ") {
						continue
					}
					name, err := strconv.Unquote(strings.TrimSpace(strings.TrimPrefix(text, "//go:embed ")))
					if err != nil {
						return Metadata{}, fmt.Errorf("artifact.go has an unquoted go:embed directive: %w", err)
					}
					embedCount++
					payloadName = name
				}
			}
		case token.CONST:
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Values) != len(value.Names) {
					return Metadata{}, errors.New("artifact.go metadata constants must be plain literals")
				}
				for index, name := range value.Names {
					literal, ok := value.Values[index].(*ast.BasicLit)
					if !ok {
						return Metadata{}, fmt.Errorf("artifact.go constant %s is not a literal", name.Name)
					}
					switch literal.Kind {
					case token.STRING:
						text, err := strconv.Unquote(literal.Value)
						if err != nil {
							return Metadata{}, fmt.Errorf("artifact.go constant %s: %w", name.Name, err)
						}
						stringValues[name.Name] = text
					case token.INT:
						number, err := strconv.ParseInt(literal.Value, 10, 64)
						if err != nil {
							return Metadata{}, fmt.Errorf("artifact.go constant %s: %w", name.Name, err)
						}
						intValues[name.Name] = number
					default:
						return Metadata{}, fmt.Errorf("artifact.go constant %s has unsupported type", name.Name)
					}
				}
			}
		default:
			return Metadata{}, errors.New("artifact.go must not contain type declarations or other top-level statements")
		}
	}

	if embedCount != 1 {
		return Metadata{}, fmt.Errorf("artifact.go must have exactly one go:embed directive, found %d", embedCount)
	}
	for _, name := range requiredStrings {
		if _, ok := stringValues[name]; !ok {
			return Metadata{}, fmt.Errorf("artifact.go is missing constant %s", name)
		}
	}
	for _, name := range requiredInts {
		if _, ok := intValues[name]; !ok {
			return Metadata{}, fmt.Errorf("artifact.go is missing constant %s", name)
		}
	}

	meta := Metadata{
		Target:             stringValues["Target"],
		LibraryName:        stringValues["LibraryName"],
		PayloadName:        payloadName,
		CompressedSize:     intValues["CompressedSize"],
		CompressedSHA256:   stringValues["CompressedSHA256"],
		UncompressedSize:   intValues["UncompressedSize"],
		UncompressedSHA256: stringValues["UncompressedSHA256"],
		UpstreamRepo:       stringValues["UpstreamRepo"],
		UpstreamCommit:     stringValues["UpstreamCommit"],
		HeaderSHA256:       stringValues["HeaderSHA256"],
	}
	if err := meta.validate(); err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

func commentsFor(gen *ast.GenDecl, value *ast.ValueSpec) []*ast.Comment {
	var comments []*ast.Comment
	if gen.Doc != nil {
		comments = append(comments, gen.Doc.List...)
	}
	if value.Doc != nil {
		comments = append(comments, value.Doc.List...)
	}
	if value.Comment != nil {
		comments = append(comments, value.Comment.List...)
	}
	return comments
}

func isByteSlice(expr ast.Expr) bool {
	array, ok := expr.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return false
	}
	element, ok := array.Elt.(*ast.Ident)
	return ok && element.Name == "byte"
}

func (m Metadata) validate() error {
	if m.Target == "" {
		return errors.New("artifact metadata Target is empty")
	}
	if m.LibraryName == "" {
		return errors.New("artifact metadata LibraryName is empty")
	}
	if m.PayloadName == "" {
		return errors.New("artifact metadata PayloadName is empty")
	}
	if m.CompressedSize <= 0 {
		return fmt.Errorf("artifact metadata CompressedSize %d is not positive", m.CompressedSize)
	}
	if m.UncompressedSize <= 0 {
		return fmt.Errorf("artifact metadata UncompressedSize %d is not positive", m.UncompressedSize)
	}
	if !IsLowerHex(m.CompressedSHA256, 64) {
		return fmt.Errorf("artifact metadata CompressedSHA256 %q is not 64 lowercase hex characters", m.CompressedSHA256)
	}
	if !IsLowerHex(m.UncompressedSHA256, 64) {
		return fmt.Errorf("artifact metadata UncompressedSHA256 %q is not 64 lowercase hex characters", m.UncompressedSHA256)
	}
	if m.UpstreamRepo == "" {
		return errors.New("artifact metadata UpstreamRepo is empty")
	}
	if !IsLowerHex(m.UpstreamCommit, 40) {
		return fmt.Errorf("artifact metadata UpstreamCommit %q is not a 40-character lowercase hex commit", m.UpstreamCommit)
	}
	if !IsLowerHex(m.HeaderSHA256, 64) {
		return fmt.Errorf("artifact metadata HeaderSHA256 %q is not 64 lowercase hex characters", m.HeaderSHA256)
	}
	return nil
}

// IsLowerHex reports whether value is a lowercase hex string of length size.
func IsLowerHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	for index := 0; index < len(value); index++ {
		c := value[index]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
