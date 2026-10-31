package main

import (
	"fmt"
	"time"

	monitorbackend "conch/backend/monitor"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StartMonitor begins sampling server stats every 1 s and emits
// "monitor:stats:{connID}" events with a MonitorStats payload.
func (a *App) StartMonitor(connID string) error {
	entry := a.getConn(connID)
	if entry == nil {
		return fmt.Errorf("connection %s not found", connID)
	}

	a.monitorMu.Lock()
	defer a.monitorMu.Unlock()

	// Closing the old channel signals the previous goroutine to exit.
	// There is a brief window where both goroutines run in parallel until
	// the old one receives the stop signal; this is benign because each
	// goroutine emits to the same event name and the frontend merges updates.
	if ch, ok := a.monitorStop[connID]; ok {
		close(ch)
	}
	stop := make(chan struct{})
	a.monitorStop[connID] = stop

	collector := monitorbackend.NewCollector(entry.sshConn)

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				runtime.EventsEmit(a.ctx, "monitor:stats:"+connID, collector.Collect())
			}
		}
	}()

	return nil
}

// StopMonitor stops the background sampler for a connection.
func (a *App) StopMonitor(connID string) {
	a.monitorMu.Lock()
	defer a.monitorMu.Unlock()
	if ch, ok := a.monitorStop[connID]; ok {
		close(ch)
		delete(a.monitorStop, connID)
	}
}
