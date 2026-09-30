package runtimeplan

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"time"
)

// Source declares an existing direct-Source configuration and credential paths.
// Decoding declarations never reads the referenced credentials.
type Source struct {
	Schema               string         `json:"schema"`
	NetworkID            string         `json:"network_id"`
	AuthorityPublic      []string       `json:"authority_public"`
	Threshold            int            `json:"threshold"`
	ClockObservedAt      string         `json:"clock_observed_at"`
	ClockObservationFile string         `json:"clock_observation_file,omitempty"`
	OrderSeed            string         `json:"order_seed"`
	MaterializationIndex uint32         `json:"materialization_index"`
	RefreshIntervalMS    uint32         `json:"refresh_interval_ms,omitempty"`
	RuntimeProfile       string         `json:"runtime_profile,omitempty"`
	LocalRoleStateRoot   string         `json:"local_role_state_root"`
	ClientCertificate    string         `json:"client_certificate"`
	ClientKey            string         `json:"client_key"`
	Sources              []SourceMember `json:"sources"`
}

// SourceMember declares one pinned direct-Source endpoint.
type SourceMember struct {
	Address        string `json:"address"`
	ServerName     string `json:"server_name"`
	Identity       string `json:"identity"`
	Family         string `json:"family"`
	EndpointHandle string `json:"endpoint_handle"`
	RootCA         string `json:"root_ca"`
	LeafKeyDigest  string `json:"leaf_key_digest"`
}

// DecodedSource contains checked public declarations, without TLS material.
// State remains responsible for accepting the resulting runtime configuration.
type DecodedSource struct {
	Source
	NetworkID, OrderSeed       [32]byte
	Authorities                map[[32]byte]ed25519.PublicKey
	ClockObservation           time.Time
	Identities, LeafKeyDigests [2][32]byte
}

// DecodeSource checks the existing bounded Source grammar without file or network effects.
func DecodeSource(input []byte) (DecodedSource, error) {
	if len(input) > 32<<10 {
		return DecodedSource{}, errors.New("source plan exceeds its bound")
	}
	var plan Source
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return DecodedSource{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return DecodedSource{}, errors.New("source plan contains trailing JSON")
	}
	if plan.Schema != "ardents-source-plan-v1" || plan.LocalRoleStateRoot == "" || len(plan.Sources) != 2 {
		return DecodedSource{}, errors.New("source plan is not canonical or complete")
	}
	result := DecodedSource{Source: plan}
	if err := decodeFixedHex(plan.NetworkID, result.NetworkID[:]); err != nil {
		return DecodedSource{}, err
	}
	var err error
	result.Authorities, err = decodeAuthorities(plan.AuthorityPublic, 16)
	if err != nil {
		return DecodedSource{}, err
	}
	result.ClockObservation, err = time.Parse(time.RFC3339, plan.ClockObservedAt)
	if err != nil {
		return DecodedSource{}, err
	}
	if err := decodeFixedHex(plan.OrderSeed, result.OrderSeed[:]); err != nil {
		return DecodedSource{}, err
	}
	for index, member := range plan.Sources {
		if err := decodeFixedHex(member.Identity, result.Identities[index][:]); err != nil {
			return DecodedSource{}, err
		}
		if err := decodeFixedHex(member.LeafKeyDigest, result.LeafKeyDigests[index][:]); err != nil {
			return DecodedSource{}, err
		}
	}
	return result, nil
}
