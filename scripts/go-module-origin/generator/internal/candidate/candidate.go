// Package candidate builds one data-only Go companion module from an already
// extracted lancedb-c shared library and its matching header.
//
// The generated module embeds a zstd-compressed copy of the library. It does
// not import the root module, link native code, or download anything. The
// library binary format and architecture are checked so a payload cannot be
// published for the wrong target.
package candidate

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/hex"
	"errors"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/klauspost/compress/zstd"
)

// DefaultGoVersion is the Go directive written into every generated companion
// module. It matches the root module's effective Go floor.
const DefaultGoVersion = "1.21.0"

var supportedTargets = []targetSpec{
	{goos: "linux", goarch: "amd64", libraryName: "liblancedb.so"},
	{goos: "linux", goarch: "arm64", libraryName: "liblancedb.so"},
	{goos: "darwin", goarch: "arm64", libraryName: "liblancedb.dylib"},
	{goos: "windows", goarch: "amd64", libraryName: "lancedb.dll"},
	{goos: "windows", goarch: "arm64", libraryName: "lancedb.dll"},
}

type targetSpec struct {
	goos        string
	goarch      string
	libraryName string
}

// Options configures one generated companion module.
type Options struct {
	LibraryPath    string
	LicensePath    string
	Target         string
	ModulePath     string
	OutDir         string
	GoVersion      string
	UpstreamRepo   string
	UpstreamCommit string
	HeaderSHA256   string
}

// Metadata records the checksums and provenance written into the module.
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

// Generate verifies the inputs, compresses the library, and publishes a new
// output directory. It never overwrites an existing output directory.
func Generate(opts Options) (Metadata, error) {
	var zero Metadata

	libraryPath, err := existingRegularFile(opts.LibraryPath, "library")
	if err != nil {
		return zero, err
	}
	licensePath, err := existingRegularFile(opts.LicensePath, "license")
	if err != nil {
		return zero, err
	}
	if opts.OutDir == "" {
		return zero, errors.New("output directory is required")
	}
	opts.OutDir = filepath.Clean(opts.OutDir)

	spec, err := parseTarget(opts.Target)
	if err != nil {
		return zero, err
	}
	if filepath.Base(libraryPath) != spec.libraryName {
		return zero, fmt.Errorf("library basename is %q, want %q for %s", filepath.Base(libraryPath), spec.libraryName, opts.Target)
	}
	if err := validateModulePath(opts.ModulePath); err != nil {
		return zero, err
	}
	if opts.GoVersion == "" {
		opts.GoVersion = DefaultGoVersion
	}
	if !validGoVersion(opts.GoVersion) {
		return zero, fmt.Errorf("Go version %q is not a valid go directive", opts.GoVersion)
	}
	if strings.TrimSpace(opts.UpstreamRepo) == "" {
		return zero, errors.New("upstream repository is required")
	}
	if !isLowerHex(opts.UpstreamCommit, 40) {
		return zero, fmt.Errorf("upstream commit %q is not a 40-character lowercase hex commit", opts.UpstreamCommit)
	}
	if !isLowerHex(opts.HeaderSHA256, 64) {
		return zero, fmt.Errorf("header SHA-256 %q is not 64 lowercase hex characters", opts.HeaderSHA256)
	}
	if err := rejectExistingOutput(opts.OutDir); err != nil {
		return zero, err
	}
	if err := inspectLibrary(libraryPath, spec); err != nil {
		return zero, err
	}

	parent := filepath.Dir(opts.OutDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return zero, fmt.Errorf("create output parent %s: %w", parent, err)
	}
	staging, err := os.MkdirTemp(parent, ".gencandidate-*")
	if err != nil {
		return zero, fmt.Errorf("create staging directory in %s: %w", parent, err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(staging)
		}
	}()

	payloadName := spec.libraryName + ".zst"
	payloadPath := filepath.Join(staging, payloadName)
	compressedSize, uncompressedSize, uncompressedSHA, compressedSHA, err := compressLibrary(libraryPath, payloadPath)
	if err != nil {
		return zero, err
	}
	meta := Metadata{
		Target:             opts.Target,
		LibraryName:        spec.libraryName,
		PayloadName:        payloadName,
		CompressedSize:     compressedSize,
		CompressedSHA256:   compressedSHA,
		UncompressedSize:   uncompressedSize,
		UncompressedSHA256: uncompressedSHA,
		UpstreamRepo:       opts.UpstreamRepo,
		UpstreamCommit:     opts.UpstreamCommit,
		HeaderSHA256:       opts.HeaderSHA256,
	}

	if err := writeTemplate(filepath.Join(staging, "artifact.go"), artifactTemplate, meta); err != nil {
		return zero, err
	}
	goMod := []byte("module " + opts.ModulePath + "\n\ngo " + opts.GoVersion + "\n")
	if err := os.WriteFile(filepath.Join(staging, "go.mod"), goMod, 0o644); err != nil {
		return zero, fmt.Errorf("write generated go.mod: %w", err)
	}
	if err := copyFile(licensePath, filepath.Join(staging, "LICENSE")); err != nil {
		return zero, err
	}

	if err := rejectExistingOutput(opts.OutDir); err != nil {
		return zero, err
	}
	if err := os.Chmod(staging, 0o755); err != nil {
		return zero, fmt.Errorf("set output directory permissions: %w", err)
	}
	if err := os.Rename(staging, opts.OutDir); err != nil {
		return zero, fmt.Errorf("publish output directory %s: %w", opts.OutDir, err)
	}
	published = true
	return meta, nil
}

