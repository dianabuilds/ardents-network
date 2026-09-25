//go:build linux

package endpoint

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestQualificationReopensRetiredSourcePrefixForIssuerReserve(t *testing.T) {
	endpoint, owner, source := startTextRoleNetwork(t, textRoleNetworkFixture{carrier: route.ClosedCarrierTCP})
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
	for _, receiver := range [][32]byte{selection.EntryNodeID, selection.InteriorNodeID} {
		if err := owner.issueTextTokens(t.Context(), [][32]byte{receiver}, 2); err != nil {
			t.Fatal(err)
		}
	}
	ready := func() int {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		count := 0
		for _, stock := range owner.permission.stock {
			if stock.challenge.ReceiverNodeID == source.view.Profile.IssuerNodeID && stock.challenge.Class == 1 {
				count += len(stock.tokens)
			}
		}
		return count
	}
	if err := owner.ensureQualificationIssuerReserve(t.Context(), qualificationIssuerReserve); err != nil {
		t.Fatal(err)
	}
	for ready() > 4 {
		owner.mu.Lock()
		for slot := range owner.permission.stock {
			stock := &owner.permission.stock[slot]
			if stock.challenge.ReceiverNodeID == source.view.Profile.IssuerNodeID && stock.challenge.Class == 1 && len(stock.tokens) > 0 {
				clear(stock.tokens[0])
				stock.tokens = stock.tokens[1:]
				break
			}
		}
		owner.mu.Unlock()
	}
	if err := prefix.Close(); err != nil {
		t.Fatal(err)
	}
	before := ready()
	if err := owner.ensureQualificationIssuerReserve(t.Context(), qualificationIssuerReserve); err != nil {
		t.Fatalf("retired Source prefix failed the issuer reserve: %v", err)
	}
	owner.mu.Lock()
	reopened := owner.currentTextSourceLocked()
	owner.mu.Unlock()
	if reopened == nil || reopened == prefix {
		t.Fatalf("issuer reserve did not reopen the retired Source prefix: %p", reopened)
	}
	if after := ready(); after <= before {
		t.Fatalf("qualification issuer reserve = %d after %d", after, before)
	}
}

func TestQualificationReaderOpeningDelayStaggersFourReaders(t *testing.T) {
	want := []time.Duration{0, 250 * time.Millisecond, 500 * time.Millisecond, 750 * time.Millisecond}
	for reader, expected := range want {
		if got := qualificationReaderOpeningDelay(reader); got != expected {
			t.Fatalf("Reader %d opening delay = %s, want %s", reader, got, expected)
		}
	}
}

func TestQualificationRefillsPublisherIssuerReserveBetweenStreams(t *testing.T) {
	endpoint, owner, source := startTextRoleNetwork(t, textRoleNetworkFixture{carrier: route.ClosedCarrierTCP})
	defer func() {
		if err := endpoint.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := owner.openTextPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	selection := selectTextSource(t, owner)
	ready := func() int {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		count := 0
		for _, stock := range owner.permission.stock {
			if stock.challenge.ReceiverNodeID == source.view.Profile.IssuerNodeID && stock.challenge.Class == 1 {
				count += len(stock.tokens)
			}
		}
		return count
	}
	for attempts := 0; ready() >= qualificationIssuerReserve && attempts < 64; attempts++ {
		if err := owner.issueTextTokens(t.Context(), [][32]byte{selection.EntryNodeID}, 2); err != nil {
			t.Fatal(err)
		}
	}
	before := ready()
	if before >= qualificationIssuerReserve {
		t.Fatalf("could not reach qualification issuer refill boundary: %d", before)
	}
	if err := owner.ensureQualificationIssuerReserve(t.Context(), qualificationIssuerReserve); err != nil {
		t.Fatal(err)
	}
	if after := ready(); after < qualificationIssuerReserve || after <= before {
		t.Fatalf("qualification issuer reserve = %d after %d", after, before)
	}
}
