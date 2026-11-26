//go:build !linux

package provisioning

import "context"

func PrepareNetworkArtifact(context.Context, string, string, string) (PreparedNetworkArtifact, error) {
	return PreparedNetworkArtifact{}, errNetworkPreparation
}
