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

	"github.com/dianabuilds/ardents-network/internal/browserentry"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// alphaBrowserRuntimePlan contains the non-route local inputs required to
// retain one named-alpha participant runtime. It deliberately has no Target,
// Descriptor, Gateway, Node endpoint, Grant, certificate, or browser URL.
type alphaBrowserRuntimePlan struct {
	Schema           string `json:"schema"`
	NetworkStateRoot string `json:"network_state_root"`
	// NetworkSourcePlan is an optional existing direct-Source plan. When it
	// is present, this runtime owns the State root's initial refresh and its
	// automatic refresh loop; a separate process cannot share that root lease.
	NetworkSourcePlan    string   `json:"network_source_plan,omitempty"`
	EntryStateRoot       string   `json:"entry_state_root"`
	AlphaCorpusStateRoot string   `json:"alpha_corpus_state_root"`
	LocalRoleStateRoot   string   `json:"local_role_state_root"`
	TimeConfidenceFile   string   `json:"time_confidence_file"`
	NetworkID            string   `json:"network_id"`
	NetworkAuthorities   []string `json:"network_authorities"`
	NetworkThreshold     int      `json:"network_threshold"`
	NetworkProfile       string   `json:"network_profile"`
	AlphaCorpusAuthority string   `json:"alpha_corpus_authority"`
	AlphaCohort          string   `json:"alpha_cohort"`
	BrokerID             string   `json:"broker_id"`
	ConnectionPrincipal  string   `json:"connection_principal"`
	BytesEachDirection   uint32   `json:"bytes_each_direction"`
}

// runAlphaBrowserRuntime adapts the selected participant-owned Browser Entry
// runtime to one explicit local plan. Browser Entry state uses the fixed
// per-user path expected by the separately installed native host.
func runAlphaBrowserRuntime(ctx context.Context, path string, output io.Writer) error {
	return runAlphaBrowserRuntimeWithStatePath(ctx, path, output, browserentry.DefaultStatePath)
}

