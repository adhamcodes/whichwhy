//go:build !windows

package pathdiag

import "os"

// Preserve the existing diagnostic behavior outside Windows.
func ordinaryNotFound(err error) bool { return os.IsNotExist(err) }
