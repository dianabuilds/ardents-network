// Package binding derives transport-owned Application identity bindings.
// It does not own durable identity state or product authorization.
package binding

import (
	"context"

	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"
)

// Application is shared by the protected Application identity and product
// call adapters so a session cannot move across listener or peer.
func Application(ctx context.Context, node string, fallbackPeer [32]byte, fallbackSource identityaccess.SourceKey) (identityaccess.AuthenticationBinding, identityaccess.SourceKey) {
	peer, source, ok := identityaccess.TransportPeerFromContext(ctx)
	if !ok {
		peer, source = fallbackPeer, fallbackSource
	}
	return identityaccess.AuthenticationBinding{
		Audience: identityaccess.Audience{
			Node:          node,
			Interface:     identityprotocol.Interface_INTERFACE_APPLICATION,
			ProtocolMajor: 1,
		},
		TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1,
		PeerBinding:      peer,
	}, source
}