func runAlphaBrowserRuntimeWithStatePath(ctx context.Context, path string, output io.Writer,
	browserStatePath func() (string, error)) (runErr error) {
	if ctx == nil || path == "" || output == nil || browserStatePath == nil {
		return errors.New("alpha browser runtime input is incomplete")
	}
	plan, err := loadAlphaBrowserRuntimePlan(path)
	if err != nil {
		return err
	}
	clock := time.Now
	networkConfig, refreshState, err := alphaBrowserNetworkConfig(plan, clock)
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
	browserState, err := browserStatePath()
	if err != nil {
		return fmt.Errorf("resolve Browser Entry state path: %w", err)
	}
	serviceAuthority := append(ed25519.PublicKey(nil), epoch.Authorities[0].PublicKey[:]...)
	endpoint, err := endpointapi.New(endpointapi.Setup{NetworkID: plan.NetworkID, BrokerID: plan.BrokerID,
		AuthorityPublic: serviceAuthority, IntroductionPublic: serviceAuthority, ConnectionPrincipal: plan.ConnectionPrincipal,
		BrowserEntryStatePath: browserState, Clock: clock})
	if err != nil {
		return fmt.Errorf("open participant Endpoint: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, endpoint.Close()) }()
	runtime, err := endpoint.OpenAlphaBrowserRuntime(ctx, endpointapi.AlphaBrowserRuntimeRequest{Floor: floor,
		Current: func() (endpointapi.AlphaBrowserStateView, error) { return network.CurrentResolution() }, Entry: owner,
		Principal: plan.ConnectionPrincipal, BytesEachDirection: plan.BytesEachDirection, Clock: clock})
	if err != nil {
		return fmt.Errorf("open alpha browser runtime: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, runtime.Close()) }()
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(alphaBrowserRuntimeEvent{"alpha-browser-runtime-ready", hex.EncodeToString(plan.NetworkID[:])}); err != nil {
		return err
	}
	<-ctx.Done()
	return encoder.Encode(alphaBrowserRuntimeEvent{"alpha-browser-runtime-stopped", hex.EncodeToString(plan.NetworkID[:])})
}

// alphaBrowserNetworkConfig preserves one owner for a State root. A static
// root is useful for deliberately offline qualification. A participant that
// supplies a source plan must give the same State trust anchors and local
// clock owner to this runtime; it cannot make a second process mutate the
// retained root underneath the browser session.
func alphaBrowserNetworkConfig(plan decodedAlphaBrowserRuntimePlan, clock func() time.Time) (state.Config, bool, error) {
	if clock == nil {
		return state.Config{}, false, errors.New("alpha browser runtime clock is unavailable")
	}
	if plan.NetworkSourcePlan == "" {
		return state.Config{Root: plan.NetworkStateRoot, NetworkID: plan.NetworkID, Authorities: plan.NetworkAuthorities,
			Threshold: plan.NetworkThreshold, AcceptedProfile: route.Profile, Clock: clock}, false, nil
	}
	config, err := readSourcePlan(plan.NetworkStateRoot, plan.NetworkSourcePlan)
	if err != nil {
		return state.Config{}, false, fmt.Errorf("read participant Network State source plan: %w", err)
	}
	if !matchesAlphaBrowserSourcePlan(plan, config) {
		return state.Config{}, false, errors.New("participant Network State source plan does not match the alpha browser runtime")
	}
	config.AcceptedProfile, config.Clock = route.Profile, clock
	return config, true, nil
}

func matchesAlphaBrowserSourcePlan(plan decodedAlphaBrowserRuntimePlan, config state.Config) bool {
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

type alphaBrowserRuntimeEvent struct {
	Kind      string `json:"kind"`
	NetworkID string `json:"network_id"`
}

type decodedAlphaBrowserRuntimePlan struct {
	alphaBrowserRuntimePlan
	NetworkID, BrokerID, ConnectionPrincipal [32]byte
	NetworkAuthorities                       map[[32]byte]ed25519.PublicKey
	AlphaCorpusAuthority                     ed25519.PublicKey
}

func loadAlphaBrowserRuntimePlan(path string) (decodedAlphaBrowserRuntimePlan, error) {
	var raw alphaBrowserRuntimePlan
	if err := decodeOperatorInput(path, 16<<10, &raw); err != nil {
		return decodedAlphaBrowserRuntimePlan{}, err
	}
	if raw.Schema != "ardents-alpha-browser-runtime-v1" || raw.NetworkStateRoot == "" || raw.EntryStateRoot == "" ||
		raw.AlphaCorpusStateRoot == "" || raw.LocalRoleStateRoot == "" || raw.TimeConfidenceFile == "" || raw.NetworkProfile != route.Profile ||
		raw.AlphaCohort == "" || raw.BrokerID == "" || raw.ConnectionPrincipal == "" || raw.BytesEachDirection == 0 {
		return decodedAlphaBrowserRuntimePlan{}, errors.New("alpha browser runtime plan is incomplete")
	}
	result := decodedAlphaBrowserRuntimePlan{alphaBrowserRuntimePlan: raw}
	for _, field := range []struct {
		encoded     string
		destination []byte
	}{{raw.NetworkID, result.NetworkID[:]}, {raw.BrokerID, result.BrokerID[:]}, {raw.ConnectionPrincipal, result.ConnectionPrincipal[:]}} {
		if err := decodeOperatorFixedHex(field.encoded, field.destination); err != nil {
			return decodedAlphaBrowserRuntimePlan{}, err
		}
	}
	authorities, err := decodeOperatorAuthorities(raw.NetworkAuthorities, 16)
	if err != nil {
		return decodedAlphaBrowserRuntimePlan{}, err
	}
	result.NetworkAuthorities = authorities
	corpusAuthority := make(ed25519.PublicKey, ed25519.PublicKeySize)
	if err := decodeOperatorFixedHex(raw.AlphaCorpusAuthority, corpusAuthority); err != nil {
		return decodedAlphaBrowserRuntimePlan{}, err
	}
	result.AlphaCorpusAuthority = corpusAuthority
	return result, nil
}
