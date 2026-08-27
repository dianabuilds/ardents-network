//go:build ignore

// R-094 disposable two-adapter Carrier experiment. See README.md.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const nativeProfile = "ardents-interactive-route-v1"

type profileID string

const (
	tcpProfile  profileID = "r094-tcp-tls-v1"
	quicProfile profileID = "r094-quic-v1"
)

type resultClass string

const (
	classSuccess      resultClass = "success"
	classStale        resultClass = "stale"
	classIncompatible resultClass = "incompatible"
	classUnauthorized resultClass = "unauthorized"
	classCanceled     resultClass = "canceled"
	classTimeout      resultClass = "timeout"
	classUnavailable  resultClass = "unavailable"
	classClosed       resultClass = "closed"
	classInternal     resultClass = "internal"
)

var (
	errStale        = errors.New("carrier offer is stale")
	errIncompatible = errors.New("carrier profile is incompatible")
	errUnauthorized = errors.New("carrier peer or binding is unauthorized")
	errClosed       = errors.New("carrier is closed")
)

type carrierAttempt struct {
	AuthorityDigest [32]byte
	Profile         profileID
	Endpoint        string
	ExpectedPeer    [32]byte
	Certificate     tls.Certificate
	Binding         route.LegBinding
	Deadline        time.Time
}

type deadlineLane interface {
	io.ReadWriteCloser
	SetDeadline(time.Time) error
}

type transportResult struct {
	lane  deadlineLane
	state tls.ConnectionState
	close func() error
}

type transportAdapter func(context.Context, carrierAttempt) (transportResult, error)

type carrierModule struct {
	adapters map[profileID]transportAdapter
	calls    map[profileID]int
}

type authenticatedCarrier struct {
	mu     sync.Mutex
	lane   deadlineLane
	close  func() error
	closed bool
}

type classifiedError struct {
	class resultClass
	cause error
}

func (failure *classifiedError) Error() string {
	return string(failure.class) + ": " + failure.cause.Error()
}
func (failure *classifiedError) Unwrap() error { return failure.cause }

func newCarrierModule() *carrierModule {
	return &carrierModule{
		adapters: map[profileID]transportAdapter{tcpProfile: openTCP, quicProfile: openQUIC},
		calls:    make(map[profileID]int, 2),
	}
}

// Open is the complete experiment Interface. It returns only an authenticated
// ordered carrier. Transport state, profile fallback, and retry never cross it.
func (module *carrierModule) Open(ctx context.Context, attempt carrierAttempt) (*authenticatedCarrier, error) {
	adapter, err := module.preflight(ctx, attempt)
	if err != nil {
		return nil, classify(err)
	}
	module.calls[attempt.Profile]++
	result, err := adapter(ctx, attempt)
	if err != nil {
		return nil, classify(err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = result.close()
		}
	}()
	if err := verifyTransportState(result.state, attempt.ExpectedPeer); err != nil {
		return nil, classify(err)
	}
	if err := route.ConfirmNodeLegBinding(result.lane, attempt.Binding); err != nil {
		return nil, classify(errors.Join(errUnauthorized, err))
	}
	cleanup = false
	return &authenticatedCarrier{lane: result.lane, close: result.close}, nil
}

func (module *carrierModule) preflight(ctx context.Context, attempt carrierAttempt) (transportAdapter, error) {
	if ctx == nil || attempt.AuthorityDigest == [32]byte{} || attempt.Endpoint == "" ||
		attempt.ExpectedPeer == [32]byte{} || attempt.Certificate.PrivateKey == nil || attempt.Deadline.IsZero() {
		return nil, errIncompatible
	}
	if !time.Now().Before(attempt.Deadline) {
		return nil, errStale
	}
	adapter, ok := module.adapters[attempt.Profile]
	if !ok {
		return nil, errIncompatible
	}
	if _, err := route.EncodeLegBinding(attempt.Binding); err != nil {
		return nil, errors.Join(errIncompatible, err)
	}
	local, err := certificateIdentity(attempt.Certificate)
	if err != nil || local != attempt.Binding.SenderNodeID || attempt.ExpectedPeer != attempt.Binding.PeerNodeID ||
		attempt.Binding.NotAfter.After(attempt.Deadline) {
		return nil, errIncompatible
	}
	return adapter, nil
}

func verifyTransportState(state tls.ConnectionState, expected [32]byte) error {
	if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != nativeProfile || len(state.PeerCertificates) != 1 {
		return errUnauthorized
	}
	public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize || !bytes.Equal(public, expected[:]) {
		return errUnauthorized
	}
	return nil
}

func (carrier *authenticatedCarrier) Read(buffer []byte) (int, error) {
	carrier.mu.Lock()
	if carrier.closed {
		carrier.mu.Unlock()
		return 0, classify(errClosed)
	}
	lane := carrier.lane
	carrier.mu.Unlock()
	count, err := lane.Read(buffer)
	if err != nil {
		return count, classify(err)
	}
	return count, nil
}

func (carrier *authenticatedCarrier) Write(buffer []byte) (int, error) {
	carrier.mu.Lock()
	if carrier.closed {
		carrier.mu.Unlock()
		return 0, classify(errClosed)
	}
	lane := carrier.lane
	carrier.mu.Unlock()
	count, err := lane.Write(buffer)
	if err != nil {
		return count, classify(err)
	}
	return count, nil
}

func (carrier *authenticatedCarrier) Close() error {
	carrier.mu.Lock()
	if carrier.closed {
		carrier.mu.Unlock()
		return classify(errClosed)
	}
	carrier.closed = true
	close := carrier.close
	carrier.mu.Unlock()
	if err := close(); err != nil {
		return classify(err)
	}
	return nil
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	var existing *classifiedError
	if errors.As(err, &existing) {
		return err
	}
	class := classUnavailable
	switch {
	case errors.Is(err, errStale):
		class = classStale
	case errors.Is(err, errIncompatible):
		class = classIncompatible
	case errors.Is(err, errUnauthorized):
		class = classUnauthorized
	case errors.Is(err, context.Canceled):
		class = classCanceled
	case errors.Is(err, context.DeadlineExceeded):
		class = classTimeout
	case errors.Is(err, errClosed):
		class = classClosed
	default:
		var networkError net.Error
		if errors.As(err, &networkError) && networkError.Timeout() {
			class = classTimeout
		}
	}
	return &classifiedError{class: class, cause: err}
}

func classOf(err error) resultClass {
	if err == nil {
		return classSuccess
	}
	var failure *classifiedError
	if errors.As(err, &failure) {
		return failure.class
	}
	return classInternal
}
