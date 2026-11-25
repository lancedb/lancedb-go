// Package origin builds the static Go module origin tree for the per-target
// lancedb-go companion modules.
//
// The output is staging data: standard Go module proxy files under
// modules/<module path>/@v/, go-import discovery HTML, and a manifest that the
// publisher validates before any network write. Payloads never belong in Git.
package origin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
	modzip "golang.org/x/mod/zip"

	"github.com/lancedb/lancedb-go/scripts/go-module-origin/generator/internal/artifactmeta"
)

// DefaultModulePrefix is the import prefix of the companion modules.
const DefaultModulePrefix = "go.lancedb.io/lancedb-go/artifacts"

// DefaultBaseURL is the public origin that serves the module files.
const DefaultBaseURL = "https://go.lancedb.io"

type targetSpec struct {
	name        string
	goos        string
	goarch      string
	libraryName string
}

var targets = []targetSpec{
	{name: "linux-amd64", goos: "linux", goarch: "amd64", libraryName: "liblancedb.so"},
	{name: "linux-arm64", goos: "linux", goarch: "arm64", libraryName: "liblancedb.so"},
	{name: "darwin-arm64", goos: "darwin", goarch: "arm64", libraryName: "liblancedb.dylib"},
	{name: "windows-amd64", goos: "windows", goarch: "amd64", libraryName: "lancedb.dll"},
	{name: "windows-arm64", goos: "windows", goarch: "arm64", libraryName: "lancedb.dll"},
}

// Options configures one origin build.
type Options struct {
	CandidatesDir string
	OutDir        string
	Version       string
	Time          string
	ModulePrefix  string
	BaseURL       string
	GoVersion     string
	SourceRepo    string
	SourceCommit  string
	SourceRelease string
}

