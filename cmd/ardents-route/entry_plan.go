package main

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/bridge"
	"github.com/dianabuilds/ardents-network/internal/camouflage"
	"github.com/dianabuilds/ardents-network/internal/localroles"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

type entryPlan struct {
	Schema              string   `json:"schema"`
	BridgeStateRoot     string   `json:"bridge_state_root"`
	NetworkStateRoot    string   `json:"network_state_root"`
	NetworkID           string   `json:"network_id"`
	NetworkAuthorities  []string `json:"network_authorities"`
	NetworkThreshold    int      `json:"network_threshold"`
	NetworkProfile      string   `json:"network_profile"`
	RouteProfile        string   `json:"route_profile"`
	LocalRoleStateRoot  string   `json:"local_role_state_root"`
	Binary              string   `json:"binary"`
	CandidateStateRoot  string   `json:"candidate_state_root"`
	Deadline            string   `json:"deadline"`
	RouteManifestDigest string   `json:"route_manifest_digest"`
	TransitionHandle    string   `json:"transition_handle"`
}

func loadEntryPlan(path string) (runtime *entryRuntime, runErr error) {
	var raw entryPlan
	if err := planfile.Decode(path, 32<<10, &raw); err != nil {
		return nil, err
	}
	if raw.Schema != "ardents-h3-bridge-entry-plan-v1" || raw.BridgeStateRoot == "" ||
		raw.NetworkStateRoot == "" || raw.NetworkProfile == "" || raw.RouteProfile == "" ||
		raw.LocalRoleStateRoot == "" || raw.Binary == "" || raw.CandidateStateRoot == "" {
		return nil, errors.New("bridge entry plan is not canonical or complete")
	}
	var networkID [32]byte
	if err := planfile.FixedHex(raw.NetworkID, networkID[:]); err != nil {
		return nil, err
	}
	authorities, err := planfile.Authorities(raw.NetworkAuthorities, 16)
	if err != nil {
		return nil, err
	}
	deadline, err := time.Parse(time.RFC3339, raw.Deadline)
	if err != nil {
		return nil, err
	}
	transition, err := openInheritedPipe(raw.TransitionHandle)
	if err != nil {
		return nil, err
	}
	defer func() {
		if runErr != nil {
			runErr = errors.Join(runErr, transition.Close())
		}
	}()
	network, err := state.Open(state.Config{Root: raw.NetworkStateRoot, NetworkID: networkID,
		Authorities: authorities, Threshold: raw.NetworkThreshold, AcceptedProfile: raw.NetworkProfile, Clock: time.Now})
	if err != nil {
		return nil, err
	}
	defer func() {
		if runErr != nil {
			runErr = errors.Join(runErr, network.Close())
		}
	}()
	roles, err := localroles.Open(localroles.Config{Root: raw.LocalRoleStateRoot, Clock: time.Now})
	if err != nil {
		return nil, err
	}
	defer func() {
		if runErr != nil {
			runErr = errors.Join(runErr, roles.Close())
		}
	}()
	config := bridge.Config{Root: raw.BridgeStateRoot, RouteProfile: raw.RouteProfile,
		CurrentNetwork: network.Current, Clock: time.Now, RoleConflict: roles.Conflict,
		ValidateCandidate: func(value []byte, identity [32]byte) ([32]byte, string, error) {
			candidate, validateErr := camouflage.Validate(value, identity)
			return candidate.Commitment(), "webtunnel-v0.0.6", validateErr
		}}
	bridgeOwner, err := bridge.Open(config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if runErr != nil {
			runErr = errors.Join(runErr, bridgeOwner.Close())
		}
	}()
	runtime = &entryRuntime{bridge: bridgeOwner, closeNetwork: network.Close, closeRoles: roles.Close,
		transition: transition, deadline: deadline,
		client: camouflage.Client{Binary: raw.Binary, StateRoot: raw.CandidateStateRoot, Deadline: deadline}}
	if err := planfile.FixedHex(raw.RouteManifestDigest, runtime.manifest[:]); err != nil {
		return nil, err
	}
	return runtime, nil
}