// GoMod returns the exact go.mod content for a module path and Go version.
func GoMod(modulePath, goVersion string) []byte {
	return []byte("module " + modulePath + "\n\ngo " + goVersion + "\n")
}

func existingRegularFile(path, kind string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s path is required", kind)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s %s: %w", kind, path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s %s is not a regular file (mode %s)", kind, path, info.Mode())
	}
	if info.Size() == 0 {
		return "", fmt.Errorf("%s %s is empty", kind, path)
	}
	return filepath.Clean(path), nil
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy %s: %w", source, err)
	}
	return out.Close()
}

func parseTarget(value string) (targetSpec, error) {
	var names []string
	for _, spec := range supportedTargets {
		target := spec.goos + "/" + spec.goarch
		if value == target {
			return spec, nil
		}
		names = append(names, target)
	}
	return targetSpec{}, fmt.Errorf("unsupported target %q; supported targets are %s", value, strings.Join(names, ", "))
}

func rejectExistingOutput(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf("output %s already exists (mode %s); choose a new output directory", path, info.Mode())
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output %s: %w", path, err)
	}
	return nil
}

func inspectLibrary(path string, spec targetSpec) error {
	switch spec.goos {
	case "linux":
		return inspectELF(path, spec)
	case "darwin":
		return inspectMachO(path, spec)
	case "windows":
		return inspectPE(path, spec)
	default:
		return fmt.Errorf("unsupported target %q", spec.goos+"/"+spec.goarch)
	}
}

func inspectELF(path string, spec targetSpec) error {
	file, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("parse ELF library %s: %w", path, err)
	}
	defer file.Close()
	if file.Class != elf.ELFCLASS64 {
		return fmt.Errorf("ELF library %s has class %s, want ELFCLASS64", path, file.Class)
	}
	if file.Type != elf.ET_DYN {
		return fmt.Errorf("ELF library %s has type %s, want ET_DYN shared object", path, file.Type)
	}
	want := elf.EM_X86_64
	if spec.goarch == "arm64" {
		want = elf.EM_AARCH64
	}
	if file.Machine != want {
		return fmt.Errorf("ELF library %s has machine %s, want %s", path, file.Machine, want)
	}
	return nil
}

func inspectMachO(path string, spec targetSpec) error {
	file, err := macho.Open(path)
	if err != nil {
		if fat, fatErr := macho.OpenFat(path); fatErr == nil {
			fat.Close()
			return fmt.Errorf("Mach-O library %s is a universal binary; provide a thin %s %s dylib", path, spec.goos, spec.goarch)
		}
		return fmt.Errorf("parse Mach-O library %s: %w", path, err)
	}
	defer file.Close()
	if file.Type != macho.TypeDylib {
		return fmt.Errorf("Mach-O library %s has type %s, want dylib", path, file.Type)
	}
	if file.Cpu != macho.CpuArm64 {
		return fmt.Errorf("Mach-O library %s has CPU %s, want arm64", path, file.Cpu)
	}
	return nil
}

