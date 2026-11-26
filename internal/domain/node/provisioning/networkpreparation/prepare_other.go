//go:build !linux

package networkpreparation

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
)

func Prepare(context.Context, string, string, string) (Artifact, error) {
	return Artifact{}, networkdraft.ErrUnavailable
}
