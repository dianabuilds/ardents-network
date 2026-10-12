//go:build !linux

package main

import (
	"context"
	"io"
)

// Native refusal precedes config, roots, keys and State effects.
func runNodeForwarding(_ context.Context, _ []string, out, _ io.Writer) int {
	if nodeReport(out, "refused", "unsupported-platform") != nil {
		return 2
	}
	return 1
}
