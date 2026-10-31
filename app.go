package main

import (
	"context"
	"sync"

	appconfig "conch/backend/config"
	sftpbackend "conch/backend/sftp"
	sshbackend "conch/backend/ssh"
	"conch/backend/store"
	tunnelbackend "conch/backend/tunnel"

	gossh "golang.org/x/crypto/ssh"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main Wails application struct.
type App struct {
	ctx context.Context

	connMu      sync.RWMutex
	connections map[string]*connEntry

	monitorMu   sync.Mutex
	monitorStop map[string]chan struct{}

	// pending host-key confirmations: requestID → answer channel
	hkMu      sync.Mutex
	hkPending map[string]chan bool

	// in-flight SFTP transfers: transferID → context.CancelFunc
	transferCancels sync.Map

	// local-terminal sessions (bottom dock tabs): id → session
	localMu    sync.Mutex
	localTerms map[string]*localTermSession
}

type connEntry struct {
	client     *sshbackend.Client
	session    *sshbackend.Session
	sshConn    *gossh.Client
	sftpClient *sftpbackend.SFTPClient
	sftpMu     sync.Mutex
	tunnels    *tunnelbackend.Manager
	sessionID  string // empty for ad-hoc connections without a saved session

	// 首次 sudo 提权成功后缓存的密码（仅内存），避免每次保存重复询问
	sudoPassword string

	// 前端就绪前的输出缓冲，TerminalReady 调用后 flush
	outputMu  sync.Mutex
	outputBuf []string
	ready     bool
}

func NewApp() *App {
	return &App{
		connections: make(map[string]*connEntry),
		monitorStop: make(map[string]chan struct{}),
		hkPending:   make(map[string]chan bool),
		localTerms:  make(map[string]*localTermSession),
	}
}

// startup is called by Wails when the application starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := store.Init(); err != nil {
		runtime.LogError(ctx, "store init: "+err.Error())
	}
	if err := store.InitSnippets(); err != nil {
		runtime.LogError(ctx, "snippets init: "+err.Error())
	}
	if err := appconfig.Init(); err != nil {
		runtime.LogError(ctx, "config init: "+err.Error())
	}
	// Restore maximized state if the user left the window maximized last session.
	if appconfig.Get().WindowMaximized {
		runtime.WindowMaximise(ctx)
	}
	// 预热文件对话框线程（Windows: 在后台初始化 COM STA，消除首次打开的延迟）
	go initDialogThread()
}

// beforeClose is called by Wails just before the window closes.
// It persists the current window geometry so the next launch restores it.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	a.StopAllLocalTerms()
	s := appconfig.Get()
	s.WindowMaximized = runtime.WindowIsMaximised(ctx)
	if !s.WindowMaximized {
		w, h := runtime.WindowGetSize(ctx)
		if w >= 800 {
			s.WindowWidth = w
		}
		if h >= 600 {
			s.WindowHeight = h
		}
	}
	_ = appconfig.Save(s)
	return false
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (a *App) getConn(connID string) *connEntry {
	a.connMu.RLock()
	defer a.connMu.RUnlock()
	return a.connections[connID]
}

func (a *App) removeConn(connID string) {
	a.connMu.Lock()
	delete(a.connections, connID)
	a.connMu.Unlock()
}
