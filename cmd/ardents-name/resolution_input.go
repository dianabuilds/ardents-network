package main

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameresolution"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

func (input resolutionInput) runtimeValues() (state.Config, nameresolution.Selection, error) {
	config := state.Config{Root: input.StateRoot, Threshold: input.AuthorityThreshold,
		AcceptedProfile: input.AcceptedProfile}
	if err := planfile.FixedHex(input.NetworkID, config.NetworkID[:]); err != nil {
		return config, nameresolution.Selection{}, err
	}
	var err error
	config.Authorities, err = planfile.Authorities(input.AuthorityPublic, 16)
	if err != nil {
		return config, nameresolution.Selection{}, err
	}
	selection := nameresolution.Selection{ExcludedFamilies: append([]string(nil), input.ExcludedFamilies...)}
	if selection.At, err = time.Parse(time.RFC3339Nano, input.SelectionAt); err != nil {
		return config, selection, errors.New("selection_at is not canonical RFC3339")
	}
	if selection.Deadline, err = time.Parse(time.RFC3339Nano, input.Deadline); err != nil ||
		!selection.At.Before(selection.Deadline) || selection.Deadline.After(selection.At.Add(15*time.Second)) {
		return config, selection, errors.New("resolution deadline is invalid")
	}
	if selection.At.Format(time.RFC3339Nano) != input.SelectionAt || selection.Deadline.Format(time.RFC3339Nano) != input.Deadline {
		return config, selection, errors.New("resolution clocks are non-canonical")
	}
	config.Now = selection.At
	identities := []struct {
		raw    string
		target *[32]byte
	}{{input.RelayNodeID, &selection.RelayNodeID}, {input.GatewayNodeID, &selection.GatewayNodeID},
		{input.ConnectionRendezvousNodeID, &selection.ConnectionRendezvousNodeID}}
	for _, identity := range identities {
		if err := planfile.FixedHex(identity.raw, identity.target[:]); err != nil {
			return config, selection, err
		}
	}
	for _, raw := range input.ExcludedIdentities {
		var identity [32]byte
		if err := planfile.FixedHex(raw, identity[:]); err != nil {
			return config, selection, err
		}
		selection.ExcludedIdentities = append(selection.ExcludedIdentities, identity)
	}
	return config, selection, nil
}
