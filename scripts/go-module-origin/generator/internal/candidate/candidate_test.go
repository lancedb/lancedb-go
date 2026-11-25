package candidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/lancedb/lancedb-go/scripts/go-module-origin/generator/internal/artifactmeta"
	"github.com/lancedb/lancedb-go/scripts/go-module-origin/generator/internal/fixture"
)

const testCommit = "3106de158b8da5dcb47e2a4acc820ed1b74b3e01"

func writeLibrary(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeLicense(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "LICENSE")
	if err := os.WriteFile(path, []byte("Apache License 2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func testOptions(t *testing.T, target, libraryName string, library []byte) Options {
	t.Helper()
	work := t.TempDir()
	return Options{
		LibraryPath:    writeLibrary(t, work, libraryName, library),
		LicensePath:    writeLicense(t, work),
		Target:         target,
		ModulePath:     "go.lancedb.io/lancedb-go/artifacts/" + strings.ReplaceAll(target, "/", "-"),
		OutDir:         filepath.Join(work, "out"),
		UpstreamRepo:   "lancedb/lancedb-c",
		UpstreamCommit: testCommit,
		HeaderSHA256:   strings.Repeat("a", 64),
	}
}

func TestGenerateProducesVerifiedModule(t *testing.T) {
	opts := testOptions(t, "linux/amd64", "liblancedb.so", fixture.ELF("amd64"))
	meta, err := Generate(opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	entries, err := os.ReadDir(opts.OutDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	want := []string{"LICENSE", "artifact.go", "go.mod", "liblancedb.so.zst"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("module files are %v, want %v", names, want)
	}

	goMod, err := os.ReadFile(filepath.Join(opts.OutDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if string(goMod) != string(GoMod(opts.ModulePath, DefaultGoVersion)) {
		t.Fatalf("go.mod is %q", goMod)
	}

	parsed, err := artifactmeta.ParseFile(filepath.Join(opts.OutDir, "artifact.go"))
	if err != nil {
		t.Fatalf("published artifact.go does not parse: %v", err)
	}
	if parsed != (artifactmeta.Metadata{
		Target:             "linux/amd64",
		LibraryName:        "liblancedb.so",
		PayloadName:        "liblancedb.so.zst",
		CompressedSize:     meta.CompressedSize,
		CompressedSHA256:   meta.CompressedSHA256,
		UncompressedSize:   meta.UncompressedSize,
		UncompressedSHA256: meta.UncompressedSHA256,
		UpstreamRepo:       "lancedb/lancedb-c",
		UpstreamCommit:     testCommit,
		HeaderSHA256:       strings.Repeat("a", 64),
	}) {
		t.Fatalf("published metadata %+v does not match generation result %+v", parsed, meta)
	}

	payload, err := os.ReadFile(filepath.Join(opts.OutDir, parsed.PayloadName))
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close()
	decoded, err := decoder.DecodeAll(payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != string(fixture.ELF("amd64")) {
		t.Fatal("decoded payload does not match the library")
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	opts := testOptions(t, "windows/arm64", "lancedb.dll", fixture.PE("arm64"))
	first, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	firstDir := opts.OutDir
	opts.OutDir += "-second"
	second, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("metadata differs between runs: %+v vs %+v", first, second)
	}
	for _, name := range []string{"artifact.go", "go.mod", "LICENSE", first.PayloadName} {
		left, err := os.ReadFile(filepath.Join(firstDir, name))
		if err != nil {
			t.Fatal(err)
		}
		right, err := os.ReadFile(filepath.Join(opts.OutDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(left) != string(right) {
			t.Fatalf("%s differs between runs", name)
		}
	}
}

func TestGenerateRejectsWrongLibrary(t *testing.T) {
	cases := []struct {
		name        string
		target      string
		libraryName string
		library     []byte
	}{
		{"elf arch mismatch", "linux/arm64", "liblancedb.so", fixture.ELF("amd64")},
		{"pe arch mismatch", "windows/amd64", "lancedb.dll", fixture.PE("arm64")},
		{"not a dylib", "darwin/arm64", "liblancedb.dylib", fixture.ELF("arm64")},
		{"wrong basename", "linux/amd64", "liblancedb.dylib", fixture.ELF("amd64")},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			opts := testOptions(t, testCase.target, testCase.libraryName, testCase.library)
			if _, err := Generate(opts); err == nil {
				t.Fatal("Generate accepted a mismatched library")
			}
		})
	}
}

func TestGenerateRefusesExistingOutput(t *testing.T) {
	opts := testOptions(t, "linux/amd64", "liblancedb.so", fixture.ELF("amd64"))
	if _, err := Generate(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(opts); err == nil {
		t.Fatal("Generate overwrote an existing output directory")
	}
}
