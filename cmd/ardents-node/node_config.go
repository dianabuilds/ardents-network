package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
	"github.com/dianabuilds/ardents-network/internal/nodelifecycle"
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
	state networkstate.Config
	node  nodelifecycle.Config
}

func readNodePlan(path string) (nodeRuntime, error) {
	var err error
	var plan nodePlan
	if err := decodeNodeJSON(path, 64<<10, &plan); err != nil || plan.Schema != "ardents-h3-node-plan-v1" || len(plan.Sources) != 2 || len(plan.AuthorityPublic) == 0 || len(plan.AuthorityPublic) > 16 {
		return nodeRuntime{}, errors.New("node plan is not canonical or complete")
	}
	state := networkstate.Config{Root: plan.StateRoot, Threshold: plan.Threshold, Authorities: make(map[[32]byte]ed25519.PublicKey), Clock: time.Now,
		SourceMaterializationIndex: plan.MaterializationIndex, AutomaticRefreshInterval: 5 * time.Second, ClockObservationFile: plan.ClockObservationFile}
	if err := decodeNodeHex(plan.NetworkID, state.NetworkID[:]); err != nil {
		return nodeRuntime{}, err
	}
	for _, encoded := range plan.AuthorityPublic {
		public := make([]byte, ed25519.PublicKeySize)
		if err := decodeNodeHex(encoded, public); err != nil {
			return nodeRuntime{}, err
		}
		state.Authorities[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	if err := decodeNodeHex(plan.OrderSeed, state.SourceOrderSeed[:]); err != nil {
		return nodeRuntime{}, err
	}
	if state.SourceClientCertificate, err = loadNodeKeyPair(plan.SourceClientCertificate, plan.SourceClientKey); err != nil {
		return nodeRuntime{}, err
	}
	for index, source := range plan.Sources {
		state.SourceAddresses[index], state.SourceServerNames[index] = source.Address, source.ServerName
		state.SourceFamilies[index], state.SourceEndpointHandles[index] = source.Family, source.EndpointHandle
		if err := decodeNodeHex(source.Identity, state.SourceIdentities[index][:]); err != nil {
			return nodeRuntime{}, err
		}
		if err := decodeNodeHex(source.LeafKeyDigest, state.SourceLeafKeyDigests[index][:]); err != nil {
			return nodeRuntime{}, err
		}
		if state.SourceRootPEM[index], err = readNodeFile(source.RootCA, 64<<10); err != nil {
			return nodeRuntime{}, err
		}
	}
	node, err := loadNodeIdentity(plan, state.NetworkID)
	if err != nil {
		return nodeRuntime{}, err
	}
	return nodeRuntime{state: state, node: node}, nil
}
