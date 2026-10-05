package quic

import (
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"github.com/quic-go/quic-go"
)

// classifyIOError recognizes only the selected protocol's actual remote close.
// Preserve the exact native object for errors.As and errors.Is; no string-based
// fallback and no conversion of local close, other codes or unknown messages.
func classifyIOError(err error) error {
	if peer, ok := err.(*quic.ApplicationError); ok && peer.Remote && peer.ErrorCode == 0 && (peer.ErrorMessage == "carrier-close" || peer.ErrorMessage == "role-close") {
		return transport.MarkPeerRetirement(peer)
	}
	return err
}
