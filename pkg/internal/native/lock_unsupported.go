//go:build !linux && !darwin && !windows

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import (
	"fmt"
	"os"
)

func lockCacheFile(*os.File) error {
	return fmt.Errorf("native cache locking is supported on linux, darwin and windows only")
}
