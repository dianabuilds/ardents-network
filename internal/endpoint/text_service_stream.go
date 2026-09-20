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
	binding    *textServiceBinding
	cancel     context.CancelFunc
	done       chan applicationconnection.Outcome
	retired    chan struct{}
	finished   chan struct{}
	waitClose  func(<-chan struct{}) bool
	retireTail func() error
	once       sync.Once
	closeErr   error
	finishErr  error
	runErr     error // Internal terminal cause, read only after finished closes.
}

// textServiceTransport gives TLS, cancellation and final cleanup one physical
// retirement. A close error remains observable after TLS has already closed it.
type textServiceTransport struct {
	net.Conn
	once sync.Once
	err  error
}

// textServiceAttachmentOpener returns one already authorized protected Route
// transport and its exact fresh capsule digest. The native Connection owns TLS,
// exporter and retained-continuity verification before committing it.
type textServiceAttachmentOpener func(context.Context, nativeconnection.Recovery) (net.Conn, [32]byte, error)

func (transport *textServiceTransport) Close() error {
	transport.once.Do(func() {
		transport.err = transport.Conn.Close()
		// A retirement attempt after an upstream cancellation has already torn
		// down TLS is not a separate cleanup failure. Without this guard the
		// per-stream Join cascade reproduces "text Service transport retirement
		// failed" once per stream and the workload criteria never see a quiet
		// shutdown.
		if transport.err != nil && (errors.Is(transport.err, net.ErrClosed) ||
			transport.err.Error() == "use of closed network connection") {
			transport.err = nil
		}
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
func (binding *textServiceBinding) openTextServiceStreamWithRecovery(ctx context.Context, raw net.Conn, capsuleDigest [32]byte,
	open textServiceAttachmentOpener) (_ *textServiceStream, resultErr error) {
	recoveryTransferred := false
	defer func() {
		if !recoveryTransferred {
			resultErr = errors.Join(resultErr, binding.releaseTextIntroductionRecovery())
		}
	}()
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
		done: make(chan applicationconnection.Outcome, 1), retired: make(chan struct{}),
		finished: make(chan struct{}), waitClose: waitTextServiceClose}
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
		cleanupErr := errors.Join(owned.Close(), transport.Close(), binding.releaseTextIntroductionRecovery())
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
		secured, continuity, err = secureTextClient(lifetime, transport, binding.credential, exporterContext, 1)
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
		secured, continuity, err = secureTextPublisher(lifetime, transport, binding.credential, lease, exporterContext, 1)
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
	recovery := nativeconnection.Recovery{WorkSafetyNotAfter: binding.facts.WorkSafetyNotAfter,
		WorkSafetyMaximum: binding.facts.WorkSafetyMaximum, NoNewRecoveryAfter: binding.facts.NoNewRecoveryAfter}
	if open != nil {
		recovery = binding.textServiceRecovery()
		if err := nativeconnection.ValidateRecovery(true, recovery, binding.owner.endpoint.clock().UTC().Unix(), binding.credential.NotAfter); err != nil {
			return nil, err
		}
	}
	var opener nativeconnection.AttachmentOpener
	if open != nil {
		opener = func(attempt context.Context, request nativeconnection.Recovery) (_ *nativeconnection.Attachment, outcome error) {
			if err := binding.validateTextServiceRecovery(request); err != nil {
				return nil, err
			}
			replacementRaw, replacementDigest, err := open(attempt, request)
			if err != nil {
				return nil, err
			}
			if replacementRaw == nil || replacementDigest == [32]byte{} {
				if replacementRaw != nil {
					_ = replacementRaw.Close()
				}
				return nil, errors.New("text Service recovery Attachment is incomplete")
			}
			ownedRaw := true
			defer func() {
				if ownedRaw {
					outcome = errors.Join(outcome, replacementRaw.Close())
				}
			}()
			if err := errors.Join(attempt.Err(), binding.current()); err != nil {
				return nil, err
			}
			freshContext, err := nativeconnection.ProtectedAttachmentContext(binding.logical, replacementDigest, request.Generation)
			if err != nil {
				return nil, err
			}
			var replacement *securedAttachment
			var freshContinuity [32]byte
			if client {
				replacement, freshContinuity, err = secureTextClient(attempt, replacementRaw, binding.credential, freshContext, request.Generation)
			} else {
				if lease == nil || !binding.matchesPublication(lease.Current()) {
					return nil, errors.New("text Publisher publication changed before recovery")
				}
				replacement, freshContinuity, err = secureTextPublisher(attempt, replacementRaw, binding.credential, lease, freshContext, request.Generation)
			}
			defer clear(freshContinuity[:])
			if err != nil {
				ownedRaw = false // TLS setup owns and closes raw on every failure.
				return nil, err
			}
			ownedRaw = false
			if err := errors.Join(attempt.Err(), binding.current()); err != nil || !client && !binding.matchesPublication(lease.Current()) {
				replacement.close()
				return nil, errors.Join(err, errors.New("text Service authority changed during recovery"))
			}
			// The exporter was derived from the fresh Attachment context; native
			// Continuity continues to authenticate the immutable logical context.
			replacement.context = binding.logical
			attached, err := nativeAttachment(replacement)
			if err != nil {
				replacement.close()
				return nil, err
			}
			return attached, nil
		}
	}
	stream, err := nativeconnection.NewAuthenticatedStream(nativeconnection.StreamConfig{
		Context: lifetime, Application: owned, NetworkID: binding.facts.Network, Initial: first,
		ContinuityKey: continuity, Authorized: binding.owner.endpoint.clock().UTC(), Client: client,
		Recovery: recovery, OpenAttachment: opener,
		Resources: binding.owner.endpoint.resources}, identity)
	if err != nil {
		return nil, err
	}
	connection.retireTail = stream.RetireTerminalTail
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
	if binding.job.qualification != nil {
		send, receive = 64<<20, 64<<20
	}
	transferred = true
	recoveryTransferred = true
	go func() {
		_, runErr := stream.RunBounded(send, receive)
		runErr = errors.Join(runErr, ctx.Err(), lifetime.Err(), binding.current())
		connection.runErr = runErr
		nativeFinished := false
		select {
		case <-stream.Done():
			nativeFinished = true
		default:
		}
		if nativeFinished {
			connection.finishErr = cleanup()
			if connection.finishErr != nil {
				connection.finishErr = errors.Join(errors.New("text Service cleanup failed"), connection.finishErr)
			}
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
		// RunBounded has completed the Application Terminal exchange. Its
		// recovery-capable terminal-control tail may intentionally keep
		// stream.Done open until the lifetime is canceled.
		close(connection.retired)
		if !nativeFinished {
			<-stream.Done()
			connection.finishErr = cleanup()
			if connection.finishErr != nil {
				connection.finishErr = errors.Join(errors.New("text Service cleanup failed"), connection.finishErr)
			}
		}
		close(connection.finished)
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
		connection.closeErr = connection.applicationHalfClose.CloseInput()
		// Let the native stream turn application EOF into its authenticated
		// terminal while the opposite direction remains readable. A missing peer
		// remains bounded and is interrupted by the same lifetime.
		wait := connection.waitClose
		if wait == nil {
			wait = waitTextServiceClose
		}
		retired := wait(connection.retired)
		if !retired {
			connection.cancel()
		} else if connection.retireTail != nil {
			retireErr := connection.retireTail()
			connection.closeErr = errors.Join(connection.closeErr, retireErr)
			if retireErr != nil {
				connection.cancel()
			}
		}
		<-connection.finished
		connection.cancel()
		connection.closeErr = errors.Join(connection.closeErr, connection.applicationHalfClose.Close())
		connection.closeErr = errors.Join(connection.closeErr, connection.finishErr)
	})
	return connection.closeErr
}

func waitTextServiceClose(finished <-chan struct{}) bool {
	<-finished
	return true
}
