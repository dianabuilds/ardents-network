//go:build linux

package connection

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// setupPeer is the deadline/read seam used during attachment admission.
type setupPeer interface {
	Read([]byte) (int, error)
	SetReadDeadline(time.Time) error
	Close() error
}

// openAuthorizedAttachment ties setup cancellation to the local peer. An early
// byte or disconnect refuses setup before accepting Application input.
func (server *server) openAuthorizedAttachment(local setupPeer, request Request) (Stream, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(server.ctx, 15*time.Second)
	var mu sync.Mutex
	finished := false
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		var unexpected [1]byte
		n, err := local.Read(unexpected[:])
		mu.Lock()
		ready := finished
		mu.Unlock()
		var networkErr net.Error
		if n != 0 || !ready || !errors.As(err, &networkErr) || !networkErr.Timeout() {
			cancel()
		}
	}()
	stream, openErr := server.owner.Open(ctx, request)
	mu.Lock()
	finished = true
	mu.Unlock()
	if err := local.SetReadDeadline(time.Now()); err != nil {
		cancel()
		_ = local.Close()
	}
	<-monitorDone
	if openErr != nil || stream == nil || ctx.Err() != nil {
		contextErr := ctx.Err()
		cancel()
		setupErr := errors.Join(openErr, contextErr, errors.New("local Application setup did not complete"))
		return nil, nil, server.closeRefusedAttachment(stream, setupErr)
	}
	if err := local.SetReadDeadline(time.Time{}); err != nil {
		cancel()
		return nil, nil, server.closeRefusedAttachment(stream, err)
	}
	return stream, cancel, nil
}

// setupCleanupError keeps the complete local cause while freezing the wire
// classification before raw cleanup errors are joined.
type setupCleanupError struct {
	cause   error
	outcome Outcome
}

func (failure setupCleanupError) Error() string { return failure.cause.Error() }
func (failure setupCleanupError) Unwrap() error { return failure.cause }

func (server *server) closeRefusedAttachment(stream Stream, setupErr error) error {
	if stream == nil {
		return setupErr
	}
	cleanupErr := stream.Close()
	server.retainCleanupFailure(cleanupErr)
	if cleanupErr == nil {
		return setupErr
	}
	return setupCleanupError{cause: errors.Join(setupErr, cleanupErr), outcome: refusal(setupErr)}
}
