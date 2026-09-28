//go:build linux

package service

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/connection"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
)

// Stream is the Endpoint's actual authenticated byte stream for a
// text worker. The Route/capsule producer supplies its owned transport only
// after its separate admission checks; this layer cannot establish reachability.
type Stream struct {
	*applicationHalfClose
	binding    Binding
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

// OpenStream binds the initial joined Route transport to a real TLS
// and generation-3 native Service Connection. It owns raw on every return.
// Recovery Attachments require the separate retained continuity owner; this
// initial attachment never retries, changes a Target or repeats a document.
func OpenStream(binding Binding, ctx context.Context, raw net.Conn, capsuleDigest [32]byte,
	open AttachmentOpener) (_ *Stream, resultErr error) {
	recoveryTransferred := false
	defer func() {
		if !recoveryTransferred {
			resultErr = errors.Join(resultErr, binding.ReleaseIntroductionRecovery())
		}
	}()
	if raw == nil {
		return nil, errors.New("text Service transport unavailable")
	}
	transport := &protectedServiceTransport{Conn: raw}
	transferred := false
	defer func() {
		if !transferred {
			resultErr = errors.Join(resultErr, transport.Close())
		}
	}()
	if ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text Service caller unavailable")
	}
	if err := binding.Current(); err != nil {
		return nil, err
	}
	send, receive, err := binding.WorkloadDirection()
	if err != nil {
		return nil, err
	}
	facts := binding.Facts()
	exporterContext, err := nativeconnection.ProtectedAttachmentContext(binding.Logical(), capsuleDigest, 1)
	if err != nil {
		return nil, err
	}
	lifetime, cancel := context.WithDeadline(binding.JobContext(), time.Unix(facts.WorkSafetyNotAfter, 0))
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); cancel() })
	owned, application := newApplicationHalfClosePair()
	connection := &Stream{applicationHalfClose: application, binding: binding, cancel: cancel,
		done: make(chan applicationconnection.Outcome, 1), retired: make(chan struct{}),
		finished: make(chan struct{}), waitClose: waitServiceClose}
	var lease *servicepublication.Lease
	interrupted := make(chan struct{})
	stopLifetime := context.AfterFunc(lifetime, func() {
		defer close(interrupted)
		_ = transport.Close()
		_ = owned.Close()
	})
	// nativeOwned records that a native Stream took the initial Attachment and
	// with it the exactly-once physical transport retirement. Its retained
	// result then reaches Close through Stream.RetirementResult, so cleanup
	// must not join the same cached transport result a second time (F-23).
	nativeOwned := false
	cleanup := func() error {
		cancel()
		if !stopCaller() {
			<-callerDone
		}
		if !stopLifetime() {
			<-interrupted
		}
		var cleanupErr error
		if nativeOwned {
			_ = transport.Close()
			cleanupErr = errors.Join(owned.Close(), binding.ReleaseIntroductionRecovery())
		} else {
			cleanupErr = errors.Join(owned.Close(), transport.Close(), binding.ReleaseIntroductionRecovery())
		}
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
	credential := binding.Credential()
	client := binding.Surface() == broker.Connection
	var continuity [32]byte
	defer clear(continuity[:])
	first, acquired, err := openInitialAttachment(binding, lifetime, transport, exporterContext, client, &continuity)
	lease = acquired
	if err != nil {
		return nil, err
	}
	identity := nativeconnection.InstanceAuthentication{Network: credential.NetworkID, Target: credential.Target,
		Public: credential.InstancePublic, Generation: credential.Generation}
	if lease != nil {
		identity.Signer = lease
	}
	recovery := nativeconnection.Recovery{WorkSafetyNotAfter: facts.WorkSafetyNotAfter,
		WorkSafetyMaximum: facts.WorkSafetyMaximum, NoNewRecoveryAfter: facts.NoNewRecoveryAfter}
	if open != nil {
		recovery = binding.Recovery()
		if err := nativeconnection.ValidateRecovery(true, recovery, binding.Clock().UTC().Unix(), credential.NotAfter); err != nil {
			return nil, err
		}
	}
	var opener nativeconnection.AttachmentOpener
	if open != nil {
		opener = func(attempt context.Context, request nativeconnection.Recovery) (*nativeconnection.Attachment, error) {
			return openRecoveryAttachment(binding, attempt, request, open, lease, client)
		}
	}
	stream, err := nativeconnection.NewAuthenticatedStream(nativeconnection.StreamConfig{
		Context: lifetime, Application: owned, NetworkID: facts.Network, Initial: first,
		ContinuityKey: continuity, Authorized: binding.Clock().UTC(), Client: client,
		Recovery: recovery, OpenAttachment: opener,
		Resources: binding.Resources()}, identity)
	if err != nil {
		return nil, err
	}
	nativeOwned = true
	// Only recovery-capable streams retain terminal-control ownership after
	// RunBounded. A one-Attachment stream completes and closes immediately, so
	// presenting its RetireTerminalTail method would turn an ordinary local
	// Close into the false "terminal-control tail is unavailable" failure.
	if open != nil {
		connection.retireTail = stream.RetireTerminalTail
	}
	admissionErr := errors.Join(ctx.Err(), lifetime.Err(), binding.Current())
	if admissionErr != nil {
		// The native lifecycle still owns its initial secret/receipt. Run it
		// under cancellation and join it instead of abandoning the owner.
		cancel()
	}
	transferred = true
	recoveryTransferred = true
	go connection.runNative(ctx, lifetime, stream, send, receive, cleanup)
	if admissionErr != nil {
		return nil, errors.Join(admissionErr, connection.Close())
	}
	return connection, nil
}

