package origin

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
	modzip "golang.org/x/mod/zip"

	"github.com/lancedb/lancedb-go/scripts/go-module-origin/generator/internal/candidate"
	"github.com/lancedb/lancedb-go/scripts/go-module-origin/generator/internal/fixture"
)

const (
	testVersion = "v0.9.9-fixture"
	testTime    = "2026-11-25T00:00:00Z"
	testCommit  = "3106de158b8da5dcb47e2a4acc820ed1b74b3e01"
)

var testLibraries = map[string]struct {
	libraryName string
	library     []byte
}{
	"linux-amd64":   {"liblancedb.so", fixture.ELF("amd64")},
	"linux-arm64":   {"liblancedb.so", fixture.ELF("arm64")},
	"darwin-arm64":  {"liblancedb.dylib", fixture.MachO()},
	"windows-amd64": {"lancedb.dll", fixture.PE("amd64")},
	"windows-arm64": {"lancedb.dll", fixture.PE("arm64")},
}

func buildCandidates(t *testing.T, root string) {
	t.Helper()
	license := filepath.Join(root, "LICENSE")
	if err := os.WriteFile(license, []byte("Apache License 2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for target, spec := range testLibraries {
		library := filepath.Join(root, target, spec.libraryName)
		if err := os.MkdirAll(filepath.Dir(library), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(library, spec.library, 0o644); err != nil {
			t.Fatal(err)
		}
		goos, goarch, _ := strings.Cut(target, "-")
		_, err := candidate.Generate(candidate.Options{
			LibraryPath:    library,
			LicensePath:    license,
			Target:         goos + "/" + goarch,
			ModulePath:     DefaultModulePrefix + "/" + target,
			OutDir:         filepath.Join(root, "candidates", target, "module"),
			UpstreamRepo:   "lancedb/lancedb-c",
			UpstreamCommit: testCommit,
			HeaderSHA256:   strings.Repeat("a", 64),
		})
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
	}
}

func testOptions(t *testing.T) Options {
	t.Helper()
	work := t.TempDir()
	buildCandidates(t, work)
	return Options{
		CandidatesDir: filepath.Join(work, "candidates"),
		OutDir:        filepath.Join(work, "origin"),
		Version:       testVersion,
		Time:          testTime,
		SourceRepo:    "lancedb/lancedb-c",
		SourceCommit:  testCommit,
		SourceRelease: "v0.1.0-preview.5",
	}
}

func TestBuildWritesProtocolTree(t *testing.T) {
	opts := testOptions(t)
	manifest, err := Build(opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if manifest.Version != testVersion || manifest.ModulePrefix != DefaultModulePrefix || manifest.BaseURL != DefaultBaseURL {
		t.Fatalf("unexpected manifest header: %+v", manifest)
	}
	if manifest.GoVersion != "1.21.0" {
		t.Fatalf("manifest Go version is %q", manifest.GoVersion)
	}
	if len(manifest.Objects) != 25 {
		t.Fatalf("manifest has %d objects, want 25", len(manifest.Objects))
	}

	objects := map[string]Object{}
	for _, object := range manifest.Objects {
		objects[object.Key] = object
	}
	for target := range testLibraries {
		modulePath := DefaultModulePrefix + "/" + target
		discoveryKey := "lancedb-go/artifacts/" + target
		if _, ok := objects[discoveryKey]; !ok {
			t.Fatalf("missing discovery object %s", discoveryKey)
		}
		for _, suffix := range []string{"list", testVersion + ".info", testVersion + ".mod", testVersion + ".zip"} {
			key := "modules/" + modulePath + "/@v/" + suffix
			if _, ok := objects[key]; !ok {
				t.Fatalf("missing module object %s", key)
			}
		}
		zipObject := objects["modules/"+modulePath+"/@v/"+testVersion+".zip"]
		zipPath := filepath.Join(opts.OutDir, filepath.FromSlash(zipObject.Key))
		h1, err := dirhash.HashZip(zipPath, dirhash.Hash1)
		if err != nil {
			t.Fatal(err)
		}
		if h1 != zipObject.ZIPH1 {
			t.Fatalf("%s zip_h1 %s, want %s", target, zipObject.ZIPH1, h1)
		}
		modObject := objects["modules/"+modulePath+"/@v/"+testVersion+".mod"]
		if !strings.Contains(modObject.GoModH1, "h1:") {
			t.Fatalf("%s has no go.mod h1", target)
		}
	}
}

func TestBuildZipContents(t *testing.T) {
	opts := testOptions(t)
	if _, err := Build(opts); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(opts.OutDir, "modules", DefaultModulePrefix+"/linux-amd64", "@v", testVersion+".zip")
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	prefix := DefaultModulePrefix + "/linux-amd64@" + testVersion + "/"
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, want := range []string{prefix + "go.mod", prefix + "artifact.go", prefix + "LICENSE", prefix + "liblancedb.so.zst"} {
		if !names[want] {
			t.Fatalf("zip is missing %s; entries: %v", want, names)
		}
	}

	// The module zip must satisfy the shared modzip verifier.
	if _, err := modzip.CheckZip(module.Version{Path: DefaultModulePrefix + "/linux-amd64", Version: testVersion}, zipPath); err != nil {
		t.Fatalf("generated zip is not a valid module zip: %v", err)
	}
}

func TestBuildDiscoveryAndInfo(t *testing.T) {
	opts := testOptions(t)
	if _, err := Build(opts); err != nil {
		t.Fatal(err)
	}
	discovery, err := os.ReadFile(filepath.Join(opts.OutDir, "lancedb-go", "artifacts", "linux-amd64"))
	if err != nil {
		t.Fatal(err)
	}
	wantMeta := `content="go.lancedb.io/lancedb-go/artifacts/linux-amd64 mod https://go.lancedb.io/modules"`
	if !strings.Contains(string(discovery), wantMeta) {
		t.Fatalf("discovery HTML is %q, want %q", discovery, wantMeta)
	}
	info, err := os.ReadFile(filepath.Join(opts.OutDir, "modules", DefaultModulePrefix+"/linux-amd64", "@v", testVersion+".info"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Version string `json:"Version"`
		Time    string `json:"Time"`
	}
	if err := json.Unmarshal(info, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Version != testVersion || parsed.Time != testTime {
		t.Fatalf("info is %s", info)
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	opts := testOptions(t)
	if _, err := Build(opts); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(opts.OutDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	zipLeft, err := os.ReadFile(filepath.Join(opts.OutDir, "modules", DefaultModulePrefix+"/linux-amd64", "@v", testVersion+".zip"))
	if err != nil {
		t.Fatal(err)
	}
	opts.OutDir += "-second"
	if _, err := Build(opts); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(opts.OutDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("manifest differs between builds")
	}
	zipRight, err := os.ReadFile(filepath.Join(opts.OutDir, "modules", DefaultModulePrefix+"/linux-amd64", "@v", testVersion+".zip"))
	if err != nil {
		t.Fatal(err)
	}
	if string(zipLeft) != string(zipRight) {
		t.Fatal("module zip differs between builds")
	}
}

func TestBuildRejectsTamperedPayload(t *testing.T) {
	opts := testOptions(t)
	payload := filepath.Join(opts.CandidatesDir, "linux-amd64", "module", "liblancedb.so.zst")
	file, err := os.OpenFile(payload, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("tampered")); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(opts); err == nil {
		t.Fatal("Build accepted a tampered payload")
	}
}

func TestBuildRejectsForeignCandidateFiles(t *testing.T) {
	opts := testOptions(t)
	foreign := filepath.Join(opts.CandidatesDir, "linux-amd64", "module", "extra.txt")
	if err := os.WriteFile(foreign, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(opts); err == nil {
		t.Fatal("Build accepted an unexpected candidate file")
	}
}

func TestBuildRejectsWrongGoMod(t *testing.T) {
	opts := testOptions(t)
	goMod := filepath.Join(opts.CandidatesDir, "linux-amd64", "module", "go.mod")
	if err := os.WriteFile(goMod, []byte("module example.com/wrong\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Build(opts)
	if err == nil {
		t.Fatal("Build accepted a mismatched go.mod")
	}
	if !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestContentTypeFor(t *testing.T) {
	for _, testCase := range []struct {
		suffix    string
		immutable bool
	}{
		{"list", false},
		{"v1.0.0.info", true},
		{"v1.0.0.mod", true},
		{"v1.0.0.zip", true},
	} {
		_, _, immutable := contentTypeFor(testCase.suffix)
		if immutable != testCase.immutable {
			t.Fatalf("%s immutable=%t, want %t", testCase.suffix, immutable, testCase.immutable)
		}
	}
}
