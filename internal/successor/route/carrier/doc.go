// Package carrier owns the selected generation-three TCP/TLS and QUIC physical
// mechanisms. It authenticates exact keys supplied by Route's fresh Network
// checks, uses one ordered QUIC stream, and offers no fallback or authority.
// The caller joins all borrowers before releasing its domain reservations.
package carrier
