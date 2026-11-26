package provisioning

import "errors"

// ErrNetworkPreparationOutput reports an incomplete or refused output attempt.
var ErrNetworkPreparationOutput = errors.New("network preparation output unavailable")

// PreparedNetworkArtifact identifies the exact completed unsigned artifact.
type PreparedNetworkArtifact struct {
	Name   string
	Digest [32]byte
}
