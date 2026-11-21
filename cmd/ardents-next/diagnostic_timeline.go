package main

import (
	"context"
	"fmt"
	"io"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/timeline"
)

func runDiagnostics(ctx context.Context, args []string, input io.ReadCloser, output, diagnostic io.Writer) int {
	if len(args) != 1 || args[0] != "timeline" {
		fmt.Fprintln(diagnostic, "usage: ardents-next diagnostics timeline")
		return 2
	}
	if err := timeline.Project(ctx, input, output); err != nil {
		fmt.Fprintln(diagnostic, err)
		return 1
	}
	return 0
}
