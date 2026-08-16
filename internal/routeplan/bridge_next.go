package routeplan

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/planfile"
)

// BridgeNext returns only the exact Initiator listener committed by one
// validated, single-Attachment role manifest.
func BridgeNext(path string, manifest [32]byte) (string, error) {
	var value actorPlan
	if manifest == ([32]byte{}) || planfile.Decode(path, 64<<10, &value) != nil ||
		value.Role != "initiator" || value.attachmentCount() != 1 || value.Listen == "" {
		return "", errors.New("bridge next Initiator manifest is invalid")
	}
	if err := value.validateRoleLocal(); err != nil {
		return "", err
	}
	var encoded [32]byte
	if err := fixedHex(value.ManifestDigest, encoded[:]); err != nil || encoded != manifest {
		return "", errors.New("bridge next Initiator manifest does not match")
	}
	return value.Listen, nil
}
