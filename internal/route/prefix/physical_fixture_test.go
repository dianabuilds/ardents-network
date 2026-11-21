package prefix

import (
	"bytes"
	"net"
	"sync"
	"time"
)

// lifecycleConn gates actual physical output and completion independently.
// It supplies no successful authority, Admission, or resource-transfer result.
type lifecycleConn struct {
	mu                    sync.Mutex
	closed                chan struct{}
	closeOnce             sync.Once
	writeGate             chan struct{}
	writes                chan []byte
	output                []byte
	writeEnd              time.Time
	partial, closeFailure error
	input                 *bytes.Reader
	readGate              chan struct{}
	writeIgnoresClose     bool
}

func newLifecycleConn(gated bool) *lifecycleConn {
	c := &lifecycleConn{closed: make(chan struct{}), writeGate: make(chan struct{}), writes: make(chan []byte, 32)}
	if !gated {
		close(c.writeGate)
	}
	return c
}
func (c *lifecycleConn) Read(p []byte) (int, error) {
	if c.input != nil && c.input.Len() > 0 {
		return c.input.Read(p)
	}
	if c.readGate != nil {
		<-c.readGate
	}
	<-c.closed
	return 0, net.ErrClosed
}
func (c *lifecycleConn) Write(p []byte) (int, error) {
	c.writes <- append([]byte(nil), p...)
	if c.writeIgnoresClose {
		<-c.writeGate
	} else {
		select {
		case <-c.writeGate:
		case <-c.closed:
			return 0, net.ErrClosed
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.partial != nil {
		c.output = append(c.output, p[:3]...)
		return 3, c.partial
	}
	c.output = append(c.output, p...)
	return len(p), nil
}
func (c *lifecycleConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.closeFailure
}
func (c *lifecycleConn) LocalAddr() net.Addr             { return &net.TCPAddr{} }
func (c *lifecycleConn) RemoteAddr() net.Addr            { return &net.TCPAddr{} }
func (c *lifecycleConn) SetDeadline(t time.Time) error   { return c.SetWriteDeadline(t) }
func (c *lifecycleConn) SetReadDeadline(time.Time) error { return nil }
func (c *lifecycleConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	c.writeEnd = t
	c.mu.Unlock()
	return nil
}
