package editor

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch tracks a temp file for changes and calls onChange when modified.
type Watch struct {
	ID         string
	LocalPath  string
	RemotePath string
	onChange   func(watchID string)
	watcher    *fsnotify.Watcher
	cancel     chan struct{}
}

var (
	mu      sync.Mutex
	watches = make(map[string]*Watch)
)

// Start creates a fsnotify watcher on localPath.
// onChange is called (debounced 1 s) whenever the file is written.
func Start(id, localPath, remotePath string, onChange func(watchID string)) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(localPath); err != nil {
		w.Close()
		return err
	}

	watch := &Watch{
		ID:         id,
		LocalPath:  localPath,
		RemotePath: remotePath,
		onChange:   onChange,
		watcher:    w,
		cancel:     make(chan struct{}),
	}

	mu.Lock()
	watches[id] = watch
	mu.Unlock()

	go watch.run()
	return nil
}

func (w *Watch) run() {
	var timer *time.Timer
	for {
		select {
		case <-w.cancel:
			w.watcher.Close()
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				// Debounce: reset timer on every write
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(800*time.Millisecond, func() {
					w.onChange(w.ID)
				})
			}
		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// Stop cancels the watcher and removes the temp file.
func Stop(id string) {
	mu.Lock()
	w, ok := watches[id]
	if ok {
		delete(watches, id)
	}
	mu.Unlock()

	if !ok {
		return
	}
	close(w.cancel)
	os.Remove(w.LocalPath)
	// Remove parent temp dir if empty
	os.Remove(filepath.Dir(w.LocalPath))
}

// Get returns a watch by ID (nil if not found).
func Get(id string) *Watch {
	mu.Lock()
	defer mu.Unlock()
	return watches[id]
}

// StopAll cancels all active watchers.
func StopAll() {
	mu.Lock()
	ids := make([]string, 0, len(watches))
	for id := range watches {
		ids = append(ids, id)
	}
	mu.Unlock()
	for _, id := range ids {
		Stop(id)
	}
}
