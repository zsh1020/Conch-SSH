package ssh

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

// ErrSessionClosed is returned by Write when the session has already been closed.
var ErrSessionClosed = errors.New("session closed")

// OutputFunc is called with base64-encoded terminal output chunks.
type OutputFunc func(data string)

// Session wraps an interactive SSH shell session with PTY.
type Session struct {
	mu      sync.Mutex
	session *gossh.Session
	stdin   io.WriteCloser
	closed  bool
}

// StartShell opens a PTY-backed shell on the given client.
// outputFn is called from a goroutine whenever data arrives from the remote.
func StartShell(client *Client, cols, rows uint32, outputFn OutputFunc) (*Session, error) {
	sess, err := client.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}

	modes := gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.IUTF8:         1, // tell kernel PTY to treat input as UTF-8 multi-byte
		gossh.TTY_OP_ISPEED: 38400,
		gossh.TTY_OP_OSPEED: 38400,
	}
	if err := sess.RequestPty("xterm-256color", int(rows), int(cols), modes); err != nil {
		sess.Close()
		return nil, fmt.Errorf("request pty: %w", err)
	}

	// Best-effort: set UTF-8 locale so the remote shell renders multi-byte
	// characters (e.g. CJK) correctly instead of as \M-^x sequences.
	// These calls are silently ignored if the server denies AcceptEnv.
	_ = sess.Setenv("LANG", "en_US.UTF-8")
	_ = sess.Setenv("LC_CTYPE", "en_US.UTF-8")

	stdout, err := sess.StdoutPipe()
	if err != nil {
		sess.Close()
		return nil, err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		sess.Close()
		return nil, err
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		sess.Close()
		return nil, err
	}

	if err := sess.Shell(); err != nil {
		sess.Close()
		return nil, fmt.Errorf("start shell: %w", err)
	}

	s := &Session{session: sess, stdin: stdin}

	// pump stdout and stderr to outputFn, coalescing rapid reads into a
	// single IPC event (up to 64 KB or 16 ms) to reduce V8 GC pressure.
	pump := func(r io.Reader) {
		const (
			maxBatch = 64 * 1024        // flush when accumulated bytes exceed 64 KB
			maxDelay = 16 * time.Millisecond // flush after at most 16 ms idle
		)
		batch := make([]byte, 0, maxBatch)
		timer := time.NewTimer(maxDelay)
		timer.Stop()
		timerArmed := false

		flush := func() {
			if len(batch) > 0 && outputFn != nil {
				outputFn(base64.StdEncoding.EncodeToString(batch))
				batch = batch[:0]
			}
			if timerArmed {
				timer.Stop()
				timerArmed = false
			}
		}

		readCh := make(chan []byte, 32)
		go func() {
			buf := make([]byte, 32*1024)
			for {
				n, err := r.Read(buf)
				if n > 0 {
					cp := make([]byte, n)
					copy(cp, buf[:n])
					readCh <- cp
				}
				if err != nil {
					close(readCh)
					return
				}
			}
		}()

		for {
			select {
			case chunk, ok := <-readCh:
				if !ok {
					flush()
					return
				}
				batch = append(batch, chunk...)
				if len(batch) >= maxBatch {
					flush()
				} else if !timerArmed {
					timer.Reset(maxDelay)
					timerArmed = true
				}
			case <-timer.C:
				timerArmed = false
				flush()
			}
		}
	}

	go pump(stdout)
	go pump(stderr)

	go func() {
		_ = sess.Wait()
		if outputFn != nil {
			// signal EOF with empty string sentinel
			outputFn("")
		}
	}()

	return s, nil
}

// Write sends raw bytes to the remote shell stdin.
func (s *Session) Write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrSessionClosed
	}
	_, err := s.stdin.Write(data)
	return err
}

// Resize sends a window-change request to the remote PTY.
func (s *Session) Resize(cols, rows uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	return s.session.WindowChange(int(rows), int(cols))
}

// Close terminates the session.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	_ = s.stdin.Close()
	return s.session.Close()
}
