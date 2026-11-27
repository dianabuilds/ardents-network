//go:build linux

package administration

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

// PresentLocalLink presents a locally authorized Link on the owned console.
func PresentLocalLink(ctx context.Context, socket string) error {
	output, err := endpointstream.OpenLocalConsoleOutput()
	if err != nil {
		return err
	}
	return PresentPublishedLink(ctx, socket, output)
}
