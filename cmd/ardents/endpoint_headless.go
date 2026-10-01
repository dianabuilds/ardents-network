package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/runtimeplan"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type headlessPermissionPlan = runtimeplan.Permission
type headlessRuntimePlan = runtimeplan.Headless
type decodedHeadlessRuntimePlan = runtimeplan.DecodedHeadless

var errHeadlessRuntimeV1Retired = runtimeplan.ErrHeadlessV1Retired

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
	return runTextHeadlessRuntime(ctx, plan, output)
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
			Threshold: plan.NetworkThreshold, AcceptedProfile: plan.NetworkProfile, ClosedProfileAuthority: plan.ClosedProfileAuthority, Clock: clock}, false, nil
	}
	config, err := readSourcePlan(plan.NetworkStateRoot, plan.NetworkSourcePlan)
	if err != nil {
		return state.Config{}, false, fmt.Errorf("read participant Network State source plan: %w", err)
	}
	if !matchesHeadlessSourcePlan(plan, config) {
		return state.Config{}, false, errors.New("participant Network State source plan does not match the headless runtime")
	}
	config.AcceptedProfile, config.Clock = plan.NetworkProfile, clock
	config.ClosedProfileAuthority = append(ed25519.PublicKey(nil), plan.ClosedProfileAuthority...)
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

func loadHeadlessRuntimePlan(path string) (decodedHeadlessRuntimePlan, error) {
	raw, err := readOperatorInput(path, 16<<10)
	if err != nil {
		return decodedHeadlessRuntimePlan{}, err
	}
	return runtimeplan.DecodeHeadless(raw)
}
