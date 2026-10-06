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

// Half-close completes an orderly exchange without returning physical ownership.
// Preserve this optional capability only when the underlying connection has it.
type retainedHalfClose struct {
	*retainedConn
	writer interface{ CloseWrite() error }
}

func (c *retainedHalfClose) CloseWrite() error { return c.writer.CloseWrite() }

// Retain owns one physical interruption result across independent cancellation
// and joined borrower cleanup. It neither authenticates nor changes a principal.
func Retain(conn net.Conn) net.Conn {
	if conn == nil {
		return nil
	}
	if _, ok := conn.(*retainedConn); ok {
		return conn
	}
	if _, ok := conn.(*retainedHalfClose); ok {
		return conn
	}
	retained := &retainedConn{Conn: conn}
	if writer, ok := conn.(interface{ CloseWrite() error }); ok {
		return &retainedHalfClose{retainedConn: retained, writer: writer}
	}
	return retained
}
