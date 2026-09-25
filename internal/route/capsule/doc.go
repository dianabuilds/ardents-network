// Package capsule owns the fixed closed Introduction submission envelope,
// recipient-only plaintext, and selected HPKE transcript. It does not admit
// a Route, choose a peer, hold Instance keys, or accept the decoded facts as
// authority. Route carries the opaque operation; Endpoint checks live State,
// registration, local authorization, and Rendezvous eligibility after Open.
package capsule
