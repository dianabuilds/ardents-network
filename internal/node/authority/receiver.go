package authority

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// Source borrows only State's already authenticated current closed views.
// A missing or successor view must fail each admission recheck.
type Source struct {
	CurrentRoute   func() (state.ClosedRouteView, error)
	CurrentProfile func() (state.ClosedProfileView, bool)
}

// Receiver projects one local recipient from the exact State
// accepted closed profile. It never accepts a plan-supplied digest, role or
// duty, and a State successor makes the receiver unavailable before dial.
func (source Source) Receiver(snapshot state.NodeDuty, purpose ardp.Purpose, now time.Time) (route.ClosedRoleReceiver, bool) {
	view, err := source.Route(snapshot, now)
	if err != nil {
		return route.ClosedRoleReceiver{}, false
	}
	var recipient state.ClosedRouteNodeView
	found := false
	for index := uint8(0); index < view.NodeCount; index++ {
		node := view.Nodes[index]
		if node.NodeID != snapshot.NodeID {
			continue
		}
		if found || node.RecordDigest == [32]byte{} || node.DutyGeneration != snapshot.RecordGeneration {
			return route.ClosedRoleReceiver{}, false
		}
		recipient, found = node, true
	}
	if !found || !route.ClosedPurposePermitsDuty(purpose, recipient.RoleDomain, recipient.Subrole) {
		return route.ClosedRoleReceiver{}, false
	}
	return route.ClosedRoleReceiver{NetworkID: view.Profile.NetworkID, StateGeneration: view.Profile.StateGeneration,
		StateDigest: view.Profile.StateDigest, ProfileDigest: view.Profile.Digest, NodeID: recipient.NodeID,
		RecordDigest: recipient.RecordDigest, DutyGeneration: recipient.DutyGeneration, RoleDomain: recipient.RoleDomain,
		Subrole: recipient.Subrole, ExpectedPurpose: purpose, NotAfter: view.Profile.NotAfter}, true
}

// Route checks a borrowed view against the exact copied duty before use.
func (source Source) Route(snapshot state.NodeDuty, now time.Time) (state.ClosedRouteView, error) {
	if source.CurrentRoute == nil || snapshot.Profile != carrier.ClosedRouteProfile || snapshot.NodeID == [32]byte{} || snapshot.RecordGeneration == 0 {
		return state.ClosedRouteView{}, errors.New("closed Route snapshot prerequisites are not satisfied")
	}
	view, err := source.CurrentRoute()
	if err != nil {
		return state.ClosedRouteView{}, fmt.Errorf("read current closed Route: %w", err)
	}
	if !snapshot.Fresh || snapshot.Conflicting || !now.Before(snapshot.ValidUntil) || !now.Before(snapshot.RecordValidUntil) {
		return state.ClosedRouteView{}, errors.New("closed Route snapshot prerequisites are not satisfied")
	}
	if !ProfileMatchesSnapshot(view.Profile, snapshot, now) {
		return state.ClosedRouteView{}, errors.New("current closed Route does not match the duty snapshot")
	}
	return view, nil
}

// ProfileMatchesSnapshot rejects a profile successor or stale duty.
func ProfileMatchesSnapshot(profile state.ClosedProfileView, snapshot state.NodeDuty, now time.Time) bool {
	return profile.NetworkID == snapshot.NetworkID && profile.StateDigest == snapshot.Digest && profile.Epoch == snapshot.Epoch &&
		profile.Digest != [32]byte{} && !profile.NotBefore.After(now) && now.Before(profile.NotAfter) &&
		!profile.NotBefore.Before(snapshot.EpochValidFrom) && !profile.NotAfter.After(snapshot.ValidUntil) &&
		!profile.NotAfter.After(snapshot.RecordValidUntil) && StateGenerationMatches(profile.StateGeneration, snapshot.Generation)
}

// StateGenerationMatches accepts only the canonical lower-case hex encoding
// of the authenticated State generation.
func StateGenerationMatches(generation [32]byte, encoded string) bool {
	if len(encoded) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(encoded)
	if err != nil || len(decoded) != len(generation) || hex.EncodeToString(decoded) != encoded {
		return false
	}
	for index := range generation {
		if decoded[index] != generation[index] {
			return false
		}
	}
	return true
}
