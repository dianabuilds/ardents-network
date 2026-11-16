//go:build !linux && !darwin

package main

import (
	"context"
	"io"
)

func runNodeRelease(_ context.Context, _ []string, out io.Writer) int {
	return enrollmentReport(out, "release-incompatible", 1)
}
