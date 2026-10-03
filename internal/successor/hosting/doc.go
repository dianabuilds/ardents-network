// Package hosting owns one shared durable provider-period budget and opaque
// work/termination reservations. Reserve commits capacity before work and
// Release refunds only reserved capacity after its consumer joins that work.
// Sample shares bounded confirmed observations; it cannot authorize work.
// Hosting grants no token permission, placement or network authority.
package hosting