// Object describes one published origin object.
type Object struct {
	Key          string `json:"key"`
	Kind         string `json:"kind"`
	Module       string `json:"module,omitempty"`
	Version      string `json:"version,omitempty"`
	Target       string `json:"target,omitempty"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
	ContentType  string `json:"content_type"`
	CacheControl string `json:"cache_control"`
	Immutable    bool   `json:"immutable"`
	ZIPH1        string `json:"zip_h1,omitempty"`
	GoModH1      string `json:"gomod_h1,omitempty"`
}

// Manifest is the publication manifest written to the origin root.
type Manifest struct {
	Version        string   `json:"version"`
	ModulePrefix   string   `json:"module_prefix"`
	BaseURL        string   `json:"base_url"`
	GoVersion      string   `json:"go_version"`
	SourceRepo     string   `json:"source_repo"`
	SourceCommit   string   `json:"source_commit"`
	SourceRelease  string   `json:"source_release"`
	Time           string   `json:"time"`
	Objects        []Object `json:"objects"`
	PublicKeyOrder []string `json:"public_key_order"`
}

// Build writes the origin tree and returns its manifest.
func Build(opts Options) (Manifest, error) {
	var manifest Manifest
	if opts.CandidatesDir == "" || opts.OutDir == "" {
		return manifest, fmt.Errorf("candidates directory and output directory are required")
	}
	if !strings.HasPrefix(opts.Version, "v") {
		return manifest, fmt.Errorf("version %q must start with v", opts.Version)
	}
	if _, err := time.Parse(time.RFC3339, opts.Time); err != nil {
		return manifest, fmt.Errorf("invalid time %q: %w", opts.Time, err)
	}
	if opts.ModulePrefix == "" {
		opts.ModulePrefix = DefaultModulePrefix
	}
	opts.ModulePrefix = strings.TrimSuffix(opts.ModulePrefix, "/")
	if opts.BaseURL == "" {
		opts.BaseURL = DefaultBaseURL
	}
	opts.BaseURL = strings.TrimSuffix(opts.BaseURL, "/")
	if opts.GoVersion == "" {
		opts.GoVersion = "1.21.0"
	}

	if err := os.RemoveAll(opts.OutDir); err != nil {
		return manifest, fmt.Errorf("clean output: %w", err)
	}
	manifest = Manifest{
		Version:       opts.Version,
		ModulePrefix:  opts.ModulePrefix,
		BaseURL:       opts.BaseURL,
		GoVersion:     opts.GoVersion,
		SourceRepo:    opts.SourceRepo,
		SourceCommit:  opts.SourceCommit,
		SourceRelease: opts.SourceRelease,
		Time:          opts.Time,
	}
	for _, target := range targets {
		modulePath := opts.ModulePrefix + "/" + target.name
		candidateDir := filepath.Join(opts.CandidatesDir, target.name, "module")
		if err := buildModule(opts, target, modulePath, candidateDir, &manifest); err != nil {
			return manifest, fmt.Errorf("%s: %w", target.name, err)
		}
	}
	sort.Slice(manifest.Objects, func(i, j int) bool { return manifest.Objects[i].Key < manifest.Objects[j].Key })
	manifest.PublicKeyOrder = make([]string, len(manifest.Objects))
	for index, object := range manifest.Objects {
		manifest.PublicKeyOrder[index] = object.Key
	}
	if err := writeManifest(opts.OutDir, manifest); err != nil {
		return manifest, fmt.Errorf("write manifest: %w", err)
	}
	return manifest, nil
}

func buildModule(opts Options, target targetSpec, modulePath, candidateDir string, manifest *Manifest) error {
	files, err := os.ReadDir(candidateDir)
	if err != nil {
		return fmt.Errorf("read candidate module: %w", err)
	}
	var payloadName string
	candidateFiles := map[string]bool{}
	for _, file := range files {
		if file.IsDir() {
			return fmt.Errorf("candidate module contains unexpected directory %q", file.Name())
		}
		name := file.Name()
		candidateFiles[name] = true
		if strings.HasSuffix(name, ".zst") {
			if payloadName != "" {
				return fmt.Errorf("candidate module contains more than one payload")
			}
			payloadName = name
		}
	}
	if payloadName == "" {
		return fmt.Errorf("candidate module has no .zst payload")
	}
	for name := range candidateFiles {
		switch name {
		case "artifact.go", "go.mod", "LICENSE", payloadName:
		default:
			return fmt.Errorf("candidate module contains unexpected file %q", name)
		}
	}

	meta, err := artifactmeta.ParseFile(filepath.Join(candidateDir, "artifact.go"))
	if err != nil {
		return err
	}
	if meta.Target != target.goos+"/"+target.goarch {
		return fmt.Errorf("artifact target is %q, want %q", meta.Target, target.goos+"/"+target.goarch)
	}
	if meta.LibraryName != target.libraryName {
		return fmt.Errorf("artifact library name is %q, want %q", meta.LibraryName, target.libraryName)
	}
	if meta.PayloadName != payloadName {
		return fmt.Errorf("artifact embeds %q, candidate payload is %q", meta.PayloadName, payloadName)
	}

	payloadPath := filepath.Join(candidateDir, payloadName)
	payload, err := os.ReadFile(payloadPath)
	if err != nil {
		return err
	}
	if err := verifyPayload(meta, payload); err != nil {
		return err
	}

	goModPath := filepath.Join(candidateDir, "go.mod")
	goMod, err := os.ReadFile(goModPath)
	if err != nil {
		return err
	}
	wantGoMod := "module " + modulePath + "\n\ngo " + opts.GoVersion + "\n"
	if string(goMod) != wantGoMod {
		return fmt.Errorf("candidate go.mod does not match the generated module declaration:\n%s\nwant:\n%s", goMod, wantGoMod)
	}
	license, err := os.ReadFile(filepath.Join(candidateDir, "LICENSE"))
	if err != nil {
		return err
	}

	moduleDir := filepath.Join(opts.OutDir, "modules", modulePath, "@v")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(moduleDir, opts.Version+".mod"), []byte(wantGoMod), 0o644); err != nil {
		return err
	}
	info, err := json.Marshal(struct {
		Version string `json:"Version"`
		Time    string `json:"Time"`
	}{Version: opts.Version, Time: opts.Time})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(moduleDir, opts.Version+".info"), info, 0o644); err != nil {
		return err
	}

	artifactSource, err := os.ReadFile(filepath.Join(candidateDir, "artifact.go"))
	if err != nil {
		return err
	}
	stage, err := os.MkdirTemp("", "go-module-origin-stage-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	staged := []struct {
		name string
		data []byte
	}{
		{"go.mod", []byte(wantGoMod)},
		{"artifact.go", artifactSource},
		{"LICENSE", license},
		{payloadName, payload},
	}
	for _, file := range staged {
		if err := os.WriteFile(filepath.Join(stage, file.name), file.data, 0o644); err != nil {
			return err
		}
	}
	zipPath := filepath.Join(moduleDir, opts.Version+".zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	if err := modzip.CreateFromDir(zipFile, module.Version{Path: modulePath, Version: opts.Version}, stage); err != nil {
		_ = zipFile.Close()
		return err
	}
	if err := zipFile.Close(); err != nil {
		return err
	}
	zipH1, err := dirhash.HashZip(zipPath, dirhash.Hash1)
	if err != nil {
		return err
	}
	goModH1, err := dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) {
		return os.Open(filepath.Join(stage, "go.mod"))
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "list"), []byte(opts.Version+"\n"), 0o644); err != nil {
		return err
	}

	discovery := []byte(fmt.Sprintf("<!doctype html>\n<html><head><meta name=\"go-import\" content=\"%s mod %s/modules\"></head><body>%s</body></html>\n", modulePath, opts.BaseURL, modulePath))
	discoveryPath, err := safeJoin(opts.OutDir, "lancedb-go", "artifacts", target.name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(discoveryPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(discoveryPath, discovery, 0o644); err != nil {
		return err
	}

	add := func(key, kind, path, contentType, cacheControl string, immutable bool, zipValue, goModValue string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		manifest.Objects = append(manifest.Objects, Object{
			Key:          key,
			Kind:         kind,
			Module:       modulePath,
			Version:      opts.Version,
			Target:       target.name,
			SHA256:       hex.EncodeToString(digest[:]),
			Size:         int64(len(data)),
			ContentType:  contentType,
			CacheControl: cacheControl,
			Immutable:    immutable,
			ZIPH1:        zipValue,
			GoModH1:      goModValue,
		})
		return nil
	}
	if err := add(filepath.ToSlash(filepath.Join("lancedb-go", "artifacts", target.name)), "discovery", discoveryPath, "text/html; charset=utf-8", "public, max-age=300", true, "", ""); err != nil {
		return err
	}
	for _, suffix := range []string{"list", opts.Version + ".info", opts.Version + ".mod", opts.Version + ".zip"} {
		contentType, cacheControl, immutable := contentTypeFor(suffix)
		path := filepath.Join(moduleDir, suffix)
		key := filepath.ToSlash(filepath.Join("modules", modulePath, "@v", suffix))
		zipValue, goModValue := "", ""
		if suffix == opts.Version+".zip" {
			zipValue = zipH1
		}
		if suffix == opts.Version+".mod" {
			goModValue = goModH1
		}
		if err := add(key, strings.TrimSuffix(suffix, filepath.Ext(suffix)), path, contentType, cacheControl, immutable, zipValue, goModValue); err != nil {
			return err
		}
	}
	return nil
}

func verifyPayload(meta artifactmeta.Metadata, payload []byte) error {
	if int64(len(payload)) != meta.CompressedSize {
		return fmt.Errorf("payload size %d, metadata says %d", len(payload), meta.CompressedSize)
	}
	compressed := sha256.Sum256(payload)
	if hex.EncodeToString(compressed[:]) != meta.CompressedSHA256 {
		return fmt.Errorf("payload SHA-256 %x, metadata says %s", compressed, meta.CompressedSHA256)
	}
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return err
	}
	defer decoder.Close()
	stream, err := decoder.DecodeAll(payload, nil)
	if err != nil {
		return fmt.Errorf("decompress payload: %w", err)
	}
	if int64(len(stream)) != meta.UncompressedSize {
		return fmt.Errorf("uncompressed size %d, metadata says %d", len(stream), meta.UncompressedSize)
	}
	uncompressed := sha256.Sum256(stream)
	if hex.EncodeToString(uncompressed[:]) != meta.UncompressedSHA256 {
		return fmt.Errorf("uncompressed SHA-256 %x, metadata says %s", uncompressed, meta.UncompressedSHA256)
	}
	return nil
}

func contentTypeFor(suffix string) (contentType, cacheControl string, immutable bool) {
	switch {
	case suffix == "list":
		return "text/plain; charset=utf-8", "public, max-age=60", false
	case strings.HasSuffix(suffix, ".info"):
		return "application/json", "public, max-age=31536000, immutable", true
	case strings.HasSuffix(suffix, ".mod"):
		return "text/plain; charset=utf-8", "public, max-age=31536000, immutable", true
	case strings.HasSuffix(suffix, ".zip"):
		return "application/zip", "public, max-age=31536000, immutable", true
	default:
		return "application/octet-stream", "public, max-age=31536000, immutable", true
	}
}

func safeJoin(root string, elements ...string) (string, error) {
	for _, element := range elements {
		if strings.Contains(element, "..") || strings.ContainsAny(element, "/\\") {
			return "", fmt.Errorf("unsafe path element %q", element)
		}
	}
	return filepath.Join(append([]string{root}, elements...)...), nil
}

func writeManifest(outDir string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), data, 0o644); err != nil {
		return err
	}
	var tsv strings.Builder
	tsv.WriteString("key\tsha256\tsize\tkind\timmutable\tcontent_type\tmodule\tversion\ttarget\tzip_h1\tgomod_h1\n")
	for _, object := range manifest.Objects {
		fmt.Fprintf(&tsv, "%s\t%s\t%d\t%s\t%t\t%s\t%s\t%s\t%s\t%s\t%s\n",
			object.Key, object.SHA256, object.Size, object.Kind, object.Immutable,
			object.ContentType, object.Module, object.Version, object.Target,
			object.ZIPH1, object.GoModH1)
	}
	return os.WriteFile(filepath.Join(outDir, "manifest.tsv"), []byte(tsv.String()), 0o644)
}
