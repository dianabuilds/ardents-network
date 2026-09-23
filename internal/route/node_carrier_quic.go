package route

import (
	"errors"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

// quicNodeCarrier is the authenticated QUIC byte lane used by the current
// closed Node dial.
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

var _ Carrier = (*quicNodeCarrier)(nil)

func (carrier *quicNodeCarrier) SetWriteDeadline(deadline time.Time) error {
	return carrier.stream.SetWriteDeadline(deadline)
}
