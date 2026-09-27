//go:build linux

package client

import (
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func (owner *closedSourceChannels) childCapacityLocked(purpose ardp.Purpose) (reserved, available bool) {
	extra := 0
	for _, lane := range owner.lanes {
		if lane.reservedControl {
			extra++
		}
	}
	if len(owner.lanes)-extra < route.ClosedForwardChildren {
		return false, true
	}
	return true, route.ClosedControlPurpose(purpose) && extra < 2
}
