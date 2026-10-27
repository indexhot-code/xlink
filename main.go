package main

import (
	"context"
	"embed"
	"fmt"
	"runtime/debug"

	"xlink/backend/appsettings"
	"xlink/backend/connection"
	"xlink/backend/fatal"
	"xlink/backend/layout"
	"xlink/backend/sftp"
	"xlink/backend/store"
	"xlink/backend/terminal"
	"xlink/backend/vault"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// 前端资源通过 go:embed 嵌入，产物为单一 .exe（架构规则 7）。

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

func main() {
	// 全局 panic 恢复：以可见弹窗呈现而非静默退出（阶段 5）。
	defer func() {
		if r := recover(); r != nil {
			fatal.Die("XLink 已崩溃",
				fmt.Sprintf("%v\n\n%s", r, debug.Stack()))
		}
	}()

	run()
}

func run() {
	dbPath, err := store.DefaultDBPath()
	if err != nil {
		fatal.Die("初始化失败", fmt.Sprintf("无法确定数据目录：%v", err))
	}
	db, err := store.Open(store.DefaultDSN(dbPath))
	if err != nil {
		fatal.Die("初始化失败", fmt.Sprintf("无法打开数据库 %s：%v", dbPath, err))
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		fatal.Die("初始化失败", fmt.Sprintf("数据库迁移失败：%v", err))
	}

	vaultSvc := vault.New(db)
	connSvc := connection.New(vaultSvc)
	connSvc.SetStore(db)
	termSvc := terminal.New(connSvc, vaultSvc)
	setSvc := appsettings.New(db)

	app := application.New(application.Options{
		Name:        "XLink",
		Description: "轻量级 SSH 客户端：终端 + SFTP",
		Services: []application.Service{
			// 控制面 Binding：仅会话管理/配置/文件列表/传输进度/通知/托盘。
			// 终端字节流与 SFTP 分块进度绝不走这里（架构规则 1、2）。
			application.NewService(connSvc),
			application.NewService(termSvc),
			application.NewService(sftp.New(termSvc, db)),
			application.NewService(vaultSvc),
			application.NewService(layout.New(db)),
			application.NewService(setSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "XLink",
		Width:          1280,
		Height:         800,
		MinWidth:       960,
		MinHeight:      600,
		BackgroundType: application.BackgroundTypeTranslucent,
		Frameless:      true, // 无边框：自定义标题栏（原生标题栏在透明材质下清晰度差）
		Windows: application.WindowsWindow{
			BackdropType: preferredBackdropType(), // Win11 22621+ → Mica，Win10 → Acrylic
			Theme:        application.Dark,        // 深色主题：标题栏强制深色
			CustomTheme: application.ThemeSettings{
				DarkModeActive: &application.WindowTheme{
					TitleBarColour:  rgb(0x18, 0x1d, 0x2b), // 不透明标题栏底
					TitleTextColour: rgb(0xe8, 0xec, 0xf6),
					BorderColour:    rgb(0x2a, 0x31, 0x44),
				},
				DarkModeInactive: &application.WindowTheme{
					TitleBarColour:  rgb(0x14, 0x18, 0x24),
					TitleTextColour: rgb(0x9a, 0xa6, 0xc0),
					BorderColour:    rgb(0x24, 0x2a, 0x3a),
				},
			},
		},
		URL: "/",
	})

	// 关闭行为：按设置最小化到托盘（默认开启）或真正退出。
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if setSvc.SettingsNonBinding().CloseToTray {
			e.Cancel()
			win.Hide()
		}
	})

	setupTray(app, win, setSvc)

	if err := app.Run(); err != nil {
		fatal.Die("XLink 启动失败", fmt.Sprintf("%v", err))
	}
}

// setupTray 系统托盘：左键显示主窗口，右键菜单（显示/退出）。
func setupTray(app *application.App, win *application.WebviewWindow, setSvc *appsettings.Service) {
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("XLink")

	menu := application.NewMenu()
	menu.Add("显示主窗口").OnClick(func(*application.Context) {
		win.Show()
	})
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)
	tray.OnClick(func() { win.Show() })
	tray.Show()
}

// rgb 构造 Windows COLORREF（0x00BBGGRR）。
func rgb(r, g, b byte) *uint32 {
	c := uint32(r) | uint32(g)<<8 | uint32(b)<<16
	return &c
}