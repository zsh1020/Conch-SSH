package sftp

import (
	"context"
	"fmt"
	"io"
	"os"
	pathutil "path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gosftppkg "github.com/pkg/sftp"
)

// progressFileReader wraps *os.File to satisfy Size/Read/ReadAt that
// pkg/sftp.File.ReadFrom uses for the concurrent-write code path.
// Bytes consumed are accumulated atomically into *done.
type progressFileReader struct {
	ctx      context.Context
	src      *os.File
	total    int64
	done     *int64
	progress func(done, total int64)
}

// Size returns the file size so pkg/sftp picks the concurrent path.
func (p *progressFileReader) Size() int64 { return p.total }

func (p *progressFileReader) Read(buf []byte) (int, error) {
	if err := p.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := p.src.Read(buf)
	if n > 0 {
		d := atomic.AddInt64(p.done, int64(n))
		if p.progress != nil {
			p.progress(d, p.total)
		}
	}
	return n, err
}

func (p *progressFileReader) ReadAt(buf []byte, off int64) (int, error) {
	if err := p.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := p.src.ReadAt(buf, off)
	if n > 0 {
		d := atomic.AddInt64(p.done, int64(n))
		if p.progress != nil {
			p.progress(d, p.total)
		}
	}
	return n, err
}

// FileInfo is the JSON-serializable representation of a remote file.
type FileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
	Owner   string `json:"owner"`
	Group   string `json:"group"`
}

// List returns the contents of a remote directory.
func (s *SFTPClient) List(path string) ([]FileInfo, error) {
	entries, err := s.client.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		owner, group := s.resolveOwnerGroup(e)
		result = append(result, FileInfo{
			Name:    pathutil.Base(e.Name()),
			Size:    e.Size(),
			Mode:    e.Mode().String(),
			ModTime: e.ModTime().UTC().Format(time.RFC3339),
			IsDir:   e.IsDir(),
			Owner:   owner,
			Group:   group,
		})
	}
	return result, nil
}

// Stat returns metadata for a single remote path.
func (s *SFTPClient) Stat(path string) (*FileInfo, error) {
	fi, err := s.client.Stat(path)
	if err != nil {
		return nil, err
	}
	owner, group := s.resolveOwnerGroup(fi)
	return &FileInfo{
		Name:    pathutil.Base(fi.Name()),
		Size:    fi.Size(),
		Mode:    fi.Mode().String(),
		ModTime: fi.ModTime().UTC().Format(time.RFC3339),
		IsDir:   fi.IsDir(),
		Owner:   owner,
		Group:   group,
	}, nil
}

// Mkdir creates a remote directory (and any missing parents).
func (s *SFTPClient) Mkdir(path string) error {
	return s.client.MkdirAll(path)
}

// CreateFile creates an empty file at the given remote path.
func (s *SFTPClient) CreateFile(path string) error {
	f, err := s.client.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	return f.Close()
}

// Delete removes a remote file or directory. Non-empty directories are removed
// recursively (SFTP's RemoveDirectory only accepts empty dirs).
func (s *SFTPClient) Delete(path string) error {
	// 用 Lstat 而非 Stat：符号链接本身当作文件 unlink，避免跟随链接删到目标目录内容。
	fi, err := s.client.Lstat(path)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return s.removeDirAll(path)
	}
	return s.client.Remove(path)
}

// removeDirAll 递归删除目录及其全部内容（自底向上）。
func (s *SFTPClient) removeDirAll(dir string) error {
	entries, err := s.client.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		child := pathutil.Join(dir, e.Name())
		// ReadDir 返回 lstat 语义：符号链接的 IsDir() 为 false，会走 Remove（仅解链，不跟随）。
		if e.IsDir() {
			if err := s.removeDirAll(child); err != nil {
				return err
			}
		} else {
			if err := s.client.Remove(child); err != nil {
				return err
			}
		}
	}
	return s.client.RemoveDirectory(dir)
}

// DeleteManyWithProgress 删除多个路径（含递归目录），并按「已删除条目数 / 总条目数」
// 回调进度。先整体统计一遍总数，再逐个删除。
func (s *SFTPClient) DeleteManyWithProgress(paths []string, progress func(done, total int64)) error {
	var total int64
	for _, p := range paths {
		c, err := s.countEntries(p)
		if err != nil {
			return err
		}
		total += c
	}
	var done int64
	if progress != nil {
		progress(0, total)
	}
	for _, p := range paths {
		if err := s.deletePathProgress(p, &done, total, progress); err != nil {
			return err
		}
	}
	return nil
}

