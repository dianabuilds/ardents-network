package credential

import (
	"errors"
	"net"
)

// closeCarrier retains physical cleanup failures for the joined listener result.
func (listener *ClosedTokenListener) closeCarrier(connection net.Conn) {
	err := connection.Close()
	if err == nil || errors.Is(err, net.ErrClosed) {
		return
	}
	listener.closeMu.Lock()
	listener.closeErr = errors.Join(listener.closeErr, err)
	listener.closeMu.Unlock()
}
