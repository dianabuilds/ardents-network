//go:build !linux

package main

import (
	"context"
	"io"
)

// Unsupported native platforms refuse before private configuration or effects.
func runNodeDescriptor(_ context.Context, _ []string, out, _ io.Writer) int {
	if nodeDescriptorReport(out, "refused", "unsupported-platform", "", "") != nil {
		return 2
	}
	return 1
}
