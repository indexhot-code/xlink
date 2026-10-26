//go:build windows

// Package fatal — 致命错误可见化：GUI 子系统下 log.Fatal 无人可见（阶段 5 打磨），
// 以 Win32 MessageBox 弹窗呈现后再退出。
package fatal

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Die 弹出错误对话框并退出进程。
func Die(title, msg string) {
	m, _ := windows.UTF16PtrFromString(msg)
	t, _ := windows.UTF16PtrFromString(title)
	user32 := windows.NewLazySystemDLL("user32.dll")
	// MB_ICONERROR = 0x10
	user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x10)
	os.Exit(1)
}
