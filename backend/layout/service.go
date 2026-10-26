// Package layout 负责标签布局快照的持久化。
// 只存 hostId 等重建信息，不存内存态会话 ID；重启后由前端按 hostId 重新建会话。
package layout

import (
	"context"
	"fmt"

	"xlink/backend/store"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// TabSnapshot 是单个标签的持久化信息。
type TabSnapshot struct {
	Position int   `json:"position"`
	HostID   int64 `json:"hostId"`
}

// Service 是 Wails 绑定的控制面服务。
type Service struct {
	db *store.Store
}

func New(db *store.Store) *Service { return &Service{db: db} }

func (s *Service) ServiceName() string { return "LayoutService" }

func (s *Service) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	return nil // 迁移由 vault.ServiceStartup 统一执行
}

func (s *Service) ServiceShutdown() error { return nil }

// GetSnapshot 返回标签快照（按 position 排序）。
func (s *Service) GetSnapshot(ctx context.Context) ([]TabSnapshot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT position, host_id FROM sessions_snapshot ORDER BY position`)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	defer rows.Close()
	var out []TabSnapshot
	for rows.Next() {
		var t TabSnapshot
		if err := rows.Scan(&t.Position, &t.HostID); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SaveSnapshot 全量替换标签快照（单事务）。
func (s *Service) SaveSnapshot(ctx context.Context, tabs []TabSnapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions_snapshot`); err != nil {
		return fmt.Errorf("clear snapshot: %w", err)
	}
	for i, t := range tabs {
		pos := t.Position
		if pos == 0 {
			pos = i + 1
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sessions_snapshot(position, host_id) VALUES(?, ?)`, pos, t.HostID); err != nil {
			return fmt.Errorf("insert snapshot: %w", err)
		}
	}
	return tx.Commit()
}
