package main

// AppVersion is the current release version.
// Can be overridden at build time:
//
//	wails build -ldflags "-X main.AppVersion=1.0.0"
//
// 项目更名为 Conch 后版本号重新起算，1.0.0 为开源首发版本。
var AppVersion = "1.0.0"

// GetVersion returns the application version string.
func (a *App) GetVersion() string {
	return AppVersion
}
