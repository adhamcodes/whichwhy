package pathdiag

import (
	"errors"
	"os"
	"syscall"
)

func ordinaryNotFound(err error) bool {
	// Match the resolver's narrow absence rule. Go also maps ERROR_BAD_NETPATH
	// to ErrNotExist, but that failure does not establish directory absence.
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.ERROR_FILE_NOT_FOUND || errno == syscall.ERROR_PATH_NOT_FOUND || errno == syscall.ENOENT
	}
	return errors.Is(err, os.ErrNotExist)
}
