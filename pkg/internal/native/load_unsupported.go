//go:build !linux && !darwin && !windows

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import "fmt"

func openNativeLibrary(string) (uintptr, error) {
	return 0, fmt.Errorf("lancedb-c native loading is supported on linux, darwin and windows only")
}
