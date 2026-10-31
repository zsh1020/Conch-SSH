package main

import (
	"embed"
	"net/http"
	"strings"

	appconfig "conch/backend/config"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// blockBrowserMiddleware rejects requests that don't carry the "wails.io"
// User-Agent marker that Wails v2 automatically appends to every request made
// by the embedded WebView2 window.  Plain browsers lack this marker, so they
// receive a 403 and cannot load the UI.
func blockBrowserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("User-Agent"), "wails.io") {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	app := NewApp()

	// Load settings before wails.Run so we can restore window geometry.
	// app.startup will call Init() again via sync.Once — safe, it's a no-op.
	_ = appconfig.Init()
	s := appconfig.Get()

	w, h := s.WindowWidth, s.WindowHeight
	if w < 800 {
		w = 1200
	}
	if h < 600 {
		h = 800
	}

	err := wails.Run(&options.App{
		Title:     "Conch",
		Width:     w,
		Height:    h,
		MinWidth:  800,
		MinHeight: 600,
		Frameless: true, // 去掉原生标题栏，由前端 .title-bar 充当窗口栏（含最小化/最大化/关闭）
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: blockBrowserMiddleware,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 17, B: 23, A: 1}, // #0d1117 GitHub Dark
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true, // 禁用 WebView2 原生 drop，让 Wails OnFileDrop 完整接管
			CSSDropProperty:    "--wails-drop-target",
			CSSDropValue:       "drop",
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
