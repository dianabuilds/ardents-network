//go:build linux

// Package tokens owns the duty context's token authority: the granted holder
// permission with its exact hour, approved request, per-class reservations,
// exact pending blind batch and finalized stock, plus the single in-flight
// issuance operation slot bound to that permission and the durable token
// consumption boundary.
//
// The package is mechanism plus authority checks only. Duty orchestration
// (Source operation acquisition, bootstrap selection, opening flights, JOIN
// acquisitions and worker launch) stays with the endpoint package, which
// supplies the narrow Host seam and calls this owner under the shared duty
// context lock. Blinding state and finalized stock never leave the owner;
// revocation erases secrets under the same shared lock while cancellation
// interrupts and joins the transport tree.
package tokens
