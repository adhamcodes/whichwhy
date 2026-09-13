//go:build !windows

package resolver

import (
	"errors"
	"os"
	"syscall"
)

func ordinaryNotFound(err error) bool { return errors.Is(err, os.ErrNotExist) }

func notDirectoryError(err error) bool { return errors.Is(err, syscall.ENOTDIR) }
