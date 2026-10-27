// Package hosting owns one shared durable provider-period budget and opaque
// work/termination reservations. Reserve commits capacity before work and
// Release refunds only reserved capacity after its consumer joins that work.
// Sample shares bounded confirmed observations; it cannot authorize work.
// Hosting grants no token permission, placement or network authority.
// Linux Inspection separately retains complete existing period history and
// zero-reservation provenance under its own original lease. It mutates nothing
// and grants no current capacity; Close joins its original physical users.
package hosting
