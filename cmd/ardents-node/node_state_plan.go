package main

import "errors"

// The unselected Node plan still embeds this legacy declaration. Source --config
// is retired before reading it; installed Source uses the separate bound grammar.
type sourceServerPlan struct {
	StateProfileAuthority   string   `json:"state_profile_authority,omitempty"`
	StateProfile            string   `json:"state_profile,omitempty"`
	Schema                  string   `json:"schema"`
	StateRoot               string   `json:"state_root"`
	LocalRoleStateRoot      string   `json:"local_role_state_root"`
	NetworkID               string   `json:"network_id"`
	AuthorityPublic         []string `json:"authority_public"`
	Threshold               int      `json:"threshold"`
	At                      string   `json:"at"`
	Listen                  string   `json:"listen"`
	ServerCertificate       string   `json:"server_certificate"`
	ServerKey               string   `json:"server_key"`
	ClientRoot              string   `json:"client_root"`
	ClientKeyDigests        []string `json:"client_key_digests"`
	MaterializationIndex    uint32   `json:"materialization_index"`
	NativeRendezvousProfile bool     `json:"native_rendezvous_profile,omitempty"`
	RuntimeProfile          string   `json:"runtime_profile,omitempty"`
}

var errOldSourceProfileRetired = errors.New("old Source profile is retired")
