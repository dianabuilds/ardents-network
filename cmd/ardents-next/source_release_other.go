//go:build !linux && !darwin

package main

import (
	"context"
	"io"
)

func runSourceRelease(_ context.Context, _ []string, out io.Writer) int {
	return enrollmentReport(out, "release-incompatible", 1)
}
