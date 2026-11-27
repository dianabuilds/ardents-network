//go:build linux

package server

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

// setupPeer is the deadline/read seam used during attachment admission.
type setupPeer interface {
	Read([]byte) (int, error)
	SetReadDeadline(time.Time) error
	Close() error
}

// openAuthorizedAttachment ties setup cancellation to the local peer. An early
// byte or disconnect refuses setup before accepting Application input.
func (server *server) openAuthorizedAttachment(local setupPeer, request endpointstream.Request, deadline time.Time) (endpointstream.Stream, context.CancelFunc, error) {
	ctx, cancel := context.WithDeadline(server.ctx, deadline)
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
	if err := local.SetReadDeadline(deadline); err != nil {
		cancel()
		return nil, nil, server.closeRefusedAttachment(stream, err)
	}
	return stream, cancel, nil
}

// setupCleanupError keeps the complete local cause while freezing the wire
// classification before raw cleanup errors are joined.
type setupCleanupError struct {
	cause   error
	outcome endpointstream.Outcome
}

func (failure setupCleanupError) Error() string { return failure.cause.Error() }
func (failure setupCleanupError) Unwrap() error { return failure.cause }

func (server *server) closeRefusedAttachment(stream endpointstream.Stream, setupErr error) error {
	if stream == nil {
		return setupErr
	}
	cleanupErr := stream.Close()
	server.retainCleanupFailure(cleanupErr)
	if cleanupErr == nil {
		return setupErr
	}
	return setupCleanupError{cause: errors.Join(setupErr, cleanupErr), outcome: classifySetupRefusal(setupErr)}
}

// classifySetupRefusal freezes setup classification before cleanup errors join.
func classifySetupRefusal(cause error) endpointstream.Outcome {
	var cleanup setupCleanupError
	if errors.As(cause, &cleanup) {
		return cleanup.outcome
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return endpointstream.Outcome{Class: endpointstream.LocalTimeout, Reason: "local Application setup timed out"}
	}
	if errors.Is(cause, context.Canceled) {
		return endpointstream.Outcome{Class: endpointstream.LocalCancellation, Reason: "local Application setup was cancelled"}
	}
	var classified endpointstream.SetupRefusalError
	if errors.As(cause, &classified) {
		return classified.Outcome()
	}
	return endpointstream.Outcome{Class: endpointstream.ServiceUnavailable, Reason: "Endpoint could not open the selected Target Link"}
}
