//go:build linux

package endpoint

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// textServiceStream is the Endpoint's actual authenticated byte stream for a
// text worker. The Route/capsule producer supplies its owned transport only
// after its separate admission checks; this layer cannot establish reachability.
type textServiceStream struct {
	*applicationHalfClose
	binding   *textServiceBinding
	cancel    context.CancelFunc
	done      chan applicationconnection.Outcome
	finished  chan struct{}
	once      sync.Once
	closeErr  error
	finishErr error
	runErr    error // Internal terminal cause, read only after finished closes.
}

// textServiceTransport gives TLS, cancellation and final cleanup one physical
// retirement. A close error remains observable after TLS has already closed it.
type textServiceTransport struct {
	net.Conn
	once sync.Once
	err  error
}

func (transport *textServiceTransport) Close() error {
	transport.once.Do(func() {
		transport.err = transport.Conn.Close()
		if transport.err != nil {
			transport.err = errors.Join(errors.New("text Service transport retirement failed"), transport.err)
		}
	})
	return transport.err
}

// openTextServiceStream binds the initial joined Route transport to a real TLS
// and generation-3 native Service Connection. It owns raw on every return.
// Recovery Attachments require the separate retained continuity owner; this
// initial attachment never retries, changes a Target or repeats a document.
func (binding *textServiceBinding) openTextServiceStream(ctx context.Context, raw net.Conn, capsuleDigest [32]byte) (_ *textServiceStream, resultErr error) {
	if raw == nil {
		return nil, errors.New("text Service transport unavailable")
	}
	transport := &textServiceTransport{Conn: raw}
	transferred := false
	defer func() {
		if !transferred {
			resultErr = errors.Join(resultErr, transport.Close())
		}
	}()
	if ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text Service caller unavailable")
	}
	if err := binding.current(); err != nil {
		return nil, err
	}
	exporterContext, err := nativeconnection.ProtectedAttachmentContext(binding.logical, capsuleDigest, 1)
	if err != nil {
		return nil, err
	}
	lifetime, cancel := context.WithDeadline(binding.job.context, time.Unix(binding.facts.WorkSafetyNotAfter, 0))
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); cancel() })
	owned, application := newApplicationHalfClosePair()
	connection := &textServiceStream{applicationHalfClose: application, binding: binding, cancel: cancel,
		done: make(chan applicationconnection.Outcome, 1), finished: make(chan struct{})}
	var lease *publication.Lease
	interrupted := make(chan struct{})
	stopLifetime := context.AfterFunc(lifetime, func() {
		defer close(interrupted)
		_ = transport.Close()
		_ = owned.Close()
	})
	cleanup := func() error {
		cancel()
		if !stopCaller() {
			<-callerDone
		}
		if !stopLifetime() {
			<-interrupted
		}
		cleanupErr := errors.Join(owned.Close(), transport.Close())
		if lease != nil {
			cleanupErr = errors.Join(cleanupErr, lease.Close())
		}
		return cleanupErr
	}
	defer func() {
		if !transferred {
			resultErr = errors.Join(resultErr, cleanup(), application.Close())
		}
	}()
	client := binding.owner.surface == broker.Connection
	identity := nativeconnection.InstanceAuthentication{Network: binding.credential.NetworkID, Target: binding.credential.Target,
		Public: binding.credential.InstancePublic, Generation: binding.credential.Generation}
	var secured *securedAttachment
	var continuity [32]byte
	if client {
		secured, continuity, err = secureTextClient(lifetime, transport, binding.credential, exporterContext)
	} else {
		if binding.owner.endpoint.publications == nil {
			return nil, errors.New("text Publisher publication owner unavailable")
		}
		lease, err = binding.owner.endpoint.publications.AcquireAt(lifetime, binding.owner.endpoint.clock().UTC())
		if err != nil {
			return nil, err
		}
		if !binding.matchesPublication(lease.Current()) {
			return nil, errors.New("text Publisher publication changed")
		}
		identity.Signer = lease
		secured, continuity, err = secureTextPublisher(lifetime, transport, binding.credential, lease, exporterContext)
	}
	defer clear(continuity[:])
	if err != nil {
		return nil, err
	}
	// TLS exporter used the fresh Attachment context. Native records must
	// continue to bind the immutable logical context shared by both Endpoints.
	secured.context = binding.logical
	first, err := nativeAttachment(secured)
	if err != nil {
		return nil, err
	}
	stream, err := nativeconnection.NewAuthenticatedStream(nativeconnection.StreamConfig{
		Context: lifetime, Application: owned, NetworkID: binding.facts.Network, Initial: first,
		ContinuityKey: continuity, Authorized: binding.owner.endpoint.clock().UTC(), Client: client,
		Recovery: nativeconnection.Recovery{WorkSafetyNotAfter: binding.facts.WorkSafetyNotAfter,
			WorkSafetyMaximum: binding.facts.WorkSafetyMaximum, NoNewRecoveryAfter: binding.facts.NoNewRecoveryAfter},
		Resources: binding.owner.endpoint.resources}, identity)
	if err != nil {
		return nil, err
	}
	admissionErr := errors.Join(ctx.Err(), lifetime.Err(), binding.current())
	if admissionErr != nil {
		// The native lifecycle still owns its initial secret/receipt. Run it
		// under cancellation and join it instead of abandoning the owner.
		cancel()
	}
	send, receive := uint32(512), uint32(textdocument.MaximumBytes+13)
	if !client {
		send, receive = receive, send
	}
	transferred = true
	go func() {
		defer close(connection.finished)
		_, runErr := stream.RunBounded(send, receive)
		runErr = errors.Join(runErr, ctx.Err(), lifetime.Err(), binding.current())
		connection.runErr = runErr
		connection.finishErr = cleanup()
		if connection.finishErr != nil {
			connection.finishErr = errors.Join(errors.New("text Service cleanup failed"), connection.finishErr)
		}
		outcome := applicationconnection.Outcome{Class: applicationconnection.CleanClose}
		if runErr != nil || connection.finishErr != nil {
			outcome = applicationconnection.Outcome{Class: applicationconnection.ServiceUnavailable, Reason: "text Service Connection interrupted"}
			if errors.Is(runErr, context.DeadlineExceeded) {
				outcome.Class = applicationconnection.LocalTimeout
			} else if ctx.Err() != nil {
				outcome.Class = applicationconnection.LocalCancellation
			}
		}
		connection.done <- outcome
		close(connection.done)
	}()
	if admissionErr != nil {
		return nil, errors.Join(admissionErr, connection.Close())
	}
	return connection, nil
}

func (connection *textServiceStream) Done() <-chan applicationconnection.Outcome {
	return connection.done
}

func (connection *textServiceStream) Close() error {
	if connection == nil {
		return nil
	}
	connection.once.Do(func() {
		connection.cancel()
		connection.closeErr = connection.applicationHalfClose.Close()
		<-connection.finished
		connection.closeErr = errors.Join(connection.closeErr, connection.finishErr)
	})
	return connection.closeErr
}
