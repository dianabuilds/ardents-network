// Package durableroot owns the private filesystem root mechanics shared by
// Endpoint durable journals: owner-only access, an exclusive root lease, and
// directory sync after durable replacement. Callers retain their own marker,
// file-shape, recovery, and lifetime rules.
package durableroot
