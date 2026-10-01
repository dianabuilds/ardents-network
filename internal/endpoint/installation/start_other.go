//go:build !linux

package installation

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/endpoint/runtimeplan"
)

func admitInstalledStart(context.Context, string) (runtimeplan.DecodedHeadless, error) {
	return runtimeplan.DecodedHeadless{}, errors.New("installed Endpoint start requires admitted Ubuntu LTS amd64")
}
