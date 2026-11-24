//go:build linux || darwin

package artifacts

import (
	"slices"
	"strings"
)

// UniqueOptions rejects repeated options except those explicitly repeated by the command.
func UniqueOptions(args []string, repeatable ...string) bool {
	seen := map[string]bool{}
	for _, argument := range args {
		if len(argument) < 2 || argument[0] != '-' {
			continue
		}
		name := strings.TrimLeft(argument, "-")
		name, _, _ = strings.Cut(name, "=")
		if seen[name] && !slices.Contains(repeatable, name) {
			return false
		}
		seen[name] = true
	}
	return true
}
