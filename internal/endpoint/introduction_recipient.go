//go:build linux

package endpoint

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"time"
)

// Inspect an idle Publisher's retained selection without dialing from a capsule.
// A live Source continues to impose its original transport authority horizon.
func (owner *textContext) introductionRecipientLocked() ([32]byte, uint64, time.Time, error) {
	if prefix := owner.source.currentLocked(); prefix != nil {
		return prefix.dataJoinRecipient()
	}
	source, ok := owner.endpoint.closedState.(client.ClosedBootstrapState)
	if !ok || !owner.source.hasMembersLocked() {
		return [32]byte{}, 0, time.Time{}, errors.New("text Introduction retained Source unavailable")
	}
	selection, err := owner.selectTextBootstrapLocked()
	if err != nil {
		return [32]byte{}, 0, time.Time{}, err
	}
	return client.InspectClosedDataJoinRecipient(source, selection)
}
