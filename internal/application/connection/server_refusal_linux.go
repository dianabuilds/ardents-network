//go:build linux

package connection

import (
	"context"
	"errors"
	"io"
)

// refusal stays with the protected Linux server admission: it classifies
// setupCleanupError, which the Linux-only server_admission.go owns. It is not a
// portable projection until that admission type is itself ported. The pure
// Refuse constructor is the portable server_refusal.go projection.
func refusal(cause error) Outcome {
	var cleanup setupCleanupError
	if errors.As(cause, &cleanup) {
		return cleanup.outcome
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return Outcome{Class: LocalTimeout, Reason: "local Application setup timed out"}
	}
	if errors.Is(cause, context.Canceled) {
		return Outcome{Class: LocalCancellation, Reason: "local Application setup was cancelled"}
	}
	var classified SetupRefusalError
	if errors.As(cause, &classified) {
		return classified.outcome
	}
	return Outcome{Class: ServiceUnavailable, Reason: "Endpoint could not open the selected Target Link"}
}

func WriteLocalStreamRefusal(writer io.Writer, cause error) error {
	if _, err := writer.Write([]byte{0}); err != nil {
		return err
	}
	return writeTerminal(writer, refusal(cause))
}
