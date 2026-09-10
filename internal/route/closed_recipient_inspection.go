//go:build linux

package route

import (
	"errors"
	"time"
)

// InspectClosedDataJoinRecipient verifies the retained Source selection and
// current Rendezvous/control separation without opening or admitting a channel.
// An idle Publisher uses this before accepting an Introduction; later forwarding
// still requires its own admitted Source and Responder ownership.
func InspectClosedDataJoinRecipient(source ClosedBootstrapState, selection ClosedBootstrapSelection) ([32]byte, uint64, time.Time, error) {
	plan, err := prepareClosedPrefix(source, selection, 1, time.Now().UTC())
	if err != nil {
		return [32]byte{}, 0, time.Time{}, err
	}
	snapshot, err := source.Current()
	if err != nil || snapshot.Digest != plan.profile.StateDigest {
		return [32]byte{}, 0, time.Time{}, errors.Join(err, errors.New("closed recipient State changed"))
	}
	// This is an authority horizon, not a new handshake or transport lifetime.
	plan.deadline = plan.profile.NotAfter
	for _, limit := range []time.Time{snapshot.ValidUntil, plan.peers[0].notAfter, plan.peers[1].notAfter, plan.peers[2].notAfter} {
		if limit.Before(plan.deadline) {
			plan.deadline = limit
		}
	}
	return closedDataJoinRecipient(source, selection, plan)
}
