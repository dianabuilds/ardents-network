//go:build linux

package endpoint

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"time"
)

// Inspect an idle Publisher's retained selection without dialing from a capsule.
// A live Source continues to impose its original transport authority horizon.
func (owner *dutyContext) introductionRecipientLocked() ([32]byte, uint64, time.Time, error) {
	if prefix := owner.source.CurrentLocked(); prefix != nil {
		return prefix.DataJoinRecipient()
	}
	bootstrapState, ok := owner.endpoint.closedState.(client.ClosedBootstrapState)
	if !ok || owner.sourceSet == nil {
		return [32]byte{}, 0, time.Time{}, errors.New("text Introduction retained Source unavailable")
	}
	selection, err := owner.selectBootstrapLocked()
	if err != nil {
		return [32]byte{}, 0, time.Time{}, err
	}
	return client.InspectClosedDataJoinRecipient(bootstrapState, selection)
}
