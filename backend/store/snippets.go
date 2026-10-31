package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// Snippet is a named command template for one-click sending.
type Snippet struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`
	Group   string `json:"group"`
}

type snippetStore struct {
	Version  int       `json:"version"`
	Snippets []Snippet `json:"snippets"`
}

var (
	snMu       sync.RWMutex
	snDataFile string
)

// InitSnippets initialises the snippet store (called alongside store.Init).
func InitSnippets() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	snDataFile = filepath.Join(dir, "snippets.json")
	if _, err := os.Stat(snDataFile); os.IsNotExist(err) {
		return saveSnippets(&snippetStore{Version: 1, Snippets: []Snippet{}})
	}
	return nil
}

func loadSnippets() (*snippetStore, error) {
	data, err := os.ReadFile(snDataFile)
	if err != nil {
		return nil, err
	}
	var s snippetStore
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func saveSnippets(s *snippetStore) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(snDataFile, data, 0600)
}

// GetSnippets returns all snippets.
func GetSnippets() ([]Snippet, error) {
	snMu.RLock()
	defer snMu.RUnlock()
	s, err := loadSnippets()
	if err != nil {
		return nil, err
	}
	return s.Snippets, nil
}

// SaveSnippet creates or updates a snippet.
func SaveSnippet(sn Snippet) error {
	snMu.Lock()
	defer snMu.Unlock()
	s, err := loadSnippets()
	if err != nil {
		return err
	}
	if sn.ID == "" {
		sn.ID = uuid.New().String()
		s.Snippets = append(s.Snippets, sn)
	} else {
		found := false
		for i := range s.Snippets {
			if s.Snippets[i].ID == sn.ID {
				s.Snippets[i] = sn
				found = true
				break
			}
		}
		if !found {
			s.Snippets = append(s.Snippets, sn)
		}
	}
	return saveSnippets(s)
}

// DeleteSnippet removes a snippet by ID.
func DeleteSnippet(id string) error {
	snMu.Lock()
	defer snMu.Unlock()
	s, err := loadSnippets()
	if err != nil {
		return err
	}
	filtered := s.Snippets[:0]
	for _, sn := range s.Snippets {
		if sn.ID != id {
			filtered = append(filtered, sn)
		}
	}
	s.Snippets = filtered
	return saveSnippets(s)
}
