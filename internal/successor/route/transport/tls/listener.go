package tls

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// ListenShared opens one literal TCP socket for direct role and mutual Node TLS.
// It owns bounded concurrent handshakes; accepted connections remain the caller's
// lifetime. The first admitted Accept latches its handshake bound for the
// listener's dispatch loop; Close interrupts in-flight authentications and joins
// them before returning.
func ListenShared(endpoint string, certificate tls.Certificate, verify transport.ClosedSharedPeerVerifier, handshakeLimit uint16) (transport.ClosedSharedCarrierListener, error) {
	if err := transport.ValidateSharedListener(endpoint, certificate, verify, handshakeLimit); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", endpoint)
	if err != nil {
		return nil, err
	}
	lifetime, stop := context.WithCancel(context.Background())
	return &sharedListener{
		listener:    listener,
		certificate: certificate,
		verify:      verify,
		handshakes:  make(chan struct{}, handshakeLimit),
		results:     make(chan handshakeResult, handshakeLimit),
		acceptDone:  make(chan struct{}),
		lifetime:    lifetime,
		stop:        stop,
	}, nil
}

// handshakeResult is one bounded authentication outcome. A refused peer retains
// its original classified failure cause; a successful Connection transfers to
// the receiving Accept caller, or is closed joined by Close when no caller
// receives it.
type handshakeResult struct {
	carrier transport.ClosedSharedCarrier
	err     error
}

type sharedListener struct {
	listener    net.Listener
	certificate tls.Certificate
	verify      transport.ClosedSharedPeerVerifier
	handshakes  chan struct{}
	results     chan handshakeResult

	lifetime context.Context
	stop     context.CancelFunc

	mu         sync.Mutex
	started    bool
	timeout    time.Duration
	acceptErr  error
	closed     bool
	cleanupErr error

	acceptDone  chan struct{}
	handshakeWG sync.WaitGroup
	acceptWG    sync.WaitGroup
	closeOnce   sync.Once
	closeErr    error
}

// Accept waits for the next classified arrival. Authentication runs
// concurrently within the listener's finite handshake capacity, so one stalled
// peer consumes only its own bounded slot and cannot delay other arrivals. The
// first admitted call latches the handshake bound for every dispatched
// authentication; later calls must retain that exact bound, and their context
// bounds only the wait for the next outcome.
func (listener *sharedListener) Accept(ctx context.Context, handshakeTimeout time.Duration) (transport.ClosedSharedCarrier, error) {
	if err := transport.ValidateSharedAcceptance(ctx, handshakeTimeout); err != nil {
		return transport.ClosedSharedCarrier{}, err
	}
	started := false
	listener.mu.Lock()
	err := listener.admissionFailureLocked(ctx)
	if err == nil && listener.started && listener.timeout != handshakeTimeout {
		err = errors.New("closed shared carrier acceptance timeout differs")
	}
	if err == nil {
		if !listener.started {
			listener.started = true
			listener.timeout = handshakeTimeout
			started = true
		}
		listener.acceptWG.Add(1)
	}
	listener.mu.Unlock()
	if err != nil {
		return transport.ClosedSharedCarrier{}, err
	}
	defer listener.acceptWG.Done()
	if started {
		go listener.acceptLoop(handshakeTimeout)
	}
	for {
		select {
		case result := <-listener.results:
			return listener.handoff(ctx, result)
		default:
		}
		if err := listener.interruption(ctx); err != nil {
			return transport.ClosedSharedCarrier{}, err
		}
		select {
		case result := <-listener.results:
			return listener.handoff(ctx, result)
		case <-listener.acceptDone:
		case <-ctx.Done():
		case <-listener.lifetime.Done():
		}
	}
}

// admissionFailureLocked compares only the original listener and caller. The
// caller holds mu; native Close and all other physical I/O remain outside it.
func (listener *sharedListener) admissionFailureLocked(ctx context.Context) error {
	if listener.cleanupErr != nil {
		return listener.cleanupErr
	}
	if listener.closed || listener.lifetime.Err() != nil {
		return net.ErrClosed
	}
	return ctx.Err()
}

// A queued successful authentication is still owned by this listener. Only an
// original admitted waiter may take it; cancellation or Close joins a rejected
// transfer and cannot lose a physical close failure.
func (listener *sharedListener) handoff(ctx context.Context, result handshakeResult) (transport.ClosedSharedCarrier, error) {
	if result.err != nil {
		return transport.ClosedSharedCarrier{}, result.err
	}
	listener.mu.Lock()
	err := listener.admissionFailureLocked(ctx)
	listener.mu.Unlock()
	if err != nil {
		return transport.ClosedSharedCarrier{}, errors.Join(err, listener.closeOwned(result.carrier.Connection))
	}
	return result.carrier, nil
}

