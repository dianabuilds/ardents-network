//go:build linux

package endpoint

import "sync"

// dutyContextGuard protects Endpoint context admission.
type dutyContextGuard struct {
	sync.Mutex
}
