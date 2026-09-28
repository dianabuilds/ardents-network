//go:build linux

package endpoint

import (
	"testing"
	"time"

	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// A listening duty may be idle longer than one incoming TLS handshake. Its
// next legitimate Endpoint must still complete both bootstrap and ordinary
// issuance through the real Entry/Interior and issuer consumers.
func TestTextIssuanceStartsAfterIdleListeners(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			t.Parallel()
			endpoint, owner, _ := startRoleNetwork(t, roleNetworkFixture{carrier: carrier})
			defer func() {
				if err := endpoint.Close(); err != nil {
					t.Error(err)
				}
			}()
			timer := time.NewTimer(11 * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-t.Context().Done():
				t.Fatal(t.Context().Err())
			}
			if _, err := owner.openPrefix(t.Context()); err != nil {
				t.Fatalf("bootstrap after idle listeners: %v", err)
			}
			selection := selectSource(t, owner)
			if err := owner.issueTokens(t.Context(), [][32]byte{selection.EntryNodeID}, 2); err != nil {
				t.Fatalf("ordinary issuance after idle listeners: %v", err)
			}
		})
	}
}
