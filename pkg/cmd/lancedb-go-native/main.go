// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// Command lancedb-go-native verifies and extracts the compiled-in native
// payload for the running target into the private cache. Consumers do not need
// it: the first Connect call performs the same verified installation lazily. It
// lives under pkg/ so it can use the internal cache/artifact implementation
// without exporting installer helpers.
package main

import (
	"fmt"
	"os"

	"github.com/lancedb/lancedb-go/pkg/internal/native"
)

func main() {
	path, err := native.Install()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lancedb-go-native: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(path)
}
