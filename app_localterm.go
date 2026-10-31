package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	goruntime "runtime"

	appconfig "conch/backend/config"

	"github.com/aymanbagabas/go-pty"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// localTermSession is one running local shell attached to a PTY.
// The app can host several at once (one per dock tab), keyed by a
// frontend-generated id in App.localTerms.
type localTermSession struct {
	pty pty.Pty
	cmd *pty.Cmd
}

// resolveLocalShell picks the default shell when no explicit choice is given.
// Priority: explicit setting → platform defaults.
func resolveLocalShell() (name string, args []string) {
	if s := appconfig.Get().LocalShell; s != "" {
		return s, nil
	}
	if goruntime.GOOS == "windows" {
		if p, err := exec.LookPath("pwsh.exe"); err == nil {
			return p, nil
		}
		if p, err := exec.LookPath("powershell.exe"); err == nil {
			return p, nil
		}
		return "cmd.exe", nil
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh, nil
	}
	if p, err := exec.LookPath("bash"); err == nil {
		return p, nil
	}
	return "/bin/sh", nil
}

// resolveShellChoice maps a frontend shell selector to an executable.
//   "powershell" → pwsh.exe (PS7) → powershell.exe
//   "cmd"        → cmd.exe
//   "custom"     → settings.LocalShell (falls back to default if unset)
//   ""/other     → platform default (resolveLocalShell)
func resolveShellChoice(choice string) (name string, args []string) {
	switch choice {
	case "cmd":
		if goruntime.GOOS != "windows" {
			return resolveLocalShell()
		}
		return "cmd.exe", nil
	case "powershell":
		if goruntime.GOOS != "windows" {
			return resolveLocalShell()
		}
		if p, err := exec.LookPath("pwsh.exe"); err == nil {
			return p, nil
		}
		if p, err := exec.LookPath("powershell.exe"); err == nil {
			return p, nil
		}
		return "powershell.exe", nil
	case "custom":
		if s := appconfig.Get().LocalShell; s != "" {
			return s, nil
		}
		return resolveLocalShell()
	default:
		return resolveLocalShell()
	}
}

// StartLocalTerm spawns a local shell for the given id at the given size.
// Output streams to "localterm:output:{id}" events (base64); on exit a
// "localterm:exit:{id}" event is emitted. An existing session with the same
// id is torn down first.
func (a *App) StartLocalTerm(id, shell string, cols, rows int) error {
	a.localMu.Lock()
	// Tear down any previous session under this id.
	if old, ok := a.localTerms[id]; ok {
		old.shutdown()
		delete(a.localTerms, id)
	}

	p, err := pty.New()
	if err != nil {
		a.localMu.Unlock()
		return fmt.Errorf("create pty: %w", err)
	}
	if cols > 0 && rows > 0 {
		_ = p.Resize(cols, rows)
	}

	name, args := resolveShellChoice(shell)
	cmd := p.Command(name, args...)
	if err := cmd.Start(); err != nil {
		_ = p.Close()
		a.localMu.Unlock()
		return fmt.Errorf("start shell %q: %w", name, err)
	}

	sess := &localTermSession{pty: p, cmd: cmd}
	a.localTerms[id] = sess
	a.localMu.Unlock()

	// Read pump: stream PTY output → frontend until EOF/exit.
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, readErr := p.Read(buf)
			if n > 0 {
				if !a.localTermIsCurrent(id, sess) {
					return // session was replaced/stopped; stay quiet
				}
				enc := base64.StdEncoding.EncodeToString(buf[:n])
				runtime.EventsEmit(a.ctx, "localterm:output:"+id, enc)
			}
			if readErr != nil {
				break
			}
		}
		_ = cmd.Wait()
		if a.localTermIsCurrent(id, sess) {
			runtime.EventsEmit(a.ctx, "localterm:exit:"+id)
		}
	}()

	return nil
}

// localTermIsCurrent reports whether id still maps to the same session pointer,
// i.e. it hasn't been stopped or replaced by a restart.
func (a *App) localTermIsCurrent(id string, sess *localTermSession) bool {
	a.localMu.Lock()
	defer a.localMu.Unlock()
	return a.localTerms[id] == sess
}

// SendLocalInput writes user keystrokes to the local shell with the given id.
func (a *App) SendLocalInput(id, data string) error {
	a.localMu.Lock()
	sess := a.localTerms[id]
	a.localMu.Unlock()
	if sess == nil {
		return fmt.Errorf("local terminal %s not started", id)
	}
	_, err := io.WriteString(sess.pty, data)
	return err
}

// ResizeLocalTerm adjusts the PTY window size for the given id.
func (a *App) ResizeLocalTerm(id string, cols, rows int) error {
	a.localMu.Lock()
	sess := a.localTerms[id]
	a.localMu.Unlock()
	if sess == nil || cols <= 0 || rows <= 0 {
		return nil
	}
	return sess.pty.Resize(cols, rows)
}

// StopLocalTerm kills the local shell with the given id and releases its PTY.
func (a *App) StopLocalTerm(id string) {
	a.localMu.Lock()
	sess, ok := a.localTerms[id]
	if ok {
		delete(a.localTerms, id)
	}
	a.localMu.Unlock()
	if ok {
		sess.shutdown()
	}
}

// StopAllLocalTerms tears down every local shell (called on app close).
func (a *App) StopAllLocalTerms() {
	a.localMu.Lock()
	sessions := a.localTerms
	a.localTerms = make(map[string]*localTermSession)
	a.localMu.Unlock()
	for _, sess := range sessions {
		sess.shutdown()
	}
}

// shutdown kills the process and closes the PTY.
func (s *localTermSession) shutdown() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	if s.pty != nil {
		_ = s.pty.Close()
	}
}
