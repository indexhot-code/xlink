package connection

import (
	"context"
	"fmt"
	"log"

	"xlink/backend/store"
	"xlink/backend/vault"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Service 是 Wails 绑定的控制面服务（阶段 3 起暴露主机 CRUD）。
// 仅暴露会话管理类方法；终端字节流不经过此服务（架构规则 1、2）。
type Service struct {
	verifier *HostKeyVerifier
	vault    *vault.Service
	db       *store.Store
}

// New 创建服务；vault 可为 nil（部分测试场景）。
func New(v *vault.Service) *Service { return &Service{vault: v} }

func (s *Service) ServiceName() string { return "ConnectionService" }

func (s *Service) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	path, err := DefaultKnownHostsPath()
	if err != nil {
		return err
	}
	v, err := NewHostKeyVerifier(path)
	if err != nil {
		return err
	}
	s.verifier = v
	log.Printf("[connection] known_hosts: %s", path)
	return nil
}

func (s *Service) ServiceShutdown() error { return nil }

// Verifier 供 terminal 包复用同一份 known_hosts。
func (s *Service) Verifier() *HostKeyVerifier { return s.verifier }

// SecretResolver 返回注入 Dial 的凭据解密函数（vault 未配置时为 nil）。
func (s *Service) SecretResolver() func(int64) (string, error) {
	if s.vault == nil {
		return nil
	}
	return func(id int64) (string, error) {
		_, secret, _, err := s.vault.ResolveSecret(id)
		return secret, err
	}
}

// GetPendingHostKeyInfo 返回指定主机最近一次不匹配的新密钥信息（前端确认框展示）。
func (s *Service) GetPendingHostKeyInfo(ctx context.Context, host string) (HostKeyInfo, error) {
	info, ok := s.verifier.PendingHostKey(host)
	if !ok {
		return HostKeyInfo{}, fmt.Errorf("no pending host key for %q", host)
	}
	return info, nil
}

// ApproveHostKey 用户显式批准后调用：重写 known_hosts 中该主机条目。
// 前端须先调 GetPendingHostKeyInfo 展示指纹，用户确认后传入相同指纹。
func (s *Service) ApproveHostKey(ctx context.Context, host, keyType, fingerprint string) error {
	return s.verifier.ApproveHostKey(host, keyType, fingerprint)
}
