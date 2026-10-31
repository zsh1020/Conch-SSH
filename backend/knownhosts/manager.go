package knownhosts

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ConfirmFunc is called when an unknown host is encountered.
// Returns true if the user accepts the fingerprint.
type ConfirmFunc func(host, fingerprint string) bool

// HostKeyChangedFunc is called when a known host's key has changed.
// oldFP and newFP are SHA-256 fingerprints. Returns true to trust the new key.
type HostKeyChangedFunc func(host, oldFP, newFP string) bool

// Manager handles known_hosts verification with TOFU strategy.
type Manager struct {
	path      string
	confirm   ConfirmFunc
	onChanged HostKeyChangedFunc
}

func New(confirm ConfirmFunc, onChanged HostKeyChangedFunc) (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".ssh", "known_hosts")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, nil, 0600); err != nil {
			return nil, err
		}
	}
	return &Manager{path: path, confirm: confirm, onChanged: onChanged}, nil
}

// Callback returns an ssh.HostKeyCallback implementing TOFU.
func (m *Manager) Callback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		cb, err := knownhosts.New(m.path)
		if err != nil {
			return err
		}
		err = cb(hostname, remote, key)
		if err == nil {
			return nil // known and matched
		}

		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
			oldFP := ssh.FingerprintSHA256(keyErr.Want[0].Key)
			newFP := ssh.FingerprintSHA256(key)
			if m.onChanged != nil && m.onChanged(hostname, oldFP, newFP) {
				// User accepted: replace old entry with new key
				if err := m.removeHost(hostname); err != nil {
					return fmt.Errorf("failed to remove old host key: %w", err)
				}
				return m.append(hostname, key)
			}
			return fmt.Errorf("host key verification failed: host key changed for %s\nExpected: %s\nGot: %s",
				hostname, oldFP, newFP)
		}

		// Unknown host — ask the user
		fingerprint := ssh.FingerprintSHA256(key)
		if m.confirm != nil && m.confirm(hostname, fingerprint) {
			return m.append(hostname, key)
		}
		return fmt.Errorf("host key verification failed for %s", hostname)
	}
}

func (m *Manager) append(hostname string, key ssh.PublicKey) error {
	f, err := os.OpenFile(m.path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	line := knownhosts.Line([]string{hostname}, key)
	_, err = fmt.Fprintln(f, line)
	return err
}

// removeHost removes all known_hosts lines that match hostname.
func (m *Manager) removeHost(hostname string) error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	lines := strings.Split(string(data), "\n")
	kept := lines[:0]
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			kept = append(kept, line)
			continue
		}
		// knownhosts line format: "hostname keytype key [comment]"
		// hostname field may contain comma-separated hosts or [host]:port patterns
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			kept = append(kept, line)
			continue
		}
		hosts := strings.Split(fields[0], ",")
		match := false
		for _, h := range hosts {
			if h == hostname {
				match = true
				break
			}
		}
		if !match {
			kept = append(kept, line)
		}
	}
	return os.WriteFile(m.path, []byte(strings.Join(kept, "\n")), 0600)
}
