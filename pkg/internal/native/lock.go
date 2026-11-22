// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import (
	"fmt"
	"os"
	"path/filepath"
)

// openCacheLock serializes all cache mutations for one target directory. The
// returned file must stay open until publication is complete; closing it
// releases the OS lock. Installers use this shared lock because Windows can
// deny unlink/rename while another process holds an open handle, and unique
// temporary names alone cannot make publication atomic across processes.
func openCacheLock(dir string) (*os.File, error) {
	lockPath := filepath.Join(dir, ".lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open native cache lock %s: %w", lockPath, err)
	}
	if err := lockCacheFile(lock); err != nil {
		lock.Close()
		return nil, fmt.Errorf("lock native cache %s: %w", lockPath, err)
	}
	return lock, nil
}
