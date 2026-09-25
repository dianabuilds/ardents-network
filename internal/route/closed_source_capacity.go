//go:build linux

package route

import "github.com/dianabuilds/ardents-network/internal/route/ardp"

func (owner *closedSourceChannels) childCapacityLocked(purpose ardp.Purpose) (reserved, available bool) {
	extra := 0
	for _, lane := range owner.lanes {
		if lane.reservedControl {
			extra++
		}
	}
	if len(owner.lanes)-extra < closedForwardChildren {
		return false, true
	}
	return true, closedControlPurpose(purpose) && extra < 2
}