// countEntries 统计 path 下的条目总数（含自身；目录递归；符号链接按 1 个计）。
func (s *SFTPClient) countEntries(path string) (int64, error) {
	fi, err := s.client.Lstat(path)
	if err != nil {
		return 0, err
	}
	if !fi.IsDir() {
		return 1, nil
	}
	var n int64 = 1 // 目录自身
	entries, err := s.client.ReadDir(path)
	if err != nil {
		return n, err
	}
	for _, e := range entries {
		if e.IsDir() {
			c, err := s.countEntries(pathutil.Join(path, e.Name()))
			if err != nil {
				return n, err
			}
			n += c
		} else {
			n++
		}
	}
	return n, nil
}

func (s *SFTPClient) deletePathProgress(path string, done *int64, total int64, progress func(done, total int64)) error {
	fi, err := s.client.Lstat(path)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return s.removeDirAllProgress(path, done, total, progress)
	}
	if err := s.client.Remove(path); err != nil {
		return err
	}
	*done++
	if progress != nil {
		progress(*done, total)
	}
	return nil
}

func (s *SFTPClient) removeDirAllProgress(dir string, done *int64, total int64, progress func(done, total int64)) error {
	entries, err := s.client.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		child := pathutil.Join(dir, e.Name())
		if e.IsDir() {
			if err := s.removeDirAllProgress(child, done, total, progress); err != nil {
				return err
			}
		} else {
			if err := s.client.Remove(child); err != nil {
				return err
			}
			*done++
			if progress != nil {
				progress(*done, total)
			}
		}
	}
	if err := s.client.RemoveDirectory(dir); err != nil {
		return err
	}
	*done++
	if progress != nil {
		progress(*done, total)
	}
	return nil
}

// Rename moves or renames a remote path.
func (s *SFTPClient) Rename(oldPath, newPath string) error {
	return s.client.Rename(oldPath, newPath)
}

// Chmod changes the permission bits of a remote file/directory.
func (s *SFTPClient) Chmod(path string, mode uint32) error {
	return s.client.Chmod(path, os.FileMode(mode))
}

// Upload copies a local file to a remote path, using concurrent writes for throughput.
func (s *SFTPClient) Upload(ctx context.Context, localPath, remotePath string, progressFn func(done, total int64)) error {
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer src.Close()

	fi, err := src.Stat()
	if err != nil {
		return err
	}
	total := fi.Size()

	if err := s.client.MkdirAll(filepath.ToSlash(filepath.Dir(remotePath))); err != nil {
		return fmt.Errorf("mkdir remote parent: %w", err)
	}

	dst, err := s.client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote file: %w", err)
	}
	defer dst.Close()

	var done int64
	pr := &progressFileReader{ctx: ctx, src: src, total: total, done: &done, progress: progressFn}
	if _, err := dst.ReadFrom(pr); err != nil {
		return err
	}
	if progressFn != nil {
		progressFn(total, total)
	}
	return nil
}

// Download copies a remote file to a local path, calling progressFn every 256 KB.
func (s *SFTPClient) Download(ctx context.Context, remotePath, localPath string, progressFn func(done, total int64)) error {
	src, err := s.client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	defer src.Close()

	fi, err := src.Stat()
	if err != nil {
		return err
	}
	total := fi.Size()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("mkdir local parent: %w", err)
	}

	dst, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local file: %w", err)
	}
	defer dst.Close()

	return copyWithProgress(ctx, dst, src, total, progressFn)
}

