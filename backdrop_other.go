//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// 非 Windows 平台不启用材质（本项目面向 Windows 分发，此文件仅为可编译性）。
func preferredBackdropType() application.BackdropType {
	return application.Auto
}
