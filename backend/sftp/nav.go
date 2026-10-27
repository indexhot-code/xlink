package sftp

import (
	"context"
	"encoding/json"
)

// UserDirs 本地高频目录。
type UserDirs struct {
	Desktop   string `json:"desktop"`
	Downloads string `json:"downloads"`
	Documents string `json:"documents"`
}

// UserDirs 返回本地高频目录（Windows 经 Known Folders 查询真实路径，
// 兼容桌面重定向/OneDrive 接管；资源管理器看到的桌面在哪，这里就是哪）。
func (s *Service) UserDirs(ctx context.Context) (UserDirs, error) { return userDirs(), nil }

// RemoteHome 返回远端主目录（SFTP 登录默认工作目录，等价 Linux ~）。
func (s *Service) RemoteHome(ctx context.Context, sessionID string) (string, error) {
	sc, closeFn, err := s.sftpClient(sessionID)
	if err != nil {
		return "", err
	}
	defer closeFn()
	return sc.Getwd()
}

// LastDirs 两端最后所在目录（重启恢复，对齐资源管理器停在上次位置的惯例）。
type LastDirs struct {
	Local  string `json:"local"`
	Remote string `json:"remote"`
}

const lastDirsKey = "sftp_lastdirs"

func (s *Service) GetLastDirs(ctx context.Context) (LastDirs, error) { return s.lastDirs, nil }

func (s *Service) SaveLastDirs(ctx context.Context, in LastDirs) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`INSERT INTO settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, lastDirsKey, string(raw)); err != nil {
		return err
	}
	s.lastDirs = in
	return nil
}