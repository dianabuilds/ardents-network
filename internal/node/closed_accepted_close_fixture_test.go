package node

import (
	"context"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

type acceptedCloseFailureConn struct {
	net.Conn
	closed chan struct{}
	err    error
}

func (connection *acceptedCloseFailureConn) Close() error {
	_ = connection.Conn.Close()
	select {
	case <-connection.closed:
	default:
		close(connection.closed)
	}
	return connection.err
}

type oneAcceptedCarrierListener struct {
	ready      chan struct{}
	connection net.Conn
	kind       carrier.ClosedSharedCarrierKind
	served     bool
}

func (listener *oneAcceptedCarrierListener) Accept(ctx context.Context, _ time.Duration) (carrier.ClosedSharedCarrier, error) {
	if !listener.served {
		select {
		case <-listener.ready:
		case <-ctx.Done():
			return carrier.ClosedSharedCarrier{}, ctx.Err()
		}
		listener.served = true
		return carrier.ClosedSharedCarrier{Kind: listener.kind, Connection: listener.connection}, nil
	}
	<-ctx.Done()
	return carrier.ClosedSharedCarrier{}, ctx.Err()
}

func (*oneAcceptedCarrierListener) Close() error { return nil }
