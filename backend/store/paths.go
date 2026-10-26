package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultDBPath 返回应用数据目录下的 SQLite 路径（%AppData%/XLink/xlink.db）。
func DefaultDBPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	dbDir := filepath.Join(dir, "XLink")
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	return filepath.Join(dbDir, "xlink.db"), nil
}

// DefaultDSN 返回带推荐 pragma 的 DSN（WAL + 忙等待 + 外键）。
func DefaultDSN(path string) string {
	return fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", filepath.ToSlash(path))
}
