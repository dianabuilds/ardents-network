package node

import (
	"errors"
	"net"
)

// closeAcceptedCarrier records cleanup failures for the joined duty result.
func (server *closedForwardingServer) closeAcceptedCarrier(connection net.Conn) {
	err := connection.Close()
	if err == nil || errors.Is(err, net.ErrClosed) {
		return
	}
	server.acceptedCloseMu.Lock()
	server.acceptedCloseErr = errors.Join(server.acceptedCloseErr, err)
	server.acceptedCloseMu.Unlock()
}
