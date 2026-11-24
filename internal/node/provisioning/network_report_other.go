//go:build !linux

package provisioning

import (
	"context"
	"errors"
	"io"
)

func writeNetworkArtifactReport(context.Context, io.Writer, []byte) error {
	return errors.New("network public preparation unavailable")
}
