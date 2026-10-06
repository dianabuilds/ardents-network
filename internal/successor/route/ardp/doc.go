// Package ardp owns the generation-3 ARDP lane header, bounded frame grammar,
// HELLO and OPEN bodies, bootstrap body, fixed admission acknowledgement and
// opaque issuer operation/result envelopes. Admission owns the enclosed batch
// and result grammar. Route owns
// assignment checks, authenticated channel and Carrier lifetimes, lane credit,
// and forwarding; this package grants no authority to use a decoded frame.
package ardp
