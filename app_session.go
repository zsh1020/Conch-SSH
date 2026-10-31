package main

import (
	"encoding/base64"
	"fmt"

	sshbackend "conch/backend/ssh"
	"conch/backend/store"
	tunnelbackend "conch/backend/tunnel"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// requestHostKeyConfirm emits a Wails event carrying a unique requestID, then
// blocks until the frontend calls RespondHostKey with that ID.
func (a *App) requestHostKeyConfirm(event string, payload map[string]string) bool {
	id := payload["id"]
	ch := make(chan bool, 1)

	a.hkMu.Lock()
	a.hkPending[id] = ch
	a.hkMu.Unlock()

	runtime.EventsEmit(a.ctx, event, payload)

	select {
	case result := <-ch:
		return result
	case <-a.ctx.Done():
		return false
	}
}

// RespondHostKey is called by the frontend to resolve a pending host-key dialog.
func (a *App) RespondHostKey(requestID string, accepted bool) {
	a.hkMu.Lock()
	ch, ok := a.hkPending[requestID]
	if ok {
		delete(a.hkPending, requestID)
	}
	a.hkMu.Unlock()
	if ok {
		select {
		case ch <- accepted:
		default:
		}
	}
}

// ConnectRequest is the payload sent from the frontend.
type ConnectRequest struct {
	SessionID    string `json:"session_id,omitempty"`
	Host         string `json:"host,omitempty"`
	Port         int    `json:"port,omitempty"`
	Username     string `json:"username,omitempty"`
	AuthType     string `json:"auth_type,omitempty"`
	Password     string `json:"password,omitempty"`
	IdentityFile string `json:"identity_file,omitempty"`
}

// Connect establishes an SSH connection and starts an interactive shell.
// Returns a connID that the frontend uses for subsequent calls.
func (a *App) Connect(req ConnectRequest, cols, rows uint32) (string, error) {
	var sess store.Session

	if req.SessionID != "" {
		s, err := store.GetByID(req.SessionID)
		if err != nil {
			return "", fmt.Errorf("session not found: %w", err)
		}
		sess = *s
		if req.Password != "" {
			sess.Password = req.Password
		}
	} else {
		sess = store.Session{
			Host:         req.Host,
			Port:         req.Port,
			Username:     req.Username,
			AuthType:     req.AuthType,
			Password:     req.Password,
			IdentityFile: req.IdentityFile,
		}
	}

	connID := uuid.New().String()

	confirm := func(host, fingerprint string) bool {
		return a.requestHostKeyConfirm("hostkey:confirm", map[string]string{
			"id":          uuid.New().String(),
			"host":        host,
			"fingerprint": fingerprint,
		})
	}

	onChanged := func(host, oldFP, newFP string) bool {
		return a.requestHostKeyConfirm("hostkey:changed", map[string]string{
			"id":     uuid.New().String(),
			"host":   host,
			"old_fp": oldFP,
			"new_fp": newFP,
		})
	}

	client, err := sshbackend.Dial(a.ctx, sshbackend.ConnectOptions{
		Session:   sess,
		Password:  sess.Password,
		Confirm:   confirm,
		OnChanged: onChanged,
	})
	if err != nil {
		return "", err
	}

	outputFn := func(data string) {
		entry := a.getConn(connID)
		if data == "" {
			runtime.EventsEmit(a.ctx, "terminal:closed:"+connID)
			a.removeConn(connID)
			return
		}
		if entry != nil {
			entry.outputMu.Lock()
			if !entry.ready {
				entry.outputBuf = append(entry.outputBuf, data)
				entry.outputMu.Unlock()
				return
			}
			entry.outputMu.Unlock()
		}
		runtime.EventsEmit(a.ctx, "terminal:output:"+connID, data)
	}

	shell, err := sshbackend.StartShell(client, cols, rows, outputFn)
	if err != nil {
		client.Close()
		return "", err
	}

	entry := &connEntry{
		client:    client,
		session:   shell,
		sshConn:   client.Underlying(),
		tunnels:   tunnelbackend.NewManager(),
		sessionID: req.SessionID,
	}

	a.connMu.Lock()
	a.connections[connID] = entry
	a.connMu.Unlock()

	if req.SessionID != "" {
		store.TouchLastUsed(req.SessionID)

		// Auto-start any tunnel rules saved with this session.
		if savedSess, err := store.GetByID(req.SessionID); err == nil {
			for _, rule := range savedSess.Tunnels {
				id := uuid.New().String()
				cfg := tunnelbackend.TunnelConfig{
					Type:       rule.Type,
					LocalAddr:  rule.LocalAddr,
					RemoteAddr: rule.RemoteAddr,
				}
				_, _ = entry.tunnels.Start(a.ctx, id, cfg, entry.sshConn)
			}
		}
	}

	return connID, nil
}

// SendInput sends terminal input to the remote shell.
func (a *App) SendInput(connID, data string) error {
	entry := a.getConn(connID)
	if entry == nil {
		return fmt.Errorf("connection %s not found", connID)
	}
	return entry.session.Write([]byte(data))
}

// SendInputBase64 sends base64-encoded terminal input (for binary data).
func (a *App) SendInputBase64(connID, data string) error {
	entry := a.getConn(connID)
	if entry == nil {
		return fmt.Errorf("connection %s not found", connID)
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return err
	}
	return entry.session.Write(decoded)
}

// ResizeTerminal sends a window-change to the remote PTY.
func (a *App) ResizeTerminal(connID string, cols, rows uint32) error {
	entry := a.getConn(connID)
	if entry == nil {
		return nil
	}
	return entry.session.Resize(cols, rows)
}

// TerminalReady is called by the frontend once EventsOn is registered.
// It flushes any output buffered before the frontend was ready.
func (a *App) TerminalReady(connID string) {
	entry := a.getConn(connID)
	if entry == nil {
		return
	}
	entry.outputMu.Lock()
	entry.ready = true
	buf := entry.outputBuf
	entry.outputBuf = nil
	entry.outputMu.Unlock()

	for _, data := range buf {
		runtime.EventsEmit(a.ctx, "terminal:output:"+connID, data)
	}
}

// Disconnect closes the SSH connection for the given connID.
func (a *App) Disconnect(connID string) error {
	a.connMu.Lock()
	entry, ok := a.connections[connID]
	if ok {
		delete(a.connections, connID)
	}
	a.connMu.Unlock()

	if !ok {
		return nil
	}
	entry.tunnels.StopAll()
	if entry.sftpClient != nil {
		_ = entry.sftpClient.Close()
	}
	_ = entry.session.Close()
	return entry.client.Close()
}
