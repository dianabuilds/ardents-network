//go:build !linux

package provisioning

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
)

func PrepareNetworkArtifact(context.Context, string, string, string) (PreparedNetworkArtifact, error) {
	return PreparedNetworkArtifact{}, networkdraft.ErrUnavailable
}
