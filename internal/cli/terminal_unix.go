//go:build linux || darwin

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

func probeTerminal(f *os.File) terminalFacts {
	var size struct{ Rows, Columns, X, Y uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return terminalFacts{}
	}
	term := os.Getenv("TERM")
	return terminalFacts{tty: true, ansi: term != "" && term != "dumb", utf8: utf8Locale(os.Getenv), width: int(size.Columns)}
}
