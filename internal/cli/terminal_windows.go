package cli

import (
	"os"
	"syscall"
	"unsafe"
)

var consoleDLL = syscall.NewLazyDLL("kernel32.dll")
var getConsoleMode = consoleDLL.NewProc("GetConsoleMode")
var getConsoleScreenBufferInfo = consoleDLL.NewProc("GetConsoleScreenBufferInfo")
var getConsoleOutputCP = consoleDLL.NewProc("GetConsoleOutputCP")

func probeTerminal(f *os.File) terminalFacts {
	var mode uint32
	ok, _, _ := getConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&mode)))
	if ok == 0 {
		return terminalFacts{}
	}
	// CONSOLE_SCREEN_BUFFER_INFO uses signed 16-bit coordinates. Window width,
	// not backing-buffer width, is the available display space.
	var info struct {
		Size, Cursor [2]int16
		Attributes   uint16
		Window       [4]int16
		Maximum      [2]int16
	}
	width := 0
	if ok, _, _ := getConsoleScreenBufferInfo.Call(f.Fd(), uintptr(unsafe.Pointer(&info))); ok != 0 {
		width = int(info.Window[2]-info.Window[0]) + 1
	}
	cp, _, _ := getConsoleOutputCP.Call()
	return terminalFacts{tty: true, ansi: mode&0x0004 != 0, utf8: cp == 65001, width: width}
}
