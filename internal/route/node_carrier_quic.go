package route

import (
	"errors"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

// quicNodeCarrier is the shared authenticated QUIC byte lane used by current
// closed dial and listener adapters and by the retained native listener.
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

func nodeQUICConfig() *quic.Config {
	return &quic.Config{Versions: []quic.Version{quic.Version1}, HandshakeIdleTimeout: time.Second,
		MaxIdleTimeout: 5 * time.Second, KeepAlivePeriod: time.Second, MaxIncomingStreams: -1, MaxIncomingUniStreams: -1,
		InitialPacketSize: 1200, InitialStreamReceiveWindow: 32 << 10, MaxStreamReceiveWindow: 32 << 10,
		InitialConnectionReceiveWindow: 64 << 10, MaxConnectionReceiveWindow: 64 << 10,
		AllowConnectionWindowIncrease: func(*quic.Conn, uint64) bool { return false }, EnableDatagrams: false, Allow0RTT: false}
}

var _ Carrier = (*quicNodeCarrier)(nil)

func (carrier *quicNodeCarrier) SetWriteDeadline(deadline time.Time) error {
	return carrier.stream.SetWriteDeadline(deadline)
}
