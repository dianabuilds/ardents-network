//go:build linux

// Package introduction owns the pure delivery-dispatch and Publication/
// Registration pair mechanism of one duty context: the cryptographic
// admission window, the bounded exchange reservation set, the waiter and
// recovery slot bookkeeping of the single-consumer dispatch, and the pair
// lifecycle that decides when an acknowledged Registration is usable.
//
// Every transition is valid only under the owning dutyContext mutex. All
// orchestration — the consumer receive loop, capsule acceptance, submission
// and recovery exchanges, registration opening and withdrawal, and the
// admitted Introduction prefix lifecycle (which shares the endpoint root's
// role prefix machinery with the Responder) — stays with the endpoint root,
// which supplies authority through the Host and RecoveryBinding seams and
// flight identity through FlightRef.
package introduction
