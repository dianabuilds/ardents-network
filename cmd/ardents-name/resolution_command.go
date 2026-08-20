package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/dianabuilds/ardents-network/internal/nameresolution"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

const resolutionInputSchema = "ardents-private-resolution-input-v1"

type resolutionInput struct {
	Schema                     string                        `json:"schema"`
	StateRoot                  string                        `json:"state_root"`
	NetworkID                  string                        `json:"network_id"`
	AuthorityPublic            []string                      `json:"authority_public"`
	AuthorityThreshold         int                           `json:"authority_threshold"`
	AcceptedProfile            string                        `json:"accepted_profile"`
	SelectionAt                string                        `json:"selection_at"`
	Deadline                   string                        `json:"deadline"`
	RelayNodeID                string                        `json:"relay_node_id"`
	GatewayNodeID              string                        `json:"gateway_node_id"`
	ConnectionRendezvousNodeID string                        `json:"connection_rendezvous_node_id"`
	ExcludedIdentities         []string                      `json:"excluded_identities"`
	ExcludedFamilies           []string                      `json:"excluded_families"`
	GatewayProfile             nameresolution.GatewayProfile `json:"gateway_profile"`
}

type snapshotLoader func(state.Config) (state.Snapshot, error)

func runResolution(path, name string, isolation [32]byte, output io.Writer, transport *http.Transport, load snapshotLoader) error {
	input, config, selection, err := readResolutionInput(path)
	if err != nil {
		return err
	}
	view, err := load(config)
	if err != nil {
		return fmt.Errorf("open authenticated Network State: %w", err)
	}
	resolver, err := nameresolution.Open(view, selection, input.GatewayProfile, isolation, transport)
	if err != nil {
		return err
	}
	result, err := resolver.Resolve(context.Background(), name, selection.At)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "resolved %s\n", result.Record.Target)
	return err
}

func currentSnapshot(config state.Config) (state.Snapshot, error) {
	store, err := state.Open(config)
	if err != nil {
		return state.Snapshot{}, err
	}
	defer store.Close()
	return store.Current()
}

func readResolutionInput(path string) (resolutionInput, state.Config, nameresolution.Selection, error) {
	var input resolutionInput
	if err := planfile.Decode(path, maxResolutionInput, &input); err != nil {
		return input, state.Config{}, nameresolution.Selection{}, err
	}
	if input.Schema != resolutionInputSchema || input.StateRoot == "" || len(input.AuthorityPublic) == 0 ||
		len(input.AuthorityPublic) > 16 || len(input.ExcludedIdentities) > 64 || len(input.ExcludedFamilies) > 64 {
		return input, state.Config{}, nameresolution.Selection{}, errors.New("private resolution input is incomplete")
	}
	config, selection, err := input.runtimeValues()
	return input, config, selection, err
}
