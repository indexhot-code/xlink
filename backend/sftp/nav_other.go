//go:build !windows

package sftp

import "os"

// userDirs 非 Windows 平台按主目录拼接。
func userDirs() UserDirs {
	home, _ := os.UserHomeDir()
	return UserDirs{
		Desktop:   home + "/Desktop",
		Downloads: home + "/Downloads",
		Documents: home + "/Documents",
	}
}