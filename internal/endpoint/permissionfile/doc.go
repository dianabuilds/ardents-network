// Package permissionfile owns the owner-private Linux file handover for one
// public permission request and its separately approved response. It verifies
// canonical paths, ownership, no-follow opens, exact retries, and durability.
// Endpoint owns the live permission, Custody approval, and context lifetime.
package permissionfile
