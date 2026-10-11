package transport

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"sync/atomic"
)

// retainedConn retains one physical close result across independent cancellation
// and borrower cleanup. A later benign ErrClosed cannot mask the first failure.
type retainedConn struct {
	net.Conn
	once     sync.Once
	err      error
	closed   atomic.Bool
	peerRead atomic.Bool
}

// Read preserves the exact TLS result. Only an original, completed TLS channel
// may retain a peer EOF observation, and local physical Close cannot create it.
func (c *retainedConn) Read(value []byte) (int, error) {
	local := c.closed.Load()
	n, err := c.Conn.Read(value)
	if n == 0 && err == io.EOF && !local && !c.closed.Load() {
		if secured, ok := c.Conn.(*tls.Conn); ok && secured.ConnectionState().HandshakeComplete {
			c.peerRead.Store(true)
		}
	}
	return n, err
}

func (c *retainedConn) Close() error {
	c.once.Do(func() {
		c.closed.Store(true)
		// Revocation interrupts physical I/O; closeNotify would be a new write.
		if secured, ok := c.Conn.(*tls.Conn); ok {
			c.err = secured.NetConn().Close()
		} else {
			c.err = c.Conn.Close()
		}
	})
	return c.err
}

// RetainedPeerReadCause projects only an EOF actually observed on this original
// retained TLS channel before local Close. It preserves the failed cause and
// grants no completion, join or authority; a raw supplied EOF has no provenance.
func RetainedPeerReadCause(conn net.Conn, cause error) error {
	if cause != io.EOF {
		return cause
	}
	var retained *retainedConn
	switch original := conn.(type) {
	case *retainedConn:
		retained = original
	case *retainedHalfClose:
		retained = original.retainedConn
	}
	if retained != nil && retained.peerRead.Load() {
		return MarkPeerRetirement(cause)
	}
	return cause
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
