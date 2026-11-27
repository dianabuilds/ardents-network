package admission

import (
	"slices"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
)

func TestAuthorityKeepsDistinctNetworkBindings(t *testing.T) {
	notBefore := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	profile := network.ProfileFacts{
		Network:              [32]byte{1},
		Generation:           [32]byte{2},
		EpochDigest:          [32]byte{3},
		Digest:               [32]byte{4},
		Epoch:                7,
		NotBefore:            notBefore,
		NotAfter:             notBefore.Add(time.Hour),
		IssuanceAuthorityKey: [32]byte{5},
		IssuerNodeID:         [32]byte{6},
		IssuerDutyGeneration: 9,
	}
	authority := AuthorityFactsFromProfile(profile)
	if authority.NetworkID != profile.Network || authority.StateGeneration != profile.Generation ||
		authority.StateDigest != profile.EpochDigest || authority.Digest != profile.Digest {
		t.Fatal("Admission received a different Network, State or profile binding")
	}
	if authority.IssuanceAuthorityKey != profile.IssuanceAuthorityKey || authority.IssuerNodeID != profile.IssuerNodeID {
		t.Fatal("Admission received a different permission authority or Issuer identity")
	}
	if authority.IssuerDutyGeneration != profile.IssuerDutyGeneration || authority.Epoch != profile.Epoch ||
		!authority.NotBefore.Equal(profile.NotBefore) || !authority.NotAfter.Equal(profile.NotAfter) {
		t.Fatal("Admission received a different Issuer duty, epoch or validity interval")
	}
}

func TestAuthorityRetainsPublicTokenKeysAfterSourceChanges(t *testing.T) {
	hour := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	controlKey := network.TokenKey{WindowStart: hour, Class: uint8(admission.ControlClass)}
	forwardingKey := network.TokenKey{WindowStart: hour.Add(time.Hour), Class: uint8(admission.ForwardClass)}
	// Mark both ends of each opaque encoding to detect truncation or reordering.
	controlKey.SPKI[0], controlKey.SPKI[len(controlKey.SPKI)-1] = 1, 2
	forwardingKey.SPKI[0], forwardingKey.SPKI[len(forwardingKey.SPKI)-1] = 3, 4
	keys := []network.TokenKey{controlKey, forwardingKey}

	original := slices.Clone(keys)
	authority := AuthorityFactsFromProfile(network.ProfileFacts{TokenKeys: keys})
	if !slices.Equal(keys, original) {
		t.Fatal("Admission projection changed the original Network keys")
	}
	authority.TokenKeys[0].SPKI[0] ^= 0xff
	if !slices.Equal(keys, original) {
		t.Fatal("changing projected keys changed the original Network keys")
	}
	authority.TokenKeys[0].SPKI[0] ^= 0xff
	clear(keys)

	if int(authority.TokenKeyCount) != len(original) ||
		authority.TokenKeys[0] != admission.TokenKey(controlKey) ||
		authority.TokenKeys[1] != admission.TokenKey(forwardingKey) {
		t.Fatal("Admission lost or reordered the original public token keys")
	}
}
