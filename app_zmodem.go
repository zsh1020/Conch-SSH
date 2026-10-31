package main

import (
	"encoding/base64"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ZmodemSave opens a native save-file dialog and writes the ZMODEM-received file.
// Returns the saved path (empty string if the user cancelled).
func (a *App) ZmodemSave(suggestedName, b64data string) (string, error) {
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存文件",
		DefaultFilename: suggestedName,
	})
	if err != nil || savePath == "" {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		return "", err
	}
	return savePath, os.WriteFile(savePath, data, 0o644)
}
