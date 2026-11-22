// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// Command lancedb-go-native pre-downloads and verifies the native library for
// the running target. Consumers do not need it: the first Connect call performs
// the same verified installation lazily.
package main

import (
	"fmt"
	"os"

	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func main() {
	path, err := lancedb.InstallNativeLibrary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lancedb-go-native: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(path)
}
