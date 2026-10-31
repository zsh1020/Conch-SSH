package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

	sftpbackend "conch/backend/sftp"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// getSFTP lazily initialises the SFTP subsystem for a connection.
func (a *App) getSFTP(connID string) (*sftpbackend.SFTPClient, error) {
	entry := a.getConn(connID)
	if entry == nil {
		return nil, fmt.Errorf("connection %s not found", connID)
	}

	entry.sftpMu.Lock()
	defer entry.sftpMu.Unlock()

	if entry.sftpClient != nil {
		return entry.sftpClient, nil
	}

	sc, err := sftpbackend.New(entry.sshConn)
	if err != nil {
		return nil, fmt.Errorf("sftp not available: %w", err)
	}
	entry.sftpClient = sc
	return sc, nil
}

func (a *App) SFTPList(connID, path string) ([]sftpbackend.FileInfo, error) {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return nil, err
	}
	return sc.List(path)
}

func (a *App) SFTPHomePath(connID string) (string, error) {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return "", err
	}
	return sc.Getwd()
}

func (a *App) SFTPMkdir(connID, path string) error {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return err
	}
	return sc.Mkdir(path)
}

func (a *App) SFTPCreateFile(connID, path string) error {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return err
	}
	return sc.CreateFile(path)
}

func (a *App) SFTPDelete(connID, path string) error {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return err
	}
	return sc.Delete(path)
}

// SFTPDeleteWithProgress 异步删除一批路径（含递归目录），通过
// "sftp:progress:{transferID}"（done/total 为条目数）、"sftp:done"、"sftp:error"
// 事件上报进度，供底部传输框展示。返回 transferID。
func (a *App) SFTPDeleteWithProgress(connID string, paths []string) (string, error) {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return "", err
	}
	transferID := uuid.New().String()
	ps := append([]string(nil), paths...)
	go func() {
		if err := sc.DeleteManyWithProgress(ps, a.makeProgressFn(transferID)); err != nil {
			runtime.EventsEmit(a.ctx, "sftp:error:"+transferID, err.Error())
			return
		}
		runtime.EventsEmit(a.ctx, "sftp:done:"+transferID, nil)
	}()
	return transferID, nil
}

func (a *App) SFTPRename(connID, oldPath, newPath string) error {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return err
	}
	return sc.Rename(oldPath, newPath)
}

// SFTPChmod changes the mode of a remote file/directory.
// mode is an octal string such as "755" or "644".
func (a *App) SFTPChmod(connID, path string, mode uint32) error {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return err
	}
	return sc.Chmod(path, mode)
}

// SFTPStat returns file info (including mode) for a single remote path.
func (a *App) SFTPStat(connID, path string) (map[string]interface{}, error) {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return nil, err
	}
	info, err := sc.Stat(path)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"mode": info.Mode,
		"name": info.Name,
	}, nil
}

// SFTPUpload uploads a local file to a remote path.
// The frontend receives progress via "sftp:progress:{transferID}" events.
func (a *App) SFTPUpload(connID, localPath, remotePath string) (string, error) {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return "", err
	}
	transferID := uuid.New().String()
	ctx, cancel := context.WithCancel(a.ctx)
	a.transferCancels.Store(transferID, cancel)
	go func() {
		defer a.transferCancels.Delete(transferID)
		defer cancel()
		if err := sc.Upload(ctx, localPath, remotePath, a.makeProgressFn(transferID)); err != nil {
			if ctx.Err() != nil {
				runtime.EventsEmit(a.ctx, "sftp:cancelled:"+transferID, nil)
			} else {
				runtime.EventsEmit(a.ctx, "sftp:error:"+transferID, err.Error())
			}
		} else {
			runtime.EventsEmit(a.ctx, "sftp:done:"+transferID, nil)
		}
	}()
	return transferID, nil
}

