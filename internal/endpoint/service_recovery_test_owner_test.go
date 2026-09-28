//go:build linux

package endpoint

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/endpoint/service"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// serviceRecoveryTestOwner gives every recovery-test worker and transport
// one cancellation, close, join and residue-check owner, including t.Fatal
// paths that leave the main test before the normal protocol shutdown.
type serviceRecoveryTestOwner struct {
	t           *testing.T
	cancel      context.CancelFunc
	mu          sync.Mutex
	connections []net.Conn
	streams     []*service.Stream
	contexts    []*dutyContext
	workers     sync.WaitGroup
}

func newServiceRecoveryTestOwner(t *testing.T, cancel context.CancelFunc, connections ...net.Conn) *serviceRecoveryTestOwner {
	return &serviceRecoveryTestOwner{t: t, cancel: cancel, connections: append([]net.Conn(nil), connections...)}
}

func (owner *serviceRecoveryTestOwner) retainConnection(connection net.Conn) {
	if connection == nil {
		return
	}
	owner.mu.Lock()
	owner.connections = append(owner.connections, connection)
	owner.mu.Unlock()
}

func (owner *serviceRecoveryTestOwner) retainStream(stream *service.Stream) {
	if stream == nil {
		return
	}
	owner.mu.Lock()
	owner.streams = append(owner.streams, stream)
	owner.mu.Unlock()
}

func (owner *serviceRecoveryTestOwner) retainContext(text *dutyContext) {
	if text == nil {
		return
	}
	owner.mu.Lock()
	owner.contexts = append(owner.contexts, text)
	owner.mu.Unlock()
}

func (owner *serviceRecoveryTestOwner) Go(run func()) {
	owner.workers.Go(run)
}

func (owner *serviceRecoveryTestOwner) Close() {
	owner.cancel()
	owner.mu.Lock()
	connections := append([]net.Conn(nil), owner.connections...)
	streams := append([]*service.Stream(nil), owner.streams...)
	contexts := append([]*dutyContext(nil), owner.contexts...)
	owner.mu.Unlock()
	for _, connection := range connections {
		if err := connection.Close(); !recoveryTestCleanupOnly(err) {
			owner.t.Errorf("recovery test transport cleanup: %v", err)
		}
	}
	for _, stream := range streams {
		if err := stream.Close(); err != nil && !readCancellationOnly(err) {
			owner.t.Errorf("recovery test stream cleanup: %v", err)
		}
	}
	owner.workers.Wait()
	owner.mu.Lock()
	lateStreams := append([]*service.Stream(nil), owner.streams[len(streams):]...)
	owner.mu.Unlock()
	for _, stream := range lateStreams {
		if err := stream.Close(); err != nil && !readCancellationOnly(err) {
			owner.t.Errorf("late recovery test stream cleanup: %v", err)
		}
	}
	for _, text := range contexts {
		text.mu.Lock()
		pending := introduction.ActiveExchangeCount(&text.introduction.exchanges)
		text.mu.Unlock()
		if pending != 0 {
			owner.t.Errorf("recovery test retained %d Introduction exchanges", pending)
		}
	}
}

// Cleanup may observe only cancellation and already-closed transport leaves.
// Inspect every joined cause so that one expected close cannot hide a physical
// retirement failure carried beside it.
func recoveryTestCleanupOnly(err error) bool {
	if err == nil || err == context.Canceled || err == context.DeadlineExceeded || err == net.ErrClosed ||
		err == io.ErrClosedPipe || err == client.ErrClosedSourceStopped {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		if len(causes) > 1 {
			switch causes[0].Error() {
			case "text Service cleanup failed", "text Service transport retirement failed":
				causes = causes[1:]
			}
		}
		for _, cause := range causes {
			if !recoveryTestCleanupOnly(cause) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return recoveryTestCleanupOnly(wrapped)
	}
	return false
}
