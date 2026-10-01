//go:build !linux

package installation

import (
	"context"
	"errors"
)

func upgradeInstalled(context.Context, string) (ProvisionResult, error) {
	return ProvisionResult{}, errors.New("protected Endpoint upgrade requires admitted Ubuntu LTS amd64")
}
