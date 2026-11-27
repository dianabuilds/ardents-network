// Package join owns exact Source/Responder acquisitions, one holder attempt,
// its Context and joined stream over opaque Prefix contracts. Prefix owns
// generation admission, locks and physical terminal opening; native selection
// owns roots. The separately receiving Rendezvous Pairing, two-RESULT barrier
// and relay accounting/credit live in the join/pairing subpackage, which
// Receiver consumes directly without the holder's Prefix/selection imports.
// Holder exchange and stream use portable ordered I/O, context, sync and
// original bounds, with Windows and Linux tests. Context currently consumes
// native selection; it grants no Service authority.
// Copied holder acquisitions cannot cancel or return original Prefix claims.
// Exact dedicated-child decoded CLOSE evidence remains distinct from local
// retirement, raw EOF, Service authentication and actual cleanup success.
// Original joined CleanupResult receipts separate failed transport work from
// actual physical write/close failures without discarding either Close result.
// InterruptIO performs no new deadline I/O after framing already stopped.
//
// Start at Context.Open for one logical attempt, acquisition.go for ownership
// of the original Prefix borrows and client.go for the bounded handshake.
// stream.go exposes the accepted ordered bytes and returns physical ownership
// through the acquisition on Close. The receiver's pairing package owns the
// independent two-sided barrier and relay.
package join
