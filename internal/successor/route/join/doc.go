// Package join owns exact Source/Responder acquisitions, one holder attempt,
// its Context and joined stream, and separately receiving Rendezvous Pairing.
// Prefix owns generation admission, locks and physical terminal opening.
// Pairing owns two complete RESULT writes before data, relay accounting/credit
// and joined retirement of both original sides. Receiver supplies admitted
// channels and retains Grant/Hosting return until Serve completes; Pairing
// grants no authority. Pairing, holder exchange and stream use portable ordered
// I/O, context, sync and original bounds, with Windows and Linux tests. Context
// currently consumes native selection; it grants no Service authority.
package join
