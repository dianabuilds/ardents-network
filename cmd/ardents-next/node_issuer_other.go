//go:build !linux

package main

import (
	"context"
	"io"
)

// Refuse before reading private configuration or opening any runtime owner.
func runNodeIssuer(_ context.Context, _ []string, out, _ io.Writer) int {
	if nodeIssuerReport(out, "refused", "unsupported-platform", "", "") != nil {
		return 2
	}
	return 1
}
