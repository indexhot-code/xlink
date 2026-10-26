package connection

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func testKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func newTestVerifier(t *testing.T) *HostKeyVerifier {
	t.Helper()
	v, err := NewHostKeyVerifier(filepath.Join(t.TempDir(), "known_hosts"))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

var testAddr = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}

// 首次连接：未知主机 → TOFU 自动追加，不报错。
func TestTOFUFirstConnect(t *testing.T) {
	v := newTestVerifier(t)
	key := testKey(t)
	cb := v.Callback()

	if err := cb("example.com:22", testAddr, key); err != nil {
		t.Fatalf("first connect should pass via TOFU, got: %v", err)
	}
	data, err := os.ReadFile(v.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("known_hosts should not be empty after TOFU")
	}
}

// 二次连接：密钥匹配 → 通过。
func TestTOFUMatchOnSecondConnect(t *testing.T) {
	v := newTestVerifier(t)
	key := testKey(t)
	cb := v.Callback()

	if err := cb("example.com:22", testAddr, key); err != nil {
		t.Fatal(err)
	}
	if err := cb("example.com:22", testAddr, key); err != nil {
		t.Fatalf("second connect with same key should pass, got: %v", err)
	}
}

// 密钥不匹配 → ErrHostKeyMismatch，且 known_hosts 不被改写。
func TestHostKeyMismatch(t *testing.T) {
	v := newTestVerifier(t)
	keyA := testKey(t)
	keyB := testKey(t)
	cb := v.Callback()

	if err := cb("example.com:22", testAddr, keyA); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, v.path)

	err := cb("example.com:22", testAddr, keyB)
	if !errors.Is(err, ErrHostKeyMismatch) {
		t.Fatalf("want ErrHostKeyMismatch, got: %v", err)
	}
	if after := readFile(t, v.path); string(before) != string(after) {
		t.Fatal("known_hosts must not be modified on mismatch")
	}
}

// 非 22 端口的条目格式（[host]:port）能被正确匹配。
func TestTOFUNonStandardPort(t *testing.T) {
	v := newTestVerifier(t)
	key := testKey(t)
	cb := v.Callback()

	if err := cb("example.com:2222", testAddr, key); err != nil {
		t.Fatal(err)
	}
	if err := cb("example.com:2222", testAddr, key); err != nil {
		t.Fatalf("same port+key should match, got: %v", err)
	}
	// 不同端口视作不同主机（TOFU 各自记录）。
	if err := cb("example.com:2200", testAddr, key); err != nil {
		t.Fatalf("different port is a new host, got: %v", err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
