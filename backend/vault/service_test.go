package vault

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"xlink/backend/store"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// testService 用临时库构建已迁移的 Service，返回 (service, db文件路径)。
func testService(t *testing.T) (*Service, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(store.DefaultDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := New(db)
	if err := s.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}
	return s, path
}

// 加解密往返：明文一致；密文不含明文；nonce 每次不同。
func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	pt := []byte("secret-password-123")

	ct1, err := Encrypt(key, pt)
	if err != nil {
		t.Fatal(err)
	}
	ct2, err := Encrypt(key, pt)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Decrypt(key, ct1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatalf("round trip mismatch: %q", got)
	}
	if bytes.Equal(ct1, ct2) {
		t.Fatal("nonce must differ between encryptions")
	}
	if bytes.Contains(ct1, pt) {
		t.Fatal("ciphertext must not contain plaintext")
	}
}

// 错误密钥解密必须失败。
func TestDecryptWrongKey(t *testing.T) {
	key := make([]byte, 32)
	other := make([]byte, 32)
	other[0] = 1
	ct, err := Encrypt(key, []byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(other, ct); err == nil {
		t.Fatal("decrypt with wrong key must fail")
	}
}

// 完整流程：setup → save → lock → unlock → 解密一致；未解锁报 ErrLocked。
func TestVaultLifecycle(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()

	st, err := s.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Configured || st.Unlocked {
		t.Fatal("fresh vault should be locked & unconfigured")
	}

	if _, err := s.SaveCredential(ctx, CredentialInput{Name: "x", Auth: "password", Secret: "p"}); err != ErrLocked {
		t.Fatalf("want ErrLocked, got %v", err)
	}

	if err := s.SetupMasterPassword(ctx, "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	id, err := s.SaveCredential(ctx, CredentialInput{Name: "srv", Auth: "password", Secret: "hunter2"})
	if err != nil {
		t.Fatal(err)
	}

	auth, secret, _, err := s.ResolveSecret(id)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "password" || secret != "hunter2" {
		t.Fatalf("unexpected credential %q/%q", auth, secret)
	}

	s.Lock()
	if _, _, _, err := s.ResolveSecret(id); err != ErrLocked {
		t.Fatalf("resolve after lock: want ErrLocked, got %v", err)
	}

	if err := s.Unlock(ctx, "wrong password"); err != ErrAuthFailed {
		t.Fatalf("unlock with wrong password: want ErrAuthFailed, got %v", err)
	}
	if err := s.Unlock(ctx, "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	_, secret2, _, err := s.ResolveSecret(id)
	if err != nil {
		t.Fatal(err)
	}
	if secret2 != "hunter2" {
		t.Fatalf("secret changed after relock: %q", secret2)
	}

	list, err := s.ListCredentials(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "srv" {
		t.Fatalf("unexpected list: %+v", list)
	}
}

// Setup 仅允许一次；口令过短拒绝。
func TestSetupConstraints(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	if err := s.SetupMasterPassword(ctx, "short"); err == nil {
		t.Fatal("short password must be rejected")
	}
	if err := s.SetupMasterPassword(ctx, "first password"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetupMasterPassword(ctx, "second password"); err == nil {
		t.Fatal("second setup must be rejected")
	}
}
