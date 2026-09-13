package resolver

import (
	"errors"
	"os"
	"syscall"
)

func ordinaryNotFound(err error) bool {
	// Go also maps ERROR_BAD_NETPATH to ErrNotExist. A network-path failure
	// does not establish that a candidate is absent; retain it as incomplete.
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.ERROR_FILE_NOT_FOUND || errno == syscall.ERROR_PATH_NOT_FOUND || errno == syscall.ENOENT
	}
	return errors.Is(err, os.ErrNotExist)
}

// Windows ENOTDIR aliases ERROR_PATH_NOT_FOUND in Go. It cannot identify an
// existing non-directory component separately from an absent path.
func notDirectoryError(error) bool { return false }
