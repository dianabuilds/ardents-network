//go:build linux

// Package permissionfile owns the owner-private Linux file handover for one
// public permission request and its separately approved response. It verifies
// canonical paths, ownership, no-follow opens, exact retries, and durability.
// Admission stock owns the live permission, Custody owns approval, and Endpoint
// owns the authorized context lifetime and bounded wait.
package permissionfile
