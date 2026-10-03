//go:build linux

package endpoint

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// Observe the nonrefundable debit from the public grant and current balances,
// rather than reading or changing the holder's private reservation counters.
func reservedStockAllocation(permission stock.Permission) [3]uint32 {
	maximum := permission.Grant().Maxima
	var reserved [3]uint32
	for index := range reserved {
		reserved[index] = maximum[index] - permission.Remaining(uint8(index+1))
	}
	return reserved
}

// usableStockCountLocked observes current receiver/class balances through the
// same queries used by Endpoint preflight. It never obtains reusable token bytes.
func usableStockCountLocked(t *testing.T, owner *dutyContext) int {
	t.Helper()
	state, ok := owner.endpoint.closedState.(client.ClosedBootstrapState)
	if !ok {
		t.Error("stock observation has no current Route view")
		return -1
	}
	view, err := state.CurrentClosedRoute()
	if err != nil || int(view.NodeCount) > len(view.Nodes) {
		t.Errorf("stock observation view unavailable: %v", err)
		return -1
	}
	count := 0
	for _, receiver := range view.Nodes[:view.NodeCount] {
		for class := uint8(1); class <= 3; class++ {
			count += owner.tokens.PermissionLocked().StockCountForDuty(view.Profile.Digest, receiver.NodeID, receiver.DutyGeneration, class)
		}
	}
	return count
}
