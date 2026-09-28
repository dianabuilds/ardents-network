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
		count := 0
		for _, stock := range owner.tokens.Permission.Stock {
			if stock.Challenge.ReceiverNodeID == source.view.Profile.IssuerNodeID && stock.Challenge.Class == 1 {
				count += len(stock.Tokens)
			}
		}
		return count
	}
	if err := owner.ensureQualificationIssuerReserve(t.Context(), qualification.IssuerReserveMinimum); err != nil {
		t.Fatal(err)
	}
	for ready() > 4 {
		owner.mu.Lock()
		for slot := range owner.tokens.Permission.Stock {
			stock := &owner.tokens.Permission.Stock[slot]
			if stock.Challenge.ReceiverNodeID == source.view.Profile.IssuerNodeID && stock.Challenge.Class == 1 && len(stock.Tokens) > 0 {
				clear(stock.Tokens[0])
				stock.Tokens = stock.Tokens[1:]
				break
			}
		}
		owner.mu.Unlock()
	}
	if err := prefix.Close(); err != nil {
		t.Fatal(err)
	}
	before := ready()
	if err := owner.ensureQualificationIssuerReserve(t.Context(), qualification.IssuerReserveMinimum); err != nil {
		t.Fatalf("retired Source prefix failed the issuer reserve: %v", err)
	}
	owner.mu.Lock()
	reopened := owner.source.currentLocked()
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
		count := 0
		for _, stock := range owner.tokens.Permission.Stock {
			if stock.Challenge.ReceiverNodeID == source.view.Profile.IssuerNodeID && stock.Challenge.Class == 1 {
				count += len(stock.Tokens)
			}
		}
		return count
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
