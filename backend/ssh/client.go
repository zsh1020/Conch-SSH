package ssh

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"conch/backend/auth"
	"conch/backend/knownhosts"
	"conch/backend/store"

	gossh "golang.org/x/crypto/ssh"
)

// ConnectOptions holds runtime connection parameters.
type ConnectOptions struct {
	Session   store.Session
	Password  string // collected from UI
	Confirm   knownhosts.ConfirmFunc
	OnChanged knownhosts.HostKeyChangedFunc
}

// Client wraps an established SSH connection.
type Client struct {
	id     string
	client *gossh.Client
	conn   net.Conn
	cancel context.CancelFunc
}

func (c *Client) ID() string                    { return c.id }
func (c *Client) Underlying() *gossh.Client     { return c.client }

func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	return c.client.Close()
}

// Dial establishes an SSH connection, optionally through a ProxyJump bastion.
func Dial(ctx context.Context, opts ConnectOptions) (*Client, error) {
	sess := opts.Session

	if sess.ProxyJump != "" {
		return dialViaJump(ctx, opts)
	}
	return dialDirect(ctx, opts, nil)
}

// dialDirect opens a TCP connection (or reuses netConn) and performs SSH handshake.
func dialDirect(ctx context.Context, opts ConnectOptions, netConn net.Conn) (*Client, error) {
	sess := opts.Session

	authMethods, err := auth.BuildMethods(sess.AuthType, sess.IdentityFile, opts.Password)
	if err != nil {
		return nil, fmt.Errorf("build auth methods: %w", err)
	}

	km, err := knownhosts.New(opts.Confirm, opts.OnChanged)
	if err != nil {
		return nil, fmt.Errorf("known hosts: %w", err)
	}

	port := sess.Port
	if port == 0 {
		port = 22
	}
	addr := fmt.Sprintf("%s:%d", sess.Host, port)

	cfg := &gossh.ClientConfig{
		User:            sess.Username,
		Auth:            authMethods,
		HostKeyCallback: km.Callback(),
		BannerCallback:  gossh.BannerDisplayStderr(),
		Timeout:         15 * time.Second,
	}

	if netConn == nil {
		dialer := net.Dialer{}
		netConn, err = dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("tcp dial %s: %w", addr, err)
		}
	}

	sshConn, chans, reqs, err := gossh.NewClientConn(netConn, addr, cfg)
	if err != nil {
		netConn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	sshClient := gossh.NewClient(sshConn, chans, reqs)

	go startKeepalive(childCtx, sshClient, 30*time.Second)

	return &Client{
		client: sshClient,
		conn:   netConn,
		cancel: cancel,
	}, nil
}

// dialViaJump connects through a ProxyJump bastion host.
// ProxyJump format: [user@]host[:port]
func dialViaJump(ctx context.Context, opts ConnectOptions) (*Client, error) {
	jump := opts.Session.ProxyJump

	// Parse bastion address
	bastionUser, bastionHost, bastionPort := parseJumpSpec(jump)

	bastionSess := store.Session{
		Host:     bastionHost,
		Port:     bastionPort,
		Username: bastionUser,
		// Reuse same auth for bastion; caller may override if needed
		AuthType:     opts.Session.AuthType,
		IdentityFile: opts.Session.IdentityFile,
	}
	bastionOpts := ConnectOptions{
		Session:   bastionSess,
		Password:  opts.Password,
		Confirm:   opts.Confirm,
		OnChanged: opts.OnChanged,
	}

	bastion, err := dialDirect(ctx, bastionOpts, nil)
	if err != nil {
		return nil, fmt.Errorf("proxyjump bastion %s: %w", jump, err)
	}

	targetPort := opts.Session.Port
	if targetPort == 0 {
		targetPort = 22
	}
	targetAddr := fmt.Sprintf("%s:%d", opts.Session.Host, targetPort)

	// Open a TCP channel through the bastion to the target
	tunnelConn, err := bastion.client.Dial("tcp", targetAddr)
	if err != nil {
		bastion.Close()
		return nil, fmt.Errorf("proxyjump dial target %s: %w", targetAddr, err)
	}

	// SSH handshake on the tunnelled connection
	targetClient, err := dialDirect(ctx, opts, tunnelConn)
	if err != nil {
		tunnelConn.Close()
		bastion.Close()
		return nil, err
	}

	// Ensure bastion is closed when target is closed
	origCancel := targetClient.cancel
	targetClient.cancel = func() {
		origCancel()
		bastion.Close()
	}

	return targetClient, nil
}

// parseJumpSpec parses "[user@]host[:port]" into components.
func parseJumpSpec(spec string) (user, host string, port int) {
	port = 22
	user = ""

	if idx := strings.LastIndex(spec, "@"); idx >= 0 {
		user = spec[:idx]
		spec = spec[idx+1:]
	}

	host, portStr, err := net.SplitHostPort(spec)
	if err != nil {
		// No port
		host = spec
		return
	}
	fmt.Sscanf(portStr, "%d", &port)
	return
}

func startKeepalive(ctx context.Context, c *gossh.Client, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, _, err := c.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return
			}
		}
	}
}
