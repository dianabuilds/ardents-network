package source

import (
	"context"
	"net"
	"sync"
)

// One original socket has one physical Close, whether cancellation or normal
// retirement gets there first. Copies of its cleanup result retain late native
// failure; joining the cancellation callback still belongs to its caller.
type sourceSocket struct {
	net.Conn
	once   sync.Once
	result error
}

// Peer-controlled exchange failures end only that exchange. This private marker
// lets the serving owner retain physical cleanup failure through its own join.
type sourcePhysicalClose struct{ cause error }

func (e *sourcePhysicalClose) Error() string { return "source physical close failed" }
func (e *sourcePhysicalClose) Unwrap() error { return e.cause }

func sourceCloseResult(err error) error {
	if err == nil {
		return nil
	}
	return &sourcePhysicalClose{cause: err}
}

func (s *sourceSocket) Close() error {
	s.once.Do(func() { s.result = s.Conn.Close() })
	return s.result
}

// sourceConnectionCancellation retains the exact original socket until its
// cancellation callback has physically returned. Each caller defers the one
// returned retirement before releasing its connection/key/user ownership.
func sourceConnectionCancellation(ctx context.Context, connection net.Conn) func() {
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = connection.Close()
		close(interrupted)
	})
	return func() {
		if !stop() {
			<-interrupted
		}
	}
}
