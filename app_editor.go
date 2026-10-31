package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"

	editorwatcher "conch/backend/editor"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	gossh "golang.org/x/crypto/ssh"
)

// OpenWithEditor downloads a remote file to a temp directory, opens it with the
// OS default application, and watches for changes.
// The frontend receives "editor:changed:{watchID}" events on save.
func (a *App) OpenWithEditor(connID, remotePath string) (string, error) {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return "", err
	}

	watchID := uuid.New().String()
	tmpDir := filepath.Join(os.TempDir(), "gssh-edit", watchID)
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return "", err
	}

	localPath := filepath.Join(tmpDir, filepath.Base(remotePath))

	if err := sc.Download(context.Background(), remotePath, localPath, nil); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("download for editing: %w", err)
	}

	if err := openWithDefaultApp(localPath); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("open application: %w", err)
	}

	onChange := func(wid string) {
		runtime.EventsEmit(a.ctx, "editor:changed:"+wid, map[string]string{
			"watch_id":    wid,
			"remote_path": remotePath,
			"local_path":  localPath,
		})
	}
	if err := editorwatcher.Start(watchID, localPath, remotePath, onChange); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	return watchID, nil
}

// UploadEditedFile uploads a locally edited temp file back to the remote path.
func (a *App) UploadEditedFile(connID, watchID string) error {
	w := editorwatcher.Get(watchID)
	if w == nil {
		return fmt.Errorf("watch %s not found", watchID)
	}
	sc, err := a.getSFTP(connID)
	if err != nil {
		return err
	}
	return sc.Upload(context.Background(), w.LocalPath, w.RemotePath, nil)
}

// UploadEditedFileSudo saves the edited file via sudo when the SSH user lacks
// write permission on the target: the temp file is uploaded to /tmp over SFTP
// (world-writable), then copied over the target with `sudo cp`. On success the
// password is cached in memory for the lifetime of the connection; pass an
// empty sudoPassword to reuse the cached one.
func (a *App) UploadEditedFileSudo(connID, watchID, sudoPassword string) error {
	w := editorwatcher.Get(watchID)
	if w == nil {
		return fmt.Errorf("watch %s not found", watchID)
	}
	entry := a.getConn(connID)
	if entry == nil {
		return fmt.Errorf("connection %s not found", connID)
	}
	sc, err := a.getSFTP(connID)
	if err != nil {
		return err
	}

	password := sudoPassword
	if password == "" {
		password = entry.sudoPassword
	}
	if password == "" {
		return fmt.Errorf("sudo password required")
	}

	tmpRemote := "/tmp/.conch-edit-" + uuid.New().String()
	if err := sc.Upload(context.Background(), w.LocalPath, tmpRemote, nil); err != nil {
		return fmt.Errorf("upload temp file: %w", err)
	}
	defer func() { _ = sc.Delete(tmpRemote) }()

	if err := runSudoCopy(entry.sshConn, tmpRemote, w.RemotePath, password); err != nil {
		if errors.Is(err, errSudoAuth) {
			entry.sudoPassword = ""
			return fmt.Errorf("sudo auth failed: %w", err)
		}
		return err
	}
	entry.sudoPassword = password
	return nil
}

// errSudoAuth marks sudo password failures so callers can re-prompt.
var errSudoAuth = errors.New("sudo authentication failed")

// runSudoCopy copies src over dst on the remote host via `sudo cp`.
// A PTY with echo disabled is requested so hosts with sudo's requiretty
// option still work, and the password never echoes into captured output.
// Overwriting an existing file with cp keeps its inode, owner and mode.
func runSudoCopy(client *gossh.Client, src, dst, password string) error {
	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	modes := gossh.TerminalModes{
		gossh.ECHO:          0,
		gossh.TTY_OP_ISPEED: 38400,
		gossh.TTY_OP_OSPEED: 38400,
	}
	if err := sess.RequestPty("xterm", 24, 80, modes); err != nil {
		return fmt.Errorf("request pty: %w", err)
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	var out bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &out

	cmd := fmt.Sprintf("sudo -S -p '' cp -f -- %s %s", shellQuote(src), shellQuote(dst))
	if err := sess.Start(cmd); err != nil {
		return fmt.Errorf("start sudo: %w", err)
	}
	_, _ = io.WriteString(stdin, password+"\n")
	_ = stdin.Close()

	runErr := sess.Wait()
	if runErr == nil {
		return nil
	}

	output := strings.TrimSpace(strings.ReplaceAll(out.String(), password, "***"))
	lower := strings.ToLower(output)
	if strings.Contains(lower, "try again") ||
		strings.Contains(lower, "no password") ||
		strings.Contains(lower, "a password is required") ||
		strings.Contains(lower, "incorrect password") {
		return fmt.Errorf("%w: %s", errSudoAuth, output)
	}
	if output == "" {
		output = runErr.Error()
	}
	return fmt.Errorf("sudo copy: %s", output)
}

// shellQuote single-quotes a string for POSIX sh.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// StopEditorWatch stops watching a file and cleans up the temp copy.
func (a *App) StopEditorWatch(watchID string) {
	editorwatcher.Stop(watchID)
}

// OpenDirectoryDialog shows the system directory picker and returns the chosen path.
func (a *App) OpenDirectoryDialog() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择下载目录",
	})
}

// OpenFilesDialog shows the system file picker (multi-select) and returns chosen file paths.
func (a *App) OpenFilesDialog() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择要上传的文件",
	})
}

// GetDownloadsDir returns the user's Downloads directory path.
func (a *App) GetDownloadsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}

// OpenLocalPath opens a local file or directory with the OS default application.
// Used by the transfer list to open a freshly downloaded file on click.
func (a *App) OpenLocalPath(path string) error {
	return openWithDefaultApp(path)
}

// RevealLocalPath opens the OS file manager with the given file selected
// (Windows/macOS) or its containing directory opened (Linux).
func (a *App) RevealLocalPath(path string) error {
	clean := filepath.Clean(path)
	switch goruntime.GOOS {
	case "windows":
		// explorer.exe always returns exit code 1 even on success → ignore error.
		_ = exec.Command("explorer", "/select,"+clean).Start()
		return nil
	case "darwin":
		return exec.Command("open", "-R", clean).Start()
	default:
		return exec.Command("xdg-open", filepath.Dir(clean)).Start()
	}
}

func openWithDefaultApp(path string) error {
	switch goruntime.GOOS {
	case "windows":
		// 用 explorer 打开（等价双击，走默认关联程序），而不是 `cmd /c start`。
		// 后者会把文件名交给 cmd.exe 二次解析：Go 的 EscapeArg 不转义 cmd 元字符
		// (& | ^ < >)，而下载文件名来自远端主机，会造成命令注入。
		// explorer 不经 shell，参数按单个字符串传入，安全。filepath.Clean 顺带把
		// 前端传来的正斜杠路径规范成反斜杠。
		return exec.Command("explorer", filepath.Clean(path)).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
