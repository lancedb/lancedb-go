// Command gencandidate packages one extracted lancedb-c shared library as a
// data-only Go companion module.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/lancedb/lancedb-go/scripts/go-module-origin/generator/internal/candidate"
)

func main() {
	var opts candidate.Options
	flag.StringVar(&opts.LibraryPath, "library", "", "extracted shared library for the target (required)")
	flag.StringVar(&opts.LicensePath, "license", "", "license file copied into the module (required)")
	flag.StringVar(&opts.Target, "target", "", "GOOS/GOARCH target (required)")
	flag.StringVar(&opts.ModulePath, "module-path", "", "generated module import path (required)")
	flag.StringVar(&opts.OutDir, "out", "", "new output directory (required)")
	flag.StringVar(&opts.GoVersion, "go-version", candidate.DefaultGoVersion, "go directive written into the generated go.mod")
	flag.StringVar(&opts.UpstreamRepo, "upstream-repo", "lancedb/lancedb-c", "upstream source repository recorded in the module")
	flag.StringVar(&opts.UpstreamCommit, "upstream-commit", "", "upstream commit recorded in the module (required)")
	flag.StringVar(&opts.HeaderSHA256, "header-sha256", "", "packaged C header SHA-256 recorded in the module (required)")
	flag.Parse()

	for name, value := range map[string]string{
		"library": opts.LibraryPath, "license": opts.LicensePath, "target": opts.Target,
		"module-path": opts.ModulePath, "out": opts.OutDir,
		"upstream-commit": opts.UpstreamCommit, "header-sha256": opts.HeaderSHA256,
	} {
		if value == "" {
			fmt.Fprintf(os.Stderr, "gencandidate: -%s is required\n", name)
			os.Exit(2)
		}
	}

	meta, err := candidate.Generate(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gencandidate: %v\n", err)
		os.Exit(1)
	}
	encoded, err := json.Marshal(meta)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gencandidate: encode metadata: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
