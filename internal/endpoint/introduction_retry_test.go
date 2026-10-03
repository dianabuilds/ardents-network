//go:build linux

package endpoint

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// Only the retained Route prefix sees this outage; Endpoint can still prepare
// its real blinded request from the independently available accepted State.
// Node runtimes retain the original State and keep their real network duties.
type issuerOutageState struct {
	*sourceStateFixture
	unavailable atomic.Bool
}

func (source *issuerOutageState) CurrentClosedRoute() (state.ClosedRouteView, error) {
	if source.unavailable.Load() {
		return state.ClosedRouteView{}, errors.New("issuer attempt State temporarily unavailable")
	}
	return source.sourceStateFixture.CurrentClosedRoute()
}

func TestTextIntroductionRetryResumesExactIssuance(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		name := "matching"
		if foreign {
			name = "foreign"
		}
		t.Run(name, func(t *testing.T) {
			endpoint, owner, source := startRoleNetwork(t, roleNetworkFixture{carrier: carrier.ClosedCarrierTCP, resolution: true, publisher: true})
			outage := &issuerOutageState{sourceStateFixture: source}
			endpoint.closedState = outage
			if _, err := owner.openPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			endpoint.closedState = source
			receiver := source.view.Nodes[7].NodeID
			if err := owner.prepareIssuerStock(t.Context(), [][32]byte{receiver}, 2, nil, nil, nil); err != nil {
				t.Fatal(err)
			}
			outage.unavailable.Store(true)
			if foreign {
				receiver = source.view.Nodes[5].NodeID
				if err := owner.issueTokens(t.Context(), [][32]byte{receiver}, 1); err == nil {
					t.Fatal("outage issued foreign tokens")
				}
			} else {
				if _, err := owner.openIntroductionPrefix(t.Context()); err == nil {
					t.Fatal("outage opened Introduction")
				}
			}
			owner.mu.Lock()
			permission := owner.tokens.PermissionLocked()
			if !permission.HasPending() {
				owner.mu.Unlock()
				t.Fatal("outage did not retain requested batch")
			}
			reserved := reservedStockAllocation(owner.tokens.PermissionLocked())
			owner.mu.Unlock()
			outage.unavailable.Store(false)
			prefix, err := owner.openIntroductionPrefix(t.Context())
			if foreign {
				if err == nil || prefix != nil {
					t.Fatal("Introduction replaced foreign pending batch")
				}
				owner.mu.Lock()
				unchanged := owner.tokens.PermissionLocked() == permission && permission.HasPending() && reserved == reservedStockAllocation(owner.tokens.PermissionLocked())
				owner.mu.Unlock()
				if !unchanged {
					t.Fatal("foreign pending batch mutated")
				}
				if err := owner.issueTokens(t.Context(), [][32]byte{receiver}, 1); err != nil {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatalf("matching retry did not resume: %v", err)
			}
			owner.mu.Lock()
			completed := !owner.tokens.PermissionLocked().HasPending() && reservedStockAllocation(owner.tokens.PermissionLocked())[1] == reserved[1] && owner.introduction.prefix.currentLocked() == prefix
			owner.mu.Unlock()
			if !completed {
				t.Fatal("retry replaced or charged requested issuance again")
			}
		})
	}
}
