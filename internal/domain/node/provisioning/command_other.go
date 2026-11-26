//go:build !linux

package provisioning

import (
	"context"
	"errors"
)

func PrepareInitialNetworkKeys(context.Context, string, string) ([]byte, error) {
	return nil, errors.New("initial Network key preparation requires Linux")
}
