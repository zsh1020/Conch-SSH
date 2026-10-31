package main

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"time"

	appconfig "conch/backend/config"
	sshconfigimport "conch/backend/sshconfig"
	"conch/backend/store"
	tunnelbackend "conch/backend/tunnel"

	"github.com/google/uuid"
)

// ─── Snippets ─────────────────────────────────────────────────────────────────

func (a *App) GetSnippets() ([]store.Snippet, error) { return store.GetSnippets() }
func (a *App) SaveSnippet(sn store.Snippet) error    { return store.SaveSnippet(sn) }
func (a *App) DeleteSnippet(id string) error          { return store.DeleteSnippet(id) }

// ─── Sessions ─────────────────────────────────────────────────────────────────

func (a *App) GetSessions() ([]store.Session, error)    { return store.GetAll() }
func (a *App) GetGroups() ([]string, error)              { return store.GetGroups() }
func (a *App) SaveSession(sess store.Session) error      { return store.Save(sess) }
func (a *App) DeleteSession(id string) error             { return store.Delete(id) }
func (a *App) SaveGroupOrder(order []string) error       { return store.SaveGroupOrder(order) }
func (a *App) RenameGroup(oldName, newName string) error { return store.RenameGroup(oldName, newName) }
func (a *App) DeleteGroup(name string) error             { return store.DeleteGroup(name) }
func (a *App) ReorderSessions(ids []string) error        { return store.ReorderSessions(ids) }

// ─── Settings ─────────────────────────────────────────────────────────────────

func (a *App) GetSettings() (appconfig.Settings, error) { return appconfig.Get(), nil }
func (a *App) SaveSettings(s appconfig.Settings) error  { return appconfig.Save(s) }

// ─── SSH config import ────────────────────────────────────────────────────────

// ImportSSHConfig parses ~/.ssh/config and saves the sessions found.
// Returns the number of sessions successfully imported.
func (a *App) ImportSSHConfig() (int, error) {
	sessions, err := sshconfigimport.Import(sshconfigimport.DefaultPath())
	if err != nil {
		return 0, err
	}
	n := 0
	for _, s := range sessions {
		if store.Save(s) == nil {
			n++
		}
	}
	return n, nil
}

// ─── Local filesystem ─────────────────────────────────────────────────────────

// LocalFileInfo is a JSON-serializable local directory entry.
type LocalFileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	Path    string    `json:"path"`
	ModTime time.Time `json:"mod_time"`
}

func (a *App) LocalList(path string) ([]LocalFileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]LocalFileInfo, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, LocalFileInfo{
			Name:    e.Name(),
			Size:    info.Size(),
			IsDir:   e.IsDir(),
			Path:    filepath.Join(path, e.Name()),
			ModTime: info.ModTime(),
		})
	}
	return result, nil
}

func (a *App) LocalHomePath() (string, error) { return os.UserHomeDir() }

// LocalListRoots returns root paths: drive letters on Windows, ["/"] elsewhere.
func (a *App) LocalListRoots() []string {
	if goruntime.GOOS == "windows" {
		var roots []string
		for c := 'A'; c <= 'Z'; c++ {
			p := string(c) + ":\\"
			if _, err := os.Stat(p); err == nil {
				roots = append(roots, p)
			}
		}
		return roots
	}
	return []string{"/"}
}

// LocalIsDir returns true if the given local path is a directory.
func (a *App) LocalIsDir(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}

// QuickDir is a named shortcut to a local directory.
type QuickDir struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// LocalQuickDirs returns the standard user directories (Desktop, Documents,
// Downloads, Pictures, Music, Videos) that actually exist on this system.
func (a *App) LocalQuickDirs() ([]QuickDir, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	candidates := []struct{ name, sub string }{
		{"桌面", "Desktop"},
		{"文档", "Documents"},
		{"下载", "Downloads"},
		{"图片", "Pictures"},
		{"音乐", "Music"},
		{"视频", "Videos"},
	}
	result := make([]QuickDir, 0, len(candidates))
	for _, c := range candidates {
		p := filepath.Join(home, c.sub)
		if _, err := os.Stat(p); err == nil {
			result = append(result, QuickDir{Name: c.name, Path: p})
		}
	}
	return result, nil
}

// ─── Tunnels ──────────────────────────────────────────────────────────────────

// GetSessionSavedTunnels returns the saved tunnel rules for the session
// associated with connID. Returns an empty slice for ad-hoc connections.
func (a *App) GetSessionSavedTunnels(connID string) ([]store.TunnelRule, error) {
	entry := a.getConn(connID)
	if entry == nil {
		return nil, fmt.Errorf("connection %s not found", connID)
	}
	if entry.sessionID == "" {
		return []store.TunnelRule{}, nil
	}
	sess, err := store.GetByID(entry.sessionID)
	if err != nil {
		return nil, err
	}
	if sess.Tunnels == nil {
		return []store.TunnelRule{}, nil
	}
	return sess.Tunnels, nil
}

// UpdateSavedTunnels replaces the saved tunnel rules with an explicit list.
// Used when the user removes or reorders individual saved rules.
func (a *App) UpdateSavedTunnels(connID string, rules []store.TunnelRule) error {
	entry := a.getConn(connID)
	if entry == nil {
		return fmt.Errorf("connection %s not found", connID)
	}
	if entry.sessionID == "" {
		return fmt.Errorf("此连接未关联已保存的会话，无法更新规则")
	}
	return store.SaveTunnels(entry.sessionID, rules)
}

// SaveSessionTunnels persists all currently running tunnel rules to the saved
// session associated with connID. Returns an error if connID has no saved session.
func (a *App) SaveSessionTunnels(connID string) error {
	entry := a.getConn(connID)
	if entry == nil {
		return fmt.Errorf("connection %s not found", connID)
	}
	if entry.sessionID == "" {
		return fmt.Errorf("此连接未关联已保存的会话，无法保存规则")
	}
	running := entry.tunnels.List()
	rules := make([]store.TunnelRule, 0, len(running))
	for _, t := range running {
		rules = append(rules, store.TunnelRule{
			Type:       t.Config.Type,
			LocalAddr:  t.Config.LocalAddr,
			RemoteAddr: t.Config.RemoteAddr,
		})
	}
	return store.SaveTunnels(entry.sessionID, rules)
}

func (a *App) StartTunnel(connID string, cfg tunnelbackend.TunnelConfig) (string, error) {
	entry := a.getConn(connID)
	if entry == nil {
		return "", fmt.Errorf("connection %s not found", connID)
	}
	id := uuid.New().String()
	if _, err := entry.tunnels.Start(a.ctx, id, cfg, entry.sshConn); err != nil {
		return "", err
	}
	return id, nil
}

func (a *App) StopTunnel(connID, tunnelID string) error {
	entry := a.getConn(connID)
	if entry == nil {
		return fmt.Errorf("connection %s not found", connID)
	}
	entry.tunnels.Stop(tunnelID)
	return nil
}

func (a *App) ListTunnels(connID string) ([]tunnelbackend.TunnelStatus, error) {
	entry := a.getConn(connID)
	if entry == nil {
		return nil, fmt.Errorf("connection %s not found", connID)
	}
	return entry.tunnels.List(), nil
}