// SFTPDownload downloads a remote file to a local path.
func (a *App) SFTPDownload(connID, remotePath, localPath string) (string, error) {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return "", err
	}
	transferID := uuid.New().String()
	ctx, cancel := context.WithCancel(a.ctx)
	a.transferCancels.Store(transferID, cancel)
	go func() {
		defer a.transferCancels.Delete(transferID)
		defer cancel()
		if err := sc.Download(ctx, remotePath, localPath, a.makeProgressFn(transferID)); err != nil {
			if ctx.Err() != nil {
				runtime.EventsEmit(a.ctx, "sftp:cancelled:"+transferID, nil)
			} else {
				runtime.EventsEmit(a.ctx, "sftp:error:"+transferID, err.Error())
			}
		} else {
			runtime.EventsEmit(a.ctx, "sftp:done:"+transferID, nil)
		}
	}()
	return transferID, nil
}

// SFTPUploadDir recursively uploads a local directory to a remote path.
func (a *App) SFTPUploadDir(connID, localPath, remotePath string) (string, error) {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return "", err
	}
	transferID := uuid.New().String()
	ctx, cancel := context.WithCancel(a.ctx)
	a.transferCancels.Store(transferID, cancel)
	go func() {
		defer a.transferCancels.Delete(transferID)
		defer cancel()
		if err := sc.UploadDir(ctx, localPath, remotePath, a.makeProgressFn(transferID)); err != nil {
			if ctx.Err() != nil {
				runtime.EventsEmit(a.ctx, "sftp:cancelled:"+transferID, nil)
			} else {
				runtime.EventsEmit(a.ctx, "sftp:error:"+transferID, err.Error())
			}
		} else {
			runtime.EventsEmit(a.ctx, "sftp:done:"+transferID, nil)
		}
	}()
	return transferID, nil
}

// SFTPDownloadDir recursively downloads a remote directory to a local path.
func (a *App) SFTPDownloadDir(connID, remotePath, localPath string) (string, error) {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return "", err
	}
	transferID := uuid.New().String()
	ctx, cancel := context.WithCancel(a.ctx)
	a.transferCancels.Store(transferID, cancel)
	go func() {
		defer a.transferCancels.Delete(transferID)
		defer cancel()
		if err := sc.DownloadDir(ctx, remotePath, localPath, a.makeProgressFn(transferID)); err != nil {
			if ctx.Err() != nil {
				runtime.EventsEmit(a.ctx, "sftp:cancelled:"+transferID, nil)
			} else {
				runtime.EventsEmit(a.ctx, "sftp:error:"+transferID, err.Error())
			}
		} else {
			runtime.EventsEmit(a.ctx, "sftp:done:"+transferID, nil)
		}
	}()
	return transferID, nil
}

// SFTPCancelTransfer cancels an in-flight transfer by its ID.
func (a *App) SFTPCancelTransfer(transferID string) {
	if v, ok := a.transferCancels.LoadAndDelete(transferID); ok {
		v.(context.CancelFunc)()
	}
}

// makeProgressFn returns a throttled progress callback that emits Wails events.
// Throttled to 200ms; concurrent goroutines may share this fn, so guarded by a mutex.
func (a *App) makeProgressFn(transferID string) func(done, total int64) {
	var (
		mu       sync.Mutex
		lastEmit time.Time
	)
	return func(done, total int64) {
		mu.Lock()
		if time.Since(lastEmit) < 200*time.Millisecond && done < total {
			mu.Unlock()
			return
		}
		lastEmit = time.Now()
		mu.Unlock()
		runtime.EventsEmit(a.ctx, "sftp:progress:"+transferID,
			map[string]int64{"done": done, "total": total})
	}
}

// SFTPReadFile reads up to maxBytes of a remote file and returns base64-encoded content.
func (a *App) SFTPReadFile(connID, remotePath string, maxBytes int64) (string, error) {
	sc, err := a.getSFTP(connID)
	if err != nil {
		return "", err
	}
	f, err := sc.OpenRemote(remotePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	limit := maxBytes
	if limit <= 0 {
		limit = 512 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
