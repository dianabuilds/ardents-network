//go:build linux

package endpoint

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestTextIssuanceUsesRetainedPrefixAfterTwoBootstrapBatches(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			endpoint, owner, source := startTextIssuanceNetwork(t, carrier)
			defer func() {
				if err := endpoint.Close(); err != nil {
					t.Error(err)
				}
			}()
			prefix, err := owner.openTextPrefix(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			selection := selectTextSource(t, owner)
			// The last initial Control token must fund a real two-token refill
			// before the final requested receiver batch can be issued.
			for batch := range 32 {
				if err := owner.issueTextTokens(t.Context(), [][32]byte{selection.EntryNodeID}, 2); err != nil {
					t.Fatalf("ordinary batch %d: %v", batch+1, err)
				}
			}
			owner.mu.Lock()
			permission := owner.permission
			valid := owner.prefix == prefix && owner.issuance == nil && owner.prefixOpening == nil &&
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
			raw, err := os.ReadFile(filepath.Join(endpoint.closedTokenRoot, "attempts"))
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) != textTokenJournalHeader+35*textTokenAttemptSize {
				t.Fatalf("expected two forwarding and 33 issuer spend receipts, got %d bytes", len(raw))
			}
			issuerAttempts := make(map[[32]byte]bool)
			for offset := textTokenJournalHeader; offset < len(raw); offset += textTokenAttemptSize {
				receipt, err := decodeTextTokenAttempt(raw[offset : offset+textTokenAttemptSize])
				if err != nil {
					t.Fatal(err)
				}
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
			if err := owner.issueTextTokens(t.Context(), [][32]byte{selection.EntryNodeID}, 2); err == nil {
				t.Fatal("retired prefix created unallocated work")
			}
			owner.mu.Lock()
			retired := owner.prefix == nil && owner.prefixCancel == nil && owner.permission == permission &&
				permission.pending == nil && permission.batches == 2 && permission.reserved == [3]uint32{34, 34, 0}
			owner.mu.Unlock()
			if !retired {
				t.Fatal("retirement lost private owner or repeated allocation")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			joined := owner.prefix == nil && owner.issuance == nil && owner.permission == nil
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
