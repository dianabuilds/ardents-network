//go:build linux && live

package network_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/bridge"
	"github.com/dianabuilds/ardents-network/internal/camouflage"
	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

type blockedRecoveryImportPlan struct {
	StateRoot          string   `json:"state_root"`
	NetworkStateRoot   string   `json:"network_state_root"`
	InviteFile         string   `json:"invite_file"`
	NetworkID          string   `json:"network_id"`
	NetworkAuthorities []string `json:"network_authorities"`
	NetworkThreshold   int      `json:"network_threshold"`
	NetworkProfile     string   `json:"network_profile"`
	RouteProfile       string   `json:"route_profile"`
	LocalRoleStateRoot string   `json:"local_role_state_root"`
}

type blockedBridgeOwner interface {
	Contact() ([32]byte, []byte, error)
	BeginContact([]byte, [32]byte, time.Time) ([32]byte, []byte, byte, uint64, time.Time, error)
	NextContact(context.Context) ([32]byte, []byte, byte, error)
	FinishContact(byte, uint64, bool, bool) error
	Acquire(context.Context, []byte, [32]byte, time.Time,
		func(context.Context, [32]byte, []byte, time.Time) (net.Conn, func() error, bool, error),
	) (net.Conn, func() error, error)
	Evidence() (bridge.AttemptEvidence, error)
	Close() error
}

func assertBlockedRecoveryEvidence(t *testing.T) bridge.AttemptEvidence {
	t.Helper()
	owner, closeOwner := openBlockedBridgeOwner(t, "/run/secure/import.json", time.Now)
	evidence, evidenceErr := owner.Evidence()
	if closeErr := closeOwner(); evidenceErr == nil {
		evidenceErr = closeErr
	}
	if evidenceErr != nil || evidence.AttemptDigest == ([32]byte{}) || evidence.ContactStarts != 1 ||
		evidence.Terminal != "bridge-deadline-exceeded" || !evidence.CleanupComplete ||
		evidence.TerminalOffset > evidence.DeadlineOffset {
		t.Fatalf("recovery Bridge evidence = %+v, %v", evidence, evidenceErr)
	}
	return evidence
}

func openBlockedBridgeOwner(t *testing.T, plan string, clock func() time.Time) (blockedBridgeOwner, func() error) {
	return openBlockedBridgeOwnerWithConfidence(t, plan, clock, func() bool { return true })
}

func openBlockedBridgeOwnerWithConfidence(t *testing.T, plan string, clock func() time.Time,
	confidence func() bool,
) (blockedBridgeOwner, func() error) {
	t.Helper()
	var raw blockedRecoveryImportPlan
	if err := planfile.Decode(plan, 16<<10, &raw); err != nil {
		t.Fatal(err)
	}
	var networkID [32]byte
	if err := planfile.FixedHex(raw.NetworkID, networkID[:]); err != nil {
		t.Fatal(err)
	}
	authorities, err := planfile.Authorities(raw.NetworkAuthorities, 16)
	if err != nil {
		t.Fatal(err)
	}
	network, err := state.Open(state.Config{Root: raw.NetworkStateRoot, NetworkID: networkID,
		Authorities: authorities, Threshold: raw.NetworkThreshold, AcceptedProfile: raw.NetworkProfile, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := bridge.Open(bridge.Config{Root: raw.StateRoot, RouteProfile: raw.RouteProfile,
		CurrentNetwork: network.Current, Clock: clock, TimeConfidence: confidence,
		RoleConflict: func(identity, family [32]byte) (bool, error) {
			return localroles.ReadConflict(raw.LocalRoleStateRoot, clock, identity, family)
		}, ValidateCandidate: func(value []byte, identity [32]byte) ([32]byte, string, error) {
			candidate, validateErr := camouflage.Validate(value, identity)
			return candidate.Commitment(), "webtunnel-v0.0.6", validateErr
		}})
	if err != nil {
		_ = network.Close()
		t.Fatal(err)
	}
	return owner, func() error { return errors.Join(owner.Close(), network.Close()) }
}
