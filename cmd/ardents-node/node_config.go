package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
)

const legacyRendezvousDedicatedHostResourceProfile = "h4-5-rendezvous-alpha-v1"

type nodePlan struct {
	sourceServerPlan
	ClockObservationFile    string             `json:"clock_observation_file"`
	OrderSeed               string             `json:"order_seed"`
	SourceClientCertificate string             `json:"source_client_certificate"`
	SourceClientKey         string             `json:"source_client_key"`
	Sources                 []nodeSource       `json:"sources"`
	NodeID                  string             `json:"node_id"`
	IdentityKey             string             `json:"identity_key"`
	MaximumDutyMS           uint32             `json:"maximum_duty_ms"`
	DrainTimeoutMS          uint32             `json:"drain_timeout_ms"`
	NodeResourceProfile     string             `json:"node_resource_profile,omitempty"`
	DiagnosticDirectory     string             `json:"diagnostic_directory,omitempty"`
	Rendezvous              *rendezvousPlan    `json:"rendezvous,omitempty"`
	Initiator               *initiatorPlan     `json:"initiator,omitempty"`
	Introduction            *introductionPlan  `json:"introduction,omitempty"`
	Responder               *responderPlan     `json:"responder,omitempty"`
	TransitIssuer           *transitIssuerPlan `json:"transit_issuer,omitempty"`
}

// rendezvousPlan contains only the local finite work bounds. State still
// supplies the listener, Node role, peer identities, epoch, and expiry.
type rendezvousPlan struct {
	LoopbackListenOverride string `json:"listen_loopback_override,omitempty"`
	HandshakeLimit         uint16 `json:"handshake_limit"`
	WaitingLimit           uint16 `json:"waiting_limit"`
	PairLimit              uint16 `json:"pair_limit"`
	PairByteLimit          uint64 `json:"pair_byte_limit"`
	AdmissionTimeoutMS     uint32 `json:"admission_timeout_ms"`
	DrainTimeoutMS         uint32 `json:"drain_timeout_ms"`
}

// initiatorPlan provides only finite local limits. State selects the listener,
// peers, and time bounds; Entry remains a separately owned future composition.
type initiatorPlan struct {
	HandshakeLimit     uint16 `json:"handshake_limit"`
	RelayLimit         uint16 `json:"relay_limit"`
	RelayByteLimit     uint64 `json:"relay_byte_limit"`
	AdmissionTimeoutMS uint32 `json:"admission_timeout_ms"`
	DrainTimeoutMS     uint32 `json:"drain_timeout_ms"`
}

// introductionPlan supplies finite local reservations only. Its admission
// verifier is always constructed from current State, never plan bytes.
type introductionPlan struct {
	HandshakeLimit     uint16 `json:"handshake_limit"`
	SlotLimit          uint16 `json:"slot_limit"`
	DeliveryLimit      uint16 `json:"delivery_limit"`
	AdmissionTimeoutMS uint32 `json:"admission_timeout_ms"`
	DrainTimeoutMS     uint32 `json:"drain_timeout_ms"`
}

// responderPlan supplies finite local relay reservations only. State selects
// its exact Rendezvous peer and its grant verifier derives from State.
type responderPlan struct {
	HandshakeLimit     uint16 `json:"handshake_limit"`
	RelayLimit         uint16 `json:"relay_limit"`
	RelayByteLimit     uint64 `json:"relay_byte_limit"`
	AdmissionTimeoutMS uint32 `json:"admission_timeout_ms"`
	DrainTimeoutMS     uint32 `json:"drain_timeout_ms"`
}

type transitIssuerPlan struct {
	Root            string `json:"root"`
	ConnectionLimit uint16 `json:"connection_limit"`
	DrainTimeoutMS  uint32 `json:"drain_timeout_ms"`
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
	state               state.Config
	node                node.Config
	diagnosticDirectory string
	clockObservation    string
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
	nativeDuty := plan.Rendezvous != nil || plan.Initiator != nil || plan.Introduction != nil || plan.Responder != nil || plan.TransitIssuer != nil
	if plan.NodeResourceProfile == legacyRendezvousDedicatedHostResourceProfile {
		plan.NodeResourceProfile = node.RendezvousDedicatedHostResourceProfile
	}
	if plan.NativeRendezvousProfile && !nativeDuty {
		return nodeRuntime{}, errors.New("native Route State profile requires one local native duty")
	}
	// H3 resource profiles were calibrated for retired role-probe duties. The
	// sole selected native profile is purpose-bound to one Rendezvous process.
	if nativeDuty && plan.NodeResourceProfile != "" {
		if plan.NodeResourceProfile != node.RendezvousDedicatedHostResourceProfile {
			return nodeRuntime{}, errors.New("native Route Node resource profile is unselected")
		}
		if plan.Rendezvous == nil || plan.Initiator != nil || plan.Introduction != nil || plan.Responder != nil || plan.TransitIssuer != nil {
			return nodeRuntime{}, errors.New("functional-alpha resource profile requires only one Rendezvous duty")
		}
	}
	if plan.DiagnosticDirectory != "" && (!filepath.IsAbs(plan.DiagnosticDirectory) || filepath.Clean(plan.DiagnosticDirectory) != plan.DiagnosticDirectory) {
		return nodeRuntime{}, errors.New("node diagnostic directory must be one clean absolute path")
	}
	state := state.Config{Root: plan.StateRoot, LocalRoleStateRoot: plan.LocalRoleStateRoot,
		Threshold: plan.Threshold, Authorities: make(map[[32]byte]ed25519.PublicKey), Clock: time.Now,
		Source:                   source.Config{MaterialIndex: plan.MaterializationIndex},
		AutomaticRefreshInterval: 5 * time.Second, ClockObservationFile: plan.ClockObservationFile}
	if err := decodeOperatorFixedHex(plan.NetworkID, state.NetworkID[:]); err != nil {
		return nodeRuntime{}, err
	}
	if nativeDuty {
		state.AcceptedProfile = route.Profile
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
	nodeConfig, err := loadNodeIdentity(plan, state.NetworkID)
	if err != nil {
		return nodeRuntime{}, err
	}
	nodeConfig.NetworkStateRoot = plan.StateRoot
	clockObservation := ""
	if plan.NodeResourceProfile == node.RendezvousDedicatedHostResourceProfile {
		clockObservation = plan.ClockObservationFile
	}
	return nodeRuntime{state: state, node: nodeConfig, diagnosticDirectory: plan.DiagnosticDirectory,
		clockObservation: clockObservation}, nil
}
