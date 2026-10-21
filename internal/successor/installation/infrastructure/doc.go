//go:build linux

// Package infrastructure binds exact Source artifact bytes to two fresh coherent
// Release authorizations and separate Enrollment initial provenance. It owns
// Source-specific read-only preflight policy, original input custody and closed
// template rendering. The native systemd adapter owns the original connection.
// It owns no installed transaction, account creation, mutable runtime root or Start.
package infrastructure