// copyWithProgress copies from src to dst reporting progress every 256 KB.
func copyWithProgress(ctx context.Context, dst io.Writer, src io.Reader, total int64, progressFn func(done, total int64)) error {
	const chunkSize = 256 * 1024
	buf := make([]byte, chunkSize)
	var done int64
	var lastReport int64

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if progressFn != nil && done-lastReport >= chunkSize {
				progressFn(done, total)
				lastReport = done
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if progressFn != nil {
		progressFn(done, total)
	}
	return nil
}

// Getwd returns the remote working directory (typically the user's home).
func (s *SFTPClient) Getwd() (string, error) {
	return s.client.Getwd()
}

// OpenRemote opens a remote file for reading.
func (s *SFTPClient) OpenRemote(path string) (*gosftppkg.File, error) {
	return s.client.Open(path)
}

// UploadDir recursively uploads a local directory to a remote path.
func (s *SFTPClient) UploadDir(ctx context.Context, localPath, remotePath string, progressFn func(done, total int64)) error {
	type fileTask struct {
		local, remote string
	}
	var files []fileTask
	var dirs []string
	var total int64

	if err := filepath.Walk(localPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(localPath, p)
		if rel == "." {
			return nil
		}
		dest := remotePath + "/" + filepath.ToSlash(rel)
		if fi.IsDir() {
			dirs = append(dirs, dest)
		} else {
			files = append(files, fileTask{p, dest})
			total += fi.Size()
		}
		return nil
	}); err != nil {
		return fmt.Errorf("scan local dir: %w", err)
	}

	if err := s.client.MkdirAll(remotePath); err != nil {
		return fmt.Errorf("mkdir remote root: %w", err)
	}
	for _, d := range dirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.client.MkdirAll(d); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	const maxWorkers = 4
	var done int64
	var wg sync.WaitGroup
	taskCh := make(chan fileTask, maxWorkers)
	var firstErr atomic.Value

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskCh {
				if firstErr.Load() != nil || ctx.Err() != nil {
					continue
				}
				if err := s.uploadFileFast(ctx, task.local, task.remote, &done, total, progressFn); err != nil {
					firstErr.CompareAndSwap(nil, err)
				}
			}
		}()
	}
	for _, f := range files {
		taskCh <- f
	}
	close(taskCh)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := firstErr.Load(); err != nil {
		return err.(error)
	}
	if progressFn != nil {
		progressFn(total, total)
	}
	return nil
}

// uploadFileFast uploads one file using ReadFrom + concurrent writes.
func (s *SFTPClient) uploadFileFast(ctx context.Context, localPath, remotePath string, done *int64, total int64, progressFn func(done, total int64)) error {
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", localPath, err)
	}
	defer src.Close()

	fi, err := src.Stat()
	if err != nil {
		return err
	}

	dst, err := s.client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}
	defer dst.Close()

	pr := &progressFileReader{ctx: ctx, src: src, total: fi.Size(), done: done, progress: progressFn}
	_, err = dst.ReadFrom(pr)
	return err
}

// DownloadDir recursively downloads a remote directory to a local path.
func (s *SFTPClient) DownloadDir(ctx context.Context, remotePath, localPath string, progressFn func(done, total int64)) error {
	remotePath = strings.TrimRight(remotePath, "/")

	var total int64
	w := s.client.Walk(remotePath)
	for w.Step() {
		if w.Err() != nil {
			continue
		}
		if !w.Stat().IsDir() {
			total += w.Stat().Size()
		}
	}

	if err := os.MkdirAll(localPath, 0o755); err != nil {
		return fmt.Errorf("mkdir local root: %w", err)
	}

	var done int64
	w = s.client.Walk(remotePath)
	for w.Step() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if w.Err() != nil {
			continue
		}
		rel := strings.TrimPrefix(w.Path(), remotePath)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			continue
		}

		localFull := filepath.Join(localPath, filepath.FromSlash(rel))

		if w.Stat().IsDir() {
			if err := os.MkdirAll(localFull, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := s.downloadFileAccum(ctx, w.Path(), localFull, &done, total, progressFn); err != nil {
			return err
		}
	}
	return nil
}

// uploadFileAccum uploads one file, accumulating bytes into *done.
func (s *SFTPClient) uploadFileAccum(ctx context.Context, localPath, remotePath string, done *int64, total int64, progressFn func(done, total int64)) error {
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", localPath, err)
	}
	defer src.Close()

	if err := s.client.MkdirAll(filepath.ToSlash(filepath.Dir(remotePath))); err != nil {
		return err
	}
	dst, err := s.client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}
	defer dst.Close()

	return copyAccum(ctx, dst, src, done, total, progressFn)
}

// downloadFileAccum downloads one remote file, accumulating bytes into *done.
func (s *SFTPClient) downloadFileAccum(ctx context.Context, remotePath, localPath string, done *int64, total int64, progressFn func(done, total int64)) error {
	src, err := s.client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remotePath, err)
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	dst, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local %s: %w", localPath, err)
	}
	defer dst.Close()

	return copyAccum(ctx, dst, src, done, total, progressFn)
}

// copyAccum copies src → dst, adding transferred bytes to *done and calling progressFn.
func copyAccum(ctx context.Context, dst io.Writer, src io.Reader, done *int64, total int64, progressFn func(done, total int64)) error {
	const chunkSize = 256 * 1024
	buf := make([]byte, chunkSize)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return werr
			}
			*done += int64(n)
			if progressFn != nil {
				progressFn(*done, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}
