//go:build e2e

package cli

import (
	"context"
	"io"
)

// RunWithIOForTest exposes the real root parser/dispatcher to tagged process
// tests that need a persistent shell input stream.
func RunWithIOForTest(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runWithIO(ctx, args, stdin, stdout, stderr)
}
