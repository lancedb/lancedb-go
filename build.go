// build.go - Simple build helper that ensures binaries are downloaded
// Usage: go run build.go

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding project root: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🚀 Building lancedb-go...")

	// Run go generate to download binaries
	fmt.Println("📦 Ensuring native binaries are available...")
	cmd := exec.Command("go", "generate", "./...")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running go generate: %v\n", err)
		os.Exit(1)
	}

	// Build the project
	fmt.Println("🔨 Building Go project...")
	cmd = exec.Command("go", "build", "./...")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error building project: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Build completed successfully!")
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
