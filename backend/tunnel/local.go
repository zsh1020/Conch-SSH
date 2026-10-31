package tunnel

import (
	"context"
	"net"

	gossh "golang.org/x/crypto/ssh"
)

// startLocal binds a local TCP listener and forwards each connection through
// the SSH client to remoteAddr (-L localAddr:remoteAddr).
func startLocal(ctx context.Context, t *Tunnel, sshConn *gossh.Client, localAddr, remoteAddr string) error {
	ln, err := net.Listen("tcp", localAddr)
	if err != nil {
		return friendlyBindErr(err, localAddr, "本地")
	}

	go func() {
		defer ln.Close()
		// Close listener when context is cancelled.
		go func() {
			<-ctx.Done()
			ln.Close()
		}()

		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			t.incConn()
			go func(c net.Conn) {
				defer t.decConn()
				defer c.Close()
				remote, err := sshConn.Dial("tcp", remoteAddr)
				if err != nil {
					return
				}
				defer remote.Close()
				pipe(t, c, remote)
			}(conn)
		}
	}()

	return nil
}

