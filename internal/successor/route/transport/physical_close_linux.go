//go:build linux

package transport

import (
	"crypto/tls"
	"net"
	"sync"
)

// retiredConn retains one physical close result across independent cancellation
// and borrower cleanup. A later benign ErrClosed cannot mask the first failure.
type retiredConn struct {
	net.Conn
	once sync.Once
	err  error
}

func (c *retiredConn) Close() error {
	c.once.Do(func() {
		// Revocation interrupts physical I/O; closeNotify would be a new write.
		if secured, ok := c.Conn.(*tls.Conn); ok {
			c.err = secured.NetConn().Close()
		} else {
			c.err = c.Conn.Close()
		}
	})
	return c.err
}
func (r *Receiver) closeListener() error {
	r.listenerOnce.Do(func() { r.listenerErr = r.listener.Close() })
	return r.listenerErr
}
