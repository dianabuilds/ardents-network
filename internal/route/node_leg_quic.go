package route

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

type quicNodeCarrier struct {
	stream     *quic.Stream
	connection *quic.Conn
	closeOnce  sync.Once
	closeErr   error
}

func (carrier *quicNodeCarrier) Read(buffer []byte) (int, error) {
	return carrier.stream.Read(buffer)
}

func (carrier *quicNodeCarrier) Write(buffer []byte) (int, error) {
	return carrier.stream.Write(buffer)
}

func (carrier *quicNodeCarrier) SetDeadline(deadline time.Time) error {
	return carrier.stream.SetDeadline(deadline)
}

func (carrier *quicNodeCarrier) Close() error {
	carrier.closeOnce.Do(func() {
		carrier.closeErr = errors.Join(carrier.stream.Close(), carrier.connection.CloseWithError(0, "carrier-close"))
	})
	return carrier.closeErr
}

func (carrier *quicNodeCarrier) abort() error {
	carrier.closeOnce.Do(func() {
		carrier.stream.CancelRead(1)
		carrier.stream.CancelWrite(1)
		carrier.closeErr = carrier.connection.CloseWithError(1, "carrier-abort")
	})
	return carrier.closeErr
}

func openQUICNodeCarrier(ctx context.Context, input NodeLegRequest) (nodeCarrierResult, error) {
	connection, err := quic.DialAddr(ctx, input.Endpoint, nativeNodeTLS(input.Certificate, input.ExpectedPeerKey), nodeQUICConfig())
	if err != nil {
		return nodeCarrierResult{}, err
	}
	stream, err := connection.OpenStreamSync(ctx)
	if err != nil {
		_ = connection.CloseWithError(1, "carrier-open-failed")
		return nodeCarrierResult{}, err
	}
	lane := &quicNodeCarrier{stream: stream, connection: connection}
	if err := lane.SetDeadline(input.Deadline); err != nil {
		_ = lane.Close()
		return nodeCarrierResult{}, err
	}
	return nodeCarrierResult{lane: lane, state: connection.ConnectionState().TLS, abort: lane.abort}, nil
}

func nodeQUICConfig() *quic.Config {
	return &quic.Config{Versions: []quic.Version{quic.Version1}, HandshakeIdleTimeout: time.Second,
		MaxIdleTimeout: 5 * time.Second, KeepAlivePeriod: time.Second, MaxIncomingStreams: -1, MaxIncomingUniStreams: -1,
		InitialPacketSize: 1200, InitialStreamReceiveWindow: 32 << 10, MaxStreamReceiveWindow: 32 << 10,
		InitialConnectionReceiveWindow: 64 << 10, MaxConnectionReceiveWindow: 64 << 10,
		AllowConnectionWindowIncrease: func(*quic.Conn, uint64) bool { return false }, EnableDatagrams: false, Allow0RTT: false}
}

var _ Carrier = (*quicNodeCarrier)(nil)
