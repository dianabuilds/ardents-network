package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/bridge"
	"github.com/dianabuilds/ardents-network/internal/localroles"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

type importPlan struct {
	StateRoot          string   `json:"state_root"`
	NetworkStateRoot   string   `json:"network_state_root"`
	InviteFile         string   `json:"invite_file"`
	NetworkID          string   `json:"network_id"`
	NetworkAuthorities []string `json:"network_authorities"`
	NetworkThreshold   int      `json:"network_threshold"`
	NetworkProfile     string   `json:"network_profile"`
	RouteProfile       string   `json:"route_profile"`
	LocalRoleStateRoot string   `json:"local_role_state_root"`
}

func loadImportPlan(path string, clock func() time.Time) (bridge.Config, string, func() error, error) {
	var raw importPlan
	if err := planfile.Decode(path, 16<<10, &raw); err != nil {
		return bridge.Config{}, "", nil, err
	}
	if raw.StateRoot == "" || raw.NetworkStateRoot == "" || raw.InviteFile == "" ||
		raw.RouteProfile == "" || raw.NetworkProfile == "" || raw.LocalRoleStateRoot == "" {
		return bridge.Config{}, "", nil, errors.New("import plan is incomplete")
	}
	var networkID [32]byte
	if err := planfile.FixedHex(raw.NetworkID, networkID[:]); err != nil {
		return bridge.Config{}, "", nil, fmt.Errorf("network_id: %w", err)
	}
	authorities, err := planfile.Authorities(raw.NetworkAuthorities, 16)
	if err != nil {
		return bridge.Config{}, "", nil, fmt.Errorf("network_authorities: %w", err)
	}
	network, err := state.Open(state.Config{
		Root: raw.NetworkStateRoot, NetworkID: networkID, Authorities: authorities,
		Threshold: raw.NetworkThreshold, AcceptedProfile: raw.NetworkProfile, Clock: clock,
	})
	if err != nil {
		return bridge.Config{}, "", nil, fmt.Errorf("open authenticated Network State: %w", err)
	}
	if _, currentErr := network.Current(); currentErr != nil {
		_ = network.Close()
		return bridge.Config{}, "", nil, fmt.Errorf("read authenticated Network State: %w", currentErr)
	}
	roles, err := localroles.Open(localroles.Config{Root: raw.LocalRoleStateRoot, Clock: clock})
	if err != nil {
		_ = network.Close()
		return bridge.Config{}, "", nil, fmt.Errorf("open current local role state: %w", err)
	}
	return bridge.Config{
		Root: raw.StateRoot, RouteProfile: raw.RouteProfile, CurrentNetwork: network.Current, Clock: clock,
		RoleConflict: roles.Conflict,
	}, raw.InviteFile, func() error { return errors.Join(roles.Close(), network.Close()) }, nil
}
