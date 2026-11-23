//go:build !linux

package provisioning

import (
	"context"
	"os"
)

func SignNetwork(context.Context, []string, func(string, *os.File)) (*NetworkSigningResult, error) {
	return nil, errNetworkSigning
}
