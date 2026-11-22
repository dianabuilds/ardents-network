package main

import (
	"crypto/sha256"

	networkdomain "github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

// nodeProfilePreparation holds one role's detached verified facts: the closed
// profile input, the authentic Epoch decision, the State signing context and
// the unsigned body. Every field is a shared public codec type; the role plan
// that produces it keeps its own schema, authority pins and validator.
type nodeProfilePreparation struct {
	input    closedprofile.Input
	decision epoch.Decision
	context  closedprofile.Context
	body     []byte
}

func (p nodeProfilePreparation) profileBody() []byte { return p.body }

func (p nodeProfilePreparation) profileContext() closedprofile.Context { return p.context }

// Adapt detached verified facts to the actual Network binding. No role mapping,
// membership policy, trusted clock, durable root or runtime right lives here.
func (p nodeProfilePreparation) bind(digest [32]byte) error {
	i := p.input
	binding := networkdomain.ProfileBinding{Network: i.NetworkID, Generation: i.StateGeneration, EpochDigest: i.EpochDigest, Digest: digest, Epoch: i.Epoch, NotBefore: i.NotBefore, NotAfter: i.NotAfter}
	facts := networkdomain.ProfileFacts{ProfileBinding: binding, IssuanceAuthorityKey: i.IssuanceAuthorityKey, IssuerNodeID: i.IssuerNodeID}
	var assignments []networkdomain.Assignment
	for _, node := range i.Nodes {
		assignments = append(assignments, networkdomain.Assignment{NodeID: node.NodeID, RecordDigest: node.RecordDigest, Generation: node.DutyGeneration, RoleDomain: node.RoleDomain, Subrole: node.Subrole})
		if node.NodeID == i.IssuerNodeID {
			facts.IssuerDutyGeneration = node.DutyGeneration
		}
	}
	var records []networkdomain.NodeRecord
	for _, record := range p.decision.Candidates {
		records = append(records, networkdomain.NodeRecord{NodeID: record.NodeID, RecordDigest: record.RecordDigest, Generation: record.RecordGeneration, PublicKey: record.PublicKey, FamilyID: record.FamilyID, KeyID: record.KeyID, DomainProofDigest: sha256.Sum256(record.DomainProof), Endpoint: record.Endpoint, CarrierProfile: record.CarrierProfile, Capacity: record.Capacity, Assignment: record.Domain, ValidFrom: record.ValidFrom, ValidUntil: record.ValidUntil, AssignmentNotAfter: record.AssignmentNotAfter})
	}
	for _, key := range i.TokenKeys {
		public := networkdomain.TokenKey{WindowStart: key.WindowStart, Class: key.Class}
		copy(public.SPKI[:], key.SPKI)
		facts.TokenKeys = append(facts.TokenKeys, public)
	}
	membership, err := networkdomain.BindMembership(binding, assignments, records)
	if err != nil {
		return err
	}
	header := p.decision.Header
	_, err = networkdomain.BindAcceptedState(networkdomain.EpochFacts{Network: header.NetworkID, Number: header.Number, Digest: header.Digest, Previous: header.Previous, ValidFrom: header.ValidFrom, ValidUntil: header.ValidUntil}, i.StateGeneration, facts, membership)
	return err
}
