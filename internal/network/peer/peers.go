package peer

import "strings"

func Normalize(addr string) (string, bool) {
	addr = strings.TrimSpace(addr)
	return addr, strings.HasPrefix(addr, "/")
}

func Has(peers []string) bool {
	for _, peer := range peers {
		if _, ok := Normalize(peer); ok {
			return true
		}
	}
	return false
}
