package route

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

// ClosedSharedCarrierKind identifies the only two post-TLS states of a
// literal successor listener. It is selected before any ARDP bytes are read.
type ClosedSharedCarrierKind uint8

const (
	ClosedSharedDirect ClosedSharedCarrierKind = iota + 1
	ClosedSharedNode
)

// ClosedSharedCarrier is one classified v3 carrier. NodeKey is set only for
// the authenticated outer Node-Carrier state.
type ClosedSharedCarrier struct {
	Kind       ClosedSharedCarrierKind
	Connection net.Conn
	NodeKey    [32]byte
}

// ClosedSharedPeerVerifier accepts exactly a current State-authorized Node
// public key. It is called only for a single syntactically valid certificate.
type ClosedSharedPeerVerifier func([32]byte) bool

// ClosedSharedCarrierListener accepts direct role TLS and outer Node Carrier
// TLS from one literal endpoint. Accept's finite handshake duration starts
// when a connection arrives, independently of listener idle time. The caller's
// context also bounds authentication; Close interrupts an idle socket accept.
// The duration is at most ten seconds and grants no ARDP authority. The caller
// owns the returned connection.
type ClosedSharedCarrierListener interface {
	Accept(context.Context, time.Duration) (ClosedSharedCarrier, error)
	Close() error
}

// ListenClosedSharedCarrier creates the one v3 listener shared by direct role
// and Node Carrier TLS. The verifier must read current authenticated State at
// classification time; a rejected certificate is closed before ARDP work.
func ListenClosedSharedCarrier(profile CarrierProfile, endpoint string, certificate tls.Certificate, verify ClosedSharedPeerVerifier, handshakeLimit uint16) (ClosedSharedCarrierListener, error) {
	if !literalEndpoint(endpoint) || certificate.PrivateKey == nil || verify == nil || handshakeLimit == 0 || handshakeLimit > 16 {
		return nil, errors.New("closed shared carrier listener is invalid")
	}
	switch profile {
	case ClosedCarrierTCP:
		listener, err := net.Listen("tcp", endpoint)
		if err != nil {
			return nil, err
		}
		return &closedSharedTCPListener{listener: listener, certificate: certificate, verify: verify, handshakes: make(chan struct{}, handshakeLimit)}, nil
	case ClosedCarrierQUIC:
		socket, err := net.ListenPacket("udp", endpoint)
		if err != nil {
			return nil, err
		}
		handshakes := make(chan struct{}, handshakeLimit)
		transport := &quic.Transport{Conn: socket, ConnContext: closedSharedQUICHandshakeContext(handshakes)}
		listener, err := transport.Listen(closedSharedServerTLS(certificate), closedRoleQUICServerConfig())
		if err != nil {
			return nil, errors.Join(err, transport.Close(), socket.Close())
		}
		return &closedSharedQUICListener{listener: listener, transport: transport, socket: socket, verify: verify}, nil
	default:
		return nil, errors.New("closed shared carrier profile is unsupported")
	}
}

type closedSharedQUICHandshakeContextKey struct{}

type closedSharedQUICHandshakeReservation struct {
	slots    chan struct{}
	stop     func() bool
	released chan struct{}
	once     sync.Once
}

func closedSharedQUICHandshakeContext(slots chan struct{}) func(context.Context, *quic.ClientInfo) (context.Context, error) {
	return func(ctx context.Context, _ *quic.ClientInfo) (context.Context, error) {
		select {
		case slots <- struct{}{}:
			reservation := &closedSharedQUICHandshakeReservation{slots: slots, released: make(chan struct{})}
			reservation.stop = context.AfterFunc(ctx, reservation.releaseSlot)
			return context.WithValue(ctx, closedSharedQUICHandshakeContextKey{}, reservation), nil
		default:
			return nil, errors.New("closed shared carrier handshake capacity is unavailable")
		}
	}
}

func (reservation *closedSharedQUICHandshakeReservation) release() {
	if reservation != nil && reservation.stop != nil && reservation.stop() {
		reservation.releaseSlot()
	}
}

func (reservation *closedSharedQUICHandshakeReservation) releaseSlot() {
	reservation.once.Do(func() {
		<-reservation.slots
		close(reservation.released)
	})
}

type closedSharedTCPListener struct {
	listener    net.Listener
	certificate tls.Certificate
	verify      ClosedSharedPeerVerifier
	handshakes  chan struct{}
}

