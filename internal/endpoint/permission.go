//go:build linux

package endpoint

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// permissionProfileLocked is the context-side authority gate for the token
// owner: it retires a dead prefix, rechecks the verified live Endpoint job and
// returns the exact current State profile and time, or fails closed. The
// tokens package reaches it only through its Host seam.
func (owner *dutyContext) permissionProfileLocked() (state.ClosedProfileView, time.Time, error) {
	if err := owner.retirePrefixLocked(); err != nil {
		return state.ClosedProfileView{}, time.Time{}, err
	}
	endpoint := owner.endpoint
	if !owner.liveLocked(endpoint, owner.surface) || owner.verifiedJob == nil || owner.verifiedJob.owner != owner ||
		owner.verifiedJob.workerGrant == nil || endpoint.closedState == nil || endpoint.clock == nil {
		return state.ClosedProfileView{}, time.Time{}, errors.New("text permission requires a verified live Endpoint context")
	}
	now := endpoint.clock().UTC()
	profile, err := endpoint.closedState.CurrentClosedProfile()
	if err != nil || profile.NetworkID != endpoint.network || profile.NetworkID == [32]byte{} || profile.StateGeneration == [32]byte{} ||
		profile.StateDigest == [32]byte{} || profile.Digest == [32]byte{} || profile.IssuanceAuthorityKey == [32]byte{} ||
		profile.IssuerNodeID == [32]byte{} || profile.IssuerDutyGeneration == 0 || now.Before(profile.NotBefore) || !now.Before(profile.NotAfter) {
		return state.ClosedProfileView{}, time.Time{}, errors.New("text permission State authority is unavailable")
	}
	return profile, now, nil
}
