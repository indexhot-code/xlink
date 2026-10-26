// Package connection 负责 SSH 连接生命周期与主机密钥校验。
package connection

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ErrHostKeyMismatch 表示主机密钥与 known_hosts 记录不匹配（潜在 MITM）。
// 必须阻止连接，并向前端传播，由用户显式批准后才能更新记录。
var ErrHostKeyMismatch = errors.New("host key mismatch")

// HostKeyInfo 是主机密钥不匹配时向前端展示的信息（用于显式批准）。
type HostKeyInfo struct {
	Host        string `json:"host"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
}

// HostKeyVerifier 实现 TOFU（Trust On First Use）：
//   - 首次连接：未知主机 → 自动追加到 known_hosts；
//   - 后续连接：密钥匹配 → 通过；不匹配 → ErrHostKeyMismatch，
//     并缓存新指纹，用户显式批准（ApproveHostKey）后才更新记录。
//
// known_hosts 为应用自管文件（应用数据目录），不读写系统 OpenSSH 文件。
type HostKeyVerifier struct {
	path string
	mu    sync.Mutex // 保护追加写入与 pending
	pending map[string]ssh.PublicKey // hostname → 不匹配时收到的新密钥
}

// DefaultKnownHostsPath 返回应用数据目录下的 known_hosts 路径。
func DefaultKnownHostsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(dir, "XLink", "known_hosts"), nil
}

// NewHostKeyVerifier 创建校验器并确保 known_hosts 文件存在（0600）。
func NewHostKeyVerifier(path string) (*HostKeyVerifier, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("touch known_hosts: %w", err)
	}
	f.Close()
	return &HostKeyVerifier{path: path, pending: make(map[string]ssh.PublicKey)}, nil
}

// Callback 返回可直接用于 ssh.ClientConfig.HostKeyCallback 的回调。
func (v *HostKeyVerifier) Callback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		kh, err := knownhosts.New(v.path)
		if err != nil {
			return fmt.Errorf("load known_hosts: %w", err)
		}
		err = kh(hostname, remote, key)
		if err == nil {
			return nil
		}
		var kerr *knownhosts.KeyError
		if !errors.As(err, &kerr) {
			return err
		}
		if len(kerr.Want) > 0 {
			// 已有记录但不匹配：绝不静默更新；缓存供显式批准后替换。
			v.mu.Lock()
			v.pending[hostname] = key
			v.mu.Unlock()
			return fmt.Errorf("%w: host %q offered key %s (%s), known keys: %d",
				ErrHostKeyMismatch, hostname, key.Type(), ssh.FingerprintSHA256(key), len(kerr.Want))
		}
		// 未知主机：TOFU 自动追加。
		if aerr := v.appendKnown(hostname, key); aerr != nil {
			return fmt.Errorf("TOFU append known_hosts: %w", aerr)
		}
		return nil
	}
}

// PendingHostKey 返回指定主机最近一次不匹配的新密钥信息（供前端确认框展示）。
func (v *HostKeyVerifier) PendingHostKey(hostname string) (HostKeyInfo, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	key, ok := v.pending[hostname]
	if !ok {
		return HostKeyInfo{}, false
	}
	return HostKeyInfo{
		Host:        hostname,
		KeyType:     key.Type(),
		Fingerprint: ssh.FingerprintSHA256(key),
	}, true
}

// ApproveHostKey 用户显式批准后调用：核对缓存的密钥类型与指纹，
// 匹配才重写 known_hosts 中该主机的条目。
func (v *HostKeyVerifier) ApproveHostKey(hostname, keyType, fingerprint string) error {
	v.mu.Lock()
	key, ok := v.pending[hostname]
	v.mu.Unlock()
	if !ok {
		return fmt.Errorf("no pending host key for %q", hostname)
	}
	if key.Type() != keyType || ssh.FingerprintSHA256(key) != fingerprint {
		return fmt.Errorf("pending key does not match approved fingerprint")
	}
	if err := v.replaceHost(hostname, key); err != nil {
		return err
	}
	v.mu.Lock()
	delete(v.pending, hostname)
	v.mu.Unlock()
	return nil
}

// replaceHost 重写 known_hosts：移除该主机全部旧条目并追加新密钥。
func (v *HostKeyVerifier) replaceHost(hostname string, key ssh.PublicKey) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	data, err := os.ReadFile(v.path)
	if err != nil {
		return err
	}
	target := knownhosts.Normalize(hostname)
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "@") {
			kept = append(kept, trimmed)
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 {
			matched := false
			for _, h := range strings.Split(fields[0], ",") {
				if h == target {
					matched = true
					break
				}
			}
			if matched {
				continue // 丢弃该主机旧条目
			}
		}
		kept = append(kept, trimmed)
	}
	kept = append(kept, knownhosts.Line([]string{target}, key))

	out := strings.Join(kept, "\n") + "\n"
	tmp := v.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, v.path)
}

// appendKnown 将主机密钥以未哈希条目追加到 known_hosts。
func (v *HostKeyVerifier) appendKnown(hostname string, key ssh.PublicKey) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	f, err := os.OpenFile(v.path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
	if _, err := fmt.Fprintln(f, line); err != nil {
		return err
	}
	return nil
}
