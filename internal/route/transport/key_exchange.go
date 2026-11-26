package transport

import "crypto/tls"

// ClosedProfileKeyExchangeGroups returns the one key-exchange group list of the
// closed TLS profile: hybrid X25519MLKEM768 first, then classical X25519. It
// takes no role, key, peer or operator input and grants no fallback: a peer
// offering neither group is refused by the handshake itself. Each call returns
// a fresh slice, so one configuration can never change another.
func ClosedProfileKeyExchangeGroups() []tls.CurveID {
	return []tls.CurveID{tls.X25519MLKEM768, tls.X25519}
}
