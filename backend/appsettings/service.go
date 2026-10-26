// Package appsettings — 应用级设置 Binding 服务。
// 存 settings 表 key='app'（与 SFTP 传输设置同表不同键）。
package appsettings

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"

	"xlink/backend/store"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Version 应用版本号（状态栏与关于信息）。
const Version = "0.5.0"

// Terminal 终端行为设置（对新建终端生效）。
type Terminal struct {
	FontSize    int  `json:"fontSize"`    // 默认 13，范围 [8,32]
	Scrollback  int  `json:"scrollback"`  // 默认 5000，范围 [100,100000]
	CursorBlink bool `json:"cursorBlink"` // 默认 true
}

// Settings 应用设置。
type Settings struct {
	Terminal    Terminal `json:"terminal"`
	CloseToTray bool     `json:"closeToTray"` // 关闭窗口 → 隐藏到托盘
}

// AppInfo 版本与运行环境（状态栏展示）。
type AppInfo struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

type Service struct {
	db  *store.Store
	set Settings
}

func New(db *store.Store) *Service {
	return &Service{db: db, set: defaultSettings()}
}

func defaultSettings() Settings {
	return Settings{
		Terminal:    Terminal{FontSize: 13, Scrollback: 5000, CursorBlink: true},
		CloseToTray: true,
	}
}

func (s *Service) ServiceName() string { return "AppSettingsService" }

func (s *Service) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	var raw string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key='app'`).Scan(&raw)
	if err != nil {
		return nil // 无记录 → 默认值
	}
	if err := json.Unmarshal([]byte(raw), &s.set); err != nil {
		return fmt.Errorf("parse app settings: %w", err)
	}
	return nil
}

// SettingsNonBinding 供 main.go 托盘逻辑读取当前设置（不走 Binding）。
func (s *Service) SettingsNonBinding() Settings { return s.set }

func (s *Service) GetAppSettings(_ context.Context) (Settings, error) { return s.set, nil }

func (s *Service) SaveAppSettings(_ context.Context, in Settings) error {
	t := in.Terminal
	if t.FontSize < 8 || t.FontSize > 32 {
		return fmt.Errorf("fontSize must be in [8,32]")
	}
	if t.Scrollback < 100 || t.Scrollback > 100000 {
		return fmt.Errorf("scrollback must be in [100,100000]")
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`INSERT INTO settings(key, value) VALUES('app', ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, string(raw)); err != nil {
		return err
	}
	s.set = in
	return nil
}

func (s *Service) GetAppInfo(_ context.Context) AppInfo {
	return AppInfo{Version: Version, OS: runtime.GOOS, Arch: runtime.GOARCH}
}
