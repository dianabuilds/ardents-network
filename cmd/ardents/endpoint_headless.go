package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// headlessRuntimePlan contains the non-route local inputs required to retain
// one named-alpha participant runtime and its local Connection Interface. It
// deliberately has no Target, Descriptor, Gateway, Node endpoint, Grant,
// certificate, Browser, or presentation input.
type headlessRuntimePlan struct {
	Schema           string `json:"schema"`
	NetworkStateRoot string `json:"network_state_root"`
	// NetworkSourcePlan is an optional existing direct-Source plan. When it
	// is present, this runtime owns the State root's initial refresh and its
	// automatic refresh loop; a separate process cannot share that root lease.
	NetworkSourcePlan       string   `json:"network_source_plan,omitempty"`
	EntryStateRoot          string   `json:"entry_state_root"`
	TransitAcquisitionRoot  string   `json:"transit_acquisition_root"`
	ApplicationSocket       string   `json:"application_socket"`
	AdministrationSocket    string   `json:"administration_socket"`
	PublicationRoot         string   `json:"publication_root"`
	ServiceInstanceRoot     string   `json:"service_instance_root,omitempty"`
	AlphaCorpusStateRoot    string   `json:"alpha_corpus_state_root"`
	LocalRoleStateRoot      string   `json:"local_role_state_root"`
	TimeConfidenceFile      string   `json:"time_confidence_file"`
	NetworkID               string   `json:"network_id"`
	NetworkAuthorities      []string `json:"network_authorities"`
	NetworkThreshold        int      `json:"network_threshold"`
	NetworkProfile          string   `json:"network_profile"`
	AlphaCorpusAuthority    string   `json:"alpha_corpus_authority"`
	AlphaCohort             string   `json:"alpha_cohort"`
	BrokerID                string   `json:"broker_id"`
	ConnectionPrincipal     string   `json:"connection_principal"`
	AdministrationPrincipal string   `json:"administration_principal"`
	BytesEachDirection      uint32   `json:"bytes_each_direction"`
}

// runHeadlessRuntime owns Network State, Entry, Endpoint, and one private local
// Connection Interface without loading Browser or presentation code.
func runHeadlessRuntime(ctx context.Context, path string, output io.Writer) error {
	if ctx == nil || path == "" || output == nil {
		return errors.New("headless runtime input is incomplete")
	}
	plan, err := loadHeadlessRuntimePlan(path)
	if err != nil {
		return err
	}
	clock := time.Now
	networkConfig, refreshState, err := headlessNetworkConfig(plan, clock)
	if err != nil {
		return err
	}
	confident := freshOperatorRegularFile(plan.TimeConfidenceFile, clock, 2*time.Second)
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return endpointapi.RunParticipant(ctx, endpointapi.ParticipantRuntimeConfig{Network: networkConfig, RefreshNetwork: refreshState,
		EntryRoot: plan.EntryStateRoot, TransitAcquisitionRoot: plan.TransitAcquisitionRoot,
		AlphaCorpusRoot: plan.AlphaCorpusStateRoot, AlphaCorpusAuthority: plan.AlphaCorpusAuthority, AlphaCohort: plan.AlphaCohort,
		LocalRoleRoot: plan.LocalRoleStateRoot, ApplicationAddress: plan.ApplicationSocket,
		AdministrationAddress: plan.AdministrationSocket, PublicationRoot: plan.PublicationRoot,
		ServiceInstanceRoot: plan.ServiceInstanceRoot, BrokerID: plan.BrokerID,
		ConnectionPrincipal: plan.ConnectionPrincipal, AdministrationPrincipal: plan.AdministrationPrincipal,
		BytesEachDirection: plan.BytesEachDirection, Clock: clock, TimeConfident: confident,
		Observe: func(event endpointapi.ParticipantRuntimeEvent) error {
			return encoder.Encode(headlessRuntimeEvent{Kind: "headless-runtime-" + event.Kind,
				NetworkID: hex.EncodeToString(event.NetworkID[:]), ApplicationSocket: event.ApplicationAddress,
				AdministrationSocket: event.AdministrationAddress})
		}})
}

