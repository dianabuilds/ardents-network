//go:build linux

package endpoint

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/qualification"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestQualificationReopensRetiredSourcePrefixForIssuerReserve(t *testing.T) {
	endpoint, owner, source := startRoleNetwork(t, roleNetworkFixture{carrier: carrier.ClosedCarrierTCP})
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
	for _, receiver := range [][32]byte{selection.EntryNodeID, selection.InteriorNodeID} {
		if err := owner.issueTokens(t.Context(), [][32]byte{receiver}, 2); err != nil {
			t.Fatal(err)
		}
	}
	ready := func() int {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		profile := source.view.Profile
		return owner.tokens.PermissionLocked().StockCountForDuty(profile.Digest, profile.IssuerNodeID, profile.IssuerDutyGeneration, 1)
	}
	if err := owner.ensureQualificationIssuerReserve(t.Context(), qualification.IssuerReserveMinimum); err != nil {
		t.Fatal(err)
	}
	// Consume issuer Control stock through real admitted issuance; changing the
	// owner's private stock would bypass both the journal and refill policy.
	for attempts := 0; ready() > 4 && attempts < 64; attempts++ {
		if err := owner.issueTokens(t.Context(), [][32]byte{selection.EntryNodeID}, 2); err != nil {
			t.Fatal(err)
		}
	}
	if remaining := ready(); remaining > 4 || remaining < 2 {
		t.Fatalf("could not reach retained-prefix refill boundary: %d", remaining)
	}
	if err := closeSourceHandle(prefix); err != nil {
		t.Fatal(err)
	}
	before := ready()
	if err := owner.ensureQualificationIssuerReserve(t.Context(), qualification.IssuerReserveMinimum); err != nil {
		t.Fatalf("retired Source prefix failed the issuer reserve: %v", err)
	}
	owner.mu.Lock()
	reopened := owner.source.CurrentLocked()
	owner.mu.Unlock()
	if reopened == nil || reopened == prefix {
		t.Fatalf("issuer reserve did not reopen the retired Source prefix: %p", reopened)
	}
	if after := ready(); after <= before {
		t.Fatalf("qualification issuer reserve = %d after %d", after, before)
	}
}

func TestQualificationRefillsPublisherIssuerReserveBetweenStreams(t *testing.T) {
	endpoint, owner, source := startRoleNetwork(t, roleNetworkFixture{carrier: carrier.ClosedCarrierTCP})
	defer func() {
		if err := endpoint.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := owner.openPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	selection := selectSource(t, owner)
	ready := func() int {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		profile := source.view.Profile
		return owner.tokens.PermissionLocked().StockCountForDuty(profile.Digest, profile.IssuerNodeID, profile.IssuerDutyGeneration, 1)
	}
	for attempts := 0; ready() >= qualification.IssuerReserveMinimum && attempts < 64; attempts++ {
		if err := owner.issueTokens(t.Context(), [][32]byte{selection.EntryNodeID}, 2); err != nil {
			t.Fatal(err)
		}
	}
	before := ready()
	if before >= qualification.IssuerReserveMinimum {
		t.Fatalf("could not reach qualification issuer refill boundary: %d", before)
	}
	if err := owner.ensureQualificationIssuerReserve(t.Context(), qualification.IssuerReserveMinimum); err != nil {
		t.Fatal(err)
	}
	if after := ready(); after < qualification.IssuerReserveMinimum || after <= before {
		t.Fatalf("qualification issuer reserve = %d after %d", after, before)
	}
}
