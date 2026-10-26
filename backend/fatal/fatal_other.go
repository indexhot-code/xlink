//go:build !windows

// Package fatal — 非 Windows 平台回退为标准错误输出。
package fatal

import (
	"fmt"
	"os"
)

// Die 输出错误到 stderr 并退出进程。
func Die(title, msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, msg)
	os.Exit(1)
}
