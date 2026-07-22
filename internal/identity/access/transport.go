package access

import "context"

type transportPeer struct {
	peer   [32]byte
	source SourceKey
}

type transportPeerKey struct{}

// WithTransportPeer binds an authenticated transport peer to the request
// context. Surface adapters consume this value when constructing the typed
// authentication binding; handlers must not derive it from request payloads.
func WithTransportPeer(ctx context.Context, peer [32]byte, source SourceKey) context.Context {
	return context.WithValue(ctx, transportPeerKey{}, transportPeer{peer: peer, source: source})
}

func TransportPeerFromContext(ctx context.Context) ([32]byte, SourceKey, bool) {
	value, ok := ctx.Value(transportPeerKey{}).(transportPeer)
	return value.peer, value.source, ok
}
