package main

import (
	"path"
	"strings"
)

// sourceControlPath checks the declared Linux target path without opening it.
func sourceControlPath(value string) bool {
	return value != "/" && len(value) > 1 && len(value) <= 4096 && path.IsAbs(value) && path.Clean(value) == value && !strings.ContainsRune(value, 0)
}
