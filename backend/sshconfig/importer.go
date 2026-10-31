package sshconfig

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"conch/backend/store"

	"github.com/google/uuid"
)

// Import parses an OpenSSH config file and returns equivalent sessions.
// Supports: Host HostName Port User IdentityFile ProxyJump
func Import(path string) ([]store.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var sessions []store.Session
	var cur *store.Session

	flush := func() {
		if cur != nil && cur.Host != "" && !strings.ContainsAny(cur.Host, "*?") {
			if cur.ID == "" {
				cur.ID = uuid.New().String()
			}
			if cur.Port == 0 {
				cur.Port = 22
			}
			if cur.Username == "" {
				cur.Username = osUser()
			}
			if cur.AuthType == "" {
				if cur.IdentityFile != "" {
					cur.AuthType = "key"
				} else {
					cur.AuthType = "password"
				}
			}
			if cur.Name == "" {
				cur.Name = cur.Host
			}
			sessions = append(sessions, *cur)
		}
		cur = nil
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, val, found := strings.Cut(line, " ")
		if !found {
			key, val, found = strings.Cut(line, "=")
		}
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		// Strip inline comments
		if idx := strings.Index(val, " #"); idx >= 0 {
			val = strings.TrimSpace(val[:idx])
		}

		switch key {
		case "host":
			flush()
			cur = &store.Session{Host: val, Group: "imported"}
		case "hostname":
			if cur != nil {
				cur.Host = val
			}
		case "port":
			if cur != nil {
				if p, err := strconv.Atoi(val); err == nil {
					cur.Port = p
				}
			}
		case "user":
			if cur != nil {
				cur.Username = val
			}
		case "identityfile":
			if cur != nil {
				cur.IdentityFile = expandHome(val)
				cur.AuthType = "key"
			}
		case "proxyjump":
			if cur != nil {
				cur.ProxyJump = val
			}
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

// DefaultPath returns the default ~/.ssh/config path.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func osUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("USERNAME"); u != "" {
		return u
	}
	return ""
}
