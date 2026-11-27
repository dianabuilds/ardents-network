// Package hosting owns one shared durable provider-period budget and opaque
// work/termination reservations. Reserve commits capacity before work and
// Release refunds only reserved capacity after its consumer joins that work.
// Sample shares bounded confirmed observations; it cannot authorize work.
// Hosting grants no token permission, placement or network authority.
//
// Follow the live path through allowance.go (directions and provider limit),
// budget.go (owning transaction and counter continuity), period_storage.go
// (pin, journal and readback), then reservation.go (reserve, detach, release).
// additional_reservation.go accounts for joint work under an original
// termination reservation. snapshot.go is the confirmed observation path.
//
// The accounting rules and command inputs are portable. The live budget uses
// a Linux/macOS exclusive lease in period_lease_unix.go and native boot and
// interface observations in counters. Other hosts receive a typed refusal.
//
// The ardents-next adapter owns flags, exact config JSON and finite outcomes.
package hosting
