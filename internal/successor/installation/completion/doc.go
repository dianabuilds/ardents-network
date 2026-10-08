// Package completion owns one bounded local Unix completion exchange with the
// original root peer. It retains independent read-only directory custody,
// private guard/socket-record metadata, socket identity and the original caller.
// It reads no root-private intent, shares no Installation lease and grants no
// Release, process, manager, archival, startup or Service readiness authority.
// Encode owns the shared closed 160-byte frame grammar used by the root
// producer and independent startup peer. It conveys detached identities only.
// Installation observes its actual invocation before and after the exchange;
// root separately authorizes, observes and durably archives the transition.
// Calls are serialized by that owning operation; cancellation callbacks join
// before Wait returns and Close physically closes all retained descriptors.
package completion

import "errors"

var (
	ErrInput   = errors.New("installation completion: invalid input")
	ErrBinding = errors.New("installation completion: original observation mismatch")
)
