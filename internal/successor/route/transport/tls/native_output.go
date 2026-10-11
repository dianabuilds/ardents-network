package tls

import (
	"net"
	"sync"
)

// nativeSocket owns admission to the original socket's writes independently
// of TLS's serialized record writer. Close does not grant a later record I/O.
type nativeSocket struct {
	net.Conn
	mu               sync.Mutex
	closed           bool
	attempts, active uint64
	failed           bool
	once             sync.Once
	closeErr         error
}

type socketObservation struct {
	attempts, active uint64
	closed, failed   bool
}

func (socket *nativeSocket) observe() socketObservation {
	socket.mu.Lock()
	defer socket.mu.Unlock()
	return socketObservation{socket.attempts, socket.active, socket.closed, socket.failed}
}

// Socket refusal is private: only the complete TLS write can establish that no
// preceding record from that write reached the socket.
type socketWriteRefusal struct{}

func (*socketWriteRefusal) Error() string   { return net.ErrClosed.Error() }
func (*socketWriteRefusal) Unwrap() error   { return net.ErrClosed }
func (*socketWriteRefusal) Timeout() bool   { return false }
func (*socketWriteRefusal) Temporary() bool { return false }

func (socket *nativeSocket) Write(value []byte) (int, error) {
	socket.mu.Lock()
	if socket.closed {
		socket.mu.Unlock()
		return 0, &socketWriteRefusal{}
	}
	socket.attempts++
	socket.active++
	socket.mu.Unlock()
	n, err := socket.Conn.Write(value)
	socket.mu.Lock()
	socket.active--
	socket.failed = socket.failed || err != nil
	socket.mu.Unlock()
	return n, err
}

func (socket *nativeSocket) Close() error {
	socket.once.Do(func() {
		socket.mu.Lock()
		socket.closed = true
		socket.mu.Unlock()
		// A started writer must be interrupted, never joined under this lock.
		socket.closeErr = socket.Conn.Close()
	})
	return socket.closeErr
}