// headlessNetworkConfig preserves one owner for a State root. A static
// root is useful for deliberately offline qualification. A participant that
// supplies a source plan must give the same State trust anchors and local
// clock owner to this runtime; it cannot make a second process mutate the
// retained root underneath the participant runtime.
func headlessNetworkConfig(plan decodedHeadlessRuntimePlan, clock func() time.Time) (state.Config, bool, error) {
	if clock == nil {
		return state.Config{}, false, errors.New("headless runtime clock is unavailable")
	}
	if plan.NetworkSourcePlan == "" {
		return state.Config{Root: plan.NetworkStateRoot, NetworkID: plan.NetworkID, Authorities: plan.NetworkAuthorities,
			Threshold: plan.NetworkThreshold, AcceptedProfile: route.Profile, Clock: clock}, false, nil
	}
	config, err := readSourcePlan(plan.NetworkStateRoot, plan.NetworkSourcePlan)
	if err != nil {
		return state.Config{}, false, fmt.Errorf("read participant Network State source plan: %w", err)
	}
	if !matchesHeadlessSourcePlan(plan, config) {
		return state.Config{}, false, errors.New("participant Network State source plan does not match the headless runtime")
	}
	config.AcceptedProfile, config.Clock = route.Profile, clock
	return config, true, nil
}

func matchesHeadlessSourcePlan(plan decodedHeadlessRuntimePlan, config state.Config) bool {
	return config.NetworkID == plan.NetworkID && config.Threshold == plan.NetworkThreshold &&
		sameAuthorities(config.Authorities, plan.NetworkAuthorities) && sameOperatorPath(config.LocalRoleStateRoot, plan.LocalRoleStateRoot) &&
		sameOperatorPath(config.ClockObservationFile, plan.TimeConfidenceFile) && config.AutomaticRefreshInterval > 0
}

func sameAuthorities(left, right map[[32]byte]ed25519.PublicKey) bool {
	if len(left) != len(right) {
		return false
	}
	for identity, public := range left {
		other, found := right[identity]
		if !found || string(public) != string(other) {
			return false
		}
	}
	return true
}

func sameOperatorPath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftPath, leftErr := filepath.Abs(left)
	rightPath, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && leftPath == rightPath
}

type headlessRuntimeEvent struct {
	Kind                 string `json:"kind"`
	NetworkID            string `json:"network_id"`
	ApplicationSocket    string `json:"application_socket,omitempty"`
	AdministrationSocket string `json:"administration_socket,omitempty"`
}

type decodedHeadlessRuntimePlan struct {
	headlessRuntimePlan
	NetworkID, BrokerID, ConnectionPrincipal, AdministrationPrincipal [32]byte
	NetworkAuthorities                                                map[[32]byte]ed25519.PublicKey
	AlphaCorpusAuthority                                              ed25519.PublicKey
}

func loadHeadlessRuntimePlan(path string) (decodedHeadlessRuntimePlan, error) {
	var raw headlessRuntimePlan
	if err := decodeOperatorInput(path, 16<<10, &raw); err != nil {
		return decodedHeadlessRuntimePlan{}, err
	}
	if raw.Schema != "ardents-headless-runtime-v1" || raw.NetworkStateRoot == "" || raw.EntryStateRoot == "" || raw.TransitAcquisitionRoot == "" ||
		raw.ApplicationSocket == "" || !filepath.IsAbs(raw.ApplicationSocket) || raw.AdministrationSocket == "" || !filepath.IsAbs(raw.AdministrationSocket) ||
		raw.ApplicationSocket == raw.AdministrationSocket || raw.PublicationRoot == "" ||
		raw.AlphaCorpusStateRoot == "" || raw.LocalRoleStateRoot == "" || raw.TimeConfidenceFile == "" || raw.NetworkProfile != route.Profile ||
		raw.AlphaCohort == "" || raw.BrokerID == "" || raw.ConnectionPrincipal == "" || raw.AdministrationPrincipal == "" || raw.BytesEachDirection == 0 {
		return decodedHeadlessRuntimePlan{}, errors.New("headless runtime plan is incomplete")
	}
	result := decodedHeadlessRuntimePlan{headlessRuntimePlan: raw}
	for _, field := range []struct {
		encoded     string
		destination []byte
	}{{raw.NetworkID, result.NetworkID[:]}, {raw.BrokerID, result.BrokerID[:]}, {raw.ConnectionPrincipal, result.ConnectionPrincipal[:]},
		{raw.AdministrationPrincipal, result.AdministrationPrincipal[:]}} {
		if err := decodeOperatorFixedHex(field.encoded, field.destination); err != nil {
			return decodedHeadlessRuntimePlan{}, err
		}
	}
	authorities, err := decodeOperatorAuthorities(raw.NetworkAuthorities, 16)
	if err != nil {
		return decodedHeadlessRuntimePlan{}, err
	}
	result.NetworkAuthorities = authorities
	corpusAuthority := make(ed25519.PublicKey, ed25519.PublicKeySize)
	if err := decodeOperatorFixedHex(raw.AlphaCorpusAuthority, corpusAuthority); err != nil {
		return decodedHeadlessRuntimePlan{}, err
	}
	result.AlphaCorpusAuthority = corpusAuthority
	return result, nil
}
