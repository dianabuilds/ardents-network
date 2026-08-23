package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
)

type nodePlan struct {
	sourceServerPlan
	ClockObservationFile    string       `json:"clock_observation_file"`
	OrderSeed               string       `json:"order_seed"`
	SourceClientCertificate string       `json:"source_client_certificate"`
	SourceClientKey         string       `json:"source_client_key"`
	Sources                 []nodeSource `json:"sources"`
	NodeID                  string       `json:"node_id"`
	IdentityKey             string       `json:"identity_key"`
	MaximumDutyMS           uint32       `json:"maximum_duty_ms"`
	DrainTimeoutMS          uint32       `json:"drain_timeout_ms"`
	NodeResourceProfile     string       `json:"node_resource_profile,omitempty"`
}
type nodeSource struct {
	Address        string `json:"address"`
	ServerName     string `json:"server_name"`
	Identity       string `json:"identity"`
	Family         string `json:"family"`
	EndpointHandle string `json:"endpoint_handle"`
	RootCA         string `json:"root_ca"`
	LeafKeyDigest  string `json:"leaf_key_digest"`
}
type nodeRuntime struct {
	state state.Config
	node  node.Config
}

func readNodePlan(path string) (nodeRuntime, error) {
	var err error
	var plan nodePlan
	if err := decodeOperatorInput(path, 64<<10, &plan); err != nil {
		return nodeRuntime{}, fmt.Errorf("decode node plan: %w", err)
	}
	if plan.Schema != "ardents-node-plan-v1" || plan.LocalRoleStateRoot == "" || len(plan.Sources) != 2 || len(plan.AuthorityPublic) == 0 || len(plan.AuthorityPublic) > 16 {
		return nodeRuntime{}, errors.New("node plan is not canonical or complete")
	}
	state := state.Config{Root: plan.StateRoot, LocalRoleStateRoot: plan.LocalRoleStateRoot,
		Threshold: plan.Threshold, Authorities: make(map[[32]byte]ed25519.PublicKey), Clock: time.Now,
		Source: source.Config{MaterialIndex: plan.MaterializationIndex}, AutomaticRefreshInterval: 5 * time.Second, ClockObservationFile: plan.ClockObservationFile}
	if err := decodeOperatorFixedHex(plan.NetworkID, state.NetworkID[:]); err != nil {
		return nodeRuntime{}, err
	}
	for _, encoded := range plan.AuthorityPublic {
		public := make([]byte, ed25519.PublicKeySize)
		if err := decodeOperatorFixedHex(encoded, public); err != nil {
			return nodeRuntime{}, err
		}
		state.Authorities[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	if err := decodeOperatorFixedHex(plan.OrderSeed, state.Source.OrderSeed[:]); err != nil {
		return nodeRuntime{}, err
	}
	if state.Source.ClientCertificate, err = readOperatorKeyPair(plan.SourceClientCertificate, plan.SourceClientKey); err != nil {
		return nodeRuntime{}, err
	}
	for index, source := range plan.Sources {
		declared := &state.Source.Sources[index]
		declared.Address, declared.ServerName = source.Address, source.ServerName
		declared.Family, declared.EndpointHandle = source.Family, source.EndpointHandle
		if err := decodeOperatorFixedHex(source.Identity, declared.Identity[:]); err != nil {
			return nodeRuntime{}, err
		}
		if err := decodeOperatorFixedHex(source.LeafKeyDigest, declared.LeafKeyDigest[:]); err != nil {
			return nodeRuntime{}, err
		}
		if declared.RootPEM, err = readOperatorInput(source.RootCA, 64<<10); err != nil {
			return nodeRuntime{}, err
		}
	}
	node, err := loadNodeIdentity(plan, state.NetworkID)
	if err != nil {
		return nodeRuntime{}, err
	}
	return nodeRuntime{state: state, node: node}, nil
}
