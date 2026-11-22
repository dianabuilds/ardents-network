package main

import (
	"slices"
	"strings"
)

// uniquePublicOptions rejects repeated named options after the command's flag
// parser has accepted the argument shape. Commands name their repeatable options
// explicitly and keep the validation of each value.
func uniquePublicOptions(args []string, repeatable ...string) bool {
	seen := make(map[string]bool)
	for _, argument := range args {
		if len(argument) < 2 || argument[0] != '-' {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimLeft(argument, "-"), "=")
		if name == "" {
			continue
		}
		if seen[name] && !slices.Contains(repeatable, name) {
			return false
		}
		seen[name] = true
	}
	return true
}
