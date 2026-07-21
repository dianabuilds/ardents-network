// Package network owns live participation contracts, reachability, peers, carrier operations, and limits.
// It does not own product encryption, discovery records, or content semantics.
package network

import (
	"os"
	"strings"
)

func ResolveBindAddress(explicit string) string {
	if address := strings.TrimSpace(explicit); address != "" {
		return address
	}
	if address := strings.TrimSpace(os.Getenv(BindAddressEnv)); address != "" {
		return address
	}
	return "0.0.0.0"
}
