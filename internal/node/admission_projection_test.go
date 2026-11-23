package node

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network"
)

// The projection copies one coherent public Network observation into detached
// admission facts. It constructs no authority; each field must arrive unchanged.
func TestAdmissionFactsProjectOriginalNetworkObservation(t *testing.T) {
	window := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	notBefore := window.Add(-time.Hour)
	notAfter := window.Add(time.Hour)
	var spki [346]byte
	spki[0], spki[345] = 0x30, 0x01
	profile := network.ProfileFacts{
		ProfileBinding: network.ProfileBinding{
			Network:     [32]byte{1},
			Generation:  [32]byte{2},
			EpochDigest: [32]byte{3},
			Digest:      [32]byte{4},
			Epoch:       7,
			NotBefore:   notBefore,
			NotAfter:    notAfter,
		},
		IssuanceAuthorityKey: [32]byte{5},
		IssuerNodeID:         [32]byte{6},
		IssuerDutyGeneration: 9,
		TokenKeys: []network.TokenKey{
			{WindowStart: window, Class: 2, SPKI: spki},
			{WindowStart: window.Add(time.Hour), Class: 1, SPKI: [346]byte{9}},
		},
	}
	facts := admissionFacts(profile)
	if facts.NetworkID != profile.Network || facts.StateGeneration != profile.Generation ||
		facts.StateDigest != profile.EpochDigest || facts.Digest != profile.Digest ||
		facts.IssuanceAuthorityKey != profile.IssuanceAuthorityKey || facts.IssuerNodeID != profile.IssuerNodeID ||
		facts.IssuerDutyGeneration != profile.IssuerDutyGeneration || facts.Epoch != profile.Epoch ||
		!facts.NotBefore.Equal(notBefore) || !facts.NotAfter.Equal(notAfter) {
		t.Fatal("public profile binding changed in projection", facts)
	}
	if facts.TokenKeyCount != 2 {
		t.Fatal("token inventory count changed", facts.TokenKeyCount)
	}
	for i, key := range profile.TokenKeys {
		if facts.TokenKeys[i].WindowStart != key.WindowStart || facts.TokenKeys[i].Class != key.Class || facts.TokenKeys[i].SPKI != key.SPKI {
			t.Fatal("token key changed in projection", i)
		}
	}
	// Mutating the projected copy must not reach the original observation.
	facts.TokenKeys[0].Class = 3
	facts.TokenKeys[0].SPKI[0] = 0
	if profile.TokenKeys[0].Class != 2 || profile.TokenKeys[0].SPKI[0] != 0x30 {
		t.Fatal("projection aliased the original token inventory")
	}
}