// runNative owns the stream's terminal outcome and joins physical cleanup
// before signaling finished. A recovery-capable native tail may outlive the
// Application outcome, so retired and finished remain distinct barriers.
func (connection *Stream) runNative(ctx, lifetime context.Context, stream *nativeconnection.Stream,
	send, receive uint32, cleanup func() error,
) {
	_, runErr := stream.RunBounded(send, receive)
	runErr = errors.Join(runErr, ctx.Err(), lifetime.Err(), connection.binding.Current())
	connection.runErr = runErr
	nativeFinished := false
	select {
	case <-stream.Done():
		nativeFinished = true
	default:
	}
	if nativeFinished {
		connection.finishErr = errors.Join(cleanup(), stream.RetirementResult())
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
		// Done closes only after the native Stream retired its current
		// Attachment, so the joined retirement result is complete here (F-23).
		connection.finishErr = errors.Join(cleanup(), stream.RetirementResult())
		if connection.finishErr != nil {
			connection.finishErr = errors.Join(errors.New("text Service cleanup failed"), connection.finishErr)
		}
	}
	close(connection.finished)
}

func (connection *Stream) Done() <-chan applicationconnection.Outcome {
	return connection.done
}

// Finished closes only after the native lifecycle joined physical cleanup.
// A recovery-capable terminal tail may keep the native stream open after the
// Application outcome, so slot owners wait on this barrier, not on Done.
func (connection *Stream) Finished() <-chan struct{} {
	return connection.finished
}

func (connection *Stream) Close() error {
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
			wait = waitServiceClose
		}
		retired := wait(connection.retired)
		if !retired {
			connection.cancel()
		} else if connection.retireTail != nil {
			retireErr := connection.retireTail()
			// Caller cancellation may already have ended an otherwise successful
			// retained tail. The Application outcome owns that cancellation; Close
			// reports only a distinct terminal or cleanup failure.
			if retireErr == context.Canceled {
				retireErr = nil
			}
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

func waitServiceClose(finished <-chan struct{}) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-finished:
		return true
	case <-timer.C:
		return false
	}
}
