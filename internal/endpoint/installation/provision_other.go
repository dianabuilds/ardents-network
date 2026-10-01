//go:build !linux

package installation

import (
	"context"
	"errors"
)

func provisionInitial(context.Context, string) (ProvisionResult, error) {
	return ProvisionResult{}, errors.New("protected Endpoint provisioning requires admitted Ubuntu LTS amd64")
}
