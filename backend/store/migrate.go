package store

import (
	"context"
	"fmt"
)

// migrations 按序执行的 schema 演进。hosts/sessions_snapshot/sftp_queue 在阶段 3。
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS vault_meta (
		key   TEXT PRIMARY KEY,
		value BLOB NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS credentials (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		name              TEXT NOT NULL,
		auth_type         TEXT NOT NULL CHECK (auth_type IN ('password','keyfile','agent')),
		secret_ciphertext BLOB,            -- AES-256-GCM：密码或私钥口令；agent 为 NULL
		key_path          TEXT,            -- 密钥文件路径（非敏感，明文）
		created_at        TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at        TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS hosts (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		name          TEXT NOT NULL,
		host          TEXT NOT NULL,
		port          TEXT NOT NULL DEFAULT '22',
		user          TEXT NOT NULL,
		auth_type     TEXT NOT NULL CHECK (auth_type IN ('password','keyfile','agent')),
		credential_id INTEGER,   -- password 或 keyfile 口令的凭据（vault.credentials.id）
		key_path      TEXT,      -- keyfile 私钥路径（非敏感）
		created_at    TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS sessions_snapshot (
		position INTEGER PRIMARY KEY,
		host_id  INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS sftp_queue (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		direction    INTEGER NOT NULL,          -- 1=upload 2=download
		session_id   TEXT NOT NULL DEFAULT '',
		host_id      INTEGER NOT NULL,
		local_path   TEXT NOT NULL,
		remote_path  TEXT NOT NULL,
		size         INTEGER NOT NULL DEFAULT 0,
		offset       INTEGER NOT NULL DEFAULT 0, -- 已传输偏移（断点续传）
		src_mtime    INTEGER NOT NULL DEFAULT 0, -- 源文件 mtime（unix nanos）
		prefix_hash  TEXT NOT NULL DEFAULT '',   -- sha256 模式：暂停时前缀哈希
		state        TEXT NOT NULL DEFAULT 'pending',
		error        TEXT NOT NULL DEFAULT '',
		created_at   TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
}

// Migrate 依次执行未应用的迁移。已用 user_version 记录进度。
func (s *Store) Migrate(ctx context.Context) error {
	var version int
	if err := s.DB.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	for i, m := range migrations {
		v := i + 1
		if v <= version {
			continue
		}
		if _, err := s.DB.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("migration %d: %w", v, err)
		}
		if _, err := s.DB.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
			return fmt.Errorf("set user_version %d: %w", v, err)
		}
	}
	return nil
}
