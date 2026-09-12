package endpoint

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev1/connection"
	"github.com/dianabuilds/ardents-network/internal/route"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

// ApplicationConnection is one authenticated, reliable, ordered byte stream
// returned by the Connection Interface. Application protocol and replay
// semantics remain entirely caller-owned.
type applicationConnection struct {
	stream    nativeconnection.Application
	cancel    context.CancelFunc
	done      chan applicationconnection.Outcome
	once      sync.Once
	finished  chan struct{}
	closeErr  error
	finishErr error
}

// openTargetRouteApplicationConnection binds Route's opaque authenticated
// Attachment to the existing Service Connection lifecycle. Endpoint supplies
// the local capability/session and no Route selection or credential input.
func (endpoint *endpoint) openTargetRouteApplicationConnection(ctx context.Context, target [32]byte, input connectionInterfaceConfig,
	clock func() time.Time, session *applicationSession, attachment *route.Attachment, evidence route.Evidence,
) (*applicationConnection, error) {
	if endpoint == nil || ctx == nil || session == nil || attachment == nil || target == [32]byte{} ||
		evidence.AuthenticatedTarget != target || evidence.AuthorityPublic == [32]byte{} ||
		len(evidence.Publication) == 0 || evidence.AttachmentID == [32]byte{} || input.Principal == [32]byte{} || input.BytesEachDirection == 0 {
		if attachment != nil {
			_ = attachment.Close()
		}
		session.Release()
		return nil, errors.New("application Connection Route evidence is incomplete")
	}
	at := clock().UTC()
	if at.IsZero() {
		_ = attachment.Close()
		session.Release()
		return nil, errors.New("application Connection clock is unavailable")
	}
	owned, application := newApplicationHalfClosePair()
	lifetime, cancel := context.WithCancel(ctx)
	ready := make(chan struct{})
	done := make(chan applicationconnection.Outcome, 1)
	finished := make(chan struct{})
	request := connectionInput{Principal: input.Principal, Target: evidence.AuthenticatedTarget, AuthorityPublic: evidence.AuthorityPublic,
		Publication: evidence.Publication, Route: attachment, Application: owned, BytesEachDirection: input.BytesEachDirection, At: at,
		OnAuthenticated: func(authenticated [32]byte) error {
			if authenticated != evidence.AuthenticatedTarget {
				return errors.New("application Connection authenticated a different Target")
			}
			close(ready)
			return nil
		}}
	connection := &applicationConnection{stream: application, cancel: cancel, done: done, finished: finished}
	go func() {
		defer close(finished)
		defer session.Release()
		result, runErr := endpoint.connectAuthorized(lifetime, request, session.receipt)
		closeErr := errors.Join(owned.Close(), attachment.Close())
		connection.finishErr = closeErr
		if result.Class == "" {
			result.Class = "indeterminate failure"
		}
		if result.Reason == "" && runErr != nil {
			result.Reason = "Application Connection ended without a classified reason"
		}
		if closeErr != nil && runErr == nil {
			result.Class, result.Reason = "indeterminate failure", "Application Connection cleanup failed"
		}
		done <- applicationconnection.Outcome{Class: applicationconnection.OutcomeClass(result.Class), Reason: result.Reason}
		close(done)
	}()
	select {
	case <-ready:
		return connection, nil
	case outcome := <-done:
		_ = connection.Close()
		return nil, errors.New(string(outcome.Class) + ": " + outcome.Reason)
	case <-ctx.Done():
		_ = connection.Close()
		return nil, ctx.Err()
	}
}

func (connection *applicationConnection) Read(destination []byte) (int, error) {
	if connection == nil || connection.stream == nil {
		return 0, io.ErrClosedPipe
	}
	return connection.stream.Read(destination)
}

func (connection *applicationConnection) Write(source []byte) (int, error) {
	if connection == nil || connection.stream == nil {
		return 0, io.ErrClosedPipe
	}
	return connection.stream.Write(source)
}

// CloseInput completes only the local Application-to-Service direction. Read
// remains available for the authenticated Service response.
func (connection *applicationConnection) CloseInput() error {
	if connection == nil || connection.stream == nil {
		return io.ErrClosedPipe
	}
	return connection.stream.CloseInput()
}

// Done carries exactly one terminal outcome and then closes.
func (connection *applicationConnection) Done() <-chan applicationconnection.Outcome {
	if connection == nil {
		return nil
	}
	return connection.done
}

// Close stops this Application stream and joins its Endpoint operation and
// local lease release. Repeated calls retain any cleanup failure. It cannot
// withdraw a Service or mutate Network, Entry, or custody state.
func (connection *applicationConnection) Close() error {
	if connection == nil {
		return nil
	}
	connection.once.Do(func() {
		connection.cancel()
		connection.closeErr = connection.stream.Close()
		<-connection.finished
		connection.closeErr = errors.Join(connection.closeErr, connection.finishErr)
	})
	return connection.closeErr
}
