package tunnel

import (
	"io"
	"sync/atomic"
)

// pipe copies bidirectionally between a and b until either side closes.
// bytesAB counts bytes flowing a→b; bytesBA counts b→a.
func pipe(t *Tunnel, a, b io.ReadWriteCloser) {
	done := make(chan struct{}, 2)
	cp := func(dst, src io.ReadWriteCloser, counter *int64) {
		buf := make([]byte, 32*1024)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				atomic.AddInt64(counter, int64(n))
				dst.Write(buf[:n]) //nolint:errcheck
			}
			if err != nil {
				break
			}
		}
		dst.Close()
		done <- struct{}{}
	}
	go cp(b, a, &t.bytesOut) // local→remote = outbound
	go cp(a, b, &t.bytesIn)  // remote→local = inbound
	<-done
}
