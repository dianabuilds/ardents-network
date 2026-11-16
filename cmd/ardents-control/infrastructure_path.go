package main

import (
	"errors"
	"path"
	"strings"
)

var errSourceControlInput = errors.New("original source preparation input unavailable")

// sourceControlPath checks the declared Linux target path without opening it.
func sourceControlPath(value string) bool {
	return value != "/" && len(value) > 1 && len(value) <= 4096 && path.IsAbs(value) && path.Clean(value) == value && !strings.ContainsRune(value, 0)
}
