package quic

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"github.com/quic-go/quic-go"
	"net"
)

// closedIOError preserves the original connection's native terminal cause.
// Reader-driven local adapter Close must not erase an already observed remote
// close before a selected writer enters Write. Only the existing exact native
// classifier grants the peer category; local and unknown causes stay failures.
func closedIOError(connection *quic.Conn) error {
	if native, ok := context.Cause(connection.Context()).(*quic.ApplicationError); ok {
		return classifyIOError(native)
	}
	return net.ErrClosed
}

// classifyIOError recognizes only the selected protocol's actual remote close.
// Preserve the exact native object for errors.As and errors.Is; no string-based
// fallback and no conversion of local close, other codes or unknown messages.
func classifyIOError(err error) error {
	if peer, ok := err.(*quic.ApplicationError); ok && peer.Remote && peer.ErrorCode == 0 && (peer.ErrorMessage == "carrier-close" || peer.ErrorMessage == "role-close") {
		return transport.MarkPeerRetirement(peer)
	}
	return err
}
