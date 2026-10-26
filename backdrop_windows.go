//go:build windows

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

// preferredBackdropType 返回当前系统支持的窗口材质：
// Windows 11 (Build >= 22621) 使用 Mica，Windows 10 回退 Acrylic。
func preferredBackdropType() application.BackdropType {
	vi := windows.RtlGetVersion()
	if vi == nil {
		return application.Acrylic
	}
	if vi.BuildNumber >= 22621 {
		return application.Mica
	}
	return application.Acrylic
}
