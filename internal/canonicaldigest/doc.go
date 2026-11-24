// Package canonicaldigest owns the one canonical hex-32 validation atom
// shared by the maintained owners (Phase 4, #835): a string is admitted only
// when it is exactly 64 lowercase hexadecimal characters whose decoded 32
// bytes re-encode to the identical string.
//
// It is a pure mechanism leaf, not a root or custody owner. The decode
// predicate takes a caller string and reports the fixed 32-byte value plus a
// boolean; it holds no root, marker, schema, authority or lifecycle of its
// own, performs no filesystem observation and no hashing. Each consuming
// package keeps its distinct refusal prose and any additional zero-value,
// presence or length bounds it genuinely needs, composed around this atom.
//
// The atom replaces the formerly repeated private canonicalDigest predicates
// and inline hex.DecodeString + len==32 + round-trip checks whose variants
// drifted across owners. Deliberate boundaries remain separate: variable
// length canonical hex (Issuance and command admission inputs, the closed
// network preparation text), the weaker no-round-trip floor and fixed-hex
// decoders, the 16-byte systemd InvocationID shape, and Custody authority
// validation are not this atom. Release builder-attestation validation joins
// the canonical atom because the same package's verification already requires
// exact equality with hex.EncodeToString of the authenticated digest (the one
// deliberate tightening in #835; no accepting path loses a reachable input).
//
// Responsibility clusters and their tests:
//   - canonical hex-32 decode and validation predicates: canonical_hex.go
//     (canonical_hex_test.go)
package canonicaldigest
