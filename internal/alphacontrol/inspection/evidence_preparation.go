package inspection

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

// PrepareInitialEvidence encodes one mutually bound initial disclosure set.
// It authenticates nothing: Release and Network owners must independently
// verify its inputs, and each component must receive its own signature.
func PrepareInitialEvidence(release ReleaseEvidence, network NetworkEvidence, compatibility CompatibilityEvidence) ([3][]byte, error) {
	var result [3][]byte
	header, err := epoch.Inspect(network.Epoch)
	if err != nil || header.NetworkID != network.NetworkID || header.Digest != network.EpochDigest ||
		header.Profile != network.Profile || header.Number != compatibility.NetworkEpoch {
		return result, errors.New("initial alpha disclosure epoch identity disagrees")
	}
	if compatibility.ReleaseDigest != release.ArtifactDigest ||
		compatibility.ReleaseBuildIdentity != release.BuildIdentity ||
		compatibility.ProtocolPhase != release.ProtocolPhase ||
		compatibility.NetworkDigest != network.EpochDigest ||
		compatibility.NetworkProfile != network.Profile {
		return result, errors.New("initial alpha disclosure evidence bindings disagree")
	}
	result[0], err = encodeReleaseEvidence(release)
	if err != nil {
		return [3][]byte{}, err
	}
	result[1], err = encodeNetworkEvidence(network)
	if err != nil {
		return [3][]byte{}, err
	}
	result[2], err = encodeCompatibilityEvidence(compatibility)
	if err != nil {
		return [3][]byte{}, err
	}
	return result, nil
}
