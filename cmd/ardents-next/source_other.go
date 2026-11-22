//go:build !linux

package main

import (
	"context"
	"io"
)

func runSource(_ context.Context, _ []string, _, diagnostic io.Writer) int {
	return unsupportedPlatformRefusal(diagnostic, "source")
}

func runSourceRepair(_ context.Context, _ []string, _, diagnostic io.Writer) int {
	return unsupportedPlatformRefusal(diagnostic, "source.repair-retained")
}

func runSourceStatePreparation(_ context.Context, _ []string, _, diagnostic io.Writer) int {
	return unsupportedPlatformRefusal(diagnostic, "source.prepare-state")
}
