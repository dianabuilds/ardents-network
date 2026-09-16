//go:build linux

package route

func (owner *closedSourceChannels) childCapacityLocked(purpose ClosedPurpose) (reserved, available bool) {
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
