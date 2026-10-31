package tunnel

import (
	"context"
	"net"

	gossh "golang.org/x/crypto/ssh"
)

// startRemote asks the SSH server to bind remoteAddr and forwards each
// accepted connection to localAddr (-R remoteAddr:localAddr).
func startRemote(ctx context.Context, t *Tunnel, sshConn *gossh.Client, localAddr, remoteAddr string) error {
	ln, err := sshConn.Listen("tcp", remoteAddr)
	if err != nil {
		return friendlyBindErr(err, remoteAddr, "服务器")
	}

	go func() {
		defer ln.Close()
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
				local, err := net.Dial("tcp", localAddr)
				if err != nil {
					return
				}
				defer local.Close()
				pipe(t, c, local)
			}(conn)
		}
	}()

	return nil
}
