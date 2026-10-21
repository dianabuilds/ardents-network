//go:build linux

// Package infrastructure binds exact Source artifact bytes to two fresh coherent
// Release authorizations and separate Enrollment initial provenance. It owns
// Source-specific read-only preflight policy, original input custody and closed
// template rendering and one initial stopped installation transition with its
// separate closed journal, account/credential births, immutable generation and
// fixed-unit selection. The native systemd adapter owns the original connection
// and one reload. It owns no State contents, continuation, runtime or Start.
package infrastructure
