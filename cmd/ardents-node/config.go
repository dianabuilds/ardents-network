package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
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
}

func openSource(path string) (interface {
	Current() (networkstate.Snapshot, error)
	Wait(context.Context) error
	Close() error
}, error) {
	var err error
	var plan sourceServerPlan
	if err := decodeNodeJSON(path, 32<<10, &plan); err != nil || plan.Schema != "ardents-h3-source-server-v1" {
		return nil, errors.New("source server plan is not canonical")
	}
	if len(plan.AuthorityPublic) == 0 || len(plan.AuthorityPublic) > 16 || len(plan.ClientKeyDigests) == 0 || len(plan.ClientKeyDigests) > 3 {
		return nil, errors.New("source server trust-map count is invalid")
	}
	config := networkstate.Config{Root: plan.StateRoot, Threshold: plan.Threshold, Authorities: make(map[[32]byte]ed25519.PublicKey), ServeAddress: plan.Listen, SourceMaterializationIndex: plan.MaterializationIndex}
	if err := decodeNodeHex(plan.NetworkID, config.NetworkID[:]); err != nil {
		return nil, err
	}
	for _, encoded := range plan.AuthorityPublic {
		public := make([]byte, ed25519.PublicKeySize)
		if err := decodeNodeHex(encoded, public); err != nil {
			return nil, err
		}
		config.Authorities[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	config.Now, err = time.Parse(time.RFC3339, plan.At)
	if err != nil {
		return nil, err
	}
	config.ServeCertificate, err = loadNodeKeyPair(plan.ServerCertificate, plan.ServerKey)
	if err != nil {
		return nil, err
	}
	config.ServeClientRootPEM, err = readNodeFile(plan.ClientRoot, 64<<10)
	if err != nil {
		return nil, err
	}
	for _, encoded := range plan.ClientKeyDigests {
		var pin [32]byte
		if err := decodeNodeHex(encoded, pin[:]); err != nil {
			return nil, err
		}
		config.ServeClientKeyDigests = append(config.ServeClientKeyDigests, pin)
	}
	return networkstate.Open(config)
}
