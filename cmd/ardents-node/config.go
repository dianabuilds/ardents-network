package main

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

type sourceServerPlan struct {
	Schema               string   `json:"schema"`
	StateRoot            string   `json:"state_root"`
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

func openSource(path string, emit func([]byte) error) (interface {
	Current() (state.Snapshot, error)
	Wait(context.Context) error
	Close() error
}, error) {
	var err error
	var plan sourceServerPlan
	if err := planfile.Decode(path, 32<<10, &plan); err != nil || plan.Schema != "ardents-h3-source-server-v1" {
		return nil, errors.New("source server plan is not canonical")
	}
	if len(plan.ClientKeyDigests) == 0 || len(plan.ClientKeyDigests) > 3 {
		return nil, errors.New("source server trust-map count is invalid")
	}
	config := state.Config{Root: plan.StateRoot, Threshold: plan.Threshold,
		Source: source.Config{ServeAddress: plan.Listen, MaterialIndex: plan.MaterializationIndex}, RuntimeProfile: plan.RuntimeProfile}
	config.ObserveResources = emit
	if err := planfile.FixedHex(plan.NetworkID, config.NetworkID[:]); err != nil {
		return nil, err
	}
	config.Authorities, err = planfile.Authorities(plan.AuthorityPublic, 16)
	if err != nil {
		return nil, err
	}
	config.Now, err = time.Parse(time.RFC3339, plan.At)
	if err != nil {
		return nil, err
	}
	config.Source.ServeCertificate, err = planfile.KeyPair(plan.ServerCertificate, plan.ServerKey)
	if err != nil {
		return nil, err
	}
	config.Source.ServeClientRootPEM, err = planfile.Read(plan.ClientRoot, 64<<10)
	if err != nil {
		return nil, err
	}
	config.Source.ServeClientKeyDigests, err = planfile.Digests(plan.ClientKeyDigests, 3)
	if err != nil {
		return nil, err
	}
	return state.Open(config)
}
