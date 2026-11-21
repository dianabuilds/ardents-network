// Package transport owns the portable ordered-stream contract, exact closed
// Carrier/profile identities, authenticated TLS exporter type and literal
// endpoint validation and exact-certificate/role authentication requirements
// shared by physical adapters. Node/role requests, shared-listener classification
// and cause-preserving handshake failures use those same portable rules.
// It grants no peer authority,
// fallback, framing allowance, Admission right or durable-root access.
// Concrete adapters and operation owners depend on this package; it imports
// neither of them.
package transport
