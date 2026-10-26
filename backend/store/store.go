// Package store 提供 SQLite（modernc.org/sqlite，纯 Go 无 CGO）的连接与访问层。
// schema（hosts / sessions_snapshot / sftp_queue）在阶段 3 建立。
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store 内嵌 *sql.DB，供各 Service 注入使用。
type Store struct {
	*sql.DB
}

// Open 打开 SQLite 数据库。dsn 建议形如：
//
//	file:<path>?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// 单写者模型：限制连接数避免 SQLITE_BUSY。
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &Store{DB: db}, nil
}
