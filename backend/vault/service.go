package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	"xlink/backend/store"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// VaultStatus 是前端查询的保险库状态。
type VaultStatus struct {
	Configured bool `json:"configured"` // 是否已设置主口令
	Unlocked   bool `json:"unlocked"`   // 是否已解锁
}

// CredentialMeta 是凭据的非敏感元数据（不含任何秘密）。
type CredentialMeta struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Auth    string `json:"auth"` // password | keyfile | agent
	KeyPath string `json:"keyPath,omitempty"`
}

// CredentialInput 是保存凭据的输入；Secret 将被 AES-256-GCM 加密落盘。
type CredentialInput struct {
	Name    string `json:"name"`
	Auth    string `json:"auth"` // password | keyfile | agent
	Secret  string `json:"secret,omitempty"`
	KeyPath string `json:"keyPath,omitempty"`
}

// Service 是 Wails 绑定的 CredentialVaultService。
// 主密钥由 scrypt 从主口令派生，仅驻留内存；Lock/退出即丢弃（架构规则 4）。
type Service struct {
	db *store.Store

	mu       sync.Mutex
	key      []byte
	params   KDFParams
	verifier []byte
}

func New(db *store.Store) *Service { return &Service{db: db} }

func (s *Service) ServiceName() string { return "CredentialVaultService" }

func (s *Service) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if err := s.db.Migrate(ctx); err != nil {
		return fmt.Errorf("vault migrate: %w", err)
	}
	if err := s.loadMeta(); err != nil {
		return fmt.Errorf("vault load meta: %w", err)
	}
	log.Printf("[vault] configured=%v", s.verifier != nil)
	return nil
}

func (s *Service) ServiceShutdown() error {
	s.Lock()
	return nil
}

// ---- 元数据存取（vault_meta）----

func (s *Service) loadMeta() error {
	paramsRaw, err := s.getMetaBlob("kdf_params")
	if errors.Is(err, sql.ErrNoRows) {
		return nil // 未配置主口令
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(paramsRaw, &s.params); err != nil {
		return fmt.Errorf("unmarshal kdf_params: %w", err)
	}
	verifier, err := s.getMetaBlob("verifier")
	if err != nil {
		return err
	}
	s.verifier = verifier
	return nil
}

func (s *Service) getMetaBlob(key string) ([]byte, error) {
	var b []byte
	err := s.db.QueryRowContext(context.Background(),
		`SELECT value FROM vault_meta WHERE key = ?`, key).Scan(&b)
	return b, err
}

func (s *Service) setMeta(key string, value []byte) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO vault_meta(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// ---- 状态与口令 ----

// Status 返回保险库配置/解锁状态。
func (s *Service) Status(ctx context.Context) (VaultStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return VaultStatus{
		Configured: s.verifier != nil,
		Unlocked:   s.key != nil,
	}, nil
}

// SetupMasterPassword 首次设置主口令（仅允许一次）。
func (s *Service) SetupMasterPassword(ctx context.Context, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.verifier != nil {
		return errors.New("master password already configured")
	}
	if len(password) < 8 {
		return errors.New("master password must be at least 8 characters")
	}
	params, err := DefaultKDFParams()
	if err != nil {
		return err
	}
	key, err := DeriveMasterKey(password, params)
	if err != nil {
		return err
	}
	verifier, err := MakeVerifier(key)
	if err != nil {
		return err
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return err
	}
	if err := s.setMeta("kdf_params", paramsJSON); err != nil {
		return err
	}
	if err := s.setMeta("verifier", verifier); err != nil {
		return err
	}
	s.params = params
	s.verifier = verifier
	s.key = key
	return nil
}

// Unlock 用主口令解锁保险库。
func (s *Service) Unlock(ctx context.Context, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.verifier == nil {
		return errors.New("master password not configured")
	}
	if s.key != nil {
		return nil // 已解锁
	}
	key, err := DeriveMasterKey(password, s.params)
	if err != nil {
		return err
	}
	if err := CheckVerifier(key, s.verifier); err != nil {
		return err // ErrAuthFailed
	}
	s.key = key
	return nil
}

// Lock 锁定并擦除内存中的主密钥。
func (s *Service) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.key != nil {
		for i := range s.key {
			s.key[i] = 0
		}
		s.key = nil
	}
}

// ---- 凭据 CRUD ----

// SaveCredential 保存凭据（Secret 加密落盘）。
func (s *Service) SaveCredential(ctx context.Context, in CredentialInput) (int64, error) {
	s.mu.Lock()
	key := s.key
	s.mu.Unlock()
	if key == nil {
		return 0, ErrLocked
	}

	var cipher []byte
	if in.Secret != "" {
		var err error
		cipher, err = Encrypt(key, []byte(in.Secret))
		if err != nil {
			return 0, fmt.Errorf("encrypt secret: %w", err)
		}
	}
	if in.Auth != "agent" && cipher == nil {
		return 0, errors.New("secret is required for password/keyfile auth")
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO credentials(name, auth_type, secret_ciphertext, key_path)
		 VALUES(?, ?, ?, ?)`, in.Name, in.Auth, cipher, nullable(in.KeyPath))
	if err != nil {
		return 0, fmt.Errorf("insert credential: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ListCredentials 返回凭据元数据（不含秘密）。
func (s *Service) ListCredentials(ctx context.Context) ([]CredentialMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, auth_type, COALESCE(key_path, '') FROM credentials ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CredentialMeta
	for rows.Next() {
		var m CredentialMeta
		if err := rows.Scan(&m.ID, &m.Name, &m.Auth, &m.KeyPath); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteCredential 删除凭据。
func (s *Service) DeleteCredential(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM credentials WHERE id = ?`, id)
	return err
}

// ResolveSecret 供 terminal/connection 内部使用：解锁后解出凭据明文。
// 不作为 Binding 暴露——秘密绝不回传前端。
func (s *Service) ResolveSecret(credentialID int64) (auth string, secret string, keyPath string, err error) {
	s.mu.Lock()
	key := s.key
	s.mu.Unlock()
	if key == nil {
		return "", "", "", ErrLocked
	}

	var (
		a        string
		cipher   []byte
		kp       sql.NullString
	)
	err = s.db.QueryRowContext(context.Background(),
		`SELECT auth_type, secret_ciphertext, key_path FROM credentials WHERE id = ?`,
		credentialID).Scan(&a, &cipher, &kp)
	if err != nil {
		return "", "", "", fmt.Errorf("load credential %d: %w", credentialID, err)
	}
	if len(cipher) > 0 {
		pt, derr := Decrypt(key, cipher)
		if derr != nil {
			return "", "", "", fmt.Errorf("decrypt credential %d: %w", credentialID, derr)
		}
		secret = string(pt)
	}
	return a, secret, kp.String, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
