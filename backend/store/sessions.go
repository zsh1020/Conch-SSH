package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"conch/backend/config"
)

// TunnelRule is a saved port-forwarding rule attached to a session.
type TunnelRule struct {
	Type       string `json:"type"`        // "local" | "remote" | "dynamic"
	LocalAddr  string `json:"local_addr"`
	RemoteAddr string `json:"remote_addr"`
}

type Session struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Group        string       `json:"group"`
	Host         string       `json:"host"`
	Port         int          `json:"port"`
	Username     string       `json:"username"`
	AuthType     string       `json:"auth_type"` // "password" | "key" | "agent"
	IdentityFile string       `json:"identity_file,omitempty"`
	Password     string       `json:"password,omitempty"`
	ProxyJump    string       `json:"proxy_jump,omitempty"`
	Tags         []string     `json:"tags"`
	LastUsed     time.Time    `json:"last_used,omitempty"`
	Starred      bool         `json:"starred,omitempty"`
	Tunnels      []TunnelRule `json:"tunnels,omitempty"`
}

type store struct {
	Version     int       `json:"version"`
	Sessions    []Session `json:"sessions"`
	GroupOrder  []string  `json:"group_order,omitempty"`
}

var (
	mu       sync.RWMutex
	dataFile string
)

func Init() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	dataFile = filepath.Join(dir, "sessions.json")
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		return save(&store{Version: 1, Sessions: []Session{}})
	}
	return nil
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".config")
	dir := filepath.Join(base, "conch")
	config.MigrateLegacyDir(filepath.Join(base, "gssh"), dir) // 老版本目录 gssh → conch
	return dir, nil
}

func load() (*store, error) {
	data, err := os.ReadFile(dataFile)
	if err != nil {
		return nil, err
	}
	var s store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	// Decrypt passwords that were stored encrypted (prefix "enc:")
	for i := range s.Sessions {
		if strings.HasPrefix(s.Sessions[i].Password, "enc:") {
			if dec, err := config.Decrypt(s.Sessions[i].Password[4:]); err == nil {
				s.Sessions[i].Password = dec
			}
		}
	}
	return &s, nil
}

func save(s *store) error {
	// Encrypt passwords before writing to disk
	toWrite := *s
	toWrite.Sessions = make([]Session, len(s.Sessions))
	copy(toWrite.Sessions, s.Sessions)
	for i := range toWrite.Sessions {
		pwd := toWrite.Sessions[i].Password
		if pwd != "" && !strings.HasPrefix(pwd, "enc:") {
			if enc, err := config.Encrypt(pwd); err == nil {
				toWrite.Sessions[i].Password = "enc:" + enc
			}
		}
	}
	data, err := json.MarshalIndent(toWrite, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataFile, data, 0600)
}

func GetGroups() ([]string, error) {
	mu.RLock()
	defer mu.RUnlock()
	s, err := load()
	if err != nil {
		return nil, err
	}
	// 如果有显式排序，直接返回
	if len(s.GroupOrder) > 0 {
		return s.GroupOrder, nil
	}
	// 否则回退到按 session 出现顺序
	seen := map[string]bool{}
	var groups []string
	for _, sess := range s.Sessions {
		g := sess.Group
		if g == "" {
			g = "默认"
		}
		if !seen[g] {
			seen[g] = true
			groups = append(groups, g)
		}
	}
	return groups, nil
}

func SaveGroupOrder(order []string) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := load()
	if err != nil {
		return err
	}
	s.GroupOrder = order
	return save(s)
}

func GetAll() ([]Session, error) {
	mu.RLock()
	defer mu.RUnlock()
	s, err := load()
	if err != nil {
		return nil, err
	}
	return s.Sessions, nil
}

func GetByID(id string) (*Session, error) {
	mu.RLock()
	defer mu.RUnlock()
	s, err := load()
	if err != nil {
		return nil, err
	}
	for i := range s.Sessions {
		if s.Sessions[i].ID == id {
			return &s.Sessions[i], nil
		}
	}
	return nil, os.ErrNotExist
}

func Save(sess Session) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := load()
	if err != nil {
		return err
	}
	if sess.ID == "" {
		sess.ID = uuid.New().String()
		s.Sessions = append(s.Sessions, sess)
	} else {
		found := false
		for i := range s.Sessions {
			if s.Sessions[i].ID == sess.ID {
				s.Sessions[i] = sess
				found = true
				break
			}
		}
		if !found {
			s.Sessions = append(s.Sessions, sess)
		}
	}
	return save(s)
}

func Delete(id string) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := load()
	if err != nil {
		return err
	}
	filtered := s.Sessions[:0]
	for _, sess := range s.Sessions {
		if sess.ID != id {
			filtered = append(filtered, sess)
		}
	}
	s.Sessions = filtered
	return save(s)
}

// RenameGroup renames all sessions in oldName to newName and updates GroupOrder.
func RenameGroup(oldName, newName string) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := load()
	if err != nil {
		return err
	}
	for i := range s.Sessions {
		if s.Sessions[i].Group == oldName {
			s.Sessions[i].Group = newName
		}
	}
	for i, g := range s.GroupOrder {
		if g == oldName {
			s.GroupOrder[i] = newName
			break
		}
	}
	return save(s)
}

// DeleteGroup removes a group from GroupOrder; sessions in that group are moved
// to the default group (Group="").
func DeleteGroup(name string) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := load()
	if err != nil {
		return err
	}
	for i := range s.Sessions {
		if s.Sessions[i].Group == name {
			s.Sessions[i].Group = ""
		}
	}
	filtered := s.GroupOrder[:0]
	for _, g := range s.GroupOrder {
		if g != name {
			filtered = append(filtered, g)
		}
	}
	s.GroupOrder = filtered
	return save(s)
}

// ReorderSessions reorders the sessions slice according to the provided id list.
// Sessions whose IDs are not in the list are appended at the end unchanged.
func ReorderSessions(ids []string) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := load()
	if err != nil {
		return err
	}
	idx := make(map[string]Session, len(s.Sessions))
	for _, sess := range s.Sessions {
		idx[sess.ID] = sess
	}
	ordered := make([]Session, 0, len(s.Sessions))
	seen := map[string]bool{}
	for _, id := range ids {
		if sess, ok := idx[id]; ok {
			ordered = append(ordered, sess)
			seen[id] = true
		}
	}
	for _, sess := range s.Sessions {
		if !seen[sess.ID] {
			ordered = append(ordered, sess)
		}
	}
	s.Sessions = ordered
	return save(s)
}

func TouchLastUsed(id string) {
	mu.Lock()
	defer mu.Unlock()
	s, _ := load()
	if s == nil {
		return
	}
	for i := range s.Sessions {
		if s.Sessions[i].ID == id {
			s.Sessions[i].LastUsed = time.Now()
			break
		}
	}
	_ = save(s)
}

// SaveTunnels replaces the saved tunnel rules for the given session ID.
func SaveTunnels(sessionID string, tunnels []TunnelRule) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := load()
	if err != nil {
		return err
	}
	for i := range s.Sessions {
		if s.Sessions[i].ID == sessionID {
			s.Sessions[i].Tunnels = tunnels
			return save(s)
		}
	}
	return os.ErrNotExist
}
