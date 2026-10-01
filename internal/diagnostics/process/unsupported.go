//go:build !linux

package process

import (
	"context"
	"errors"
)

func open(context.Context, string) (func() error, error) {
	return nil, errors.New("live process diagnostics require Linux")
}
