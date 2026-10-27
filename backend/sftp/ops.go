package sftp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/sftp"
)

// ---- 本地文件操作（危险操作的确认由前端负责） ----

// LocalDelete 递归删除本地文件/目录。
func (s *Service) LocalDelete(ctx context.Context, path string) error {
	if path == "" || path == `\` || path == "/" {
		return fmt.Errorf("invalid path")
	}
	return os.RemoveAll(path)
}

// LocalRename 重命名（仅改名字，不移动目录）。
func (s *Service) LocalRename(ctx context.Context, oldPath, newName string) error {
	if strings.ContainsAny(newName, `\/:*?"<>|`) {
		return fmt.Errorf("name contains invalid characters")
	}
	newPath := filepath.Join(filepath.Dir(oldPath), newName)
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("target already exists: %s", newName)
	}
	return os.Rename(oldPath, newPath)
}

// LocalMkdir 在 parent 下新建目录，返回完整路径。
func (s *Service) LocalMkdir(ctx context.Context, parent, name string) (string, error) {
	if strings.ContainsAny(name, `\/:*?"<>|`) {
		return "", fmt.Errorf("name contains invalid characters")
	}
	p := filepath.Join(parent, name)
	if _, err := os.Stat(p); err == nil {
		return "", fmt.Errorf("already exists: %s", name)
	}
	if err := os.Mkdir(p, 0o755); err != nil {
		return "", err
	}
	return p, nil
}

// LocalReveal 在资源管理器中显示该文件/目录。
func (s *Service) LocalReveal(ctx context.Context, path string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	return exec.Command("explorer", "/select,", path).Start()
}

// ---- 远端文件操作 ----

// RemoteDelete 递归删除远端文件/目录（拒绝删除根目录）。
func (s *Service) RemoteDelete(ctx context.Context, sessionID, path string) error {
	sc, closeFn, err := s.sftpClient(sessionID)
	if err != nil {
		return err
	}
	defer closeFn()
	return remoteRemove(sc, path)
}

func remoteRemove(sc *sftp.Client, path string) error {
	if path == "" || path == "/" {
		return fmt.Errorf("refuse to delete root")
	}
	fi, err := sc.Stat(path)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return sc.Remove(path)
	}
	entries, err := sc.ReadDir(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := remoteRemove(sc, joinRemote(path, e.Name())); err != nil {
			return err
		}
	}
	return sc.RemoveDirectory(path)
}

// RemoteRename 远端重命名。
func (s *Service) RemoteRename(ctx context.Context, sessionID, oldPath, newName string) error {
	if newName == "" || newName == "." || newName == ".." || strings.ContainsAny(newName, `/\`) {
		return fmt.Errorf("invalid name")
	}
	sc, closeFn, err := s.sftpClient(sessionID)
	if err != nil {
		return err
	}
	defer closeFn()
	parent := oldPath[:strings.LastIndex(oldPath, "/")]
	newPath := parent + "/" + newName
	if _, err := sc.Stat(newPath); err == nil {
		return fmt.Errorf("target already exists: %s", newName)
	}
	return sc.Rename(oldPath, newPath)
}

// RemoteMkdir 远端新建目录，返回完整路径。
func (s *Service) RemoteMkdir(ctx context.Context, sessionID, parent, name string) (string, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid name")
	}
	sc, closeFn, err := s.sftpClient(sessionID)
	if err != nil {
		return "", err
	}
	defer closeFn()
	p := joinRemote(parent, name)
	if err := sc.Mkdir(p); err != nil {
		return "", err
	}
	return p, nil
}