func (listener *closedSharedTCPListener) Accept(ctx context.Context, handshakeTimeout time.Duration) (ClosedSharedCarrier, error) {
	if ctx == nil || handshakeTimeout <= 0 || handshakeTimeout > 10*time.Second {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier acceptance is invalid")
	}
	var raw net.Conn
	for {
		var err error
		raw, err = listener.listener.Accept()
		if err != nil {
			return ClosedSharedCarrier{}, err
		}
		select {
		case listener.handshakes <- struct{}{}:
			defer func() { <-listener.handshakes }()
			goto admitted
		default:
			_ = raw.Close()
		}
	}
admitted:
	deadline := time.Now().Add(handshakeTimeout)
	secured := tls.Server(raw, closedSharedServerTLS(listener.certificate))
	if err := secured.SetDeadline(deadline); err != nil {
		_ = raw.Close()
		return ClosedSharedCarrier{}, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return ClosedSharedCarrier{}, err
	}
	classified, err := classifyClosedSharedTLS(secured.ConnectionState(), listener.verify)
	if err != nil {
		_ = raw.Close()
		return ClosedSharedCarrier{}, err
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		_ = raw.Close()
		return ClosedSharedCarrier{}, err
	}
	classified.Connection = secured
	if classified.Kind == ClosedSharedNode {
		classified.Connection = &closedTCPNodeTransport{Conn: secured}
	}
	return classified, nil
}

func (listener *closedSharedTCPListener) Close() error { return listener.listener.Close() }

type closedSharedQUICListener struct {
	transport *quic.Transport
	socket    net.PacketConn
	closeOnce sync.Once
	closeErr  error
	listener  *quic.Listener
	verify    ClosedSharedPeerVerifier
}

func (listener *closedSharedQUICListener) Accept(ctx context.Context, handshakeTimeout time.Duration) (ClosedSharedCarrier, error) {
	if ctx == nil || handshakeTimeout <= 0 || handshakeTimeout > 10*time.Second {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier acceptance is invalid")
	}
	var connection *quic.Conn
	for {
		var err error
		connection, err = listener.listener.Accept(ctx)
		if err != nil {
			return ClosedSharedCarrier{}, err
		}
		reservation, _ := connection.Context().Value(closedSharedQUICHandshakeContextKey{}).(*closedSharedQUICHandshakeReservation)
		if reservation == nil {
			_ = connection.CloseWithError(1, "handshake-capacity-unavailable")
			continue
		}
		defer reservation.release()
		goto admitted
	}
admitted:
	deadline := time.Now().Add(handshakeTimeout)
	classified, err := classifyClosedSharedTLS(connection.ConnectionState().TLS, listener.verify)
	if err != nil {
		_ = connection.CloseWithError(1, "carrier-peer-invalid")
		return ClosedSharedCarrier{}, err
	}
	attempt, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	stream, err := connection.AcceptStream(attempt)
	if err != nil {
		_ = connection.CloseWithError(1, "carrier-stream-invalid")
		return ClosedSharedCarrier{}, err
	}
	carrier := &closedRoleQUICCarrier{stream: stream, connection: connection}
	if err := carrier.SetDeadline(deadline); err != nil {
		_ = carrier.Close()
		return ClosedSharedCarrier{}, err
	}
	if err := carrier.SetDeadline(time.Time{}); err != nil {
		_ = carrier.Close()
		return ClosedSharedCarrier{}, err
	}
	classified.Connection = carrier
	return classified, nil
}

func (listener *closedSharedQUICListener) Close() error {
	listener.closeOnce.Do(func() {
		// Listener.Close alone leaves accepted connections and their UDP socket
		// owned by quic-go. Join the transport before releasing the local socket
		// so a completed duty can immediately reopen its State-selected address.
		listener.closeErr = errors.Join(listener.listener.Close(), listener.transport.Close(), listener.socket.Close())
	})
	return listener.closeErr
}

func closedSharedServerTLS(certificate tls.Certificate) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientAuth: tls.RequestClientCert, SessionTicketsDisabled: true, NextProtos: []string{ClosedRouteProfile},
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}, VerifyConnection: func(state tls.ConnectionState) error {
			if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != ClosedRouteProfile || len(state.PeerCertificates) > 1 {
				return errors.New("closed shared carrier TLS state is invalid")
			}
			return nil
		}}
}

func classifyClosedSharedTLS(state tls.ConnectionState, verify ClosedSharedPeerVerifier) (ClosedSharedCarrier, error) {
	if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != ClosedRouteProfile || verify == nil {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier TLS state is invalid")
	}
	if len(state.PeerCertificates) == 0 {
		return ClosedSharedCarrier{Kind: ClosedSharedDirect}, nil
	}
	if len(state.PeerCertificates) != 1 {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier certificate count is invalid")
	}
	certificate := state.PeerCertificates[0]
	if time.Now().Before(certificate.NotBefore) || !time.Now().Before(certificate.NotAfter) {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier certificate is not current")
	}
	public, ok := certificate.PublicKey.(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier certificate key is invalid")
	}
	var key [32]byte
	copy(key[:], public)
	if !verify(key) {
		return ClosedSharedCarrier{}, errors.New("closed shared carrier peer is unavailable")
	}
	return ClosedSharedCarrier{Kind: ClosedSharedNode, NodeKey: key}, nil
}

var _ ClosedSharedCarrierListener = (*closedSharedTCPListener)(nil)
var _ ClosedSharedCarrierListener = (*closedSharedQUICListener)(nil)
