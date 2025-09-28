package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lancedb/lancedb-go/internal/binaries"
)

func main() {
	// Find the project root by looking for go.mod
	projectRoot, err := findProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding project root: %v\n", err)
		os.Exit(1)
	}

	// Ensure binaries exist
	if err := binaries.EnsureBinariesExist(projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error ensuring binaries exist: %v\n", err)
		os.Exit(1)
	}
}

// findProjectRoot walks up the directory tree to find go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find go.mod in current directory or any parent directory")
}