func inspectPE(path string, spec targetSpec) error {
	file, err := pe.Open(path)
	if err != nil {
		return fmt.Errorf("parse PE library %s: %w", path, err)
	}
	defer file.Close()
	if file.Characteristics&pe.IMAGE_FILE_DLL == 0 {
		return fmt.Errorf("PE library %s is not a DLL (characteristics %#x)", path, file.Characteristics)
	}
	if _, ok := file.OptionalHeader.(*pe.OptionalHeader64); !ok {
		return fmt.Errorf("PE library %s is not a 64-bit PE image", path)
	}
	want := uint16(pe.IMAGE_FILE_MACHINE_AMD64)
	if spec.goarch == "arm64" {
		want = pe.IMAGE_FILE_MACHINE_ARM64
	}
	if file.Machine != want {
		return fmt.Errorf("PE library %s has machine %#x, want %#x", path, file.Machine, want)
	}
	return nil
}

func compressLibrary(source, destination string) (compressedSize, uncompressedSize int64, uncompressedSHA, compressedSHA string, err error) {
	in, err := os.Open(source)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("open library %s: %w", source, err)
	}
	defer in.Close()

	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("create payload %s: %w", destination, err)
	}

	compressedHash := sha256.New()
	writer, err := zstd.NewWriter(
		io.MultiWriter(out, compressedHash),
		// One encoder thread and the best level make the payload a pure
		// function of the input so a later regeneration can be byte-compared.
		zstd.WithEncoderLevel(zstd.SpeedBestCompression),
		zstd.WithEncoderConcurrency(1),
	)
	if err != nil {
		_ = out.Close()
		return 0, 0, "", "", fmt.Errorf("create zstd encoder: %w", err)
	}

	uncompressedHash := sha256.New()
	written, copyErr := io.Copy(writer, io.TeeReader(in, uncompressedHash))
	closeErr := writer.Close()
	fileCloseErr := out.Close()
	if copyErr != nil {
		return 0, 0, "", "", fmt.Errorf("compress library %s: %w", source, copyErr)
	}
	if closeErr != nil {
		return 0, 0, "", "", fmt.Errorf("finish zstd payload %s: %w", destination, closeErr)
	}
	if fileCloseErr != nil {
		return 0, 0, "", "", fmt.Errorf("close payload %s: %w", destination, fileCloseErr)
	}

	info, err := os.Stat(destination)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("stat payload %s: %w", destination, err)
	}
	return info.Size(), written, hex.EncodeToString(uncompressedHash.Sum(nil)), hex.EncodeToString(compressedHash.Sum(nil)), nil
}

func writeTemplate(path string, tmpl *template.Template, data any) error {
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return fmt.Errorf("render %s: %w", filepath.Base(path), err)
	}
	source, err := format.Source(buffer.Bytes())
	if err != nil {
		return fmt.Errorf("format %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, source, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func validateModulePath(path string) error {
	if path == "" {
		return errors.New("module path is required")
	}
	if strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") || strings.Contains(path, "//") || strings.Contains(path, "..") {
		return fmt.Errorf("module path %q is not a clean import path", path)
	}
	for _, r := range path {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '.', '-', '_', '~', '/':
			continue
		default:
			return fmt.Errorf("module path %q contains unsupported character %q", path, r)
		}
	}
	return nil
}

func validGoVersion(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r >= '0' && r <= '9' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func isLowerHex(value string, size int) bool {
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

var artifactTemplate = template.Must(template.New("artifact").Parse(`// Code generated by scripts/go-module-origin/generator; DO NOT EDIT.

// Package artifact embeds one compressed lancedb-c shared library as data.
// Importing this module does not link, load, or execute native code.
package artifact

import _ "embed"

// Payload is the zstd-compressed {{.LibraryName}} for {{.Target}}.
//
//go:embed {{printf "%q" .PayloadName}}
var Payload []byte

// Packaging metadata. UpstreamRepo, UpstreamCommit and HeaderSHA256 are
// provenance claims recorded by the packaging process; the loader verifies the
// target and payload, not the library's build commit or the header's
// provenance.
const (
	Target              = {{printf "%q" .Target}}
	LibraryName         = {{printf "%q" .LibraryName}}
	CompressedSize    int64 = {{.CompressedSize}}
	CompressedSHA256    = {{printf "%q" .CompressedSHA256}}
	UncompressedSize  int64 = {{.UncompressedSize}}
	UncompressedSHA256  = {{printf "%q" .UncompressedSHA256}}
	UpstreamRepo        = {{printf "%q" .UpstreamRepo}}
	UpstreamCommit      = {{printf "%q" .UpstreamCommit}}
	HeaderSHA256        = {{printf "%q" .HeaderSHA256}}
)
`))
