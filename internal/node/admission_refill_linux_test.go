//go:build linux

package node

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// Exercise the real refill adapter with genuinely issued blind tokens and a
// durable spend root. State availability and physical Hosting are controlled
// inputs; this is not a network refill or provider measurement claim.
func TestAdmissionRefillUsesDomainRedemption(t *testing.T) {
	fixture := newPrivateRecipientNetworkFixture(t, carrier.ClosedCarrierTCP, ardp.PurposeReachability, 1, 2)
	root := t.TempDir()
	binding := spending.Binding{NetworkID: fixture.receiver.NetworkID, ProfileDigest: fixture.receiver.ProfileDigest, ReceiverNodeID: fixture.receiver.NodeID, ReceiverDutyGeneration: fixture.receiver.DutyGeneration}
	ledger, err := spending.Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := ledger.Close(); err != nil {
			t.Error(err)
		}
	})
	now := time.Now().UTC()
	deadline := now.Add(time.Second)
	available, clockReads, expire := true, 0, false
	clock := func() time.Time {
		clockReads++
		if expire && clockReads >= 3 {
			return deadline
		}
		return now
	}
	source := authority.Source{CurrentProfile: func() (state.ClosedProfileView, bool) { return fixture.profile, available }}
	host := &refillReservationHost{}
	refill := hosting.Replenisher(source, clock, fixture.receiver, host, ledger, hostingbudget.Traffic{Tx: 1}, hostingbudget.Traffic{Rx: 1})
	input := route.ClosedAdmissionVerification{Class: 2, Token: fixture.supplementary[2][0], Deadline: deadline}
	host.refuse = true
	if release, err := refill(input); err == nil || release != nil {
		t.Fatal("host refusal admitted refill")
	}
	host.refuse = false
	release, err := refill(input)
	if err != nil || release == nil {
		t.Fatalf("retry after capacity refusal: %v", err)
	}
	if host.released != 0 {
		t.Fatal("successful refill released before transfer")
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if release, err := refill(input); err == nil || release != nil || host.released != 2 {
		t.Fatalf("duplicate refill = %v, releases %d", err, host.released)
	}

	available = false
	input.Token = fixture.supplementary[2][1]
	before := host.reserved
	if _, err := refill(input); err == nil || host.reserved != before {
		t.Fatal("missing State reserved or accepted refill")
	}
	available, expire, clockReads = true, true, 0
	if release, err := refill(input); err == nil || release != nil || host.released != 3 {
		t.Fatalf("late refill = %v, releases %d", err, host.released)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	ledger, err = spending.Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Spend(input.Token, now.Truncate(time.Hour), now); err == nil {
		t.Fatal("restart refunded expired refill")
	}
}

type refillReservationHost struct {
	refuse             bool
	reserved, released int
}

func (host *refillReservationHost) Reserve(context.Context, hostingbudget.Traffic, hostingbudget.Traffic, time.Time) (hosting.Reservation, error) {
	if host.refuse {
		return nil, errors.New("capacity unavailable")
	}
	host.reserved++
	return host, nil
}

func (host *refillReservationHost) Release(context.Context) error { host.released++; return nil }

// Real provider counters and durable Hosting reservations participate in the
// same redemption as genuine blind tokens and the durable receiving-spend root.
func TestAdmissionRefillWithRealHosting(t *testing.T) {
	fixture := newPrivateRecipientNetworkFixture(t, carrier.ClosedCarrierTCP, ardp.PurposeReachability, 1, 2)
	binding := spending.Binding{NetworkID: fixture.receiver.NetworkID, ProfileDigest: fixture.receiver.ProfileDigest, ReceiverNodeID: fixture.receiver.NodeID, ReceiverDutyGeneration: fixture.receiver.DutyGeneration}
	spendRoot := t.TempDir()
	spends, err := spending.Open(spendRoot, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = spends.Close() }()
	now := time.Now().UTC()
	deadline := now.Add(time.Minute)
	root := filepath.Join(t.TempDir(), "period")
	policy := hostingbudget.Policy{Provider: "integration", Start: now.Add(-time.Hour).Truncate(time.Second), End: now.Add(time.Hour).Truncate(time.Second), Unit: "GiB", Quantity: 1, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20}
	if err := hostingbudget.Initialize(root, policy); err != nil {
		t.Fatal(err)
	}
	host, err := hosting.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	observer, err := hostingbudget.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close()
	unrelated, err := observer.Reserve(t.Context(), hostingbudget.Traffic{Tx: 40}, hostingbudget.Traffic{Rx: 24}, deadline)
	if err != nil {
		t.Fatal(err)
	}
	defer unrelated.Release(context.Background())
	assertReserved := func(want uint64) {
		t.Helper()
		observation, err := observer.Observe(t.Context())
		if err != nil || observation.ReservedBytes != want {
			t.Fatalf("reserved = %+v, %v; want %d", observation, err, want)
		}
	}
	source := authority.Source{CurrentProfile: func() (state.ClosedProfileView, bool) { return fixture.profile, true }}
	clockReads, expire := 0, false
	clock := func() time.Time {
		clockReads++
		if expire && clockReads >= 3 {
			return deadline
		}
		return now
	}
	refill := hosting.Replenisher(source, clock, fixture.receiver, host, spends, hostingbudget.Traffic{Tx: 11}, hostingbudget.Traffic{Rx: 7})
	tooLarge := hosting.Replenisher(source, clock, fixture.receiver, host, spends, hostingbudget.Traffic{Tx: 2 << 30}, hostingbudget.Traffic{Rx: 7})
	input := route.ClosedAdmissionVerification{Class: 2, Token: fixture.supplementary[2][0], Deadline: deadline}
	if release, err := tooLarge(input); err == nil || release != nil {
		t.Fatal("capacity refusal accepted")
	}
	assertReserved(64)
	release, err := refill(input)
	if err != nil {
		t.Fatal(err)
	}
	assertReserved(82)
	// Replay cleanup cannot release the original accepted work or unrelated debit.
	if replay, err := refill(input); err == nil || replay != nil {
		t.Fatal("replay accepted")
	}
	assertReserved(82)
	if err := release(); err != nil {
		t.Fatal(err)
	}
	assertReserved(64)
	if err := release(); err != nil {
		t.Fatal(err)
	}
	assertReserved(64)
	input.Token = fixture.supplementary[2][1]
	if err := spends.Close(); err != nil {
		t.Fatal(err)
	}
	if release, err := refill(input); err == nil || release != nil {
		t.Fatal("closed spend root accepted")
	}
	assertReserved(64)
	spends, err = spending.Open(spendRoot, binding)
	if err != nil {
		t.Fatal(err)
	}
	refill = hosting.Replenisher(source, clock, fixture.receiver, host, spends, hostingbudget.Traffic{Tx: 11}, hostingbudget.Traffic{Rx: 7})
	expire, clockReads = true, 0
	if release, err := refill(input); err == nil || release != nil {
		t.Fatal("post-spend expiry accepted")
	}
	assertReserved(64)
	if err := spends.Close(); err != nil {
		t.Fatal(err)
	}
	spends, err = spending.Open(spendRoot, binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := spends.Spend(fixture.tokens[0], now.Truncate(time.Hour), now); err != nil {
		t.Fatalf("fresh token control after reopen: %v", err)
	}
	if err := spends.Spend(input.Token, now.Truncate(time.Hour), now); err == nil {
		t.Fatal("post-spend expiry refunded token")
	}
}
