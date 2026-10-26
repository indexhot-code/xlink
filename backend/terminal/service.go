// Package terminal 负责 PTY 会话管理与进程内回环 WebSocket 服务器。
//
// 数据管道硬约束（架构规则 1、3）：
//   - 监听 127.0.0.1:0（仅回环），启动时生成 32 字节随机令牌，不落盘；
//   - 握手校验失败返回 403；
//   - 二进制帧 = PTY 原始字节；文本帧 = JSON 控制消息（resize/exit/state）；
//   - 背压策略为"阻塞而非丢弃"，仅超硬上限时丢弃并计数。
package terminal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"

	"golang.org/x/crypto/ssh"

	"xlink/backend/connection"
	"xlink/backend/vault"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// SessionInfo 是 CreateSession 返回给前端的握手信息。
// Token 仅存在于前端内存（组件状态），禁止落盘或写入 localStorage。
type SessionInfo struct {
	SessionID string `json:"sessionId"`
	WSPort    int    `json:"wsPort"`
	Token     string `json:"token"`
	HostID    int64  `json:"hostId"`
}

// Service 是 Wails 绑定的控制面服务：创建/关闭 PTY 会话。
// 终端字节流走回环 WebSocket，不经过 Wails Binding/Events。
type Service struct {
	conn  *connection.Service
	vault *vault.Service

	srv         *wsServer
	sessionHost map[string]int64 // sessionId → hostId（供 sftp 恢复队列）
	mu          sync.Mutex
}

func New(conn *connection.Service, v *vault.Service) *Service {
	return &Service{conn: conn, vault: v, sessionHost: make(map[string]int64)}
}

func (s *Service) ServiceName() string { return "TerminalService" }

func (s *Service) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	srv, err := newWSServer()
	if err != nil {
		return fmt.Errorf("start loopback ws server: %w", err)
	}
	s.srv = srv
	log.Printf("[terminal] loopback ws server on 127.0.0.1:%d", srv.port)
	return nil
}

func (s *Service) ServiceShutdown() error {
	s.mu.Lock()
	srv := s.srv
	s.srv = nil
	s.mu.Unlock()
	if srv != nil {
		srv.closeAll()
	}
	return nil
}

// CreateSession 建立 SSH 连接并启动 PTY，返回回环 WS 握手信息。
// WS 数据通道随后由前端主动连接 /pty/<id>?token=。
func (s *Service) CreateSession(ctx context.Context, opts connection.ConnectOptions) (SessionInfo, error) {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv == nil {
		return SessionInfo{}, fmt.Errorf("terminal service not running")
	}

	client, err := connection.Dial(ctx, opts, s.conn.Verifier(), s.conn.SecretResolver())
	if err != nil {
		return SessionInfo{}, err
	}

	id, err := newSessionID()
	if err != nil {
		client.Close()
		return SessionInfo{}, fmt.Errorf("generate session id: %w", err)
	}

	sess, err := newSession(id, client, 80, 24)
	if err != nil {
		client.Close()
		return SessionInfo{}, err
	}
	srv.put(id, sess)

	s.mu.Lock()
	s.sessionHost[id] = opts.HostID
	s.mu.Unlock()

	return SessionInfo{
		SessionID: id,
		WSPort:    srv.port,
		Token:     srv.tokenHex(),
		HostID:    opts.HostID,
	}, nil
}

// CloseSession 主动关闭会话（对应标签关闭/断开按钮）。
func (s *Service) CloseSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	if sess := srv.get(sessionID); sess != nil {
		sess.Close()
		srv.remove(sessionID)
	}
	return nil
}

// GetClient 返回会话的 SSH 客户端（供 sftp 建立子连接）；不存在返回 nil。
func (s *Service) GetClient(sessionID string) *ssh.Client {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	if sess := srv.get(sessionID); sess != nil {
		return sess.client
	}
	return nil
}

// HostIDOf 返回会话所属主机 ID；未知返回 0。
func (s *Service) HostIDOf(sessionID string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionHost[sessionID]
}

func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
