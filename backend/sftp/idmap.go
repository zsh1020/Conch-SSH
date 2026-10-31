package sftp

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	gosftppkg "github.com/pkg/sftp"
)

// maxIDFileSize caps how much of /etc/passwd or /etc/group we read, guarding
// against pathological files on hosts with huge local user databases.
const maxIDFileSize = 8 << 20 // 8 MiB

// maxIDLoadAttempts caps how many times we retry reading the id files before
// giving up for the connection — enough to ride out a transient failure, few
// enough not to re-read on every listing for hosts that simply have no
// /etc/passwd (Windows, restricted shells, …).
const maxIDLoadAttempts = 3

// ensureIDMaps lazily loads UID→name and GID→name maps from the remote
// /etc/passwd and /etc/group (read over SFTP). It locks in the result once at
// least one map loads successfully; a transient read failure (empty maps) is
// retried on the next listing, up to maxIDLoadAttempts. Callers fall back to
// numeric ids whenever a name is unavailable.
func (s *SFTPClient) ensureIDMaps() {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	if s.idLoaded {
		return
	}
	s.uidNames = parseIDFile(s.readRemoteSmall("/etc/passwd"))
	s.gidNames = parseIDFile(s.readRemoteSmall("/etc/group"))
	s.idAttempts++
	// 读到任一映射即成功锁定（真实主机 /etc/passwd 至少含 root:0，非空即读取成功）；
	// 全空说明读取失败，保持 idLoaded=false 下次重试，超过上限才放弃。
	if len(s.uidNames) > 0 || len(s.gidNames) > 0 || s.idAttempts >= maxIDLoadAttempts {
		s.idLoaded = true
	}
}

// readRemoteSmall reads up to maxIDFileSize bytes of a remote text file.
// Returns "" on any error.
func (s *SFTPClient) readRemoteSmall(path string) string {
	f, err := s.client.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxIDFileSize))
	if err != nil {
		return ""
	}
	return string(data)
}

// parseIDFile parses colon-separated /etc/passwd or /etc/group content and
// returns an id→name map. Both formats share the layout `name:x:id:...`, so
// the same parser handles both.
func parseIDFile(content string) map[uint32]string {
	m := make(map[uint32]string)
	if content == "" {
		return m
	}
	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) < 3 {
			continue
		}
		id, err := strconv.ParseUint(strings.TrimSpace(fields[2]), 10, 32)
		if err != nil {
			continue
		}
		name := fields[0]
		if name == "" {
			continue
		}
		// First entry wins (matches getent behaviour for duplicate ids).
		if _, exists := m[uint32(id)]; !exists {
			m[uint32(id)] = name
		}
	}
	return m
}

// resolveOwnerGroup returns owner/group names for a remote entry, resolving
// numeric UID/GID via the cached id maps and falling back to the number when
// a name is unavailable.
func (s *SFTPClient) resolveOwnerGroup(fi interface{ Sys() interface{} }) (owner, group string) {
	stat, ok := fi.Sys().(*gosftppkg.FileStat)
	if !ok {
		return "", ""
	}
	s.ensureIDMaps()

	if name, ok := s.uidNames[stat.UID]; ok {
		owner = name
	} else {
		owner = strconv.FormatUint(uint64(stat.UID), 10)
	}
	if name, ok := s.gidNames[stat.GID]; ok {
		group = name
	} else {
		group = strconv.FormatUint(uint64(stat.GID), 10)
	}
	return owner, group
}
