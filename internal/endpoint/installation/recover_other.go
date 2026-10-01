//go:build !linux

package installation

import (
	"context"
	"errors"
)

func recoverInstalled(context.Context, string) (ProvisionResult, error) {
	return ProvisionResult{}, errors.New("protected Endpoint recovery requires admitted Ubuntu LTS amd64")
}
