//go:build linux

package node

import (
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

// Project one coherent public Network observation. This constructs no authority;
// each original role rechecks its genuine State before lending these facts.
func admissionFacts(p network.ProfileFacts) admission.AuthorityFacts {
	facts := admission.AuthorityFacts{NetworkID: p.Network, StateGeneration: p.Generation, StateDigest: p.EpochDigest, Digest: p.Digest, IssuanceAuthorityKey: p.IssuanceAuthorityKey, IssuerNodeID: p.IssuerNodeID, IssuerDutyGeneration: p.IssuerDutyGeneration, Epoch: p.Epoch, NotBefore: p.NotBefore, NotAfter: p.NotAfter, TokenKeyCount: uint8(len(p.TokenKeys))}
	for i, key := range p.TokenKeys {
		facts.TokenKeys[i] = admission.TokenKey{WindowStart: key.WindowStart, Class: key.Class, SPKI: key.SPKI}
	}
	return facts
}
