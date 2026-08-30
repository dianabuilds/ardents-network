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
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
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
func runHeadlessRuntime(ctx context.Context, path string, output io.Writer) (runErr error) {
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
	network, err := state.Open(networkConfig)
	if err != nil {
		return fmt.Errorf("open authenticated Network State: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, network.Close()) }()
	if refreshState {
		if _, err := network.Refresh(ctx); err != nil {
			return fmt.Errorf("refresh authenticated Network State: %w", err)
		}
	}
	now := clock().UTC()
	view, err := network.CurrentResolution()
	if err != nil {
		return fmt.Errorf("read current Network State resolution: %w", err)
	}
	epoch, available := view.Epoch(now, now.Add(time.Second))
	if !available || len(epoch.Authorities) == 0 {
		return errors.New("current Network State resolution is unavailable")
	}
	confident := freshOperatorRegularFile(plan.TimeConfidenceFile, clock, 2*time.Second)
	if !confident() {
		return errors.New("participant time confidence is unavailable")
	}
	owner, err := entry.Open(entry.Config{Root: plan.EntryStateRoot, Current: func() (entry.View, error) {
		current, currentErr := network.Current()
		if currentErr != nil {
			return entry.View{}, currentErr
		}
		return entryView(current), nil
	}, Conflict: func(identity, family [32]byte) (bool, error) {
		return duty.ReadConflict(plan.LocalRoleStateRoot, clock, identity, family)
	}, Clock: clock, TimeConfident: confident})
	if err != nil {
		return fmt.Errorf("open participant Entry state: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, owner.Close()) }()
	if _, err := owner.Contact(); err != nil {
		return fmt.Errorf("read current participant Entry contact: %w", err)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: plan.AlphaCorpusStateRoot,
		Authority: plan.AlphaCorpusAuthority, Cohort: plan.AlphaCohort, Network: plan.NetworkID})
	if err != nil {
		return fmt.Errorf("open alpha corpus floor: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, floor.Close()) }()
	corpus, err := floor.Current()
	if err != nil || corpus.ValidAt(now) != nil {
		return errors.New("accepted alpha corpus is unavailable")
	}
	serviceAuthority := append(ed25519.PublicKey(nil), epoch.Authorities[0].PublicKey[:]...)
	endpoint, err := endpointapi.New(endpointapi.Setup{NetworkID: plan.NetworkID, BrokerID: plan.BrokerID,
		AuthorityPublic: serviceAuthority, IntroductionPublic: serviceAuthority, ConnectionPrincipal: plan.ConnectionPrincipal,
		AdministrationPrincipal: plan.AdministrationPrincipal, PublicationRoot: plan.PublicationRoot,
		TransitAcquisitionRoot:       plan.TransitAcquisitionRoot,
		CreateTransitAcquisitionRoot: true, Clock: clock})
	if err != nil {
		return fmt.Errorf("open participant Endpoint: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, endpoint.Close()) }()
	connectionOwner, err := endpoint.OpenConnectionInterface(endpointapi.ConnectionInterfaceConfig{Floor: floor,
		Current: func() (endpointapi.ApplicationStateView, error) { return network.CurrentResolution() }, Entry: owner,
		Principal: plan.ConnectionPrincipal, BytesEachDirection: plan.BytesEachDirection, Clock: clock})
	if err != nil {
		return fmt.Errorf("open headless Connection Interface owner: %w", err)
	}
	application, err := endpointapi.OpenLocalConnectionInterface(plan.ApplicationSocket, connectionOwner)
	if err != nil {
		return fmt.Errorf("open headless local Connection Interface: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, application.Close()) }()
	administrationOwner, err := endpoint.OpenServiceAdministration(endpointapi.ServiceAdministrationConfig{
		Principal: plan.AdministrationPrincipal, Clock: clock})
	if err != nil {
		return fmt.Errorf("open headless Service Administration owner: %w", err)
	}
	administration, err := endpointapi.OpenLocalServiceAdministration(plan.AdministrationSocket, administrationOwner)
	if err != nil {
		return fmt.Errorf("open headless local Service Administration: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, administration.Close()) }()
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(headlessRuntimeEvent{Kind: "headless-runtime-ready", NetworkID: hex.EncodeToString(plan.NetworkID[:]),
		ApplicationSocket: plan.ApplicationSocket, AdministrationSocket: plan.AdministrationSocket}); err != nil {
		return err
	}
	<-ctx.Done()
	return encoder.Encode(headlessRuntimeEvent{Kind: "headless-runtime-stopped", NetworkID: hex.EncodeToString(plan.NetworkID[:])})
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
