//go:build linux

// Package server owns the protected Linux Endpoint's local Application
// listener, attachment admission and terminal framing. It accepts the shared
// endpointstream contract, joins admitted work and retires only its own socket.
// It does not grant worker, Route or Service authority.
package server
