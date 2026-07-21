// Package peer owns live peer connection and reachability facts.
// It does not own discovery records or route policy.
package peer

import "strings"

func Normalize(addr string) (string, bool) {
	addr = strings.TrimSpace(addr)
	return addr, strings.HasPrefix(addr, "/")
}
