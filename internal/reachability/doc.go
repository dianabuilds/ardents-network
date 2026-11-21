// Package reachability verifies exact-Target private Descriptors. Receiving
// durable Store and local lookup history are separate Reachability owners, not
// authority obtained from a verified proof. Route owns the
// transport and current recipient selection; Publication owns delegated public
// proof authority. A verified Descriptor establishes no slot or Service readiness.
package reachability
