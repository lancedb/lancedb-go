// Command genorigin builds the static Go module origin tree from candidate
// companion modules.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/lancedb/lancedb-go/scripts/go-module-origin/generator/internal/origin"
)

func main() {
	var opts origin.Options
	flag.StringVar(&opts.CandidatesDir, "candidates", "", "directory containing <target>/module candidates (required)")
	flag.StringVar(&opts.OutDir, "out", "", "output origin tree (required)")
	flag.StringVar(&opts.Version, "version", "", "companion module version (required)")
	flag.StringVar(&opts.Time, "time", "", "RFC3339 .info time (required)")
	flag.StringVar(&opts.ModulePrefix, "module-prefix", origin.DefaultModulePrefix, "module import prefix")
	flag.StringVar(&opts.BaseURL, "base-url", origin.DefaultBaseURL, "public origin base URL")
	flag.StringVar(&opts.GoVersion, "go-version", "1.21.0", "go directive written into each generated go.mod")
	flag.StringVar(&opts.SourceRepo, "source-repo", "", "upstream source repository recorded in the manifest")
	flag.StringVar(&opts.SourceCommit, "source-commit", "", "upstream commit recorded in the manifest")
	flag.StringVar(&opts.SourceRelease, "source-release", "", "upstream release recorded in the manifest")
	flag.Parse()

	if opts.CandidatesDir == "" || opts.OutDir == "" || opts.Version == "" || opts.Time == "" {
		fmt.Fprintln(os.Stderr, "genorigin: -candidates, -out, -version and -time are required")
		os.Exit(2)
	}

	manifest, err := origin.Build(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genorigin: %v\n", err)
		os.Exit(1)
	}
	path := opts.OutDir + "/manifest.json"
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genorigin: read manifest: %v\n", err)
		os.Exit(1)
	}
	digest := sha256.Sum256(data)
	encoded, err := json.Marshal(struct {
		ManifestSHA256 string `json:"manifest_sha256"`
		Objects        int    `json:"objects"`
	}{ManifestSHA256: hex.EncodeToString(digest[:]), Objects: len(manifest.Objects)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "genorigin: encode result: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
