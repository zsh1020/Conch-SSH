package sftp

import (
	"sync"

	gossh "golang.org/x/crypto/ssh"

	"github.com/pkg/sftp"
)

// SFTPClient wraps an sftp.Client and reuses an existing SSH connection.
type SFTPClient struct {
	client *sftp.Client

	// UID/GID → 名称映射，懒加载自远端 /etc/passwd、/etc/group（每连接缓存一次）
	idMu       sync.Mutex
	idLoaded   bool
	idAttempts int
	uidNames   map[uint32]string
	gidNames   map[uint32]string
}

// New opens an SFTP subsystem session on an existing SSH connection.
// Enables concurrent reads/writes so that ReadFrom/WriteTo can pipeline
// multiple SFTP packets in flight, dramatically improving throughput on
// high-latency or bandwidth-rich links.
func New(sshConn *gossh.Client) (*SFTPClient, error) {
	c, err := sftp.NewClient(sshConn,
		sftp.UseConcurrentWrites(true),
		sftp.UseConcurrentReads(true),
		sftp.MaxConcurrentRequestsPerFile(64),
	)
	if err != nil {
		return nil, err
	}
	return &SFTPClient{client: c}, nil
}

// Close closes the underlying SFTP session.
func (s *SFTPClient) Close() error {
	return s.client.Close()
}
