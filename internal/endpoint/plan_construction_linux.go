//go:build linux

package endpoint

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

// PlanDeclaration carries one operator's local declarations for a future
// preparation: storage roots, sockets, execution principals, permissions and
// role. Network trust facts are deliberately absent; ConstructPlans takes
// them only from the pin configuration that must authenticate an already
// accepted retained Network State. Finite permission allocation bounds stay
// with the installation request grammar.
type PlanDeclaration struct {
	Role                    string
	NetworkStateRoot        string
	LocalRoleStateRoot      string
	ApplicationTokenRoot    string
	EntryStateRoot          string
	PublicationRoot         string
	ServiceInstanceRoot     string
	TimeConfidenceFile      string
	ApplicationSocket       string
	AdministrationSocket    string
	BrokerID                string
	ConnectionPrincipal     string
	AdministrationPrincipal string
	ReaderPermission        PermissionPlan
	PublisherPermission     PermissionPlan
}

// ConstructPlans renders the canonical headless and Source plan files for one
// future preparation. The declared Network pins must open the referenced
// already accepted retained State and yield a current closed-profile
// observation, so a wrong network identity, authority set, threshold or
// profile authority refuses before any byte is rendered, and the plan carries
// exactly those authenticated trust facts. Local declarations are checked by
// the same offline projections preparation and runtime consume, including
// the Source declaration's credential files. Construction is preparation
// input only: it opens no preparation root, creates no State, permission,
// Stock, Service or runtime authority, and its result is not an accepting
// receipt.
func ConstructPlans(ctx context.Context, network state.Config, declaration PlanDeclaration, declared SourcePlan) (HeadlessPlan, []byte, []byte, error) {
	if ctx == nil {
		return HeadlessPlan{}, nil, nil, errors.New("plan construction is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return HeadlessPlan{}, nil, nil, err
	}
	plan, err := declaredPlan(network, declaration)
	if err != nil {
		return HeadlessPlan{}, nil, nil, err
	}
	if err := isolatedConstructionRoots(plan, network.Root); err != nil {
		return HeadlessPlan{}, nil, nil, err
	}
	// The retained opening authenticates the declared pins against the
	// accepted history; the current observation additionally refuses a State
	// without a current closed profile. The owner joins before rendering.
	owner, err := state.OpenRetained(network)
	if err != nil {
		return HeadlessPlan{}, nil, nil, err
	}
	view, viewErr := owner.CurrentRuntime()
	closeErr := owner.Close()
	if err := errors.Join(viewErr, closeErr, ctx.Err()); err != nil {
		return HeadlessPlan{}, nil, nil, err
	}
	facts, _, err := funding.NetworkFacts(view)
	if err != nil || facts.NetworkID != network.NetworkID {
		return HeadlessPlan{}, nil, nil, errors.Join(errors.New("accepted network observation is unavailable"), err)
	}
	// The same offline projection the runtime consumes validates the Source
	// declaration, its credential files and both source members under the
	// authenticated authorities.
	check := func() error { return ctx.Err() }
	config, err := NetworkConfig(plan, declared, check)
	if err != nil {
		return HeadlessPlan{}, nil, nil, err
	}
	// The installation request grammar's finite refresh and seed bounds apply
	// to constructed plans as well; State would refuse them at runtime open.
	if config.Source.OrderSeed == [32]byte{} || declared.RefreshIntervalMS < 100 || declared.RefreshIntervalMS > 60000 {
		return HeadlessPlan{}, nil, nil, errors.New("declared source refresh grammar is unavailable")
	}
	config.Source.VerificationClock = time.Now
	if _, _, err := source.New(config.Source, config.Authorities); err != nil {
		return HeadlessPlan{}, nil, nil, err
	}
	headlessJSON, err := canonicalPlanJSON(plan)
	if err != nil {
		return HeadlessPlan{}, nil, nil, err
	}
	sourceJSON, err := canonicalPlanJSON(declared)
	if err != nil {
		return HeadlessPlan{}, nil, nil, err
	}
	return plan, headlessJSON, sourceJSON, nil
}

// declaredPlan projects the operator's local declarations and the reference
// State's pins into one candidate v2 plan. Network trust fields originate
// only from the pin configuration, never from the local declaration.
func declaredPlan(network state.Config, declaration PlanDeclaration) (HeadlessPlan, error) {
	if declaration.Role != "" && declaration.Role != "reader" {
		return HeadlessPlan{}, errors.New("declared document runtime role is unavailable")
	}
	authorities := make([]string, 0, len(network.Authorities))
	for _, public := range network.Authorities {
		authorities = append(authorities, hex.EncodeToString(public))
	}
	// Canonical rendering requires one deterministic pin order.
	sort.Strings(authorities)
	plan := HeadlessPlan{Schema: "ardents-headless-runtime-v2", Role: declaration.Role,
		NetworkStateRoot: declaration.NetworkStateRoot, LocalRoleStateRoot: declaration.LocalRoleStateRoot,
		ApplicationTokenRoot: declaration.ApplicationTokenRoot, EntryStateRoot: declaration.EntryStateRoot,
		PublicationRoot: declaration.PublicationRoot, ServiceInstanceRoot: declaration.ServiceInstanceRoot,
		TimeConfidenceFile: declaration.TimeConfidenceFile, ApplicationSocket: declaration.ApplicationSocket,
		AdministrationSocket: declaration.AdministrationSocket,
		NetworkID:            hex.EncodeToString(network.NetworkID[:]), NetworkAuthorities: authorities,
		NetworkThreshold: network.Threshold, ClosedProfileAuthority: hex.EncodeToString(network.ClosedProfileAuthority),
		BrokerID: declaration.BrokerID, ConnectionPrincipal: declaration.ConnectionPrincipal,
		AdministrationPrincipal: declaration.AdministrationPrincipal,
		ReaderPermission:        declaration.ReaderPermission, PublisherPermission: declaration.PublisherPermission}
	if err := declaredLocalPaths(plan); err != nil {
		return HeadlessPlan{}, err
	}
	if _, err := plan.Network(); err != nil {
		return HeadlessPlan{}, err
	}
	if _, err := installedLocalConfig(plan); err != nil {
		return HeadlessPlan{}, err
	}
	return plan, nil
}

// declaredLocalPaths mirrors the distinct, absolute and canonical local path
// rules and the reader/dual-role split the installation declaration grammar
// owns. An empty permission allocation is refused because no later owner
// accepts one.
func declaredLocalPaths(plan HeadlessPlan) error {
	paths := []string{plan.NetworkStateRoot, plan.LocalRoleStateRoot, plan.ApplicationTokenRoot, plan.EntryStateRoot,
		plan.TimeConfidenceFile, plan.ApplicationSocket, plan.ReaderPermission.RequestPath, plan.ReaderPermission.ResponsePath}
	permissions := []PermissionPlan{plan.ReaderPermission}
	if plan.Role == "reader" {
		if plan.PublicationRoot != "" || plan.ServiceInstanceRoot != "" || plan.AdministrationSocket != "" ||
			plan.AdministrationPrincipal != "" || plan.PublisherPermission != (PermissionPlan{}) {
			return errors.New("reader runtime cannot declare publisher inputs")
		}
	} else {
		paths = append(paths, plan.PublicationRoot, plan.ServiceInstanceRoot, plan.AdministrationSocket,
			plan.PublisherPermission.RequestPath, plan.PublisherPermission.ResponsePath)
		permissions = append(permissions, plan.PublisherPermission)
	}
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		if path == "/" || !pathgrammar.Strict(path) || seen[path] {
			return errors.New("declared runtime paths must be distinct, absolute and canonical")
		}
		seen[path] = true
	}
	for _, permission := range permissions {
		if uint64(permission.Maxima[0])+uint64(permission.Maxima[1])+uint64(permission.Maxima[2]) == 0 {
			return errors.New("declared permission allocation is empty")
		}
	}
	return nil
}

// isolatedConstructionRoots mirrors preparation's own root isolation so the
// rendered plan cannot target the accepted reference State itself or overlap
// mutable roots. Preparation refuses a plan that violates these boundaries
// later; construction refuses it before rendering.
func isolatedConstructionRoots(plan HeadlessPlan, reference string) error {
	if !pathgrammar.Strict(reference) {
		return errors.New("reference State root is not canonical")
	}
	isolated := []string{plan.NetworkStateRoot, plan.LocalRoleStateRoot, plan.ApplicationTokenRoot, plan.EntryStateRoot, plan.TimeConfidenceFile}
	for i, path := range isolated {
		for _, previous := range isolated[:i] {
			if path == previous || strings.HasPrefix(path, previous+"/") || strings.HasPrefix(previous, path+"/") {
				return errors.New("declared preparation roots overlap")
			}
		}
		if path == reference || strings.HasPrefix(path, reference+"/") || strings.HasPrefix(reference, path+"/") {
			return errors.New("declared preparation roots overlap the accepted reference State")
		}
	}
	return nil
}

// canonicalPlanJSON renders the same compact canonical form Installation
// binds into its generations: declared struct field order, one trailing
// newline.
func canonicalPlanJSON(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}
