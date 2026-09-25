// Package transit owns Endpoint-local durable acquisition of one Transit Grant
// for each Introduction or Responder attachment. Its exclusive journal retains
// one-use request, key, presentation, and terminal state across restart.
// Endpoint supplies current State scope, issuer exchange, and TLS enrollment;
// transit selects no peer, Carrier, Service Target, or Application authority.
package transit
