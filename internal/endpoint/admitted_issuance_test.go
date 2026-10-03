//go:build linux

package endpoint

import (
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
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
			permission := owner.tokens.PermissionLocked()
			valid := owner.source.CurrentLocked() == prefix && !owner.tokens.BusyLocked() && !owner.source.OpeningInProgressLocked() &&
				!permission.HasPending() && (2-permission.BootstrapAllowance()) == 2 && reservedStockAllocation(permission) == [3]uint32{34, 34, 0}
			profile := source.view.Profile
			ready := permission.StockCountFor(profile.Digest, selection.EntryNodeID, 2)
			owner.mu.Unlock()
			if !valid || ready != 32 {
				t.Fatalf("retained issuance state differs: valid=%v tokens=%d", valid, ready)
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
			// Release each token through the actual stock consumer before inspecting
			// its public signature. No test reads the owner's retained token slices.
			var duty uint64
			for _, node := range source.view.Nodes[:source.view.NodeCount] {
				if node.NodeID == selection.EntryNodeID {
					duty = node.DutyGeneration
				}
			}
			challenge := admissiontoken.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest,
				IssuerNodeID: profile.IssuerNodeID, ReceiverNodeID: selection.EntryNodeID, ReceiverDutyGeneration: duty,
				Class: 2, WindowStart: permission.Grant().NotBefore}
			var spki []byte
			for _, key := range profile.TokenKeys[:profile.TokenKeyCount] {
				if key.Class == 2 && key.WindowStart == challenge.WindowStart {
					spki = append([]byte(nil), key.SPKI[:]...)
				}
			}
			for index := range 32 {
				hello := ardp.Hello{RecipientNodeID: selection.EntryNodeID, RecipientDutyGeneration: duty, ChannelNonce: fixtureID(byte(100 + index))}
				owner.mu.Lock()
				token, err := owner.tokens.TakeTokenLocked(profile, time.Now(), hello, 2, t.Context())
				owner.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				verifyErr := admissiontoken.VerifyClosedToken(challenge, spki, token)
				clear(token)
				if verifyErr != nil {
					t.Fatal(verifyErr)
				}
			}
			if err := closeSourceHandle(prefix); err != nil {
				t.Fatal(err)
			}
			select {
			case <-sourceRouteDone(prefix):
			default:
				t.Fatal("source Close returned before joined lifecycle notification")
			}
			// Observing a retired prefix cannot switch the exhausted allocation
			// back to bootstrap or manufacture another pending request.
			if err := owner.issueTokens(t.Context(), [][32]byte{selection.EntryNodeID}, 2); err == nil {
				t.Fatal("retired prefix created unallocated work")
			}
			owner.mu.Lock()
			retired := owner.source.CurrentLocked() == nil && owner.tokens.PermissionLocked() == permission &&
				!permission.HasPending() && (2-permission.BootstrapAllowance()) == 2 && reservedStockAllocation(permission) == [3]uint32{34, 34, 0}
			owner.mu.Unlock()
			if !retired {
				t.Fatal("retirement lost private owner or repeated allocation")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			joined := owner.source.CurrentLocked() == nil && !owner.tokens.BusyLocked() && !owner.tokens.PermissionLocked().Present()
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
