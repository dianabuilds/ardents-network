// Package join owns exact Source/Responder acquisitions, one holder attempt,
// its Context and joined stream, and separately receiving Rendezvous Pairing.
// Prefix owns generation admission, locks and physical terminal opening.
// Pairing owns two complete RESULT writes before data, relay accounting/credit
// and joined retirement of both original sides. Receiver supplies admitted
// channels and retains Grant/Hosting return until Serve completes; Pairing
// grants no authority. Pairing, holder exchange and stream use portable ordered
// I/O, context, sync and original bounds, with Windows and Linux tests. Context
// currently consumes native selection; it grants no Service authority.
// Copied holder acquisitions cannot cancel or return original Prefix claims.
// Exact dedicated-child decoded CLOSE evidence remains distinct from local
// retirement, raw EOF, Service authentication and actual cleanup success.
// Original joined CleanupResult receipts separate failed transport work from
// actual physical write/close failures without discarding either Close result.
// InterruptIO performs no new deadline I/O after framing already stopped.
package join
