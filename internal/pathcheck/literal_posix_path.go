package pathcheck

import (
	"path"
	"strings"
)

// LiteralPOSIXPath admits a canonical path for a literal argument in a fixed
// system unit, regardless of the host used to render or check its declaration.
func LiteralPOSIXPath(value string) bool {
	if !path.IsAbs(value) || path.Clean(value) != value || value == "/" || len(value) > 4096 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("/._-", c)) {
			return false
		}
	}
	return true
}
