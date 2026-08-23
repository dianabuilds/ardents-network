package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type sourceServerPlan struct {
	Schema               string   `json:"schema"`
	StateRoot            string   `json:"state_root"`
	LocalRoleStateRoot   string   `json:"local_role_state_root"`
	NetworkID            string   `json:"network_id"`
	AuthorityPublic      []string `json:"authority_public"`
	Threshold            int      `json:"threshold"`
	At                   string   `json:"at"`
	Listen               string   `json:"listen"`
	ServerCertificate    string   `json:"server_certificate"`
	ServerKey            string   `json:"server_key"`
	ClientRoot           string   `json:"client_root"`
	ClientKeyDigests     []string `json:"client_key_digests"`
	MaterializationIndex uint32   `json:"materialization_index"`
	RuntimeProfile       string   `json:"runtime_profile,omitempty"`
}

type sourceStore interface {
	Current() (state.Snapshot, error)
	Wait(context.Context) error
	Close() error
}

func openSource(path string, emit func([]byte) error) (sourceStore, error) {
	var err error
	var plan sourceServerPlan
	if err := decodeOperatorInput(path, 32<<10, &plan); err != nil {
		return nil, fmt.Errorf("decode source server plan: %w", err)
	}
	if plan.Schema != "ardents-source-server-v1" || plan.LocalRoleStateRoot == "" {
		return nil, errors.New("source server plan is not canonical")
	}
	if len(plan.ClientKeyDigests) == 0 || len(plan.ClientKeyDigests) > 3 {
		return nil, errors.New("source server trust-map count is invalid")
	}
	config := state.Config{Root: plan.StateRoot, LocalRoleStateRoot: plan.LocalRoleStateRoot, Threshold: plan.Threshold,
		Source: source.Config{ServeAddress: plan.Listen, MaterialIndex: plan.MaterializationIndex}, RuntimeProfile: plan.RuntimeProfile}
	config.ObserveResources = emit
	if err := decodeOperatorFixedHex(plan.NetworkID, config.NetworkID[:]); err != nil {
		return nil, err
	}
	config.Authorities, err = decodeOperatorAuthorities(plan.AuthorityPublic, 16)
	if err != nil {
		return nil, err
	}
	config.Now, err = time.Parse(time.RFC3339, plan.At)
	if err != nil {
		return nil, err
	}
	config.Source.ServeCertificate, err = readOperatorKeyPair(plan.ServerCertificate, plan.ServerKey)
	if err != nil {
		return nil, err
	}
	config.Source.ServeClientRootPEM, err = readOperatorInput(plan.ClientRoot, 64<<10)
	if err != nil {
		return nil, err
	}
	config.Source.ServeClientKeyDigests, err = decodeOperatorDigests(plan.ClientKeyDigests, 3)
	if err != nil {
		return nil, err
	}
	store, err := state.Open(config)
	if err != nil {
		return nil, err
	}
	return store, nil
}
