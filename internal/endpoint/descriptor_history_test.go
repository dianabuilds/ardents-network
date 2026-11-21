//go:build linux

package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/descriptorhistory"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// The existing real Store has revision 1. The Endpoint separately verifies a
// signed revision 2 as a prior observation; the next genuine network response
// must be refused locally even though that Node still considers revision 1 live.
func TestTextResolutionNetworkCannotRollBackLocalDescriptorFloor(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			_, owner, source := startRoleNetwork(t, roleNetworkFixture{carrier: carrier, resolution: true})
			prefix, err := owner.openPrefix(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			acquisition := owner.source.AcquireResolutionLocked()
			owner.mu.Unlock()
			if acquisition == nil {
				t.Fatal("text Source resolution acquisition unavailable")
			}
			defer acquisition.Release()
			now := time.Now().UTC().Truncate(time.Second)
			profile := source.view.Profile
			_, authority, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(authority)
			current, signer := floorPublication(t, authority, profile.NetworkID, 1, now, now.Add(10*time.Minute))
			first := floorDescriptor(t, current, signer, profile.Digest, source.view.Nodes[6].NodeID, 1, 10, now, now.Add(120*time.Second))
			second := floorDescriptor(t, current, signer, profile.Digest, source.view.Nodes[6].NodeID, 2, 20, now, now.Add(100*time.Second))
			receiver, err := acquisition.ResolutionRecipient()
			if err != nil {
				t.Fatal(err)
			}
			if err := owner.issueTokens(t.Context(), [][32]byte{receiver}, 1); err != nil {
				t.Fatal(err)
			}
			status, _, err := acquisition.ExchangeDescriptor(t.Context(), func(hello ardp.Hello, class uint8) ([]byte, error) {
				owner.mu.Lock()
				defer owner.mu.Unlock()
				profile, at, err := owner.permissionProfileLocked()
				if err != nil {
					return nil, err
				}
				return owner.tokens.TakeTokenLocked(profile, at, hello, class, t.Context())
			}, [32]byte{}, first)
			if err != nil || status != 0 {
				t.Fatalf("actual fixture publication: %d %v", status, err)
			}
			got, err := owner.lookupDescriptor(t.Context(), current.Credential.Target)
			if err != nil || !bytes.Equal(got.Current.Record, current.Record) {
				t.Fatalf("actual first lookup: %v", err)
			}
			owner.mu.Lock()
			floorRetained := owner.descriptorHistory.Matches(current.Credential.Target, current.Digest, 1)
			_, err = owner.descriptorHistory.Accept(second, current.Credential.Target, profile.NetworkID, profile.Digest, now)
			owner.mu.Unlock()
			if !floorRetained || err != nil {
				t.Fatalf("lookup failed to retain floor or prior observation invalid: %v", err)
			}
			if _, err := owner.lookupDescriptor(t.Context(), current.Credential.Target); err == nil {
				t.Fatal("actual resolver response rolled back locally retained revision")
			}
			owner.mu.Lock()
			retained := owner.descriptorHistory.Matches(current.Credential.Target, current.Digest, 2)
			healthy := owner.source.CurrentLocked() == prefix && !owner.resolution.BusyLocked() && !owner.closed
			owner.mu.Unlock()
			if !retained || !healthy {
				t.Fatal("ordinary stale response erased floor or damaged context")
			}
			// Fill the real context history through independently signed
			// Descriptors. The admitted issuer and stock remain in use.
			type observedDescriptor struct {
				target [32]byte
				raw    []byte
			}
			observed := make([]observedDescriptor, 0, descriptorhistory.MaximumTargets-1)
			for index := 1; index < descriptorhistory.MaximumTargets; index++ {
				_, nextAuthority, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				next, nextSigner := floorPublication(t, nextAuthority, profile.NetworkID, 1, now, now.Add(10*time.Minute))
				clear(nextAuthority)
				nextRaw := floorDescriptor(t, next, nextSigner, profile.Digest, source.view.Nodes[6].NodeID, 1, byte(index), now, now.Add(100*time.Second))
				observed = append(observed, observedDescriptor{target: next.Credential.Target, raw: nextRaw})
			}
			owner.mu.Lock()
			for index, next := range observed {
				if _, err := owner.descriptorHistory.Accept(next.raw, next.target, profile.NetworkID, profile.Digest, now); err != nil {
					owner.mu.Unlock()
					t.Fatalf("capacity setup Descriptor %d: %v", index, err)
				}
			}
			_, err = owner.descriptorHistory.Accept(second, current.Credential.Target, profile.NetworkID, profile.Digest, now)
			reserved, batches := reservedStockAllocation(owner.tokens.PermissionLocked()), (2 - owner.tokens.PermissionLocked().BootstrapAllowance())
			tokensBefore := usableStockCountLocked(t, owner)
			owner.mu.Unlock()
			if err != nil {
				t.Fatalf("full cache rejected existing Target: %v", err)
			}
			if _, err := owner.lookupDescriptor(t.Context(), fixtureID(199)); err == nil {
				t.Fatal("full context accepted another Target")
			}
			owner.mu.Lock()
			tokensAfter := usableStockCountLocked(t, owner)
			unchanged := !owner.descriptorHistory.CanAdmit(fixtureID(199)) &&
				owner.descriptorHistory.Matches(current.Credential.Target, current.Digest, 2) && reservedStockAllocation(owner.tokens.PermissionLocked()) == reserved &&
				(2-owner.tokens.PermissionLocked().BootstrapAllowance()) == batches && tokensBefore == tokensAfter && !owner.resolution.BusyLocked() && !owner.tokens.BusyLocked()
			owner.mu.Unlock()
			if !unchanged {
				t.Fatal("capacity refusal evicted floors or consumed network issuance/admission")
			}
		})
	}
}
