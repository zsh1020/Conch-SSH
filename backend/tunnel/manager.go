package tunnel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	gossh "golang.org/x/crypto/ssh"
)

// friendlyBindErr replaces a low-level "address already in use" OS error with
// a readable Chinese message. Works on Windows (WSAEADDRINUSE) and Unix (EADDRINUSE).
// The original error is wrapped so callers can still inspect it with errors.Is/As.
func friendlyBindErr(err error, addr, side string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "normally permitted") {
		return fmt.Errorf("%s端口 %s 已被占用，请修改为其他端口后重试: %w", side, addr, err)
	}
	return err
}

// TunnelConfig describes a port-forwarding rule.
type TunnelConfig struct {
	Type       string `json:"type"`        // "local" | "remote" | "dynamic"
	LocalAddr  string `json:"local_addr"`  // e.g. "127.0.0.1:18080"
	RemoteAddr string `json:"remote_addr"` // e.g. "db.internal:5432" (ignored for dynamic)
}

// TunnelStatus is the JSON-serialisable view of a running tunnel.
type TunnelStatus struct {
	ID         string       `json:"id"`
	Config     TunnelConfig `json:"config"`
	ActiveConn int          `json:"active_conn"`
	BytesIn    int64        `json:"bytes_in"`
	BytesOut   int64        `json:"bytes_out"`
}

// Tunnel represents one running port-forward.
type Tunnel struct {
	ID         string
	Config     TunnelConfig
	cancel     context.CancelFunc
	activeConn int32 // atomic counter
	bytesIn    int64 // atomic — bytes received from remote
	bytesOut   int64 // atomic — bytes sent to remote
}

func (t *Tunnel) Status() TunnelStatus {
	return TunnelStatus{
		ID:         t.ID,
		Config:     t.Config,
		ActiveConn: int(atomic.LoadInt32(&t.activeConn)),
		BytesIn:    atomic.LoadInt64(&t.bytesIn),
		BytesOut:   atomic.LoadInt64(&t.bytesOut),
	}
}

func (t *Tunnel) incConn()  { atomic.AddInt32(&t.activeConn, 1) }
func (t *Tunnel) decConn()  { atomic.AddInt32(&t.activeConn, -1) }
func (t *Tunnel) Stop()     { t.cancel() }

// Manager holds all tunnels for one SSH connection.
type Manager struct {
	mu      sync.RWMutex
	tunnels map[string]*Tunnel
}

func NewManager() *Manager {
	return &Manager{tunnels: make(map[string]*Tunnel)}
}

// Start launches a new tunnel and stores it.
func (m *Manager) Start(parentCtx context.Context, id string, cfg TunnelConfig, sshConn *gossh.Client) (*Tunnel, error) {
	ctx, cancel := context.WithCancel(parentCtx)
	t := &Tunnel{ID: id, Config: cfg, cancel: cancel}

	var err error
	switch cfg.Type {
	case "local":
		err = startLocal(ctx, t, sshConn, cfg.LocalAddr, cfg.RemoteAddr)
	case "remote":
		err = startRemote(ctx, t, sshConn, cfg.LocalAddr, cfg.RemoteAddr)
	case "dynamic":
		err = startDynamic(ctx, t, sshConn, cfg.LocalAddr)
	default:
		cancel()
		return nil, nil
	}

	if err != nil {
		cancel()
		return nil, err
	}

	m.mu.Lock()
	m.tunnels[id] = t
	m.mu.Unlock()
	return t, nil
}

// Stop cancels a tunnel by ID.
func (m *Manager) Stop(id string) {
	m.mu.Lock()
	t, ok := m.tunnels[id]
	if ok {
		delete(m.tunnels, id)
	}
	m.mu.Unlock()
	if ok {
		t.Stop()
	}
}

// StopAll cancels every tunnel (called on SSH disconnect).
func (m *Manager) StopAll() {
	m.mu.Lock()
	tunnels := make([]*Tunnel, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		tunnels = append(tunnels, t)
	}
	m.tunnels = make(map[string]*Tunnel)
	m.mu.Unlock()
	for _, t := range tunnels {
		t.Stop()
	}
}

// List returns status of all running tunnels.
func (m *Manager) List() []TunnelStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]TunnelStatus, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		out = append(out, t.Status())
	}
	return out
}
