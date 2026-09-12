//go:build linux

package endpoint

import (
	"bytes"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// Only the retained Route prefix sees this outage; Endpoint can still prepare
// its real blinded request from the independently available accepted State.
// Node runtimes retain the original State and keep their real network duties.
type textIssuerOutageState struct {
	*textSourceStateFixture
	unavailable atomic.Bool
}

func (source *textIssuerOutageState) CurrentClosedRoute() (state.ClosedRouteView, error) {
	if source.unavailable.Load() {
		return state.ClosedRouteView{}, errors.New("issuer attempt State temporarily unavailable")
	}
	return source.textSourceStateFixture.CurrentClosedRoute()
}

func TestTextIntroductionRetryResumesExactIssuance(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		name := "matching"
		if foreign {
			name = "foreign"
		}
		t.Run(name, func(t *testing.T) {
			endpoint, owner, source := startTextRoleNetwork(t, route.ClosedCarrierTCP, true, true)
			outage := &textIssuerOutageState{textSourceStateFixture: source}
			endpoint.closedState = outage
			if _, err := owner.openTextPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			endpoint.closedState = source
			receiver := source.view.Nodes[7].NodeID
			if err := owner.prepareTextIssuerStock(t.Context(), [][32]byte{receiver}, 2, nil); err != nil {
				t.Fatal(err)
			}
			outage.unavailable.Store(true)
			if foreign {
				receiver = source.view.Nodes[5].NodeID
				if err := owner.issueTextTokens(t.Context(), [][32]byte{receiver}, 1); err == nil {
					t.Fatal("outage issued foreign tokens")
				}
			} else {
				if _, err := owner.openTextIntroductionPrefix(t.Context()); err == nil {
					t.Fatal("outage opened Introduction")
				}
			}
			owner.mu.Lock()
			pending := owner.permission.pending
			if pending == nil || pending.refill {
				owner.mu.Unlock()
				t.Fatal("outage did not retain requested batch")
			}
			request := pending.pending.Request()
			reserved := owner.permission.reserved
			owner.mu.Unlock()
			defer clear(request)
			outage.unavailable.Store(false)
			prefix, err := owner.openTextIntroductionPrefix(t.Context())
			if foreign {
				if err == nil || prefix != nil {
					t.Fatal("Introduction replaced foreign pending batch")
				}
				owner.mu.Lock()
				unchanged := owner.permission.pending == pending && bytes.Equal(request, pending.pending.Request()) && reserved == owner.permission.reserved
				owner.mu.Unlock()
				if !unchanged {
					t.Fatal("foreign pending batch mutated")
				}
				if err := owner.issueTextTokens(t.Context(), [][32]byte{receiver}, 1); err != nil {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatalf("matching retry did not resume: %v", err)
			}
			owner.mu.Lock()
			completed := owner.permission.pending == nil && owner.permission.reserved[1] == reserved[1] && owner.introduction.prefix == prefix
			owner.mu.Unlock()
			if !completed {
				t.Fatal("retry replaced or charged requested issuance again")
			}
		})
	}
}
