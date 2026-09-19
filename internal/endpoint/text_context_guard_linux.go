//go:build linux

package endpoint

import "sync"

// textContextGuard protects Endpoint context admission.
type textContextGuard struct {
	sync.Mutex
}
