//go:build linux

package administration

import (
	"context"

	appconsole "github.com/dianabuilds/ardents-network/internal/application/console"
)

// PresentLocalLink presents a locally authorized Link on the owned console.
func PresentLocalLink(ctx context.Context, socket string) error {
	output, err := appconsole.OpenOutput()
	if err != nil {
		return err
	}
	return PresentPublishedLink(ctx, socket, output)
}
