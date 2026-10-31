package tunnel

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	gossh "golang.org/x/crypto/ssh"
)

// startDynamic binds a local SOCKS5 proxy that forwards connections through
// the SSH client (-D localAddr).
func startDynamic(ctx context.Context, t *Tunnel, sshConn *gossh.Client, localAddr string) error {
	ln, err := net.Listen("tcp", localAddr)
	if err != nil {
		return friendlyBindErr(err, localAddr, "本地")
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
				handleSocks5(t, c, sshConn)
			}(conn)
		}
	}()

	return nil
}

// handleSocks5 performs the SOCKS5 handshake then proxies through sshConn.
func handleSocks5(t *Tunnel, c net.Conn, sshConn *gossh.Client) {
	// ── Greeting ──────────────────────────────────────────────
	// VER(1) NMETHODS(1) METHODS(n)
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(c, hdr); err != nil {
		return
	}
	if hdr[0] != 0x05 {
		return
	}
	methods := make([]byte, hdr[1])
	if _, err := io.ReadFull(c, methods); err != nil {
		return
	}
	// Accept no-auth (0x00)
	c.Write([]byte{0x05, 0x00}) //nolint:errcheck

	// ── Request ───────────────────────────────────────────────
	// VER(1) CMD(1) RSV(1) ATYP(1) ...
	req := make([]byte, 4)
	if _, err := io.ReadFull(c, req); err != nil {
		return
	}
	if req[0] != 0x05 || req[1] != 0x01 { // only CONNECT
		c.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) //nolint:errcheck
		return
	}

	var target string
	switch req[3] {
	case 0x01: // IPv4
		addr := make([]byte, 4)
		if _, err := io.ReadFull(c, addr); err != nil {
			return
		}
		target = net.IP(addr).String()
	case 0x03: // Domain
		lenB := make([]byte, 1)
		if _, err := io.ReadFull(c, lenB); err != nil {
			return
		}
		domain := make([]byte, lenB[0])
		if _, err := io.ReadFull(c, domain); err != nil {
			return
		}
		target = string(domain)
	case 0x04: // IPv6
		addr := make([]byte, 16)
		if _, err := io.ReadFull(c, addr); err != nil {
			return
		}
		target = net.IP(addr).String()
	default:
		c.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) //nolint:errcheck
		return
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(c, portBytes); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(portBytes)
	addr := fmt.Sprintf("%s:%d", target, port)

	// ── Dial via SSH ──────────────────────────────────────────
	remote, err := sshConn.Dial("tcp", addr)
	if err != nil {
		c.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) //nolint:errcheck
		return
	}
	defer remote.Close()

	// Success reply
	c.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) //nolint:errcheck

	pipe(t, c, remote)
}
