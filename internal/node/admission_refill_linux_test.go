//go:build linux

package node

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
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
	refill := hosting.Replenisher(source, clock, fixture.receiver, host, ledger, resource.HostingTraffic{Tx: 1}, resource.HostingTraffic{Rx: 1})
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
	if err := ledger.Spend(input.Token, now.Truncate(time.Hour), deadline); err == nil {
		t.Fatal("restart refunded expired refill")
	}
}

type refillReservationHost struct {
	refuse             bool
	reserved, released int
}

func (host *refillReservationHost) Reserve(context.Context, resource.HostingTraffic, resource.HostingTraffic, time.Time) (hosting.Reservation, error) {
	if host.refuse {
		return nil, errors.New("capacity unavailable")
	}
	host.reserved++
	return host, nil
}

func (host *refillReservationHost) Release(context.Context) error { host.released++; return nil }
