//go:build linux

package endpoint

import "sync"

// textContextGuard protects Endpoint context admission and the lazy Linux
// launch reservation. The reservation is used by the installed-worker launcher
// and remains owned by this Endpoint across individual context lifetimes.
type textContextGuard struct {
	sync.Mutex
	launch chan struct{}
}
