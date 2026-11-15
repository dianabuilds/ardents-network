// Package completion owns one bounded local Unix completion exchange with the
// original root peer. It retains independent read-only directory custody,
// private guard/socket-record metadata, socket identity and the original caller.
// It reads no root-private intent, shares no Installation lease and grants no
// Release, process, manager, archival, startup or Service readiness authority.
// Encode owns the shared closed 160-byte frame grammar used by the root
// producer and independent startup peer. It conveys detached identities only.
// The portable history ACK codec validates the closed frame and choice; native
// peer credentials and physical exchange remain Linux-specific.
// The distinct Forwarding exchange requires the complete 161-byte ACK and
// exposes its one bounded history-opening byte only after native peer checks.
// Its own fresh physical opener shares no Source Startup or live exchange.
// Installation observes its actual invocation before and after the exchange;
// root separately authorizes, observes and durably archives the transition.
// Calls are serialized by that owning operation; cancellation callbacks join
// before Wait returns and Close physically closes all retained descriptors.
// JOIN, Descriptor, Forwarding and Introduction exchanges each retain their
// own original root-peer connection and complete 161-byte ACK. The history byte
// is a detached choice, not proof of history or permission to initialize a root;
// each role independently admits its applicable choice and original invocation.
// No sibling exchange, copied ACK or detached identity supplies Startup rights.
package completion

import "errors"

var (
	ErrInput   = errors.New("installation completion: invalid input")
	ErrBinding = errors.New("installation completion: original observation mismatch")
)
