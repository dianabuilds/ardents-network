//go:build !linux

package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/execution"
)

func (*Owner) LaunchByteStream(context.Context, [32]byte, execution.Surface) (*Invocation, error) {
	return nil, errors.New("byte-stream worker requires the selected Ubuntu profile")
}
