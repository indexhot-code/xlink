package sftp

import (
	"context"
	"os"
	"runtime"
	"strings"
)

// ListDrives 枚举本地盘符（Windows）/ 根目录（其他平台）。
// 供 SFTP 本地栏盘符下拉框使用。
func (s *Service) ListDrives(_ context.Context) ([]string, error) {
	if runtime.GOOS != "windows" {
		return []string{"/"}, nil
	}
	var drives []string
	for c := 'A'; c <= 'Z'; c++ {
		root := string(c) + `:\`
		if _, err := os.Stat(root); err == nil {
			drives = append(drives, root)
		}
	}
	if drives == nil {
		drives = []string{`C:\`}
	}
	return drives, nil
}

// isWindowsPath 判断路径是否为 Windows 风格（含盘符或反斜杠）。
func isWindowsPath(p string) bool {
	return strings.Contains(p, `\`) || (len(p) >= 2 && p[1] == ':')
}
