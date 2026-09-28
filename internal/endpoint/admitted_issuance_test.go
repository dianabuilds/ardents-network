//go:build linux

package endpoint

import (
	"testing"
	"time"

	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestTextIssuanceUsesRetainedPrefixAfterTwoBootstrapBatches(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			endpoint, owner, source := startRoleNetwork(t, roleNetworkFixture{carrier: carrier})
			defer func() {
				if err := endpoint.Close(); err != nil {
					t.Error(err)
				}
			}()
			prefix, err := owner.openPrefix(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			selection := selectSource(t, owner)
			// The last initial Control token must fund a real two-token refill
			// before the final requested receiver batch can be issued.
			for batch := range 32 {
				if err := owner.issueTokens(t.Context(), [][32]byte{selection.EntryNodeID}, 2); err != nil {
					t.Fatalf("ordinary batch %d: %v", batch+1, err)
				}
			}
			owner.mu.Lock()
			permission := owner.tokens.permission
			valid := owner.source.currentLocked() == prefix && owner.tokens.issuance == nil && owner.source.opening == nil &&
				permission.pending == nil && permission.batches == 2 && permission.reserved == [3]uint32{34, 34, 0}
			verified := 0
			profile := source.view.Profile
			for _, stock := range permission.stock {
				if stock.challenge.ReceiverNodeID != selection.EntryNodeID || stock.challenge.Class != 2 {
					continue
				}
				for _, token := range stock.tokens {
					for _, key := range profile.TokenKeys[:profile.TokenKeyCount] {
						if key.Class == 2 && key.WindowStart == permission.accepted.NotBefore {
							if err := credential.VerifyClosedToken(stock.challenge, key.SPKI[:], token); err != nil {
								owner.mu.Unlock()
								t.Fatal(err)
							}
							verified++
						}
					}
				}
			}
			owner.mu.Unlock()
			if !valid || verified != 32 {
				t.Fatalf("retained issuance state differs: valid=%v tokens=%d", valid, verified)
			}
			receipts := readTokenReceipts(t, endpoint.closedTokenRoot, endpoint.network)
			if len(receipts) != 35 {
				t.Fatalf("expected two forwarding and 33 issuer spend receipts, got %d", len(receipts))
			}
			issuerAttempts := make(map[[32]byte]bool)
			for _, receipt := range receipts {
				if receipt.receiver == profile.IssuerNodeID {
					if issuerAttempts[receipt.attempt] {
						t.Fatal("issuer reused an admission nonce")
					}
					issuerAttempts[receipt.attempt] = true
				}
			}
			if len(issuerAttempts) != 33 {
				t.Fatal("issuer tokens bypassed Endpoint journal")
			}
			if err := prefix.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-prefix.Done():
			default:
				t.Fatal("source Close returned before joined lifecycle notification")
			}
			// Observing a retired prefix cannot switch the exhausted allocation
			// back to bootstrap or manufacture another pending request.
			if err := owner.issueTokens(t.Context(), [][32]byte{selection.EntryNodeID}, 2); err == nil {
				t.Fatal("retired prefix created unallocated work")
			}
			owner.mu.Lock()
			retired := owner.source.currentLocked() == nil && owner.tokens.permission == permission &&
				permission.pending == nil && permission.batches == 2 && permission.reserved == [3]uint32{34, 34, 0}
			owner.mu.Unlock()
			if !retired {
				t.Fatal("retirement lost private owner or repeated allocation")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			joined := owner.source.currentLocked() == nil && owner.tokens.issuance == nil && owner.tokens.permission == nil
			owner.mu.Unlock()
			if !joined {
				t.Fatal("context close retained prefix or private stock")
			}
			// Keep the test's bounded network operation inside the current window.
			if !time.Now().Before(profile.NotAfter) {
				t.Fatal("test crossed its permission window")
			}
		})
	}
}
