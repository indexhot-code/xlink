package connection

import (
	"errors"
	"testing"
)

// mismatch → 查看待批准信息 → 显式批准 → 回调通过（known_hosts 被更新为新指纹）。
func TestHostKeyMismatchApproveFlow(t *testing.T) {
	v := newTestVerifier(t)
	keyA := testKey(t)
	keyB := testKey(t)
	cb := v.Callback()

	if err := cb("example.com:22", testAddr, keyA); err != nil {
		t.Fatal(err)
	}
	// 密钥变化 → mismatch
	err := cb("example.com:22", testAddr, keyB)
	if !errors.Is(err, ErrHostKeyMismatch) {
		t.Fatalf("want ErrHostKeyMismatch, got %v", err)
	}

	// 前端查询待批准信息
	info, ok := v.PendingHostKey("example.com:22")
	if !ok {
		t.Fatal("pending host key expected")
	}
	if info.KeyType != keyB.Type() {
		t.Fatalf("unexpected key type %q", info.KeyType)
	}

	// 指纹不符的批准必须被拒绝
	if err := v.ApproveHostKey("example.com:22", info.KeyType, "sha256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"); err == nil {
		t.Fatal("approve with wrong fingerprint must fail")
	}

	// 正确批准 → 后续连接通过
	if err := v.ApproveHostKey("example.com:22", info.KeyType, info.Fingerprint); err != nil {
		t.Fatal(err)
	}
	if err := cb("example.com:22", testAddr, keyB); err != nil {
		t.Fatalf("connect after approve should pass, got: %v", err)
	}
	// 旧密钥现在应 mismatch
	if err := cb("example.com:22", testAddr, keyA); !errors.Is(err, ErrHostKeyMismatch) {
		t.Fatalf("old key should now mismatch, got %v", err)
	}
}
