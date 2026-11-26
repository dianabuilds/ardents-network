//go:build !linux && !darwin

package provisioning

import (
	"context"
	"errors"
	"io"
)

// The report writer composes the Unix-family artifacts receipt port; every
// other platform receives the same explicit typed refusal (ADR-0129, #859).
func writeNetworkArtifactReport(context.Context, io.Writer, []byte) error {
	return errors.New("network public preparation unavailable")
}
