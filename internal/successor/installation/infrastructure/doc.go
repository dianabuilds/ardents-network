//go:build linux

// Package infrastructure binds exact Source artifact bytes to two fresh coherent
// Release authorizations and separate Enrollment initial provenance. It owns
// Source-specific read-only preflight policy, original input custody and closed
// template rendering and one initial stopped installation transition with its
// separate closed journal, account/credential births, immutable generation and
// fixed-unit selection. The native systemd adapter owns the original connection
// and one reload. A distinct guarded first-start transaction revalidates the
// original selected resources, current principal and fresh Release proofs;
// independent non-root Startup owns its own observations and root exchange.
// It owns no State contents, serving authority or restart/recovery continuation.
package infrastructure
