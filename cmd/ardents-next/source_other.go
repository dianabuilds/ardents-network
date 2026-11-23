//go:build !linux

package main

import (
	"context"
	"io"
)

func runSource(_ context.Context, _ []string, _, diagnostic io.Writer) int {
	return unsupportedPlatformRefusal(diagnostic, "source")
}

func runRepair(_ context.Context, role string, _ []string, out, diagnostic io.Writer) int {
	if role == "source" {
		return unsupportedPlatformRefusal(diagnostic, "source.repair-retained")
	}
	return unsupportedPlatformRefusal(out, "node."+role)
}

func runSourceStatePreparation(_ context.Context, _ []string, _, diagnostic io.Writer) int {
	return unsupportedPlatformRefusal(diagnostic, "source.prepare-state")
}

func runSourceInspection(_ context.Context, _ []string, _, diagnostic io.Writer) int {
	return unsupportedPlatformRefusal(diagnostic, "source.inspect-latest")
}
