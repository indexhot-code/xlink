package connection

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"

	sshagent "github.com/xanzy/ssh-agent"
)

// AuthMethod 认证方式。
type AuthMethod int

const (
	AuthPassword AuthMethod = iota // 密码
	AuthKeyFile                    // 密钥文件
	AuthAgent                      // SSH agent / Pageant
)

// ConnectOptions 连接参数。
// 内联的 Password/KeyPassphrase 仅在内存中传递，绝不落盘；
// 持久化场景由前端保存到 vault（CredentialID），连接时在 Go 侧解密。
type ConnectOptions struct {
	Host   string `json:"host"`
	Port   string `json:"port"`
	User   string `json:"user"`
	HostID int64  `json:"hostId"` // 关联 hosts 表（快连为 0）

	Auth AuthMethod `json:"auth"`

	// Auth == AuthPassword
	Password     string `json:"password,omitempty"`
	CredentialID int64  `json:"credentialId,omitempty"` // 优先于 Password，从 vault 解密

	// Auth == AuthKeyFile
	KeyPath         string `json:"keyPath,omitempty"`
	KeyPassphrase   string `json:"keyPassphrase,omitempty"`
	KeyCredentialID int64  `json:"keyCredentialId,omitempty"` // 口令从 vault 解密
}

func (o ConnectOptions) addr() string {
	port := o.Port
	if port == "" {
		port = "22"
	}
	return net.JoinHostPort(o.Host, port)
}

const dialTimeout = 15 * time.Second

// authMethods 组装认证方法列表。secretResolver 可为 nil（纯内联模式，测试用）。
func (o ConnectOptions) authMethods(secretResolver func(int64) (string, error)) ([]ssh.AuthMethod, error) {
	switch o.Auth {
	case AuthPassword:
		pw := o.Password
		if o.CredentialID > 0 {
			if secretResolver == nil {
				return nil, fmt.Errorf("credential %d requires vault", o.CredentialID)
			}
			s, err := secretResolver(o.CredentialID)
			if err != nil {
				return nil, err
			}
			pw = s
		}
		if pw == "" {
			return nil, fmt.Errorf("password is required")
		}
		return []ssh.AuthMethod{ssh.Password(pw)}, nil

	case AuthKeyFile:
		if o.KeyPath == "" {
			return nil, fmt.Errorf("key path is required")
		}
		pass := o.KeyPassphrase
		if o.KeyCredentialID > 0 {
			if secretResolver == nil {
				return nil, fmt.Errorf("credential %d requires vault", o.KeyCredentialID)
			}
			s, err := secretResolver(o.KeyCredentialID)
			if err != nil {
				return nil, err
			}
			pass = s
		}
		signer, err := parseKeyFile(o.KeyPath, pass)
		if err != nil {
			return nil, err
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil

	case AuthAgent:
		a, conn, err := sshagent.New()
		if err != nil {
			return nil, fmt.Errorf("connect ssh agent (named pipe/Pageant): %w", err)
		}
		defer conn.Close()
		signers, err := a.Signers()
		if err != nil {
			return nil, fmt.Errorf("list agent signers: %w", err)
		}
		if len(signers) == 0 {
			return nil, fmt.Errorf("agent has no keys loaded")
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signers...)}, nil

	default:
		return nil, fmt.Errorf("unknown auth method %d", o.Auth)
	}
}

// Dial 建立 SSH 连接。主机密钥经 verifier 的 TOFU 策略校验；
// 不匹配时返回 ErrHostKeyMismatch（errors.Is 可判定）。
// secretResolver 由调用方注入（通常为 vault.ResolveSecret 包装），可为 nil。
func Dial(ctx context.Context, opts ConnectOptions, verifier *HostKeyVerifier, secretResolver func(int64) (string, error)) (*ssh.Client, error) {
	methods, err := opts.authMethods(secretResolver)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            methods,
		HostKeyCallback: verifier.Callback(),
		Timeout:         dialTimeout,
	}

	d := net.Dialer{Timeout: dialTimeout}
	conn, err := d.DialContext(ctx, "tcp", opts.addr())
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", opts.addr(), err)
	}

	addr := opts.addr()
	cconn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, err // 保留原始错误（含 ErrHostKeyMismatch 包装）
	}
	return ssh.NewClient(cconn, chans, reqs), nil
}
