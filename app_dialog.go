//go:build !windows

package main

import "github.com/wailsapp/wails/v2/pkg/runtime"

// initDialogThread is a no-op on non-Windows platforms.
func initDialogThread() {}

// OpenUploadTargetsDialog opens a multi-file picker on non-Windows platforms.
// On macOS, the standard NSOpenPanel can return folder paths when the user
// selects a folder; LocalIsDir on the frontend handles routing.
func (a *App) OpenUploadTargetsDialog() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择要上传的文件或文件夹",
	})
}