func (listener *sharedListener) interruption(ctx context.Context) error {
	listener.mu.Lock()
	defer listener.mu.Unlock()
	if err := listener.admissionFailureLocked(ctx); err != nil {
		return err
	}
	select {
	case <-listener.acceptDone:
		if listener.acceptErr == nil {
			return errors.New("closed shared carrier acceptance is unavailable")
		}
		return listener.acceptErr
	default:
		return nil
	}
}

// A physical cleanup failure is retained independently from a peer refusal.
// Denial interrupts waiters and authenticators; the original owner still joins
// the accept loop, every waiter and every physical connection through Close.
func (listener *sharedListener) closeOwned(conn net.Conn) error {
	err := conn.Close()
	if err != nil {
		listener.mu.Lock()
		listener.cleanupErr = errors.Join(listener.cleanupErr, err)
		listener.mu.Unlock()
		listener.stop()
	}
	return err
}

// acceptLoop retains original socket admission. Each arrival acquires one
// finite handshake slot or is refused physically without waiting; an acquired
// slot runs its bounded authentication independently of any Accept caller's
// wait, keeping authentication bounded from arrival rather than from dispatch.
func (listener *sharedListener) acceptLoop(timeout time.Duration) {
	defer close(listener.acceptDone)
	for {
		raw, err := listener.listener.Accept()
		if err != nil {
			listener.mu.Lock()
			listener.acceptErr = err
			listener.mu.Unlock()
			return
		}
		deadline := time.Now().Add(timeout)
		select {
		case listener.handshakes <- struct{}{}:
			listener.handshakeWG.Add(1)
			go listener.authenticate(raw, deadline)
		default:
			_ = listener.closeOwned(raw)
		}
	}
}

// authenticate performs one bounded peer authentication. It retains its
// capacity slot until the outcome is published, and it closes the physical
// socket whenever the outcome cannot reach a caller.
func (listener *sharedListener) authenticate(raw net.Conn, deadline time.Time) {
	defer listener.handshakeWG.Done()
	defer func() { <-listener.handshakes }()
	result := listener.classify(raw, deadline)
	select {
	case listener.results <- result:
	case <-listener.lifetime.Done():
		if result.err == nil {
			_ = listener.closeOwned(result.carrier.Connection)
		}
	}
}

// classify preserves the original inline acceptance sequence: bounded
// handshake under the listener lifetime and the latched deadline, exact
// classification, deadline release, and physical close on every refusal with
// the retained peer-failure cause.
func (listener *sharedListener) classify(raw net.Conn, deadline time.Time) handshakeResult {
	ctx, cancel := context.WithDeadline(listener.lifetime, deadline)
	defer cancel()
	socket := &nativeSocket{Conn: raw}
	// Bounded ciphertext input like the inner role channels (audit RD-5); the
	// nativeSocket still owns write admission and close ownership.
	secured := tls.Server(newRoleInput(socket), transport.SharedServerTLS(listener.certificate))
	// HandshakeContext may already be closing this socket on cancellation.
	// Reuse its original once-only owner to join that Close and retain its
	// physical result instead of closing the raw connection a second time.
	if err := secured.SetDeadline(deadline); err != nil {
		return handshakeResult{err: errors.Join(transport.MarkSharedPeerFailure(err), listener.closeOwned(socket))}
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		return handshakeResult{err: errors.Join(transport.MarkSharedPeerFailure(err), listener.closeOwned(socket))}
	}
	classified, err := transport.ClassifySharedTLS(secured.ConnectionState(), listener.verify)
	if err != nil {
		return handshakeResult{err: errors.Join(transport.MarkSharedPeerFailure(err), listener.closeOwned(socket))}
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		return handshakeResult{err: errors.Join(transport.MarkSharedPeerFailure(err), listener.closeOwned(socket))}
	}
	classified.Connection = secured
	if classified.Kind == transport.ClosedSharedNode {
		classified.Connection = &nodeCarrier{Conn: secured, socket: socket}
	}
	return handshakeResult{carrier: classified}
}

// Close releases the shared socket, interrupts in-flight authentications and
// joins them. Handshake outcomes no caller received are closed, not retained.
func (listener *sharedListener) Close() error {
	listener.closeOnce.Do(func() {
		listener.mu.Lock()
		listener.closed = true
		started := listener.started
		listener.mu.Unlock()
		listener.stop()
		nativeErr := listener.listener.Close()
		if started {
			<-listener.acceptDone
			listener.handshakeWG.Wait()
		}
		listener.acceptWG.Wait()
		for {
			select {
			case result := <-listener.results:
				if result.err == nil {
					_ = listener.closeOwned(result.carrier.Connection)
				}
			default:
				listener.mu.Lock()
				listener.closeErr = errors.Join(nativeErr, listener.cleanupErr)
				listener.mu.Unlock()
				return
			}
		}
	})
	return listener.closeErr
}

var _ transport.ClosedSharedCarrierListener = (*sharedListener)(nil)
