//go:build windows

package sftp

import "golang.org/x/sys/windows"

// userDirs 经 Windows Known Folders API 查询真实高频目录。
func userDirs() UserDirs {
	get := func(id *windows.KNOWNFOLDERID) string {
		p, err := windows.KnownFolderPath(id, 0)
		if err != nil {
			return ""
		}
		return p
	}
	return UserDirs{
		Desktop:   get(windows.FOLDERID_Desktop),
		Downloads: get(windows.FOLDERID_Downloads),
		Documents: get(windows.FOLDERID_Documents),
	}
}