package release

import (
	"errors"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// verifyEmergencyThreshold reuses go-tuf signature verification with the
// H3 emergency threshold. Ordinary target metadata needs three of five
// signatures; a target carrying emergency policy needs four of the same five.
func verifyEmergencyThreshold(root *metadata.Metadata[metadata.RootType], targetsBytes []byte) error {
	if root == nil {
		return errors.New("trusted root is missing")
	}
	rootBytes, err := root.ToBytes(false)
	if err != nil {
		return err
	}
	thresholdRoot, err := metadata.Root().FromBytes(rootBytes)
	if err != nil {
		return err
	}
	role := thresholdRoot.Signed.Roles[targetRole]
	if role == nil || len(role.KeyIDs) != totalTopLevelKeys {
		return errors.New("targets role does not have the H3 five-key profile")
	}
	role.Threshold = emergencyThreshold
	targets, err := metadata.Targets().FromBytes(targetsBytes)
	if err != nil {
		return err
	}
	return thresholdRoot.VerifyDelegate(targetRole, targets)
}
