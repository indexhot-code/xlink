package vault

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"xlink/backend/store"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// 落盘数据不含明文：直接扫描 SQLite 主文件与 WAL（验收标准：凭据文件被 dump 无法看到明文）。
func TestNoPlaintextOnDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.db")
	db, err := store.Open(store.DefaultDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := New(db)
	ctx := context.Background()
	if err := s.ServiceStartup(ctx, application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetupMasterPassword(ctx, "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveCredential(ctx, CredentialInput{
		Name: "prod", Auth: "password", Secret: "super-secret-password",
	}); err != nil {
		t.Fatal(err)
	}
	// 先读原始字节，再等 t.Cleanup 关闭连接（Windows 下 TempDir 清理需要句柄释放）。
	// 检查主文件与 wal/shm 文件字节。
	var raw []byte
	for _, f := range []string{path, path + "-wal", path + "-shm"} {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		raw = append(raw, b...)
	}
	if bytes.Contains(raw, []byte("super-secret-password")) {
		t.Fatal("plaintext secret found in database files")
	}
}
