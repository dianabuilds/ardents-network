package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type importPlan struct {
	StateRoot          string   `json:"state_root"`
	NetworkStateRoot   string   `json:"network_state_root"`
	InviteFile         string   `json:"invite_file"`
	NetworkID          string   `json:"network_id"`
	NetworkAuthorities []string `json:"network_authorities"`
	NetworkThreshold   int      `json:"network_threshold"`
	NetworkProfile     string   `json:"network_profile"`
	LocalRoleStateRoot string   `json:"local_role_state_root"`
	TimeConfidenceFile string   `json:"time_confidence_file"`
}
type importRuntime struct {
	config                    entry.Config
	inviteFile, localRoleRoot string
	close                     func() error
}

func loadImportPlan(path string, clock func() time.Time) (importRuntime, error) {
	var raw importPlan
	if err := decodeOperatorInput(path, 16<<10, &raw); err != nil {
		return importRuntime{}, err
	}
	if raw.StateRoot == "" || raw.NetworkStateRoot == "" || raw.InviteFile == "" || raw.NetworkProfile == "" || raw.LocalRoleStateRoot == "" ||
		raw.TimeConfidenceFile == "" {
		return importRuntime{}, errors.New("import plan is incomplete")
	}
	var networkID [32]byte
	if err := decodeOperatorFixedHex(raw.NetworkID, networkID[:]); err != nil {
		return importRuntime{}, fmt.Errorf("network_id: %w", err)
	}
	authorities, err := decodeOperatorAuthorities(raw.NetworkAuthorities, 16)
	if err != nil {
		return importRuntime{}, fmt.Errorf("network_authorities: %w", err)
	}
	network, err := state.Open(state.Config{
		Root: raw.NetworkStateRoot, NetworkID: networkID, Authorities: authorities,
		Threshold: raw.NetworkThreshold, AcceptedProfile: raw.NetworkProfile, Clock: clock,
	})
	if err != nil {
		return importRuntime{}, fmt.Errorf("open authenticated Network State: %w", err)
	}
	if _, currentErr := network.Current(); currentErr != nil {
		_ = network.Close()
		return importRuntime{}, fmt.Errorf("read authenticated Network State: %w", currentErr)
	}
	config := entry.Config{
		Root: raw.StateRoot, Current: func() (entry.View, error) {
			current, currentErr := network.Current()
			if currentErr != nil {
				return entry.View{}, currentErr
			}
			return entryView(current), nil
		}, Clock: clock,
		TimeConfident: freshOperatorRegularFile(raw.TimeConfidenceFile, clock, 2*time.Second),
		Conflict: func(identity, family [32]byte) (bool, error) {
			return duty.ReadConflict(raw.LocalRoleStateRoot, clock, identity, family)
		},
	}
	return importRuntime{config: config, inviteFile: raw.InviteFile, localRoleRoot: raw.LocalRoleStateRoot,
		close: network.Close}, nil
}

func entryView(current state.Snapshot) entry.View {
	view := entry.View{NetworkID: current.NetworkID, Epoch: current.Epoch, Digest: current.Digest,
		Profile: current.Profile, Fresh: current.Freshness == "fresh"}
	for _, candidate := range current.Candidates[:current.CandidateCount] {
		view.Candidates = append(view.Candidates, entry.Candidate{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey,
			KeyID: candidate.KeyID, FamilyID: candidate.FamilyID, RecordDigest: candidate.RecordDigest,
			DomainProofDigest: candidate.DomainProofDigest, Endpoint: candidate.Endpoint, Capacity: candidate.Capacity,
			Domain: candidate.Domain, ValidFrom: candidate.ValidFrom, ValidUntil: candidate.ValidUntil,
			AssignmentNotAfter: candidate.AssignmentNotAfter})
	}
	return view
}
