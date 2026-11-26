package provisioning

import "errors"

// ErrNetworkPreparationOutput reports an incomplete or refused output attempt.
var ErrNetworkPreparationOutput = errors.New("network preparation output unavailable")

// PreparedNetworkArtifact identifies the exact completed unsigned artifact.
type PreparedNetworkArtifact struct {
	Name   string
	Digest [32]byte
}

const MaximumNetworkPreparationPlan = 8 << 20

// IsNetworkPreparation recognizes the fixed public preparation purposes.
func IsNetworkPreparation(operation string) bool {
	return operation == "prepare-initial-node-record" || operation == "prepare-initial-epoch" || operation == "prepare-closed-profile"
}

// PrepareNetworkPublic builds exact detached signing bytes from a public plan.
func PrepareNetworkPublic(operation string, raw []byte) (string, []byte, error) {
	switch operation {
	case "prepare-initial-node-record":
		_, message, err := PrepareInitialNodeRecord(raw)
		return "node-record.signing-input", message, err
	case "prepare-initial-epoch":
		_, message, err := PrepareInitialEpoch(raw)
		return "epoch.unsigned", message, err
	case "prepare-closed-profile":
		_, message, err := PrepareClosedNetworkProfile(raw)
		return "profile.unsigned", message, err
	default:
		return "", nil, errNetworkPreparation
	}
}
