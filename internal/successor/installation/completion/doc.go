// Package completion owns one bounded local Unix completion exchange with the
// original root peer. It retains independent read-only directory custody,
// private guard/socket-record metadata, socket identity and the original caller.
// It reads no root-private intent, shares no Installation lease and grants no
// Release, process, manager, archival, startup or Service readiness authority.
// Encode owns the shared closed 160-byte frame grammar used by the root
// producer and independent startup peer. It conveys detached identities only.
// The distinct Forwarding exchange/ACK retired with the transitional Forwarding
// orchestration (#604 FJD stage D, ADR-0125); the shared physical exchange
// remains for the transitional Endpoint startup.
// Installation observes its actual invocation before and after the exchange;
// root separately authorizes, observes and durably archives the transition.
// Calls are serialized by that owning operation; cancellation callbacks join
// before Wait returns and Close physically closes all retained descriptors.
// The distinct JOIN exchange/ACK (JoinExchange/ConnectJoin/EncodeJoinACK)
// retired with the transitional JOIN orchestration (#604 FJD stage D,
// ADR-0125); the platform-led JOIN lifecycle owns completion through the
// canonical retained runtime. No live foreign exchange or root journal is
// borrowed.
// The distinct Descriptor exchange/ACK (DescriptorExchange/ConnectDescriptor/
// EncodeDescriptorACK) retired with the transitional Descriptor orchestration
// (#604 FJD stage D, ADR-0125); the platform-led Descriptor lifecycle owns
// completion through the canonical retained runtime. The shared physical
// exchange and 160-byte frame remain for the transitional Endpoint startup; no
// live foreign exchange or root journal is borrowed.
package completion

import "errors"

var (
	ErrInput   = errors.New("installation completion: invalid input")
	ErrBinding = errors.New("installation completion: original observation mismatch")
)
