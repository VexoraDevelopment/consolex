//go:build windows

package term

import (
	"os"
	"syscall"
	"unsafe"
)

const enableVirtualTerminalProcessing = 0x0004

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

func EnableConsoleANSI() {
	enableHandleANSI(os.Stdout)
	enableHandleANSI(os.Stderr)
}

func enableHandleANSI(f *os.File) {
	if f == nil {
		return
	}
	var mode uint32
	fd := f.Fd()
	r1, _, _ := procGetConsoleMode.Call(fd, uintptr(unsafe.Pointer(&mode)))
	if r1 == 0 {
		return
	}
	_, _, _ = procSetConsoleMode.Call(fd, uintptr(mode|enableVirtualTerminalProcessing))
}
