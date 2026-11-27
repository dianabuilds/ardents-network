//go:build !linux

package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

// No qualification input can be constructed on a host where launch refuses.
type streamQualificationInput struct{}

func (*Owner) LaunchStreamQualification(context.Context, [32]byte, string, streamqualification.Profile, [32]byte) (*Invocation, error) {
	return nil, errors.New("stream qualification native launch is unavailable")
}
