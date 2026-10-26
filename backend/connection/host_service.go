package connection

import (
	"context"
	"fmt"

	"xlink/backend/store"
)

// Host 是已保存的主机配置（不含任何秘密；口令经 credentialId 在 Go 侧解密）。
type Host struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	Port         string `json:"port"`
	User         string `json:"user"`
	Auth         int    `json:"auth"` // 0=password 1=keyfile 2=agent
	CredentialID int64  `json:"credentialId"`
	KeyPath      string `json:"keyPath"`
}

const (
	authTypePassword = "password"
	authTypeKeyfile  = "keyfile"
	authTypeAgent    = "agent"
)

func authCodeToType(code int) (string, error) {
	switch code {
	case 0:
		return authTypePassword, nil
	case 1:
		return authTypeKeyfile, nil
	case 2:
		return authTypeAgent, nil
	default:
		return "", fmt.Errorf("unknown auth code %d", code)
	}
}

func authTypeToCode(t string) int {
	switch t {
	case authTypeKeyfile:
		return 1
	case authTypeAgent:
		return 2
	default:
		return 0
	}
}

// SaveHost 新增（ID=0）或更新主机配置。
func (s *Service) SaveHost(ctx context.Context, h Host) (int64, error) {
	if h.Host == "" || h.User == "" {
		return 0, fmt.Errorf("host and user are required")
	}
	if h.Port == "" {
		h.Port = "22"
	}
	authType, err := authCodeToType(h.Auth)
	if err != nil {
		return 0, err
	}
	if h.Name == "" {
		h.Name = h.Host
	}

	if h.ID == 0 {
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO hosts(name, host, port, user, auth_type, credential_id, key_path)
			 VALUES(?,?,?,?,?,?,?)`,
			h.Name, h.Host, h.Port, h.User, authType, nullableID(h.CredentialID), nullable(h.KeyPath))
		if err != nil {
			return 0, fmt.Errorf("insert host: %w", err)
		}
		return res.LastInsertId()
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE hosts SET name=?, host=?, port=?, user=?, auth_type=?, credential_id=?, key_path=?, updated_at=datetime('now')
		 WHERE id=?`,
		h.Name, h.Host, h.Port, h.User, authType, nullableID(h.CredentialID), nullable(h.KeyPath), h.ID)
	if err != nil {
		return 0, fmt.Errorf("update host: %w", err)
	}
	return h.ID, nil
}

// ListHosts 返回全部主机。
func (s *Service) ListHosts(ctx context.Context) ([]Host, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, host, port, user, auth_type, COALESCE(credential_id,0), COALESCE(key_path,'')
		 FROM hosts ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}
	defer rows.Close()
	var out []Host
	for rows.Next() {
		var h Host
		var authType string
		if err := rows.Scan(&h.ID, &h.Name, &h.Host, &h.Port, &h.User, &authType, &h.CredentialID, &h.KeyPath); err != nil {
			return nil, err
		}
		h.Auth = authTypeToCode(authType)
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetHost 按 ID 读取主机；不存在返回 sql.ErrNoRows 包装错误。
func (s *Service) GetHost(ctx context.Context, id int64) (Host, error) {
	var h Host
	var authType string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, host, port, user, auth_type, COALESCE(credential_id,0), COALESCE(key_path,'')
		 FROM hosts WHERE id=?`, id).
		Scan(&h.ID, &h.Name, &h.Host, &h.Port, &h.User, &authType, &h.CredentialID, &h.KeyPath)
	if err != nil {
		return Host{}, fmt.Errorf("get host %d: %w", id, err)
	}
	h.Auth = authTypeToCode(authType)
	return h, nil
}

// DeleteHost 删除主机。
func (s *Service) DeleteHost(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM hosts WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete host %d: %w", id, err)
	}
	return nil
}

// hostStore 由 main 装配时注入共享数据库句柄。

func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// SetStore 注入共享数据库句柄。
func (s *Service) SetStore(db *store.Store) { s.db = db }

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
