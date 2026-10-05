package quic

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"github.com/quic-go/quic-go"
	"net"
	"sync"
	"time"
)

// ListenShared opens the exact UDP listener with one ordered stream per peer,
// shared authentication and bounded arrival-owned handshake reservations.
func ListenShared(endpoint string, certificate tls.Certificate, verify transport.ClosedSharedPeerVerifier, handshakeLimit uint16) (transport.ClosedSharedCarrierListener, error) {
	if err := transport.ValidateSharedListener(endpoint, certificate, verify, handshakeLimit); err != nil {
		return nil, err
	}
	socket, err := net.ListenPacket("udp", endpoint)
	if err != nil {
		return nil, err
	}
	handshakes := make(chan struct{}, handshakeLimit)
	physical := &quic.Transport{Conn: socket, ConnContext: handshakeContext(handshakes)}
	listener, err := physical.Listen(transport.SharedServerTLS(certificate), serverConfig())
	if err != nil {
		return nil, errors.Join(err, physical.Close(), socket.Close())
	}
	return &sharedListener{listener: listener, transport: physical, socket: socket, verify: verify}, nil
}

type handshakeContextKey struct{}

type handshakeReservation struct {
	slots    chan struct{}
	arrived  time.Time
	stop     func() bool
	released chan struct{}
	once     sync.Once
}

func handshakeContext(slots chan struct{}) func(context.Context, *quic.ClientInfo) (context.Context, error) {
	return func(ctx context.Context, _ *quic.ClientInfo) (context.Context, error) {
		select {
		case slots <- struct{}{}:
			reservation := &handshakeReservation{slots: slots, arrived: time.Now(), released: make(chan struct{})}
			reservation.stop = context.AfterFunc(ctx, reservation.releaseSlot)
			return context.WithValue(ctx, handshakeContextKey{}, reservation), nil
		default:
			return nil, errors.New("closed shared carrier handshake capacity is unavailable")
		}
	}
}

func (reservation *handshakeReservation) release() {
	if reservation != nil && reservation.stop != nil && reservation.stop() {
		reservation.releaseSlot()
	}
}

func (reservation *handshakeReservation) releaseSlot() {
	reservation.once.Do(func() {
		<-reservation.slots
		close(reservation.released)
	})
}

type sharedListener struct {
	transport *quic.Transport
	socket    net.PacketConn
	closeOnce sync.Once
	closeErr  error
	listener  *quic.Listener
	verify    transport.ClosedSharedPeerVerifier
}

func (listener *sharedListener) Accept(ctx context.Context, handshakeTimeout time.Duration) (transport.ClosedSharedCarrier, error) {
	if err := transport.ValidateSharedAcceptance(ctx, handshakeTimeout); err != nil {
		return transport.ClosedSharedCarrier{}, err
	}
	var connection *quic.Conn
	var reservation *handshakeReservation
	for {
		var err error
		connection, err = listener.listener.Accept(ctx)
		if err != nil {
			return transport.ClosedSharedCarrier{}, err
		}
		reservation, _ = connection.Context().Value(handshakeContextKey{}).(*handshakeReservation)
		if reservation == nil {
			_ = connection.CloseWithError(1, "handshake-capacity-unavailable")
			continue
		}
		defer reservation.release()
		goto admitted
	}
admitted:
	deadline := reservation.arrived.Add(handshakeTimeout)
	if !time.Now().Before(deadline) {
		_ = connection.CloseWithError(1, "carrier-handshake-expired")
		return transport.ClosedSharedCarrier{}, transport.MarkSharedPeerFailure(context.DeadlineExceeded)
	}
	classified, err := transport.ClassifySharedTLS(connection.ConnectionState().TLS, listener.verify)
	if err != nil {
		_ = connection.CloseWithError(1, "carrier-peer-invalid")
		return transport.ClosedSharedCarrier{}, transport.MarkSharedPeerFailure(err)
	}
	attempt, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	stream, err := connection.AcceptStream(attempt)
	if err != nil {
		_ = connection.CloseWithError(1, "carrier-stream-invalid")
		return transport.ClosedSharedCarrier{}, transport.MarkSharedPeerFailure(err)
	}
	var carrier net.Conn = &roleCarrier{stream: stream, connection: connection}
	if classified.Kind == transport.ClosedSharedNode {
		// Preserve the original accepted-side close reason while retaining a
		// Node type with no role-exporter capability. Close still interrupts
		// the original connection; it never calls concurrent Stream.Close.
		carrier = &nodeCarrier{stream: stream, connection: connection, closeReason: "role-close"}
	}
	if err := carrier.SetDeadline(deadline); err != nil {
		_ = carrier.Close()
		return transport.ClosedSharedCarrier{}, transport.MarkSharedPeerFailure(err)
	}
	if err := carrier.SetDeadline(time.Time{}); err != nil {
		_ = carrier.Close()
		return transport.ClosedSharedCarrier{}, transport.MarkSharedPeerFailure(err)
	}
	classified.Connection = carrier
	return classified, nil
}

func (listener *sharedListener) Close() error {
	listener.closeOnce.Do(func() {
		// Listener.Close alone leaves accepted connections and their UDP socket
		// owned by quic-go. Join the transport before releasing the local socket
		// so a completed duty can immediately reopen its State-selected address.
		listener.closeErr = errors.Join(listener.listener.Close(), listener.transport.Close(), listener.socket.Close())
	})
	return listener.closeErr
}

var _ transport.ClosedSharedCarrierListener = (*sharedListener)(nil)
