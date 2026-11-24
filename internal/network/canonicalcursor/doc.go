// Package canonicalcursor owns the pure bounded cursor shared by the canonical
// Network byte grammars (R5, #804): bounded reads over one caller-supplied
// canonical byte slice, big-endian unsigned readers and the exact
// consumed-offset observation.
//
// It is a Network-domain mechanism leaf, not a grammar or authority owner:
// every consumer is a network package, so the cursor lives inside
// internal/network rather than as a top-level leaf (Phase 4, #834). It holds
// no root, marker, schema, authority or lifecycle of its own, performs no
// allocation or copying, and returns slices that alias the caller's bytes.
// Each consuming package keeps its distinct magic, field order, validation
// bounds and refusal prose; the only refusal this mechanism itself produces
// is the exact "truncated canonical bytes" bounds refusal formerly repeated
// by every consumer.
//
// Real consumers: internal/network/epoch (Record, Epoch and materialization
// grammars), internal/network/closedprofile (signed-profile body),
// internal/network/state (distribution journal) and internal/network/source
// (response bundle framing).
//
// Responsibility clusters and their tests:
//   - bounded cursor and big-endian readers: decoder.go (decoder_test.go)
package canonicalcursor
