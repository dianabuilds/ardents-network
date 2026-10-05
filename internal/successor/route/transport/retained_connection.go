package transport

import (
	"crypto/tls"
	"net"
	"sync"
)

// retainedConn retains one physical close result across independent cancellation
// and borrower cleanup. A later benign ErrClosed cannot mask the first failure.
type retainedConn struct {
	net.Conn
	once sync.Once
	err  error
}

func (c *retainedConn) Close() error {
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

func (c *retainedConn) NetConn() net.Conn { return c.Conn }

// Retain owns one physical interruption result across independent cancellation
// and joined borrower cleanup. It neither authenticates nor changes a principal.
func Retain(conn net.Conn) net.Conn {
	if conn == nil {
		return nil
	}
	if _, ok := conn.(*retainedConn); ok {
		return conn
	}
	return &retainedConn{Conn: conn}
}
